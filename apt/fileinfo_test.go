package apt

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func testFileInfoSame(t *testing.T) {
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

func testFileInfoJSON(t *testing.T) {
	t.Parallel()

	r := strings.NewReader("hello world")
	w := new(bytes.Buffer)
	p := "/abc/def"

	fi, err := CopyWithFileInfo(w, r, p)
	if err != nil {
		t.Fatal(err)
	}
	j, err := json.Marshal(fi)
	if err != nil {
		t.Fatal(err)
	}

	fi2 := new(FileInfo)
	err = json.Unmarshal(j, fi2)
	if err != nil {
		t.Fatal(err)
	}

	if !fi.Same(fi2) {
		t.Error(`!fi.Same(fi2)`)
		t.Log(fmt.Sprintf("%#v", fi2))
	}
}

func testFileInfoAddPrefix(t *testing.T) {
	t.Parallel()

	r := strings.NewReader("hello world")
	w := new(bytes.Buffer)
	p := "/abc/def"

	fi, err := CopyWithFileInfo(w, r, p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Path() != "/abc/def" {
		t.Error(`fi.Path() != "/abc/def"`)
	}

	fi = fi.AddPrefix("/prefix")
	if fi.Path() != "/prefix/abc/def" {
		t.Error(`fi.Path() != "/prefix/abc/def"`)
	}
}

func testFileInfoChecksum(t *testing.T) {
	t.Parallel()

	text := "hello world"
	r := strings.NewReader(text)
	w := new(bytes.Buffer)
	p := "/abc/def"

	md5sum := md5.Sum([]byte(text))
	sha1sum := sha1.Sum([]byte(text))
	sha256sum := sha256.Sum256([]byte(text))
	m5 := hex.EncodeToString(md5sum[:])
	s1 := hex.EncodeToString(sha1sum[:])
	s256 := hex.EncodeToString(sha256sum[:])

	fi, err := CopyWithFileInfo(w, r, p)
	if err != nil {
		t.Fatal(err)
	}

	if fi.MD5SumPath() != "/abc/by-hash/MD5Sum/"+m5 {
		t.Error(`fi.MD5SumPath() != "/abc/by-hash/MD5Sum/" + md5`)
	}
	if fi.SHA1Path() != "/abc/by-hash/SHA1/"+s1 {
		t.Error(`fi.SHA1Path() != "/abc/by-hash/SHA1/" + s1`)
	}
	if fi.SHA256Path() != "/abc/by-hash/SHA256/"+s256 {
		t.Error(`fi.SHA256Path() != "/abc/by-hash/SHA256/" + s256`)
	}
}

func testFileInfoCopy(t *testing.T) {
	t.Parallel()

	text := "hello world"
	r := strings.NewReader(text)
	w := new(bytes.Buffer)
	p := "/abc/def"

	md5sum := md5.Sum([]byte(text))
	sha1sum := sha1.Sum([]byte(text))
	sha256sum := sha256.Sum256([]byte(text))

	fi := &FileInfo{
		path:      p,
		size:      uint64(r.Size()),
		md5sum:    md5sum[:],
		sha1sum:   sha1sum[:],
		sha256sum: sha256sum[:],
	}

	fi2, err := CopyWithFileInfo(w, r, p)
	if err != nil {
		t.Fatal(err)
	}
	if w.String() != text {
		t.Errorf(
			"Copy did not work properly, expected: %s, actual: %s",
			text, w.String(),
		)
	}
	if !fi.Same(fi2) {
		t.Error("Generated FileInfo is invalid")
	}
}

func TestFileInfo(t *testing.T) {
	t.Run("Same", testFileInfoSame)
	t.Run("JSON", testFileInfoJSON)
	t.Run("AddPrefix", testFileInfoAddPrefix)
	t.Run("Checksum", testFileInfoChecksum)
	t.Run("Copy", testFileInfoCopy)
}
