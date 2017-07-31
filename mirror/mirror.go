package mirror

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"github.com/cybozu-go/aptutil/apt"
	"github.com/cybozu-go/log"
	"github.com/pkg/errors"
)

const (
	timestampFormat  = "20060102_150405"
	progressInterval = 5 * time.Minute
	httpRetries      = 5
)

var (
	validID = regexp.MustCompile(`^[a-z0-9_-]+$`)
)

// Mirror implements mirroring logics.
type Mirror struct {
	id      string
	dir     string
	mc      *MirrConfig
	storage *Storage
	current *Storage

	semaphore chan struct{}
	client    *http.Client
}

// NewMirror constructs a Mirror for given mirror id.
func NewMirror(t time.Time, id string, c *Config) (*Mirror, error) {
	dir := filepath.Clean(c.Dir)
	mc, ok := c.Mirrors[id]
	if !ok {
		return nil, errors.New("no such mirror: " + id)
	}

	// sanity checks
	if !validID.MatchString(id) {
		return nil, errors.New("invalid id: " + id)
	}
	if err := mc.Check(); err != nil {
		return nil, errors.Wrap(err, id)
	}

	var currentStorage *Storage
	curdir, err := filepath.EvalSymlinks(filepath.Join(dir, id))
	switch {
	case os.IsNotExist(err):
	case err != nil:
		return nil, errors.Wrap(err, id)
	default:
		currentStorage, err = NewStorage(filepath.Dir(curdir), id)
		if err != nil {
			return nil, errors.Wrap(err, id)
		}
		err = currentStorage.Load()
		if err != nil {
			return nil, errors.Wrap(err, id)
		}
	}

	d := filepath.Join(dir, "."+id+"."+t.Format(timestampFormat))
	err = os.Mkdir(d, 0755)
	if err != nil {
		return nil, errors.Wrap(err, id)
	}
	storage, err := NewStorage(d, id)
	if err != nil {
		return nil, errors.Wrap(err, id)
	}

	sem := make(chan struct{}, c.MaxConns)
	for i := 0; i < c.MaxConns; i++ {
		sem <- struct{}{}
	}

	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConnsPerHost: c.MaxConns,
	}

	mr := &Mirror{
		id:        id,
		dir:       dir,
		mc:        mc,
		storage:   storage,
		current:   currentStorage,
		semaphore: sem,
		client: &http.Client{
			Transport: transport,
		},
	}
	return mr, nil
}

func (m *Mirror) store(fi *apt.FileInfo, data []byte, byhash bool) error {
	if byhash {
		return m.storage.StoreWithHash(fi, data)
	}
	return m.storage.Store(fi, data)
}

func (m *Mirror) storeLink(fi *apt.FileInfo, fp string, byhash bool) error {
	if byhash {
		return m.storage.StoreLinkWithHash(fi, fp)
	}
	return m.storage.StoreLink(fi, fp)
}

func (m *Mirror) extractItems(indices []*apt.FileInfo, indexMap map[string][]*apt.FileInfo) (map[string]*apt.FileInfo, error) {
	itemMap := make(map[string]*apt.FileInfo)

	for _, index := range indices {
		p := index.Path()
		if !m.mc.MatchingIndex(p) || !apt.IsSupported(p) {
			continue
		}
		f, err := m.storage.Open(p)
		if err != nil {
			return nil, err
		}

		fil, _, err := apt.ExtractFileInfo(p, f)
		f.Close()
		if err != nil {
			return nil, err
		}

		for _, fi := range fil {
			fipath := fi.Path()
			if _, ok := indexMap[fipath]; ok {
				// already included in Release/InRelease
				continue
			}
			itemMap[fipath] = fi
		}
	}
	return itemMap, nil
}

func (m *Mirror) replaceLink() error {
	tname := filepath.Join(m.dir, m.id+".tmp")
	os.Remove(tname)
	err := os.Symlink(filepath.Join(m.storage.Dir(), m.id), tname)
	if err != nil {
		return err
	}

	// symlink exists only in dentry
	err = DirSync(m.dir)
	if err != nil {
		return err
	}

	err = os.Rename(tname, filepath.Join(m.dir, m.id))
	if err != nil {
		return err
	}

	return DirSync(m.dir)
}

// Update updates mirrored files.
func (m *Mirror) Update(ctx context.Context) error {
	log.Info("download Release/InRelease", map[string]interface{}{
		"repo": m.id,
	})
	indexMap, byhash, err := m.downloadRelease(ctx)
	if err != nil {
		return errors.Wrap(err, m.id)
	}

	if byhash {
		log.Info("detected by-hash support", map[string]interface{}{
			"repo": m.id,
		})
	}

	if len(indexMap) == 0 {
		return errors.New(m.id + ": found no Release/InRelease")
	}

	// WORKAROUND: some (zabbix) repositories returns wrong contents
	// for non-existent files such as Sources (looks like the body of
	// Sources.gz is returned).
	if !m.mc.Source {
		tmpMap := make(map[string][]*apt.FileInfo)
		for p, fil := range indexMap {
			base := path.Base(p)
			base = base[0 : len(base)-len(path.Ext(base))]
			if base == "Sources" {
				continue
			}
			tmpMap[p] = fil
		}
		indexMap = tmpMap
	}

	// download (or reuse) all indices
	indices, err := m.downloadIndices(ctx, indexMap, byhash)
	if err != nil {
		return errors.Wrap(err, m.id)
	}

	// extract file information from indices
	itemMap, err := m.extractItems(indices, indexMap)
	if err != nil {
		return errors.Wrap(err, m.id)
	}

	// download all files matching the configuration.
	log.Info("download items", map[string]interface{}{
		"repo":  m.id,
		"items": len(itemMap),
	})
	_, err = m.downloadItems(ctx, itemMap)
	if err != nil {
		return errors.Wrap(err, m.id)
	}

	// all files are downloaded (or reused)
	log.Info("saving meta data", map[string]interface{}{
		"repo": m.id,
	})
	err = m.storage.Save()
	if err != nil {
		return errors.Wrap(err, m.id)
	}

	// replace the symlink atomically
	err = m.replaceLink()
	if err != nil {
		return errors.Wrap(err, m.id)
	}

	log.Info("update succeeded", map[string]interface{}{
		"repo": m.id,
	})
	return nil
}

type dlResult struct {
	status int
	path   string
	fi     *apt.FileInfo
	data   []byte
	err    error
}

// download is a goroutine to download an item.
func (m *Mirror) download(ctx context.Context,
	p string, fi *apt.FileInfo, byhash bool, ch chan<- *dlResult) {

	r := &dlResult{
		path: p,
	}
	defer func() {
		ch <- r
		m.semaphore <- struct{}{}
	}()

	var retries uint
	targets := []string{p}
	if byhash && fi != nil {
		targets = append(targets, fi.SHA256Path())
		targets = append(targets, fi.SHA1Path())
		targets = append(targets, fi.MD5SumPath())
	}

RETRY:
	if retries > 0 {
		log.Warn("retrying download", map[string]interface{}{
			"repo": m.id,
			"path": p,
		})
		time.Sleep(time.Duration(1<<(retries-1)) * time.Second)
	}

	req := &http.Request{
		Method:     "GET",
		URL:        m.mc.Resolve(targets[0]),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
	}
	resp, err := m.client.Do(req.WithContext(ctx))
	if err != nil {
		if retries < httpRetries {
			retries++
			goto RETRY
		}
		r.err = err
		return
	}
	if log.Enabled(log.LvDebug) {
		log.Debug("downloaded", map[string]interface{}{
			"repo":               m.id,
			"path":               p,
			log.FnHTTPStatusCode: resp.StatusCode,
		})
	}

	r.status = resp.StatusCode
	data, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		if retries < httpRetries {
			retries++
			goto RETRY
		}
		r.err = err
		return
	}
	if r.status >= 500 && retries < httpRetries {
		retries++
		goto RETRY
	}
	if r.status != 200 {
		return
	}

	fi2 := apt.MakeFileInfo(p, data)
	if fi != nil && !fi.Same(fi2) {
		if len(targets) > 1 {
			targets = targets[1:]
			log.Warn("try by-hash retrieval", map[string]interface{}{
				"repo":   m.id,
				"path":   p,
				"target": targets[0],
			})
			goto RETRY
		}
		r.err = errors.New("invalid checksum for " + p)
		return
	}
	r.fi = fi2
	r.data = data
}

func (m *Mirror) downloadRelease(ctx context.Context) (map[string][]*apt.FileInfo, bool, error) {
	releases := m.mc.ReleaseFiles()
	results := make(chan *dlResult, len(releases))

	for _, p := range releases {
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case <-m.semaphore:
		}

		go m.download(ctx, p, nil, false, results)
	}

	byhash := true
	filMap := make(map[string][]*apt.FileInfo)
	for i := 0; i < len(releases); i++ {
		r := <-results
		if r.err != nil {
			return nil, byhash, errors.Wrap(r.err, "download")
		}
		switch {
		case r.status == http.StatusOK:
			err := m.storage.Store(r.fi, r.data)
			if err != nil {
				return nil, byhash, errors.Wrap(err, "storage.Store")
			}
			fil, d, err := apt.ExtractFileInfo(r.path, bytes.NewReader(r.data))
			if err != nil {
				return nil, byhash, errors.Wrap(err, "ExtractFileInfo: "+r.path)
			}

			if byhash && path.Base(r.path) != "Release.gpg" {
				byhash = apt.SupportByHash(d)
			}

		FIL:
			for _, fi := range fil {
				p2 := fi.Path()
				fil2, ok := filMap[p2]
				if !ok {
					filMap[p2] = []*apt.FileInfo{fi}
					continue
				}

				for _, fi2 := range fil2 {
					if fi2.Same(fi) {
						continue FIL
					}
				}

				// fi differs from all FileInfo in fil2
				if !byhash {
					return nil, byhash, errors.New("inconsistent checksum for " + p2)
				}
				filMap[p2] = append(fil2, fi)
			}

		case 400 <= r.status && r.status < 500:
			continue

		default:
			return nil, byhash, fmt.Errorf("status %d for %s", r.status, r.path)
		}
	}

	return filMap, byhash, nil
}

func (m *Mirror) downloadIndices(ctx context.Context,
	filMap map[string][]*apt.FileInfo, byhash bool) ([]*apt.FileInfo, error) {
	var fil []*apt.FileInfo
	for _, fil2 := range filMap {
		fil = append(fil, fil2...)
	}

	log.Info("download other indices", map[string]interface{}{
		"repo":    m.id,
		"indices": len(fil),
	})

	return m.downloadFiles(ctx, fil, true, byhash)
}

func (m *Mirror) downloadItems(ctx context.Context,
	fiMap map[string]*apt.FileInfo) ([]*apt.FileInfo, error) {
	fil := make([]*apt.FileInfo, 0, len(fiMap))
	for _, fi := range fiMap {
		fil = append(fil, fi)
	}
	return m.downloadFiles(ctx, fil, false, false)
}

func (m *Mirror) downloadFiles(ctx context.Context,
	fil []*apt.FileInfo, allowMissing, byhash bool) ([]*apt.FileInfo, error) {
	results := make(chan *dlResult, len(fil))
	dlfil := make([]*apt.FileInfo, 0, len(fil))

	loggedAt := time.Now()

	var reused, downloading, downloaded int
	for _, fi := range fil {
		p := fi.Path()
		now := time.Now()
		if now.Sub(loggedAt) > progressInterval {
			loggedAt = now
			log.Info("download progress", map[string]interface{}{
				"repo":       m.id,
				"total":      len(fil),
				"reused":     reused,
				"downloaded": downloaded,
			})
		}
		if m.current != nil {
			fi2, fullpath := m.current.Lookup(fi, byhash)
			if fi2 != nil {
				err := m.storeLink(fi2, fullpath, byhash)
				if err != nil {
					return nil, errors.Wrap(err, "storeLink")
				}
				dlfil = append(dlfil, fi2)
				reused++
				if log.Enabled(log.LvDebug) {
					log.Debug("reuse item", map[string]interface{}{
						"repo": m.id,
						"path": p,
					})
				}
				continue
			}
		}

		// check download results early
		for {
			select {
			case r := <-results:
				downloaded++
				if r.err != nil {
					return nil, errors.Wrap(r.err, "download")
				}
				if allowMissing && r.status == http.StatusNotFound {
					log.Warn("missing file", map[string]interface{}{
						"repo": m.id,
						"path": r.path,
					})
					continue
				}
				if r.status != http.StatusOK {
					return nil, fmt.Errorf("status %d for %s", r.status, r.path)
				}
				err := m.store(r.fi, r.data, byhash)
				if err != nil {
					return nil, errors.Wrap(err, "store")
				}
				dlfil = append(dlfil, r.fi)
			default:
				goto DOWNLOAD
			}
		}

	DOWNLOAD:
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-m.semaphore:
		}

		go m.download(ctx, p, fi, byhash, results)
		downloading++
	}

	for downloading != downloaded {
		r := <-results
		downloaded++
		if r.err != nil {
			return nil, errors.Wrap(r.err, "download")
		}
		if allowMissing && r.status == http.StatusNotFound {
			log.Warn("missing file", map[string]interface{}{
				"repo": m.id,
				"path": r.path,
			})
			continue
		}
		if r.status != http.StatusOK {
			return nil, fmt.Errorf("status %d for %s", r.status, r.path)
		}
		err := m.store(r.fi, r.data, byhash)
		if err != nil {
			return nil, errors.Wrap(err, "store")
		}
		dlfil = append(dlfil, r.fi)
	}

	log.Info("stats", map[string]interface{}{
		"repo":       m.id,
		"reused":     reused,
		"downloaded": downloaded,
	})

	return dlfil, nil
}
