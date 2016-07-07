package cacher

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/pkg/errors"
)

var (
	validPrefix = regexp.MustCompile(`^[a-z0-9._-]+$`)

	// ErrInvalidPrefix returned for invalid prefix.
	ErrInvalidPrefix = errors.New("invalid prefix")
)

// URLMap is a mapping between prefix and debian repository URL.
//
// To create an instance, use make(URLMap).
type URLMap map[string]*url.URL

// Register registeres a prefix for a remote URL.
func (um *URLMap) Register(prefix string, u *url.URL) error {
	if !validPrefix.MatchString(prefix) {
		return ErrInvalidPrefix
	}

	// for URL.ResolveReference
	if !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
		u.RawPath += "/"
	}

	(*um)[prefix] = u
	return nil
}

// URL returns remote URL corresponding to a local path.
//
// Preceding slashes in slash are ignored.
// For example, if p "/abc/def" is the same as "abc/def".
//
// If p does not starts with a registered prefix, nil is returned.
func (um URLMap) URL(p string) *url.URL {
	for len(p) > 0 && p[0] == '/' {
		p = p[1:]
	}
	t := strings.SplitN(p, "/", 2)
	prefix := t[0]
	u, ok := um[prefix]
	if !ok {
		return nil
	}

	if len(t) == 1 {
		return u
	}
	return u.ResolveReference(&url.URL{Path: t[1]})
}
