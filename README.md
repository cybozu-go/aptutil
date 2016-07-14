**go-apt-cacher** is a caching reverse proxy built specially for Debian (APT) repositories.  
This repository also contains a mirroring utility **go-apt-mirror**.

[![Build Status](https://travis-ci.org/cybozu-go/aptutil.svg?branch=master)](https://travis-ci.org/cybozu-go/aptutil)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)][MIT]
[![GoDoc](https://godoc.org/github.com/cybozu-go/aptutil?status.svg)](https://godoc.org/github.com/cybozu-go/aptutil)

Features
--------

### go-apt-cacher

* Automatic checksum validation for cached files  
    Cached files will **never** be broken!
* Reverse proxy for http and https repositories
* LRU-based cache eviction
* Smart caching strategy specialized for APT

### go-apt-mirror

* Atomic update of mirrors  
    Clients will never see incomplete mirrors.
* Checksum validation of mirrored files

Build
-----

Use Go 1.6 or better.

```
go get -u github.com/cybozu-go/aptutil
go install github.com/cybozu-go/aptutil/...
```

Usage
-----

* [go-apt-cacher](cmd/go-apt-cacher/USAGE.md)
* [go-apt-mirror](cmd/go-apt-mirror/USAGE.md)

License
-------

[MIT][]

[MIT]: https://opensource.org/licenses/MIT
