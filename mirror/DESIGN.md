Design notes
============

Atomicity
---------

To update mirrors atomically, mirror directories are pointed by
symlinks, as symlinks can be replaced atomically with rename(2).

Directory structure
-------------------

go-apt-mirror stores all data under a directory specified in the
configuration file.  Under the directory, data are structured as:

```
(root)
    +- .lock              Lock file to prevent running multiple go-apt-mirror.
    +- MIRROR             Symlink to .MIRROR.DATETIME/MIRROR directory.
    +- .MIRROR.DATETIME
        +- info.json      Checksum information.
        +- MIRROR         Directory for MIRROR.
    +- MIRROR2            Symlink to .MIRROR2.DATETIME/MIRROR2 directory.
    +- .MIRROR2.DATETIME
        +- info.json      Checksum information.
        +- MIRROR2        Directory for MIRROR2.
    ...
```

where MIRROR and MIRROR2 are identifiers for each mirror defined
in the configuration file.  DATETIME is the timestamp when go-apt-mirror
starts mirroring.

Checksum verification
---------------------

go-apt-mirror validates downloaded item with checksums provided by
APT indices such as `Release` or `Packages.gz`.

Reusing items
-------------

go-apt-mirror reuses previously downloaded items if they are unchanged.
In order to check items quickly, go-apt-mirror keeps checksums in
`info.json` file.
