package cacher

import "github.com/cybozu-go/well"

const (
	defaultAddress       = ":3142"
	defaultCheckInterval = 600
	defaultCachePeriod   = 3
	defaultCacheCapacity = 1
	defaultMaxConns      = 10
)

// Config is a struct to read TOML configurations.
//
// Use https://github.com/BurntSushi/toml as follows:
//
//    config := cacher.NewConfig()
//    md, err := toml.DecodeFile("/path/to/config.toml", config)
//    if err != nil {
//        ...
//    }
type Config struct {
	// Addr is the listening address of HTTP server.
	//
	// Default is ":3142".
	Addr string `toml:"listen_address"`

	// CheckInterval specifies interval in seconds to check updates for
	// Release/InRelease files.
	//
	// Default is 600 seconds.
	CheckInterval int `toml:"check_interval"`

	// CachePeriod specifies the period to cache bad HTTP response statuses.
	//
	// Default is 3 seconds.
	CachePeriod int `toml:"cache_period"`

	// MetaDirectory specifies a directory to store APT meta data files.
	//
	// This must differ from CacheDirectory.
	MetaDirectory string `toml:"meta_dir"`

	// CacheDirectory specifies a directory to cache non-meta data files.
	//
	// This must differ from MetaDirectory.
	CacheDirectory string `toml:"cache_dir"`

	// CacheCapacity specifies how many bytes can be stored in CacheDirectory.
	//
	// Unit is GiB.  Default is 1 GiB.
	CacheCapacity int `toml:"cache_capacity"`

	// MaxConns specifies the maximum concurrent connections to an
	// upstream host.
	//
	// Zero disables limit on the number of connections.
	MaxConns int `toml:"max_conns"`

	// Log is well.LogConfig
	Log well.LogConfig `toml:"log"`

	// Mapping specifies mapping between prefixes and APT URLs.
	Mapping map[string]string `toml:"mapping"`
}

// NewConfig creates Config with default values.
func NewConfig() *Config {
	return &Config{
		Addr:          defaultAddress,
		CheckInterval: defaultCheckInterval,
		CachePeriod:   defaultCachePeriod,
		CacheCapacity: defaultCacheCapacity,
		MaxConns:      defaultMaxConns,
	}
}
