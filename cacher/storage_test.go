package cacher

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/cybozu-go/aptutil/apt"
)

func TestStorage(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "gotest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	cm := NewStorage(dir, 0)

	data := []byte{'a'}
	err = cm.Insert(data, apt.MakeFileInfo("path/to/a", data))
	if err != nil {
		t.Fatal(err)
	}
	if cm.Len() != 1 {
		t.Error(`cm.Len() != 1`)
	}
	if cm.used != 1 {
		t.Error(`cm.used != 1`)
	}

	// overwrite
	err = cm.Insert(data, apt.MakeFileInfo("path/to/a", data))
	if err != nil {
		t.Fatal(err)
	}
	if cm.Len() != 1 {
		t.Error(`cm.Len() != 1`)
	}
	if cm.used != 1 {
		t.Error(`cm.used != 1`)
	}

	data = []byte{'b', 'c'}
	err = cm.Insert(data, apt.MakeFileInfo("path/to/bc", data))
	if err != nil {
		t.Fatal(err)
	}
	if cm.Len() != 2 {
		t.Error(`cm.Len() != 2`)
	}
	if cm.used != 3 {
		t.Error(`cm.used != 3`)
	}

	data = []byte{'d', 'a', 't', 'a'}
	err = cm.Insert(data, apt.MakeFileInfo("data", data))
	if err != nil {
		t.Fatal(err)
	}

	f, err := cm.Lookup(apt.MakeFileInfo("data", data))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	data2, err := ioutil.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(data, data2) != 0 {
		t.Error(`bytes.Compare(data, data2) != 0`)
	}

	differentData := []byte{'d', 'a', 't', '.'}
	_, err = cm.Lookup(apt.MakeFileInfo("data", differentData))
	if err != ErrNotFound {
		t.Error(`err != ErrNotFound`)
	}

	err = cm.Delete("data")
	if err != nil {
		t.Fatal(err)
	}
	if cm.Len() != 2 {
		t.Error(`cm.Len() != 2`)
	}
	if cm.used != 3 {
		t.Error(`cm.used != 3`)
	}
}

func TestStorageLRU(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "gotest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	cm := NewStorage(dir, 3)

	dataA := []byte{'a'}
	err = cm.Insert(dataA, apt.MakeFileInfo("path/to/a", dataA))
	if err != nil {
		t.Fatal(err)
	}
	dataBC := []byte{'b', 'c'}
	err = cm.Insert(dataBC, apt.MakeFileInfo("path/to/bc", dataBC))
	if err != nil {
		t.Fatal(err)
	}
	if cm.used != 3 {
		t.Error(`cm.used != 3`)
	}

	// a and bc will be purged
	dataDE := []byte{'d', 'e'}
	err = cm.Insert(dataDE, apt.MakeFileInfo("path/to/de", dataDE))
	if err != nil {
		t.Fatal(err)
	}
	if cm.Len() != 1 {
		t.Error(`cm.Len() != 1`)
	}
	if cm.used != 2 {
		t.Error(`cm.used != 2`)
	}

	_, err = cm.Lookup(apt.MakeFileInfo("path/to/a", dataA))
	if err != ErrNotFound {
		t.Error(`err != ErrNotFound`)
	}
	_, err = cm.Lookup(apt.MakeFileInfo("path/to/bc", dataBC))
	if err != ErrNotFound {
		t.Error(`err != ErrNotFound`)
	}

	err = cm.Insert(dataA, apt.MakeFileInfo("path/to/a", dataA))
	if err != nil {
		t.Fatal(err)
	}

	// touch de
	_, err = cm.Lookup(apt.MakeFileInfo("path/to/de", dataDE))
	if err != nil {
		t.Error(err)
	}

	// a will be purged
	dataF := []byte{'f'}
	err = cm.Insert(dataF, apt.MakeFileInfo("path/to/f", dataF))
	if err != nil {
		t.Fatal(err)
	}

	_, err = cm.Lookup(apt.MakeFileInfo("path/to/a", dataA))
	if err != ErrNotFound {
		t.Error(`err != ErrNotFound`)
	}
	_, err = cm.Lookup(apt.MakeFileInfo("path/to/de", dataDE))
	if err != nil {
		t.Error(err)
	}
	_, err = cm.Lookup(apt.MakeFileInfo("path/to/f", dataF))
	if err != nil {
		t.Error(err)
	}
}

func TestStorageLoad(t *testing.T) {
	t.Parallel()

	files := map[string][]byte{
		"a":    []byte{'a'},
		"bc":   []byte{'b', 'c'},
		"def":  []byte{'d', 'e', 'f'},
		"ghij": []byte{'g', 'h', 'i', 'j'},
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

	f, err := cm.Lookup(apt.MakeFileInfo("a", files["a"]))
	if err != nil {
		t.Error(err)
	}
	f.Close()
	f, err = cm.Lookup(apt.MakeFileInfo("bc", files["bc"]))
	if err != nil {
		t.Error(err)
	}
	f.Close()
	f, err = cm.Lookup(apt.MakeFileInfo("def", files["def"]))
	if err != nil {
		t.Error(err)
	}
	f.Close()

	f, err = cm.Lookup(apt.MakeFileInfo("ghij", files["ghij"]))
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadAll(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Compare(files["ghij"], data) != 0 {
		t.Error(`bytes.Compare(files["ghij"], data) != 0`)
	}
}

func TestStoragePathTraversal(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "gotest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	cm := NewStorage(dir, 0)

	data := []byte{'a'}
	err = cm.Insert(data, apt.MakeFileInfo("/absolute/path", data))
	if err != ErrBadPath {
		t.Error(`/absolute/path must be a bad path`)
	}

	err = cm.Insert(data, apt.MakeFileInfo("./unclean/path", data))
	if err != ErrBadPath {
		t.Error(`./unclean/path must be a bad path`)
	}

	err = cm.Insert(data, apt.MakeFileInfo("", data))
	if err != ErrBadPath {
		t.Error(`empty path must be a bad path`)
	}

	err = cm.Insert(data, apt.MakeFileInfo(".", data))
	if err != ErrBadPath {
		t.Error(`. must be a bad path`)
	}
}
