package mirror

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/cybozu-go/go-apt-cacher/apt"
	"github.com/pkg/errors"
)

const (
	infoJSON = "info.json"
)

// Storage manages a directory tree that mirrors a Debian repository.
//
// Storage also keeps checksum information for stored files.
type Storage struct {
	dir    string
	prefix string

	mu   sync.RWMutex
	info map[string]*apt.FileInfo
}

// NewStorage constructs Storage.
//
// dir must be an absolute path to an existing directory.
// prefix should be a directory name.
func NewStorage(dir, prefix string) (*Storage, error) {
	if !filepath.IsAbs(dir) {
		return nil, errors.New("none absolute: " + dir)
	}

	dir = filepath.Clean(dir)
	st, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsDir() {
		return nil, errors.New("not a directory: " + dir)
	}

	return &Storage{
		dir:    dir,
		prefix: prefix,
		info:   make(map[string]*apt.FileInfo),
	}, nil
}

// Dir returns the directory of the Storage.
func (s *Storage) Dir() string {
	return s.dir
}

// Load loads existing directory contents.
func (s *Storage) Load() error {
	infoPath := filepath.Join(s.dir, infoJSON)

	f, err := os.Open(infoPath)
	switch {
	case os.IsNotExist(err):
		return nil
	case err != nil:
		return err
	}
	defer f.Close()

	jd := json.NewDecoder(f)
	err = jd.Decode(&s.info)
	if err != nil {
		return errors.Wrap(err, "Storage.Load: "+infoPath)
	}
	return nil
}

// Save saves storage contents persistently.
func (s *Storage) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	infoPath := filepath.Join(s.dir, infoJSON)
	f, err := os.OpenFile(infoPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	err = enc.Encode(s.info)
	if err != nil {
		return err
	}

	f.Sync()
	DirSyncTree(s.dir)

	return nil
}

// Store stores a file into this storage.
func (s *Storage) Store(fi *apt.FileInfo, data []byte) error {
	p := fi.Path()

	s.mu.Lock()
	_, ok := s.info[p]
	if ok {
		s.mu.Unlock()
		return errors.New("already stored: " + p)
	}
	s.info[p] = fi
	s.mu.Unlock()

	fp := filepath.Join(s.dir, s.prefix, filepath.Clean(p))
	d := filepath.Dir(fp)

	err := os.MkdirAll(d, 0755)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(fp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		return err
	}
	return f.Sync()
}

// StoreLink stores a hard link to a file into this storage.
func (s *Storage) StoreLink(fi *apt.FileInfo, fullpath string) error {
	p := fi.Path()

	s.mu.Lock()
	_, ok := s.info[p]
	if ok {
		s.mu.Unlock()
		return errors.New("already stored: " + p)
	}
	s.info[p] = fi
	s.mu.Unlock()

	fp := filepath.Join(s.dir, s.prefix, filepath.Clean(p))
	d := filepath.Dir(fp)

	err := os.MkdirAll(d, 0755)
	if err != nil {
		return err
	}

	return os.Link(fullpath, fp)
}

// Lookup looks up a file in this storage.
//
// If a file matching fi exists, its info and full path is returned.
// Otherwise, nil and empty string is returned.
func (s *Storage) Lookup(fi *apt.FileInfo) (*apt.FileInfo, string) {
	s.mu.RLock()
	fi2, ok := s.info[fi.Path()]
	s.mu.RUnlock()

	if !ok || !fi.Same(fi2) {
		return nil, ""
	}

	return fi2, filepath.Join(s.dir, s.prefix, filepath.Clean(fi2.Path()))
}

// Open opens the named file and returns it.
func (s *Storage) Open(p string) (*os.File, error) {
	return os.Open(filepath.Join(s.dir, s.prefix, filepath.Clean(p)))
}
