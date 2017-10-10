package cacher

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/cybozu-go/aptutil/apt"
)

func insert(cm *Storage, data []byte, path string) (*apt.FileInfo, error) {
	f, err := cm.TempFile()
	if err != nil {
		return nil, err
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	fi, err := apt.CopyWithFileInfo(f, bytes.NewReader(data), path)
	if err != nil {
		return nil, err
	}

	err = f.Sync()
	if err != nil {
		return nil, err
	}

	err = cm.Insert(f.Name(), fi)
	return fi, err
}

func testStorageInsertWorksCorrectly(t *testing.T) {
	t.Parallel()
	dir, err := ioutil.TempDir("", "gotest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	cm := NewStorage(dir, 0)

	fi, err := insert(cm, []byte("a"), "path/to/a")
	if err != nil {
		t.Fatal(err)
	}

	if cm.Len() != 1 {
		t.Error(`cm.Len() != 1`)
	}

	_, err = cm.Lookup(fi)
	if err != nil {
		t.Error(`cannot lookup inserted file`)
	}
}

func testStorageInsertOverwrite(t *testing.T) {
	t.Parallel()
	dir, err := ioutil.TempDir("", "gotest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	cm := NewStorage(dir, 0)

	fi, err := insert(cm, []byte("a"), "path/to/a")
	if err != nil {
		t.Fatal(err)
	}

	fi, err = insert(cm, []byte("a"), "path/to/a")
	if err != nil {
		t.Fatal(err)
	}

	if cm.Len() != 1 {
		t.Error(`cm.Len() != 1`)
	}

	_, err = cm.Lookup(fi)
	if err != nil {
		t.Error(`cannot lookup inserted file`)
	}
}

func testStorageInsertReturnsErrorAgainstBadPath(t *testing.T) {
	t.Parallel()
	dir, err := ioutil.TempDir("", "gotest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	cm := NewStorage(dir, 0)

	cases := []struct{ Title, Path string }{
		{
			Title: "Absolute path",
			Path:  "/absolute/path",
		},
		{
			Title: "Uncleaned path",
			Path:  "./uncleaned/path",
		},
		{
			Title: "Empty path",
			Path:  "",
		},
		{
			Title: ".",
			Path:  ".",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Title, func(t *testing.T) {
			_, err = insert(cm, []byte("a"), tc.Path)
			if err != ErrBadPath {
				t.Fatal(err)
			}
		})
	}
}

func testStorageInsertPurgesFilesAllowingLRU(t *testing.T) {
	t.Parallel()
	dir, err := ioutil.TempDir("", "gotest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	cm := NewStorage(dir, 3)

	fiA, err := insert(cm, []byte("a"), "a")
	if err != nil {
		t.Fatal(err)
	}

	fiBC, err := insert(cm, []byte("bc"), "bc")
	if err != nil {
		t.Fatal(err)
	}

	// a and bc will be purged
	fiDE, err := insert(cm, []byte("de"), "de")
	if err != nil {
		t.Fatal(err)
	}

	if cm.Len() != 1 {
		t.Error(`cmd.Len() != 1`)
	}
	_, err = cm.Lookup(fiA)
	if err != ErrNotFound {
		t.Error(`err != ErrNotFound`)
	}
	_, err = cm.Lookup(fiBC)
	if err != ErrNotFound {
		t.Error(`err != ErrNotFound`)
	}

	fiA, err = insert(cm, []byte("a"), "a")
	if err != nil {
		t.Fatal(err)
	}

	// touch de
	_, err = cm.Lookup(fiDE)
	if err != nil {
		t.Error(err)
	}

	// a will be purged
	fiF, err := insert(cm, []byte("f"), "f")
	if err != nil {
		t.Fatal(err)
	}

	_, err = cm.Lookup(fiA)
	if err != ErrNotFound {
		t.Error(`err != ErrNotFound`)
	}
	_, err = cm.Lookup(fiDE)
	if err != nil {
		t.Error(err)
	}
	_, err = cm.Lookup(fiF)
	if err != nil {
		t.Error(err)
	}
}

func TestStorageInsert(t *testing.T) {
	t.Run("Storage.Insert should insert file", testStorageInsertWorksCorrectly)
	t.Run("Storage.Insert should overwrite", testStorageInsertOverwrite)
	t.Run("Storage.Insert should return error if passed FileInfo path is bad path", testStorageInsertReturnsErrorAgainstBadPath)
	t.Run("Storage.Insert should purge files allowing LRU", testStorageInsertPurgesFilesAllowingLRU)
}

func makeFileInfo(path string, data []byte) (*apt.FileInfo, error) {
	rb := bytes.NewReader(data)
	wb := new(bytes.Buffer)
	fi, err := apt.CopyWithFileInfo(wb, rb, path)
	if err != nil {
		return nil, err
	}
	return fi, nil
}

func TestStorageLoad(t *testing.T) {
	t.Parallel()

	files := map[string][]byte{
		"a":    {'a'},
		"bc":   {'b', 'c'},
		"def":  {'d', 'e', 'f'},
		"ghij": {'g', 'h', 'i', 'j'},
	}

	dir, err := ioutil.TempDir("", "gotest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	for k, v := range files {
		err := ioutil.WriteFile(filepath.Join(dir, k+fileSuffix), v, 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	// dummy should be ignored as it does not have a proper suffix.
	err = ioutil.WriteFile(filepath.Join(dir, "dummy"), []byte{'d'}, 0644)
	if err != nil {
		t.Fatal(err)
	}

	cm := NewStorage(dir, 0)
	cm.Load()

	l := cm.ListAll()
	if len(l) != len(files) {
		t.Error(`len(l) != len(files)`)
	}

	fiA, err := makeFileInfo("a", files["a"])
	if err != nil {
		t.Error(err)
	}
	fA, err := cm.Lookup(fiA)
	if err != nil {
		t.Error(err)
	}
	fA.Close()
	fiBC, err := makeFileInfo("bc", files["bc"])
	if err != nil {
		t.Error(err)
	}
	fBC, err := cm.Lookup(fiBC)
	if err != nil {
		t.Error(err)
	}
	fBC.Close()
	fiDEF, err := makeFileInfo("def", files["def"])
	if err != nil {
		t.Error(err)
	}
	fDEF, err := cm.Lookup(fiDEF)
	if err != nil {
		t.Error(err)
	}
	fDEF.Close()

	fiGHIJ, err := makeFileInfo("ghij", files["ghij"])
	if err != nil {
		t.Error(err)
	}
	fGHIJ, err := cm.Lookup(fiGHIJ)
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadAll(fGHIJ)
	fGHIJ.Close()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(files["ghij"], data) != 0 {
		t.Error(`bytes.Compare(files["ghij"], data) != 0`)
	}
}
