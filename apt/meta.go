package apt

// This file provides utilities for debian repository indices.

import (
	"compress/bzip2"
	"compress/gzip"
	"encoding/hex"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/ulikunitz/xz"
)

// IsMeta returns true if p points a debian repository index file
// containing checksums for other files.
func IsMeta(p string) bool {
	base := path.Base(p)

	// https://wiki.debian.org/RepositoryFormat#Compression_of_indices
	switch {
	case strings.HasSuffix(base, ".gz"):
		base = base[0 : len(base)-3]
	case strings.HasSuffix(base, ".bz2"):
		base = base[0 : len(base)-4]
	case strings.HasSuffix(base, ".xz"):
		base = base[0 : len(base)-3]
	case strings.HasSuffix(base, ".lzma"):
		base = base[0 : len(base)-5]
	case strings.HasSuffix(base, ".lz"):
		base = base[0 : len(base)-3]
	}

	switch base {
	case "Release", "Release.gpg", "InRelease":
		return true
	case "Packages", "Sources", "Index":
		return true
	}

	return false
}

// IsSupported returns true if the meta data is compressed that can be
// decompressed by ExtractFileInfo.
func IsSupported(p string) bool {
	switch path.Ext(p) {
	case "", ".gz", ".bz2", ".gpg", ".xz":
		return true
	}
	return false
}

// SupportByHash returns true if paragraph from Release indicates
// support for indices acquisition via hash values (by-hash).
// See https://wiki.debian.org/DebianRepository/Format#indices_acquisition_via_hashsums_.28by-hash.29
func SupportByHash(d Paragraph) bool {
	p := d["Acquire-By-Hash"]
	if len(p) != 1 {
		return false
	}
	return p[0] == "yes"
}

func parseChecksum(l string) (p string, size uint64, csum []byte, err error) {
	flds := strings.Fields(l)
	if len(flds) != 3 {
		err = errors.New("invalid checksum line: " + l)
		return
	}

	size, err = strconv.ParseUint(flds[1], 10, 64)
	if err != nil {
		return
	}
	csum, err = hex.DecodeString(flds[0])
	if err != nil {
		return
	}

	p = flds[2]
	return
}

// getFilesFromRelease parses Release or InRelease file and
// returns a list of *FileInfo pointed in the file.
func getFilesFromRelease(p string, r io.Reader) ([]*FileInfo, Paragraph, error) {
	dir := path.Dir(p)

	d, err := NewParser(r).Read()
	if err != nil {
		return nil, nil, errors.Wrap(err, "NewParser(r).Read()")
	}

	md5sums := d["MD5Sum"]
	sha1sums := d["SHA1"]
	sha256sums := d["SHA256"]

	if len(md5sums) == 0 && len(sha1sums) == 0 && len(sha256sums) == 0 {
		return nil, d, nil
	}

	m := make(map[string]*FileInfo)

	for _, l := range md5sums {
		p, size, csum, err := parseChecksum(l)
		p = path.Join(dir, path.Clean(p))
		if err != nil {
			return nil, nil, errors.Wrap(err, "parseChecksum for md5sums")
		}

		fi := &FileInfo{
			path:   p,
			size:   size,
			md5sum: csum,
		}
		m[p] = fi
	}

	for _, l := range sha1sums {
		p, size, csum, err := parseChecksum(l)
		p = path.Join(dir, path.Clean(p))
		if err != nil {
			return nil, nil, errors.Wrap(err, "parseChecksum for sha1sums")
		}

		fi, ok := m[p]
		if ok {
			fi.sha1sum = csum
		} else {
			fi := &FileInfo{
				path:    p,
				size:    size,
				sha1sum: csum,
			}
			m[p] = fi
		}
	}

	for _, l := range sha256sums {
		p, size, csum, err := parseChecksum(l)
		p = path.Join(dir, path.Clean(p))
		if err != nil {
			return nil, nil, errors.Wrap(err, "parseChecksum for sha256sums")
		}

		fi, ok := m[p]
		if ok {
			fi.sha256sum = csum
		} else {
			fi := &FileInfo{
				path:      p,
				size:      size,
				sha256sum: csum,
			}
			m[p] = fi
		}
	}

	// WORKAROUND: some (e.g. dell) repositories have invalid Release
	// that contains wrong checksum for Release itself.  Ignore them.
	delete(m, path.Join(dir, "Release"))
	delete(m, path.Join(dir, "Release.gpg"))
	delete(m, path.Join(dir, "InRelease"))

	l := make([]*FileInfo, 0, len(m))
	for _, fi := range m {
		l = append(l, fi)
	}
	return l, d, nil
}

// getFilesFromPackages parses Packages file and returns
// a list of *FileInfo pointed in the file.
func getFilesFromPackages(p string, r io.Reader) ([]*FileInfo, Paragraph, error) {
	var l []*FileInfo
	parser := NewParser(r)

	for {
		d, err := parser.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, errors.Wrap(err, "parser.Read")
		}

		filename, ok := d["Filename"]
		if !ok {
			return nil, nil, errors.New("no Filename in " + p)
		}
		fpath := path.Clean(filename[0])

		strsize, ok := d["Size"]
		if !ok {
			return nil, nil, errors.New("no Size in " + p)
		}
		size, err := strconv.ParseUint(strsize[0], 10, 64)
		if err != nil {
			return nil, nil, err
		}

		fi := &FileInfo{
			path: fpath,
			size: size,
		}
		if csum, ok := d["MD5sum"]; ok {
			b, err := hex.DecodeString(csum[0])
			if err != nil {
				return nil, nil, err
			}
			fi.md5sum = b
		}
		if csum, ok := d["SHA1"]; ok {
			b, err := hex.DecodeString(csum[0])
			if err != nil {
				return nil, nil, err
			}
			fi.sha1sum = b
		}
		if csum, ok := d["SHA256"]; ok {
			b, err := hex.DecodeString(csum[0])
			if err != nil {
				return nil, nil, err
			}
			fi.sha256sum = b
		}
		l = append(l, fi)
	}

	return l, nil, nil
}

// getFilesFromSources parses Sources file and returns
// a list of *FileInfo pointed in the file.
func getFilesFromSources(p string, r io.Reader) ([]*FileInfo, Paragraph, error) {
	var l []*FileInfo
	parser := NewParser(r)

	for {
		d, err := parser.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, errors.Wrap(err, "parser.Read")
		}

		dir, ok := d["Directory"]
		if !ok {
			return nil, nil, errors.New("no Directory in " + p)
		}

		m := make(map[string]*FileInfo)

		for _, l := range d["Files"] {
			fname, size, csum, err := parseChecksum(l)
			if err != nil {
				return nil, nil, errors.Wrap(err, "parseChecksum for Files")
			}

			fpath := path.Clean(path.Join(dir[0], fname))
			m[fpath] = &FileInfo{
				path:   fpath,
				size:   size,
				md5sum: csum,
			}
		}

		for _, l := range d["Checksums-Sha1"] {
			fname, size, csum, err := parseChecksum(l)
			if err != nil {
				return nil, nil, errors.Wrap(err, "parseChecksum for Checksums-Sha1")
			}

			fpath := path.Clean(path.Join(dir[0], fname))
			if _, ok := m[fpath]; ok {
				m[fpath].sha1sum = csum
			} else {
				m[fpath] = &FileInfo{
					path:    fpath,
					size:    size,
					sha1sum: csum,
				}
			}
		}

		for _, l := range d["Checksums-Sha256"] {
			fname, size, csum, err := parseChecksum(l)
			if err != nil {
				return nil, nil, errors.Wrap(err, "parseChecksum for Checksums-Sha256")
			}

			fpath := path.Clean(path.Join(dir[0], fname))
			if _, ok := m[fpath]; ok {
				m[fpath].sha256sum = csum
			} else {
				m[fpath] = &FileInfo{
					path:      fpath,
					size:      size,
					sha256sum: csum,
				}
			}
		}

		for _, fi := range m {
			if len(fi.md5sum) == 0 && len(fi.sha1sum) == 0 && len(fi.sha256sum) == 0 {
				return nil, nil, errors.New("no checksum in " + fi.path)
			}
			l = append(l, fi)
		}
	}

	return l, nil, nil
}

// getFilesFromIndex parses i18n/Index file and returns
// a list of *FileInfo pointed in the file.
func getFilesFromIndex(p string, r io.Reader) ([]*FileInfo, Paragraph, error) {
	return getFilesFromRelease(p, r)
}

// ExtractFileInfo parses debian repository index files such as
// Release, Packages, or Sources and return a list of *FileInfo
// listed in the file.
//
// If the index is Release, InRelease, or Index, this function
// also returns non-nil Paragraph data of the index.
//
// p is the relative path of the file.
func ExtractFileInfo(p string, r io.Reader) ([]*FileInfo, Paragraph, error) {
	if !IsMeta(p) {
		return nil, nil, errors.New("not a meta data file: " + p)
	}

	base := path.Base(p)
	ext := path.Ext(base)
	switch ext {
	case "", ".gpg":
		// do nothing
	case ".gz":
		gz, err := gzip.NewReader(r)
		if err != nil {
			return nil, nil, err
		}
		defer gz.Close()
		r = gz
		base = base[:len(base)-3]
	case ".bz2":
		r = bzip2.NewReader(r)
		base = base[:len(base)-4]
	case ".xz":
		xzr, err := xz.NewReader(r)
		if err != nil {
			return nil, nil, err
		}
		r = xzr
		base = base[:len(base)-3]
	default:
		return nil, nil, errors.New("unsupported file extension: " + ext)
	}

	switch base {
	case "Release", "InRelease":
		return getFilesFromRelease(p, r)
	case "Packages":
		return getFilesFromPackages(p, r)
	case "Sources":
		return getFilesFromSources(p, r)
	case "Index":
		return getFilesFromIndex(p, r)
	}
	return nil, nil, nil
}
