Design Notes
============

go-apt-cacher is a reverse HTTP proxy built specifically for Debian (APT)
repositories.  This document describes its design and internals briefly.

Mapping
-------

For each APT repository URL, an identifying string is defined.
The string is used as the prefix of URL path of go-apt-cacher.

For example, if "http://archive.ubuntu.com/ubuntu" is mapped to "**ubuntu**",
clients should specify "http://<go-apt-cacher IP or FQDN>/**ubuntu**" in
`sources.list` file.

Internally, the prefix is used as a directory name in the local
file system cache.

Caching strategy
----------------

go-apt-cacher determines validity of cached contents by checksums provided
by meta data files such as `Release` or `Packages`.  For details about
meta data files and debian repository formats, see [RepositoryFormat][].

Meta data files will not be removed from the cache once they are cached.
To update them, go-apt-cacher periodically checks updates for `Release`
and `InRelease` who contain checksums for other meta data files such as
`Packages` and `Sources`.  If any checksums are changed, the caches for
them are effectively invalidated.

Caches for non-meta data files may be removed in LRU fashion when the
total size of cached files exceeds the given capacity.

Note that go-apt-cacher does _not_ reference cache-related HTTP headers
such as "Last-Modified" or "Cache-Control" at all.

HTTP methods
------------

go-apt-cacher accepts only GET and HEAD methods.
For other methods, it returns HTTP 501 Not Implemented response.

Lock order
----------

Internally, go-apt-cacher have these locks.
The lock listed first has higher order than locks listed under it.

1. `Cacher.fiLock`

    This lock is to protect file information cached in Cacher.

2. `Cacher.dlLock`

    This lock is to protect download channels and cached response statuses.
    Strictly, this lock is used independently from other locks.

3. `Storage.mu`

    This lock is to protect internal data in Storage.

Recovery
--------

go-apt-cacher keeps checksums in memory.  If go-apt-cacher process restarts,
the information need to be recovered.  To do it, go-apt-cacher scans all
cached meta data files and finds checksums before accepting requests.

[RepositoryFormat]: https://wiki.debian.org/RepositoryFormat

Compression support
-------------------

As Go does not provide the standard way to decompress .xz files,
go-apt-cacher ignores requests for .xz and returns 404 Not Found response.

Other optional compression algorithms such as .lzma or .lz are handled
the same as .xz.
