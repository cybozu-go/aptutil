How to configure and run go-apt-mirror
======================================

Synopsis
--------

```
go-apt-mirror [options] [MIRROR MIRROR2...]
```

go-apt-mirror is a console application.  
Run it in your shell, or use `sudo -u USER` to run it as USER.

If `MIRROR` arguments are given, go-apt-mirror updates only the specified
Debian repository mirrors.  With no arguments, it updates all mirrors
defined in the configuration file.

Configuration
-------------

go-apt-mirror reads configurations from a [TOML][] file.  
The default location is `/etc/apt/mirror.toml`.

A sample configuration file is available [here](mirror.toml).

Proxy
-----

go-apt-mirror uses HTTP proxy as specified in [`ProxyFromEnvironment`](https://golang.org/pkg/net/http/#ProxyFromEnvironment).

Options
-------

| Option | Default | Description |
| ------ | ------- | ----------- |
| `-f`   | `/etc/apt/mirror.toml` | Configurations |
| `-l`   | `info`  | Log level [`critical|error|warning|info|debug`] |


[TOML]: https://github.com/toml-lang/toml
