[![GitHub release](https://img.shields.io/github/release/cybozu-go/aptutil.svg?maxAge=60)][releases]
[![GoDoc](https://godoc.org/github.com/cybozu-go/aptutil?status.svg)][godoc]
[![CircleCI](https://circleci.com/gh/cybozu-go/aptutil.svg?style=svg)](https://circleci.com/gh/cybozu-go/aptutil)
[![Go Report Card](https://goreportcard.com/badge/github.com/cybozu-go/aptutil)](https://goreportcard.com/report/github.com/cybozu-go/aptutil)

**go-apt-cacher** is a caching reverse proxy built specially for Debian (APT) repositories.  
This repository also contains a mirroring utility **go-apt-mirror**.

Blog: [Introducing go-apt-cacher and go-apt-mirror](http://ymmt2005.hatenablog.com/entry/2016/07/19/Introducing_go-apt-cacher_and_go-apt-mirror)

Status
--------
This project is currently in maintenance mode and is not expected to receive feature updates. Bugfixes and such will still be performed if needed.

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

Install
-------

Pre-built binaries are available on [releases][].

Unpack one and follow usage instructions.

Usage
-----

* [go-apt-cacher](cmd/go-apt-cacher/USAGE.md)
* [go-apt-mirror](cmd/go-apt-mirror/USAGE.md)

Deploy
------

Both `go-apt-cacher` and `go-apt-mirror` can be deployed via the [jacksgt/aptutil Docker Image](https://hub.docker.com/r/jacksgt/aptutil/). For more information head over to the [source repository](https://github.com/jacksgt/docker-aptutil).

Build
-----

Use an officially supported version of Go.

Run the command below exactly as shown, including the ellipsis.
They are significant - see `go help packages`.

```
go get -u github.com/cybozu-go/aptutil/...
```

License
-------

[MIT][]

Authors & Contributors
----------------------

* Yamamoto, Hirotaka [@ymmt2005](https://github.com/ymmt2005)
* Yutani, Hiroaki [@yutannihilation](https://github.com/yutannihilation)
* [@xipmix](https://github.com/xipmix)
* [@jacksgt](https://github.com/jacksgt)
* [@arnarg](https://github.com/arnarg)

[releases]: https://github.com/cybozu-go/aptutil/releases
[godoc]: https://godoc.org/github.com/cybozu-go/aptutil
[MIT]: https://opensource.org/licenses/MIT
