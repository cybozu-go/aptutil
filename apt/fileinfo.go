package apt

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/pkg/errors"
)

// FileInfo is a set of meta data of a file.
type FileInfo struct {
	path      string
	size      uint64
	md5sum    []byte // nil means no MD5 checksum to be checked.
	sha1sum   []byte // nil means no SHA1 ...
	sha256sum []byte // nil means no SHA256 ...
}

// Same returns true if t has the same checksum values.
func (fi *FileInfo) Same(t *FileInfo) bool {
	if fi == t {
		return true
	}
	if fi.path != t.path {
		return false
	}
	if fi.size != t.size {
		return false
	}
	if fi.md5sum != nil && bytes.Compare(fi.md5sum, t.md5sum) != 0 {
		return false
	}
	if fi.sha1sum != nil && bytes.Compare(fi.sha1sum, t.sha1sum) != 0 {
		return false
	}
	if fi.sha256sum != nil && bytes.Compare(fi.sha256sum, t.sha256sum) != 0 {
		return false
	}
	return true
}

// Path returns the indentifying path string of the file.
func (fi *FileInfo) Path() string {
	return fi.path
}

// Size returns the number of bytes of the file body.
func (fi *FileInfo) Size() uint64 {
	return fi.size
}

// HasChecksum returns true if fi has checksums.
func (fi *FileInfo) HasChecksum() bool {
	return fi.md5sum != nil
}

// CalcChecksums calculates checksums and stores them in fi.
func (fi *FileInfo) CalcChecksums(data []byte) {
	md5sum := md5.Sum(data)
	sha1sum := sha1.Sum(data)
	sha256sum := sha256.Sum256(data)
	fi.size = uint64(len(data))
	fi.md5sum = md5sum[:]
	fi.sha1sum = sha1sum[:]
	fi.sha256sum = sha256sum[:]
}

type fileInfoJSON struct {
	Path      string
	Size      int64
	MD5Sum    string
	SHA1Sum   string
	SHA256Sum string
}

// MarshalJSON implements json.Marshaler
func (fi *FileInfo) MarshalJSON() ([]byte, error) {
	var fij fileInfoJSON
	fij.Path = fi.path
	fij.Size = int64(fi.size)
	if fi.md5sum != nil {
		fij.MD5Sum = hex.EncodeToString(fi.md5sum)
	}
	if fi.sha1sum != nil {
		fij.SHA1Sum = hex.EncodeToString(fi.sha1sum)
	}
	if fi.sha256sum != nil {
		fij.SHA256Sum = hex.EncodeToString(fi.sha256sum)
	}
	return json.Marshal(&fij)
}

// UnmarshalJSON implements json.Unmarshaler
func (fi *FileInfo) UnmarshalJSON(data []byte) error {
	var fij fileInfoJSON
	if err := json.Unmarshal(data, &fij); err != nil {
		return err
	}
	fi.path = fij.Path
	fi.size = uint64(fij.Size)
	md5sum, err := hex.DecodeString(fij.MD5Sum)
	if err != nil {
		return errors.Wrap(err, "UnmarshalJSON for "+fij.Path)
	}
	sha1sum, err := hex.DecodeString(fij.SHA1Sum)
	if err != nil {
		return errors.Wrap(err, "UnmarshalJSON for "+fij.Path)
	}
	sha256sum, err := hex.DecodeString(fij.SHA256Sum)
	if err != nil {
		return errors.Wrap(err, "UnmarshalJSON for "+fij.Path)
	}
	fi.md5sum = md5sum
	fi.sha1sum = sha1sum
	fi.sha256sum = sha256sum
	return nil
}

// MakeFileInfoNoChecksum constructs a FileInfo without calculating checksums.
func MakeFileInfoNoChecksum(path string, size uint64) *FileInfo {
	return &FileInfo{
		path: path,
		size: size,
	}
}

// MakeFileInfo constructs a FileInfo for a given data.
func MakeFileInfo(path string, data []byte) *FileInfo {
	fi := MakeFileInfoNoChecksum(path, 0)
	fi.CalcChecksums(data)
	return fi
}
