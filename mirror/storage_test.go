package mirror

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/cybozu-go/aptutil/apt"
)

func makeFileInfo(path string, data []byte) (*apt.FileInfo, error) {
	r := bytes.NewReader(data)
	w := new(bytes.Buffer)
	fi, err := apt.CopyWithFileInfo(w, r, path)
	if err != nil {
		return nil, err
	}
	return fi, nil
}

func testStorageBadConstruction(t *testing.T) {
	t.Parallel()

	f, err := ioutil.TempFile("", "gotest")
	if err != nil {
		t.Fatal(err)
	}
	defer func(f string) {
		if _, err := os.Stat(f); err != nil {
			return
		}
		os.Remove(f)
	}(f.Name())

	_, err = NewStorage(f.Name(), "pre")
	if err == nil {
		t.Error("NewStorage must fail with regular file")
	}

	os.Remove(f.Name())
	_, err = NewStorage(f.Name(), "pre")
	if err == nil {
		t.Error("NewStorage must fail with non-existent directory")
	}
}

func testStorageLookup(t *testing.T) {
	t.Parallel()

	d, err := ioutil.TempDir("", "gotest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	s, err := NewStorage(d, "pre")
	if err != nil {
		t.Fatal(err)
	}

	err = s.Load()
	if err != nil {
		t.Error(err)
	}

	files := map[string][]byte{
		"a/b/c":   []byte{'a', 'b', 'c'},
		"def":     []byte{'d', 'e', 'f'},
		"a/pp/le": []byte{'a', 'p', 'p', 'l', 'e'},
	}

	for fn, data := range files {
		tempfile, err := s.TempFile()
		if err != nil {
			t.Fatal(err)
		}

		fi, err := apt.CopyWithFileInfo(tempfile, bytes.NewReader(data), fn)
		if err != nil {
			t.Fatal(err)
		}
		tempfile.Close()

		if err := s.StoreLink(fi, tempfile.Name()); err != nil {
			t.Fatal(err)
		}
		os.Remove(tempfile.Name())
	}

	fi, err := makeFileInfo("a/b/c", []byte{'a', 'b', 'd'})
	if err != nil {
		t.Fatal(err)
	}
	fi2, fullpath := s.Lookup(fi, false)
	if fi2 != nil {
		t.Error(`fi2 != nil`)
	}
	if len(fullpath) != 0 {
		t.Error(`len(fullpath) != 0`)
	}

	fi, err = makeFileInfo("a/b/c", files["a/b/c"])
	if err != nil {
		t.Fatal(err)
	}
	fi3, _ := s.Lookup(fi, false)
	if fi3 == nil {
		t.Error(`fi3 == nil`)
	}

	s.Save()

	s2, err := NewStorage(d, "ubuntu")
	if err != nil {
		t.Fatal(err)
	}

	err = s2.Load()
	if err != nil {
		t.Error(err)
	}

	fi, err = makeFileInfo("a/b/c", files["a/b/c"])
	if err != nil {
		t.Fatal(err)
	}
	fi4, _ := s2.Lookup(fi, false)
	if fi4 == nil {
		t.Error(`fi4 == nil`)
	}

	fi, err = makeFileInfo("def", files["def"])
	if err != nil {
		t.Fatal(err)
	}
	fi5, _ := s2.Lookup(fi, false)
	if fi5 == nil {
		t.Error(`fi5 == nil`)
	}

	fi, err = makeFileInfo("a/pp/le", files["a/pp/le"])
	if err != nil {
		t.Fatal(err)
	}
	fi6, _ := s2.Lookup(fi, false)
	if fi6 == nil {
		t.Error(`fi6 == nil`)
	}

	fi, err = makeFileInfo("a/pp/le", files["def"])
	if err != nil {
		t.Fatal(err)
	}
	fi7, _ := s2.Lookup(fi, false)
	if fi7 != nil {
		t.Error(`fi7 != nil`)
	}
}

func testStorageStore(t *testing.T) {
	t.Parallel()

	d, err := ioutil.TempDir("", "gotest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	s, err := NewStorage(d, "pre")
	if err != nil {
		t.Fatal(err)
	}

	err = s.Load()
	if err != nil {
		t.Error(err)
	}

	tempfile, err := s.TempFile()
	if err != nil {
		t.Fatal(err)
	}
	fi, err := apt.CopyWithFileInfo(tempfile, strings.NewReader("abc"), "a/b/c")
	tempfile.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = s.StoreLink(fi, tempfile.Name())
	if err != nil {
		t.Error(err)
	}
	found, _ := s.Lookup(fi, false)
	if found == nil {
		t.Error(`found == nil`)
	}

	// duplicates should not be granted
	err = s.StoreLink(fi, tempfile.Name())
	os.Remove(tempfile.Name())
	if err == nil {
		t.Error(`err == nil`)
	}

	tempfile, err = s.TempFile()
	if err != nil {
		t.Fatal(err)
	}
	fi, err = apt.CopyWithFileInfo(tempfile, strings.NewReader("def"), "a/b/c")
	tempfile.Close()
	err = s.StoreLinkWithHash(fi, tempfile.Name())
	os.Remove(tempfile.Name())
	if err != nil {
		t.Error(err)
	}
	notfound, _ := s.Lookup(fi, false)
	if notfound != nil {
		t.Error(`notfound != nil`)
	}
	found, _ = s.Lookup(fi, true)
	if found == nil {
		t.Error(`found == nil`, d, fi.SHA256Path())
	}
}

func TestStorage(t *testing.T) {
	t.Run("BadConstruction", testStorageBadConstruction)
	t.Run("Lookup", testStorageLookup)
	t.Run("Store", testStorageStore)
}
