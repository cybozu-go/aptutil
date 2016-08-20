package mirror

import (
	"errors"
	"net/url"
	"path"
	"strings"

	"github.com/cybozu-go/cmd"
)

const (
	defaultMaxConns = 10
)

type tomlURL struct {
	*url.URL
}

func (u *tomlURL) UnmarshalText(text []byte) error {
	tu, err := url.Parse(string(text))
	if err != nil {
		return err
	}
	switch tu.Scheme {
	case "http":
	case "https":
	default:
		return errors.New("unsupported scheme: " + tu.Scheme)
	}

	// for URL.ResolveReference
	if !strings.HasSuffix(tu.Path, "/") {
		tu.Path += "/"
		tu.RawPath += "/"
	}

	u.URL = tu
	return nil
}

// MirrConfig is an auxiliary struct for Config.
type MirrConfig struct {
	URL           tomlURL  `toml:"url"`
	Suites        []string `toml:"suites"`
	Sections      []string `toml:"sections"`
	Source        bool     `toml:"mirror_source"`
	Architectures []string `toml:"architectures"`
}

// isFlat returns true if suite ends with "/" as described in
// https://wiki.debian.org/RepositoryFormat#Flat_Repository_Format
func isFlat(suite string) bool {
	return strings.HasSuffix(suite, "/")
}

// Check vaildates the configuration.
func (mc *MirrConfig) Check() error {
	if len(mc.Suites) == 0 {
		return errors.New("no suites")
	}

	flat := isFlat(mc.Suites[0])
	if flat && len(mc.Sections) != 0 {
		return errors.New("flat repository cannot have sections")
	}
	if flat && len(mc.Architectures) != 0 {
		return errors.New("flat repository cannot have sections")
	}
	for _, suite := range mc.Suites[1:] {
		if flat != isFlat(suite) {
			return errors.New("mixed flat/non-flat in suites")
		}
	}

	return nil
}

// ReleaseFiles generates a list relative paths to "Release",
// "Release.gpg", or "InRelease" files.
func (mc *MirrConfig) ReleaseFiles() []string {
	var l []string

	for _, suite := range mc.Suites {
		relpath := suite
		if !isFlat(suite) {
			relpath = path.Join("dists", suite)
		}
		l = append(l, path.Clean(path.Join(relpath, "Release")))
		l = append(l, path.Clean(path.Join(relpath, "Release.gpg")))
		l = append(l, path.Clean(path.Join(relpath, "Release.gz")))
		l = append(l, path.Clean(path.Join(relpath, "Release.bz2")))
		l = append(l, path.Clean(path.Join(relpath, "Release.xz")))
		l = append(l, path.Clean(path.Join(relpath, "InRelease")))
		l = append(l, path.Clean(path.Join(relpath, "InRelease.gz")))
		l = append(l, path.Clean(path.Join(relpath, "InRelease.bz2")))
		l = append(l, path.Clean(path.Join(relpath, "InRelease.xz")))
	}

	return l
}

// Resolve returns *url.URL for a relative path.
func (mc *MirrConfig) Resolve(p string) *url.URL {
	return mc.URL.ResolveReference(&url.URL{Path: p})
}

func rawName(p string) string {
	base := path.Base(p)
	ext := path.Ext(base)
	return base[0 : len(base)-len(ext)]
}

// MatchingIndex returns true if mc is configured for the given index.
func (mc *MirrConfig) MatchingIndex(p string) bool {
	rn := rawName(p)

	if rn == "Index" || rn == "Release" {
		return true
	}

	if isFlat(mc.Suites[0]) {
		// scan Packages and Sources
		switch rn {
		case "Packages":
			return true
		case "Sources":
			return mc.Source
		}
		return false
	}

	pNoExt := p[0 : len(p)-len(path.Ext(p))]
	var archs []string
	archs = append(archs, "all")
	archs = append(archs, mc.Architectures...)
	for _, section := range mc.Sections {
		for _, arch := range archs {
			t := path.Join(path.Clean(section), "binary-"+arch, "Packages")
			if strings.HasSuffix(pNoExt, t) {
				return true
			}
		}
		if mc.Source {
			t := path.Join(path.Clean(section), "source", "Sources")
			if strings.HasSuffix(pNoExt, t) {
				return true
			}
		}
	}

	return false
}

// Config is a struct to read TOML configurations.
//
// Use https://github.com/BurntSushi/toml as follows:
//
//    config := mirror.NewConfig()
//    md, err := toml.DecodeFile("/path/to/config.toml", config)
//    if err != nil {
//        ...
//    }
type Config struct {
	Dir      string                 `toml:"dir"`
	MaxConns int                    `toml:"max_conns"`
	Log      cmd.LogConfig          `toml:"log"`
	Mirrors  map[string]*MirrConfig `toml:"mirror"`
}

// NewConfig creates Config with default values.
func NewConfig() *Config {
	return &Config{
		MaxConns: defaultMaxConns,
	}
}
