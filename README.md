[![GitHub release](https://img.shields.io/github/release/cybozu-go/aptutil.svg?maxAge=2592000)](https://github.com/cybozu-go/aptutil/releases)
[![GoDoc](https://godoc.org/github.com/cybozu-go/aptutil?status.svg)][godoc]
[![Build Status](https://travis-ci.org/cybozu-go/aptutil.svg?branch=master)](https://travis-ci.org/cybozu-go/aptutil)
[![Go Report Card](https://goreportcard.com/badge/github.com/cybozu-go/aptutil)](https://goreportcard.com/report/github.com/cybozu-go/aptutil)
[![License](https://img.shields.io/github/license/cybozu-go/aptutil.svg?maxAge=2592000)](LICENSE)

**go-apt-cacher** is a caching reverse proxy built specially for Debian (APT) repositories.  
This repository also contains a mirroring utility **go-apt-mirror**.

Blog: [Introducing go-apt-cacher and go-apt-mirror](http://ymmt2005.hatenablog.com/entry/2016/07/19/Introducing_go-apt-cacher_and_go-apt-mirror)

Features
--------

### go-apt-cacher

* Checksum awareness  
  go-apt-cacher recognizes APT indices and checks downloaded files automatically.
* Reverse proxy for http and https repositories
* LRU-based cache eviction
* Smart caching strategy specialized for APT

### go-apt-mirror

* Atomic update of mirrors  
    Clients will never see incomplete/inconsistent mirrors.
* Checksum validation of mirrored files
* Ultra fast update compared to rsync
* Parallel download
* Partial mirror

Build
-----

Use Go 1.6 or better.

```
go get -u github.com/cybozu-go/aptutil/...
```

Usage
-----

* [go-apt-cacher](cmd/go-apt-cacher/USAGE.md)
* [go-apt-mirror](cmd/go-apt-mirror/USAGE.md)

License
-------

[MIT][]

[MIT]: https://opensource.org/licenses/MIT
[godoc]: https://godoc.org/github.com/cybozu-go/aptutil
