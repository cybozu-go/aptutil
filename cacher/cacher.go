package cacher

// This file implements core logics to download and cache APT
// repository items.

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/cybozu-go/go-apt-cacher/apt"
	"github.com/cybozu-go/log"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
)

const (
	gib            = 1 << 30
	requestTimeout = 30 * time.Minute
)

// Cacher downloads and caches APT indices and deb files.
type Cacher struct {
	meta          *Storage
	items         *Storage
	um            URLMap
	checkInterval time.Duration
	cachePeriod   time.Duration
	ctx           context.Context
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
func NewCacher(ctx context.Context, config *Config) (*Cacher, error) {
	checkInterval := time.Duration(config.CheckInterval) * time.Second
	if checkInterval == 0 {
		checkInterval = defaultCheckInterval * time.Second
	}

	cachePeriod := time.Duration(config.CachePeriod) * time.Second
	if cachePeriod == 0 {
		cachePeriod = defaultCachePeriod * time.Second
	}

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

	capacity := uint64(config.CacheCapacity) * gib
	if capacity == 0 {
		capacity = defaultCacheCapacity * gib
	}

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
		ctx:           ctx,
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
		fil, err := apt.ExtractFileInfo(fi.Path(), f)
		f.Close()
		if err != nil {
			return nil, errors.Wrap(err, "ExtractFileInfo("+fi.Path()+")")
		}
		for _, fi2 := range fil {
			c.info[fi2.Path()] = fi2
		}
	}

	// add meta files w/o checksums (Release, Release.pgp, and InRelease).
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
		go c.maintRelease(p, true)
	case "InRelease":
		go c.maintRelease(p, false)
	}
}

func (c *Cacher) maintRelease(p string, withGPG bool) {
	ticker := time.NewTicker(c.checkInterval)
	defer ticker.Stop()

	if log.Enabled(log.LvDebug) {
		log.Debug("maintRelease", map[string]interface{}{
			"_path": p,
		})
	}

	for {
		select {
		case <-c.ctx.Done():
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
	go c.download(p, u, valid)
	return ch
}

// download is a goroutine to download an item.
func (c *Cacher) download(p string, u *url.URL, valid *apt.FileInfo) {
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
		go func(ctx context.Context) {
			select {
			case <-ctx.Done():
				return
			case <-time.After(c.cachePeriod):
			}
			c.dlLock.Lock()
			delete(c.results, p)
			c.dlLock.Unlock()
		}(c.ctx)
	}()

	ctx, cancel := context.WithTimeout(c.ctx, requestTimeout)
	defer cancel()

	resp, err := ctxhttp.Get(ctx, c.client, u.String())
	if err != nil {
		log.Warn("GET failed", map[string]interface{}{
			"_url": u.String(),
			"_err": err.Error(),
		})
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	statusCode = resp.StatusCode
	if statusCode != 200 {
		return
	}

	if err != nil {
		log.Warn("GET failed", map[string]interface{}{
			"_url": u.String(),
			"_err": err.Error(),
		})
		return
	}

	fi := apt.MakeFileInfo(p, body)
	if valid != nil && !valid.Same(fi) {
		log.Warn("downloaded data is not valid", map[string]interface{}{
			"_url": u.String(),
		})
		return
	}

	storage := c.items
	var fil []*apt.FileInfo
	if apt.IsMeta(p) {
		storage = c.meta
		fil, err = apt.ExtractFileInfo(p, bytes.NewReader(body))
		if err != nil {
			log.Error("invalid meta data", map[string]interface{}{
				"_path": p,
				"_err":  err.Error(),
			})
			// do not return; we accept broken meta data as is.
		}
	}

	c.fiLock.Lock()
	defer c.fiLock.Unlock()

	if err := storage.Insert(body, fi); err != nil {
		log.Error("could not save an item", map[string]interface{}{
			"_path": p,
			"_err":  err.Error(),
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
		"_path": p,
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
				"_err": err.Error(),
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
