package cacher

// This file implements core logics to download and cache APT
// repository items.

import (
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cybozu-go/aptutil/apt"
	"github.com/cybozu-go/log"
	"github.com/cybozu-go/well"
	"github.com/pkg/errors"
)

const (
	gib            = 1 << 30
	requestTimeout = 30 * time.Minute
)

// addPrefix add prefix for each *FileInfo in fil.
func addPrefix(prefix string, fil []*apt.FileInfo) []*apt.FileInfo {
	ret := make([]*apt.FileInfo, 0, len(fil))
	for _, fi := range fil {
		ret = append(ret, fi.AddPrefix(prefix))
	}
	return ret
}

// Cacher downloads and caches APT indices and deb files.
type Cacher struct {
	meta          *Storage
	items         *Storage
	um            URLMap
	checkInterval time.Duration
	cachePeriod   time.Duration
	client        *http.Client
	maxConns      int

	fiLock sync.RWMutex
	info   map[string]*apt.FileInfo

	dlLock     sync.RWMutex
	dlChannels map[string]chan struct{}
	results    map[string]int

	hostLock sync.Mutex
	hostSem  map[string]chan struct{}
}

// NewCacher constructs Cacher.
func NewCacher(config *Config) (*Cacher, error) {
	if config.CheckInterval == 0 {
		return nil, errors.New("invaild check_interval")
	}
	checkInterval := time.Duration(config.CheckInterval) * time.Second
	cachePeriod := time.Duration(config.CachePeriod) * time.Second

	metaDir := filepath.Clean(config.MetaDirectory)
	if !filepath.IsAbs(metaDir) {
		return nil, errors.New("meta_dir must be an absolute path")
	}

	cacheDir := filepath.Clean(config.CacheDirectory)
	if !filepath.IsAbs(cacheDir) {
		return nil, errors.New("cache_dir must be an absolute path")
	}

	if metaDir == cacheDir {
		return nil, errors.New("meta_dir and cache_dir must be different")
	}

	if config.CacheCapacity <= 0 {
		return nil, errors.New("cache_capacity must be > 0")
	}
	capacity := uint64(config.CacheCapacity) * gib

	meta := NewStorage(metaDir, 0)
	cache := NewStorage(cacheDir, capacity)

	if err := meta.Load(); err != nil {
		return nil, errors.Wrap(err, "meta.Load")
	}
	if err := cache.Load(); err != nil {
		return nil, errors.Wrap(err, "cache.Load")
	}

	um := make(URLMap)
	for prefix, urlString := range config.Mapping {
		u, err := url.Parse(urlString)
		if err != nil {
			return nil, errors.Wrap(err, prefix)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, errors.New("unsupported scheme: " + u.Scheme)
		}
		err = um.Register(prefix, u)
		if err != nil {
			return nil, errors.Wrap(err, prefix)
		}
	}

	c := &Cacher{
		meta:          meta,
		items:         cache,
		um:            um,
		checkInterval: checkInterval,
		cachePeriod:   cachePeriod,
		client:        &http.Client{},
		maxConns:      config.MaxConns,
		info:          make(map[string]*apt.FileInfo),
		dlChannels:    make(map[string]chan struct{}),
		results:       make(map[string]int),
		hostSem:       make(map[string]chan struct{}),
	}

	metas := meta.ListAll()
	for _, fi := range metas {
		f, err := meta.Lookup(fi)
		if err != nil {
			return nil, errors.Wrap(err, "meta.Lookup")
		}
		t := strings.SplitN(fi.Path(), "/", 2)
		if len(t) != 2 {
			panic("there should always be a prefix!")
		}
		fil, _, err := apt.ExtractFileInfo(t[1], f)
		f.Close()
		if err != nil {
			return nil, errors.Wrap(err, "ExtractFileInfo("+fi.Path()+")")
		}
		fil = addPrefix(t[0], fil)
		for _, fi2 := range fil {
			c.info[fi2.Path()] = fi2
		}
	}

	// add meta files w/o checksums (Release, Release.gpg, and InRelease).
	for _, fi := range metas {
		p := fi.Path()
		if _, ok := c.info[p]; !ok {
			c.info[p] = fi
			c.maintMeta(p)
		}
	}

	return c, nil
}

func (c *Cacher) acquireSemaphore(host string) {
	if c.maxConns == 0 {
		return
	}

	c.hostLock.Lock()
	sem, ok := c.hostSem[host]
	if !ok {
		sem = make(chan struct{}, c.maxConns)
		for i := 0; i < c.maxConns; i++ {
			sem <- struct{}{}
		}
		c.hostSem[host] = sem
	}
	c.hostLock.Unlock()

	<-sem
}

func (c *Cacher) releaseSemaphore(host string) {
	if c.maxConns == 0 {
		return
	}

	c.hostLock.Lock()
	c.hostSem[host] <- struct{}{}
	c.hostLock.Unlock()
}

func (c *Cacher) maintMeta(p string) {
	switch path.Base(p) {
	case "Release":
		well.Go(func(ctx context.Context) error {
			c.maintRelease(ctx, p, true)
			return nil
		})
	case "InRelease":
		well.Go(func(ctx context.Context) error {
			c.maintRelease(ctx, p, false)
			return nil
		})
	}
}

func (c *Cacher) maintRelease(ctx context.Context, p string, withGPG bool) {
	ticker := time.NewTicker(c.checkInterval)
	defer ticker.Stop()

	if log.Enabled(log.LvDebug) {
		log.Debug("maintRelease", map[string]interface{}{
			"path": p,
		})
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ch1 := c.Download(p, nil)
			if withGPG {
				ch2 := c.Download(p+".gpg", nil)
				<-ch2
			}
			<-ch1
		}
	}
}

func closeRespBody(r *http.Response) {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()
}

// Download downloads an item and caches it.
//
// If valid is not nil, the downloaded data is validated against it.
//
// The caller receives a channel that will be closed when the item
// is downloaded and cached.  If prefix of p is not registered
// in URLMap, nil is returned.
//
// Note that download may fail, or just invalidated soon.
// Users of this method should retry if the item is not cached
// or invalidated.
func (c *Cacher) Download(p string, valid *apt.FileInfo) <-chan struct{} {
	u := c.um.URL(p)
	if u == nil {
		return nil
	}

	c.dlLock.Lock()
	defer c.dlLock.Unlock()

	ch, ok := c.dlChannels[p]
	if ok {
		return ch
	}

	ch = make(chan struct{})
	c.dlChannels[p] = ch
	well.Go(func(ctx context.Context) error {
		c.download(ctx, p, u, valid)
		return nil
	})
	return ch
}

// download is a goroutine to download an item.
func (c *Cacher) download(ctx context.Context, p string, u *url.URL, valid *apt.FileInfo) {
	c.acquireSemaphore(u.Host)

	statusCode := http.StatusInternalServerError

	defer func() {
		c.releaseSemaphore(u.Host)
		c.dlLock.Lock()
		ch := c.dlChannels[p]
		delete(c.dlChannels, p)
		c.results[p] = statusCode
		c.dlLock.Unlock()
		close(ch)

		// invalidate result cache after some interval
		well.Go(func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(c.cachePeriod):
			}
			c.dlLock.Lock()
			delete(c.results, p)
			c.dlLock.Unlock()
			return nil
		})
	}()

	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	// imitation apt-get command
	// NOTE: apt-get sets If-Modified-Since and makes a request to the server,
	// but the current aptutil cannot handle this because it cold-starts every time.
	header := http.Header{}
	header.Add("Cache-Control", "max-age=0")
	header.Add("User-Agent", "Debian APT-HTTP/1.3 (aptutil)")

	req := &http.Request{
		Method:     "GET",
		URL:        u,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     header,
	}
	resp, err := c.client.Do(req.WithContext(ctx))
	if err != nil {
		log.Warn("GET failed", map[string]interface{}{
			"url":   u.String(),
			"error": err.Error(),
		})
		return
	}

	defer closeRespBody(resp)
	statusCode = resp.StatusCode
	if statusCode != 200 {
		return
	}

	storage := c.items
	if apt.IsMeta(p) {
		storage = c.meta
	}

	tempfile, err := storage.TempFile()
	if err != nil {
		log.Warn("GET failed", map[string]interface{}{
			"url":   u.String(),
			"error": err.Error(),
		})
		return
	}
	defer func() {
		tempfile.Close()
		os.Remove(tempfile.Name())
	}()

	fi, err := apt.CopyWithFileInfo(tempfile, resp.Body, p)
	if err != nil {
		log.Warn("GET failed", map[string]interface{}{
			"url":   u.String(),
			"error": err.Error(),
		})
		return
	}
	err = tempfile.Sync()
	if err != nil {
		log.Warn("tempfile.Sync failed", map[string]interface{}{
			"url":   u.String(),
			"error": err.Error(),
		})
		return
	}
	if valid != nil && !valid.Same(fi) {
		log.Warn("downloaded data is not valid", map[string]interface{}{
			"url": u.String(),
		})
		return
	}

	var fil []*apt.FileInfo

	if t := strings.SplitN(path.Clean(p), "/", 2); len(t) == 2 && apt.IsMeta(t[1]) {
		_, err = tempfile.Seek(0, io.SeekStart)
		if err != nil {
			log.Error("failed to reset tempfile offset", map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		fil, _, err = apt.ExtractFileInfo(t[1], tempfile)
		if err != nil {
			log.Error("invalid meta data", map[string]interface{}{
				"path":  p,
				"error": err.Error(),
			})
			// do not return; we accept broken meta data as is.
		}
		fil = addPrefix(t[0], fil)
	}

	c.fiLock.Lock()
	defer c.fiLock.Unlock()

	// To keep consistency between Cacher and Storage so that
	// both have the same set of FileInfo, storage.Insert need to be
	// guarded by c.fiLock.
	if err := storage.Insert(tempfile.Name(), fi); err != nil {
		log.Error("could not save an item", map[string]interface{}{
			"path":  p,
			"error": err.Error(),
		})
		// panic because go-apt-cacher cannot continue working
		panic(err)
	}

	for _, fi2 := range fil {
		c.info[fi2.Path()] = fi2
	}
	if apt.IsMeta(p) {
		_, ok := c.info[p]
		if !ok {
			// As this is the first time that downloaded meta file p,
			c.maintMeta(p)
		}
	}
	c.info[p] = fi
	log.Info("downloaded and cached", map[string]interface{}{
		"path": p,
	})
}

// Get looks up a cached item, and if not found, downloads it
// from the upstream server.
//
// The return values are cached HTTP status code of the response from
// an upstream server, a pointer to os.File for the cache file,
// and error.
func (c *Cacher) Get(p string) (statusCode int, f *os.File, err error) {
	u := c.um.URL(p)
	if u == nil {
		return http.StatusNotFound, nil, nil
	}

	storage := c.items
	if apt.IsMeta(p) {
		if !apt.IsSupported(p) {
			// return 404 for unsupported compression algorithms
			return http.StatusNotFound, nil, nil
		}
		storage = c.meta
	}

RETRY:
	c.fiLock.RLock()
	fi, ok := c.info[p]
	c.fiLock.RUnlock()

	if ok {
		f, err := storage.Lookup(fi)
		switch err {
		case nil:
			return http.StatusOK, f, nil
		case ErrNotFound:
		default:
			log.Error("lookup failure", map[string]interface{}{
				"error": err.Error(),
			})
			return http.StatusInternalServerError, nil, err
		}
	}

	// not found in storage.
	c.dlLock.RLock()
	ch, chOk := c.dlChannels[p]
	result, resultOk := c.results[p]
	c.dlLock.RUnlock()

	if resultOk && result != http.StatusOK {
		return result, nil, nil
	}
	if chOk {
		<-ch
	} else {
		<-c.Download(p, fi)
	}
	goto RETRY
}
