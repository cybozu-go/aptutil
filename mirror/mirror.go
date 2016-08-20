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

// Update updates mirrored files.
func (m *Mirror) Update(ctx context.Context) error {
	log.Info("download Release/InRelease", map[string]interface{}{
		"_id": m.id,
	})
	fiMap, err := m.downloadRelease(ctx)
	if err != nil {
		return errors.Wrap(err, m.id)
	}

	if len(fiMap) == 0 {
		return errors.New(m.id + ": found no Release/InRelease")
	}

	// WORKAROUND: some (dell) repositories have invalid Release
	// that contains wrong checksum for itself.  Ignore them.
	for _, p := range m.mc.ReleaseFiles() {
		delete(fiMap, p)
	}

	// WORKAROUND: some (zabbix) repositories returns wrong contents
	// for non-existent files such as Sources (looks like the body of
	// Sources.gz is returned).
	if !m.mc.Source {
		fiMap2 := make(map[string]*apt.FileInfo)
		for p, fi := range fiMap {
			base := path.Base(p)
			base = base[0 : len(base)-len(path.Ext(base))]
			if base == "Sources" {
				continue
			}
			fiMap2[p] = fi
		}
		fiMap = fiMap2
	}

	// download (or reuse) all indices
	log.Info("download other indices", map[string]interface{}{
		"_id":      m.id,
		"_indices": len(fiMap),
	})
	err = m.downloadFiles(ctx, fiMap, true)
	if err != nil {
		return errors.Wrap(err, m.id)
	}

	// extract file information
	fiMap2 := make(map[string]*apt.FileInfo)
	for p := range fiMap {
		if !m.mc.MatchingIndex(p) || !apt.IsSupported(p) {
			continue
		}
		f, err := m.storage.Open(p)
		switch {
		case err == nil:
		case os.IsNotExist(err):
			continue
		default:
			return errors.Wrap(err, m.id)
		}
		fil, err := apt.ExtractFileInfo(p, f)
		f.Close()
		if err != nil {
			return errors.Wrap(err, m.id)
		}
		for _, fi2 := range fil {
			fi2path := fi2.Path()
			if _, ok := fiMap[fi2path]; ok {
				// already included in Release/InRelease
				continue
			}
			fiMap2[fi2.Path()] = fi2
		}
	}

	// download all files matching the configuration.
	log.Info("download items", map[string]interface{}{
		"_id":    m.id,
		"_items": len(fiMap2),
	})
	err = m.downloadFiles(ctx, fiMap2, false)
	if err != nil {
		return errors.Wrap(err, m.id)
	}

	// all files are downloaded (or reused)
	log.Info("saving meta data", map[string]interface{}{
		"_id": m.id,
	})
	err = m.storage.Save()
	if err != nil {
		return errors.Wrap(err, m.id)
	}

	// replace the symlink atomically
	tname := filepath.Join(m.dir, m.id+".tmp")
	os.Remove(tname)
	err = os.Symlink(filepath.Join(m.storage.Dir(), m.id), tname)
	if err != nil {
		return errors.Wrap(err, m.id)
	}
	DirSync(m.dir)
	err = os.Rename(tname, filepath.Join(m.dir, m.id))
	if err != nil {
		return errors.Wrap(err, m.id)
	}
	DirSync(m.dir)

	log.Info("update succeeded", map[string]interface{}{
		"_id": m.id,
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
	p string, fi *apt.FileInfo, ch chan<- *dlResult) {

	r := &dlResult{
		path: p,
	}
	defer func() {
		ch <- r
		m.semaphore <- struct{}{}
	}()

	var retries uint

RETRY:
	if retries > 0 {
		log.Warn("retrying download", map[string]interface{}{
			"_id":   m.id,
			"_path": p,
		})
		time.Sleep(time.Duration(1<<(retries-1)) * time.Second)
	}

	req := &http.Request{
		Method:     "GET",
		URL:        m.mc.Resolve(p),
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
			"_id":     m.id,
			"_path":   p,
			"_status": resp.StatusCode,
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
		r.err = errors.New("invalid checksum for " + p)
		return
	}
	r.fi = fi2
	r.data = data
}

func (m *Mirror) downloadRelease(ctx context.Context) (map[string]*apt.FileInfo, error) {
	releases := m.mc.ReleaseFiles()
	results := make(chan *dlResult, len(releases))

	for _, p := range releases {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-m.semaphore:
		}

		go m.download(ctx, p, nil, results)
	}

	fiMap := make(map[string]*apt.FileInfo)
	for i := 0; i < len(releases); i++ {
		r := <-results
		if r.err != nil {
			return nil, errors.Wrap(r.err, "download")
		}
		switch {
		case r.status == http.StatusOK:
			err := m.storage.Store(r.fi, r.data)
			if err != nil {
				return nil, errors.Wrap(err, "storage.Store")
			}
			if apt.IsSupported(r.path) {
				fil, err := apt.ExtractFileInfo(r.path, bytes.NewReader(r.data))
				if err != nil {
					return nil, errors.Wrap(err, "ExtractFileInfo: "+r.path)
				}
				for _, fi := range fil {
					fiMap[fi.Path()] = fi
				}
			}

		case 400 <= r.status && r.status < 500:
			continue

		default:
			return nil, fmt.Errorf("status %d for %s", r.status, r.path)
		}
	}

	return fiMap, nil
}

func (m *Mirror) downloadFiles(ctx context.Context,
	fiMap map[string]*apt.FileInfo, allowMissing bool) error {
	results := make(chan *dlResult, len(fiMap))

	loggedAt := time.Now()

	var reused, downloading, downloaded int
	for p, fi := range fiMap {
		now := time.Now()
		if now.Sub(loggedAt) > progressInterval {
			loggedAt = now
			log.Info("download progress", map[string]interface{}{
				"_id":         m.id,
				"_total":      len(fiMap),
				"_reused":     reused,
				"_downloaded": downloaded,
			})
		}
		if m.current != nil {
			fi2, fullpath := m.current.Lookup(fi)
			if fi2 != nil {
				err := m.storage.StoreLink(fi2, fullpath)
				if err != nil {
					return errors.Wrap(err, "storage.StoreLink")
				}
				reused++
				if log.Enabled(log.LvDebug) {
					log.Debug("reuse item", map[string]interface{}{
						"_id":   m.id,
						"_path": p,
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
					return errors.Wrap(r.err, "download")
				}
				if allowMissing && r.status == http.StatusNotFound {
					log.Warn("missing file", map[string]interface{}{
						"_id":   m.id,
						"_path": r.path,
					})
					continue
				}
				if r.status != http.StatusOK {
					return fmt.Errorf("status %d for %s", r.status, r.path)
				}
				err := m.storage.Store(r.fi, r.data)
				if err != nil {
					return errors.Wrap(err, "storage.Store")
				}
			default:
				goto DOWNLOAD
			}
		}

	DOWNLOAD:
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-m.semaphore:
		}

		go m.download(ctx, p, fi, results)
		downloading++
	}

	for downloading != downloaded {
		r := <-results
		downloaded++
		if r.err != nil {
			return errors.Wrap(r.err, "download")
		}
		if allowMissing && r.status == http.StatusNotFound {
			log.Warn("missing file", map[string]interface{}{
				"_id":   m.id,
				"_path": r.path,
			})
			continue
		}
		if r.status != http.StatusOK {
			return fmt.Errorf("status %d for %s", r.status, r.path)
		}
		err := m.storage.Store(r.fi, r.data)
		if err != nil {
			return errors.Wrap(err, "storage.Store")
		}
	}

	log.Info("stats", map[string]interface{}{
		"_id":         m.id,
		"_reused":     reused,
		"_downloaded": downloaded,
	})

	return nil
}
