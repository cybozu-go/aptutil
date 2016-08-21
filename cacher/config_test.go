package cacher

import (
	"testing"

	"github.com/BurntSushi/toml"
)

func TestConfig(t *testing.T) {
	t.Parallel()

	config := NewConfig()
	md, err := toml.DecodeFile("t/cacher.toml", &config)
	if err != nil {
		t.Fatal(err)
	}

	if len(md.Undecoded()) > 0 {
		t.Errorf("%#v", md.Undecoded())
	}

	if config.CheckInterval != 10 {
		t.Error(`config.CheckInterval != 10`)
	}
	if config.CachePeriod != 5 {
		t.Error(`config.CachePeriod != 5`)
	}
	if config.MetaDirectory != "/tmp/meta" {
		t.Error(`config.MetaDirectory != "/tmp/meta"`)
	}
	if config.CacheDirectory != "/tmp/cache" {
		t.Error(`config.CacheDirectory != "/tmp/cache"`)
	}
	if config.CacheCapacity != 21 {
		t.Error(`config.CacheCapacity != 21`)
	}
	if config.MaxConns != defaultMaxConns {
		t.Error(`config.MaxConns != defaultMaxConns`)
	}

	if config.Log.Level != "error" {
		t.Error(`config.Log.Level != "error"`)
	}

	if config.Mapping["ubuntu"] != "http://archive.ubuntu.com/ubuntu" {
		t.Error(`config.Mapping["ubuntu"]`)
	}
	if config.Mapping["security"] != "http://security.ubuntu.com/ubuntu" {
		t.Error(`config.Mapping["security"]`)
	}
	if config.Mapping["dell"] != "http://linux.dell.com/repo/community/ubuntu" {
		t.Error(`config.Mapping["dell"]`)
	}
}
