package aptcacher

import (
	"container/heap"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/cybozu-go/log"
	"github.com/pkg/errors"
)

var (
	// ErrNotFound is returned by Storage.Lookup for non-existing items.
	ErrNotFound = errors.New("not found")

	// ErrBadPath is returned by Storage.Insert if path is bad
	ErrBadPath = errors.New("bad path")
)

// entry represents an item in the cache.
type entry struct {
	*FileInfo

	// for container/heap.
	// atime is used as priorities.
	atime uint64
	index int
}

// Storage stores cache items in local file system.
//
// Cached items will be removed in LRU fashion when the total size of
// items exceeds the capacity.
type Storage struct {
	dir      string // directory for cache items
	capacity uint64

	mu     sync.Mutex
	used   uint64
	cache  map[string]*entry
	lru    []*entry // for container/heap
	lclock uint64   // ditto
}

// NewStorage creates a Storage.
//
// dir is the directory for cached items.
// capacity is the maximum total size (bytes) of items in the cache.
// If capacity is zero, items will not be evicted.
func NewStorage(dir string, capacity uint64) *Storage {
	if !filepath.IsAbs(dir) {
		panic("dir must be an absolute path")
	}
	return &Storage{
		dir:      dir,
		cache:    make(map[string]*entry),
		capacity: capacity,
	}
}

// Len implements heap.Interface.
func (cm *Storage) Len() int {
	return len(cm.lru)
}

// Less implements heap.Interface.
func (cm *Storage) Less(i, j int) bool {
	return cm.lru[i].atime < cm.lru[j].atime
}

// Swap implements heap.Interface.
func (cm *Storage) Swap(i, j int) {
	cm.lru[i], cm.lru[j] = cm.lru[j], cm.lru[i]
	cm.lru[i].index = i
	cm.lru[j].index = j
}

// Push implements heap.Interface.
func (cm *Storage) Push(x interface{}) {
	e, ok := x.(*entry)
	if !ok {
		panic("Storage.Push: wrong type")
	}
	n := len(cm.lru)
	e.index = n
	cm.lru = append(cm.lru, e)
}

// Pop implements heap.Interface.
func (cm *Storage) Pop() interface{} {
	n := len(cm.lru)
	e := cm.lru[n-1]
	e.index = -1 // for safety
	cm.lru = cm.lru[0 : n-1]
	return e
}

// maint removes unused items from cache until used < capacity.
// cm.mu lock must be acquired beforehand.
func (cm *Storage) maint() {
	for cm.capacity > 0 && cm.used > cm.capacity {
		e := heap.Pop(cm).(*entry)
		delete(cm.cache, e.Path())
		cm.used -= e.Size()
		if err := os.Remove(filepath.Join(cm.dir, e.Path())); err != nil {
			log.Warn("Storage.maint", map[string]interface{}{
				"_err": err.Error(),
			})
		}
		log.Info("removed", map[string]interface{}{
			"_path": e.Path(),
		})
	}
}

func readData(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return ioutil.ReadAll(f)
}

// Load loads existing items in filesystem.
func (cm *Storage) Load() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	wf := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		subpath, err := filepath.Rel(cm.dir, path)
		if err != nil {
			return err
		}
		if _, ok := cm.cache[subpath]; ok {
			return nil
		}

		size := uint64(info.Size())
		e := &entry{
			// delay calculation of checksums.
			FileInfo: &FileInfo{
				path: subpath,
				size: size,
			},
			atime: cm.lclock,
			index: len(cm.lru),
		}
		cm.used += size
		cm.lclock++
		cm.lru = append(cm.lru, e)
		cm.cache[subpath] = e
		log.Debug("Storage.Load", map[string]interface{}{
			"_path": subpath,
		})
		return nil
	}

	if err := filepath.Walk(cm.dir, wf); err != nil {
		return err
	}
	heap.Init(cm)

	cm.maint()

	return nil
}

// Insert inserts or updates a cache item.
//
// fi.Path() must be as clean as filepath.Clean() and
// must not be filepath.IsAbs().
func (cm *Storage) Insert(data []byte, fi *FileInfo) error {
	switch {
	case fi.path != filepath.Clean(fi.path):
		return ErrBadPath
	case filepath.IsAbs(fi.path):
		return ErrBadPath
	case fi.path == ".":
		return ErrBadPath
	}

	f, err := ioutil.TempFile(cm.dir, "_tmp")
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	_, err = f.Write(data)
	if err != nil {
		return err
	}
	err = f.Sync()
	if err != nil {
		return err
	}

	p := fi.path
	destpath := filepath.Join(cm.dir, p)
	dirpath := filepath.Dir(destpath)

	_, err = os.Stat(dirpath)
	switch {
	case os.IsNotExist(err):
		err = os.MkdirAll(dirpath, 0755)
		if err != nil {
			return err
		}
	case err != nil:
		return err
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if existing, ok := cm.cache[p]; ok {
		err = os.Remove(destpath)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			log.Warn("cache file was removed already", map[string]interface{}{
				"_path": p,
			})
		}
		cm.used -= existing.Size()
		heap.Remove(cm, existing.index)
		delete(cm.cache, p)
		if log.Enabled(log.LvDebug) {
			log.Debug("deleted existing item", map[string]interface{}{
				"_path": p,
			})
		}
	}

	err = os.Rename(f.Name(), destpath)
	if err != nil {
		return err
	}

	e := &entry{
		FileInfo: fi,
		atime:    cm.lclock,
	}
	cm.used += fi.size
	cm.lclock++
	heap.Push(cm, e)
	cm.cache[p] = e

	cm.maint()

	return nil
}

func calcChecksum(dir string, e *entry) error {
	if e.FileInfo.md5sum != nil {
		return nil
	}

	data, err := readData(filepath.Join(dir, e.Path()))
	if err != nil {
		return err
	}
	md5sum := md5.Sum(data)
	sha1sum := sha1.Sum(data)
	sha256sum := sha256.Sum256(data)
	e.FileInfo.md5sum = md5sum[:]
	e.FileInfo.sha1sum = sha1sum[:]
	e.FileInfo.sha256sum = sha256sum[:]
	return nil
}

// Lookup looks up an item in the cache.
// If no item matching fi is found, ErrNotFound is returned.
//
// The caller is responsible to close the returned os.File.
func (cm *Storage) Lookup(fi *FileInfo) (*os.File, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	e, ok := cm.cache[fi.path]
	if !ok {
		return nil, ErrNotFound
	}

	// delayed checksum calculation
	err := calcChecksum(cm.dir, e)
	if err != nil {
		return nil, err
	}

	if !fi.Same(e.FileInfo) {
		// checksum mismatch
		return nil, ErrNotFound
	}

	e.atime = cm.lclock
	cm.lclock++
	heap.Fix(cm, e.index)
	return os.Open(filepath.Join(cm.dir, fi.path))
}

// ListAll returns a list of FileInfo for all cached items.
func (cm *Storage) ListAll() []*FileInfo {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	l := make([]*FileInfo, cm.Len())
	for i, e := range cm.lru {
		l[i] = e.FileInfo
	}
	return l
}

// Delete deletes an item from the cache.
func (cm *Storage) Delete(p string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	e, ok := cm.cache[p]
	if !ok {
		return nil
	}

	err := os.Remove(filepath.Join(cm.dir, p))
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		log.Warn("cached file was already removed", map[string]interface{}{
			"_path": p,
		})
	}

	cm.used -= e.size
	heap.Remove(cm, e.index)
	delete(cm.cache, p)
	log.Info("deleted item", map[string]interface{}{
		"_path": p,
	})
	return nil
}
