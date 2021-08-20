package mirror

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"github.com/cybozu-go/aptutil/apt"
	"github.com/cybozu-go/log"
	"github.com/cybozu-go/well"
	"github.com/pkg/errors"
)

const (
	timestampFormat  = "20060102_150405"
	progressInterval = 5 * time.Minute
	httpRetries      = 5
)

var (
	validID = regexp.MustCompile(`^[a-z0-9_-]+$`)
	client  = &http.Client{
		Timeout:   60 * time.Minute,
		Transport: http.DefaultTransport,
	}
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

	client.Transport.(*http.Transport).MaxIdleConnsPerHost = c.MaxConns

	mr := &Mirror{
		id:        id,
		dir:       dir,
		mc:        mc,
		storage:   storage,
		current:   currentStorage,
		semaphore: sem,
		client:    client,
	}
	return mr, nil
}

func (m *Mirror) storeLink(fi *apt.FileInfo, fp string, byhash bool) error {
	if byhash {
		return m.storage.StoreLinkWithHash(fi, fp)
	}
	return m.storage.StoreLink(fi, fp)
}

func (m *Mirror) extractItems(indices []*apt.FileInfo, indexMap map[string][]*apt.FileInfo, itemMap map[string]*apt.FileInfo, byhash bool) error {
	for _, index := range indices {
		p := index.Path()
		if !m.mc.MatchingIndex(p) || !apt.IsSupported(p) {
			continue
		}
		hashPath := p
		if byhash {
			hashPath = index.SHA256Path()
		}
		f, err := m.storage.Open(hashPath)
		if err != nil {
			return err
		}

		fil, _, err := apt.ExtractFileInfo(p, f)
		f.Close()
		if err != nil {
			return err
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
	return nil
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
	itemMap := make(map[string]*apt.FileInfo)

	for _, suite := range m.mc.Suites {
		err := m.updateSuite(ctx, suite, itemMap)
		if err != nil {
			return err
		}
	}

	// download all files matching the configuration.
	log.Info("download items", map[string]interface{}{
		"repo":  m.id,
		"items": len(itemMap),
	})
	_, err := m.downloadItems(ctx, itemMap)
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

// updateSuite partially updates mirror for a suite.
func (m *Mirror) updateSuite(ctx context.Context, suite string, itemMap map[string]*apt.FileInfo) error {
	log.Info("download Release/InRelease", map[string]interface{}{
		"repo":  m.id,
		"suite": suite,
	})
	indexMap, byhash, err := m.downloadRelease(ctx, suite)
	if err != nil {
		return errors.Wrap(err, m.id)
	}

	if byhash {
		log.Info("detected by-hash support", map[string]interface{}{
			"repo":  m.id,
			"suite": suite,
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
	err = m.extractItems(indices, indexMap, itemMap, byhash)
	if err != nil {
		return errors.Wrap(err, m.id)
	}
	return nil
}

type dlResult struct {
	status   int
	path     string
	fi       *apt.FileInfo
	tempfile *os.File
	err      error
}

func closeRespBody(r *http.Response) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()
}

func closeAndRemoveFile(f *os.File) {
	f.Close()
	os.Remove(f.Name())
}

// download is a goroutine to download an item.
func (m *Mirror) download(ctx context.Context,
	p string, fi *apt.FileInfo, byhash bool, ch chan<- *dlResult) {

	var tempfile *os.File
	r := &dlResult{
		path: p,
	}

	defer func() {
		r.tempfile = tempfile
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
	if tempfile != nil {
		closeAndRemoveFile(tempfile)
		tempfile = nil
	}

	// allow interrupts
	select {
	case <-ctx.Done():
		r.err = ctx.Err()
		return
	default:
	}

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
	defer closeRespBody(resp)

	if log.Enabled(log.LvDebug) {
		log.Debug("downloaded", map[string]interface{}{
			"repo":               m.id,
			"path":               p,
			log.FnHTTPStatusCode: resp.StatusCode,
		})
	}

	r.status = resp.StatusCode
	if r.status >= 500 && retries < httpRetries {
		retries++
		goto RETRY
	}

	if r.status != 200 {
		return
	}

	tempfile, err = m.storage.TempFile()
	if err != nil {
		r.err = err
		return
	}
	fi2, err := apt.CopyWithFileInfo(tempfile, resp.Body, p)
	if err != nil {
		if retries < httpRetries {
			retries++
			goto RETRY
		}
		r.err = err
		return
	}
	err = tempfile.Sync()
	if err != nil {
		r.err = errors.New("tempfile.Sync failed")
		return
	}
	err = os.Chmod(tempfile.Name(), 0644)
	if err != nil {
		r.err = errors.New("os.Chmod(tempfile.Name(), 0644) failed")
		return
	}

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

	_, err = tempfile.Seek(0, io.SeekStart)
	if err != nil {
		r.err = errors.New("tempfile.Seek failed")
		return
	}
	r.fi = fi2
}

func addFileInfoToList(fi *apt.FileInfo, m map[string][]*apt.FileInfo, byhash bool) error {
	p := fi.Path()
	fil, ok := m[p]
	if !ok {
		m[p] = []*apt.FileInfo{fi}
		return nil
	}

	for _, existing := range fil {
		if existing.Same(fi) {
			return nil
		}
	}

	// fi differs from all FileInfo in fil
	if !byhash {
		return errors.New("inconsistent checksum for " + p)
	}
	m[p] = append(fil, fi)
	return nil
}

func (m *Mirror) handleReleaseResults(results <-chan *dlResult, byhash *bool) ([]*apt.FileInfo, error) {
	r := <-results
	if r.tempfile != nil {
		defer closeAndRemoveFile(r.tempfile)
	}

	if r.err != nil {
		return nil, errors.Wrap(r.err, "download")
	}

	if 400 <= r.status && r.status < 500 {
		// return no error to continue
		return nil, nil
	}

	if r.status != http.StatusOK {
		return nil, fmt.Errorf("status %d for %s", r.status, r.path)
	}

	// 200 OK
	err := m.storage.StoreLink(r.fi, r.tempfile.Name())
	if err != nil {
		return nil, errors.Wrap(err, "storage.Store")
	}
	fil, d, err := apt.ExtractFileInfo(r.path, r.tempfile)
	if err != nil {
		return nil, errors.Wrap(err, "ExtractFileInfo: "+r.path)
	}

	if *byhash && path.Base(r.path) != "Release.gpg" {
		*byhash = apt.SupportByHash(d)
	}

	return fil, nil
}

func (m *Mirror) downloadRelease(ctx context.Context, suite string) (map[string][]*apt.FileInfo, bool, error) {
	releases := m.mc.ReleaseFiles(suite)
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
		fil, err := m.handleReleaseResults(results, &byhash)
		if err != nil {
			return nil, byhash, err
		}
		for _, fi := range fil {
			err = addFileInfoToList(fi, filMap, byhash)
			if err != nil {
				return nil, byhash, err
			}
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
	var reused, downloaded []*apt.FileInfo

	env := well.NewEnvironment(ctx)
	env.Go(func(ctx context.Context) error {
		var err error
		reused, err = m.reuseOrDownload(ctx, fil, byhash, results)
		return err
	})
	env.Go(func(ctx context.Context) error {
		var err error
		downloaded, err = m.recvResult(allowMissing, byhash, results)
		return err
	})
	env.Stop()
	err := env.Wait()
	if err != nil {
		return nil, err
	}

	log.Info("stats", map[string]interface{}{
		"repo":       m.id,
		"total":      len(fil),
		"reused":     len(reused),
		"downloaded": len(downloaded),
	})

	// reused has enough capacity.  See reuseOrDownload.
	return append(reused, downloaded...), nil
}

func (m *Mirror) reuseOrDownload(ctx context.Context, fil []*apt.FileInfo,
	byhash bool, results chan<- *dlResult) ([]*apt.FileInfo, error) {

	// environment to manage downloading goroutines.
	env := well.NewEnvironment(ctx)

	// on return, wait for all DL goroutines then signal recvResult
	// by closing results channel.
	defer func() {
		env.Stop()
		env.Wait()
		close(results)
	}()

	reused := make([]*apt.FileInfo, 0, len(fil))
	loggedAt := time.Now()

	for i, fi := range fil {
		// avoid assignment
		fi := fi
		now := time.Now()
		if now.Sub(loggedAt) > progressInterval {
			loggedAt = now
			log.Info("download progress", map[string]interface{}{
				"repo":      m.id,
				"total":     len(fil),
				"reused":    len(reused),
				"downloads": i - len(reused),
			})
		}

		if m.current != nil {
			localfi, fullpath := m.current.Lookup(fi, byhash)
			if localfi != nil {
				err := m.storeLink(localfi, fullpath, byhash)
				if err != nil {
					return nil, errors.Wrap(err, "storeLink")
				}
				reused = append(reused, localfi)
				if log.Enabled(log.LvDebug) {
					log.Debug("reuse item", map[string]interface{}{
						"repo": m.id,
						"path": fi.Path(),
					})
				}
				continue
			}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-m.semaphore:
		}

		env.Go(func(ctx context.Context) error {
			m.download(ctx, fi.Path(), fi, byhash, results)
			return nil
		})
	}
	return reused, nil
}

func (m *Mirror) handleResult(r *dlResult, allowMissing, byhash bool) (*apt.FileInfo, error) {
	if r.tempfile != nil {
		defer closeAndRemoveFile(r.tempfile)
	}

	if r.err != nil {
		return nil, errors.Wrap(r.err, "download")
	}

	if allowMissing && r.status == http.StatusNotFound {
		log.Warn("missing file", map[string]interface{}{
			"repo": m.id,
			"path": r.path,
		})
		// return no error to continue
		return nil, nil
	}

	if r.status != http.StatusOK {
		return nil, fmt.Errorf("status %d for %s", r.status, r.path)
	}

	err := m.storeLink(r.fi, r.tempfile.Name(), byhash)
	if err != nil {
		return nil, errors.Wrap(err, "store")
	}

	return r.fi, nil
}

func (m *Mirror) recvResult(allowMissing, byhash bool, results <-chan *dlResult) ([]*apt.FileInfo, error) {
	var dlfil []*apt.FileInfo

	for r := range results {
		fi, err := m.handleResult(r, allowMissing, byhash)
		if err != nil {
			return nil, err
		}
		if fi != nil {
			dlfil = append(dlfil, fi)
		}
	}

	return dlfil, nil
}
