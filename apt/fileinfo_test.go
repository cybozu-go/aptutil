package apt

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"testing"
)

func TestFileInfo(t *testing.T) {
	t.Parallel()

	data := []byte{'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i'}
	md5sum := md5.Sum(data)
	sha1sum := sha1.Sum(data)
	sha256sum := sha256.Sum256(data)

	data2 := []byte{'1', '2', '3'}
	md5sum2 := md5.Sum(data2)
	sha1sum2 := sha1.Sum(data2)

	fi := &FileInfo{
		path:      "/data",
		size:      uint64(len(data)),
		md5sum:    md5sum[:],
		sha1sum:   sha1sum[:],
		sha256sum: sha256sum[:],
	}

	if fi.Path() != "/data" {
		t.Error(`fi.Path() != "/data"`)
	}

	badpath := &FileInfo{
		path: "bad",
		size: uint64(len(data)),
	}
	if badpath.Same(fi) {
		t.Error(`badpath.Same(fi)`)
	}

	pathonly := &FileInfo{
		path: "/data",
		size: uint64(len(data)),
	}
	if !pathonly.Same(fi) {
		t.Error(`!pathonly.Same(fi)`)
	}

	sizemismatch := &FileInfo{
		path: "/data",
		size: 0,
	}
	if sizemismatch.Same(fi) {
		t.Error(`sizemismatch.Same(fi)`)
	}

	md5mismatch := &FileInfo{
		path:   "/data",
		size:   uint64(len(data)),
		md5sum: md5sum2[:],
	}
	if md5mismatch.Same(fi) {
		t.Error(`md5mismatch.Same(fi)`)
	}

	md5match := &FileInfo{
		path:   "/data",
		size:   uint64(len(data)),
		md5sum: md5sum[:],
	}
	if !md5match.Same(fi) {
		t.Error(`!md5match.Same(fi)`)
	}

	sha1mismatch := &FileInfo{
		path:    "/data",
		size:    uint64(len(data)),
		md5sum:  md5sum[:],
		sha1sum: sha1sum2[:],
	}
	if sha1mismatch.Same(fi) {
		t.Error(`sha1mismatch.Same(fi)`)
	}

	sha1match := &FileInfo{
		path:    "/data",
		size:    uint64(len(data)),
		md5sum:  md5sum[:],
		sha1sum: sha1sum[:],
	}
	if !sha1match.Same(fi) {
		t.Error(`!sha1match.Same(fi)`)
	}

	sha1matchmd5mismatch := &FileInfo{
		path:    "/data",
		size:    uint64(len(data)),
		md5sum:  md5sum2[:],
		sha1sum: sha1sum[:],
	}
	if sha1matchmd5mismatch.Same(fi) {
		t.Error(`sha1matchmd5mismatch.Same(fi)`)
	}

	allmatch := &FileInfo{
		path:      "/data",
		size:      uint64(len(data)),
		md5sum:    md5sum[:],
		sha1sum:   sha1sum[:],
		sha256sum: sha256sum[:],
	}
	if !allmatch.Same(fi) {
		t.Error(`!allmatch.Same(fi)`)
	}
}

func TestMakeFileInfo(t *testing.T) {
	t.Parallel()

	path := "/abc/def"
	data := []byte{'a', 'b', 'c', 'd', 'e', 'f'}

	fi := MakeFileInfo(path, data)
	if fi.Path() != path {
		t.Error(`fi.Path() != path`)
	}
	if fi.Size() != uint64(len(data)) {
		t.Error(`fi.Size() != uint64(len(data))`)
	}

	sha256sum := sha256.Sum256(data)
	if bytes.Compare(sha256sum[:], fi.sha256sum) != 0 {
		t.Error(`bytes.Compare(sha256sum[:], fi.sha256sum) != 0`)
	}
}
