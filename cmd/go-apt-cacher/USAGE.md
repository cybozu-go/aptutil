How to configure and run go-apt-cacher
======================================

Configuration
-------------

go-apt-cacher reads a configuration file at start up.  If you change
the configuration file, the change will not take effect until you
restart go-apt-cacher.

The default location of the file is `/etc/go-apt-cacher.toml`.

A sample [TOML][] file is available [here](go-apt-cacher.toml).

Directories
-----------

go-apt-cacher writes to two directories (`meta_dir` and `cache_dir`
specified in the configuration file).  These directories must be
writable by the process owner of go-apt-cacher.

Running
-------

As go programs cannot run as so-called _daemon_, running go-apt-cacher
in background shall be done via process manager such as [systemd][] or
[upstart][].

Increase `nofile` resource limit to accept massive number of clients.
For systemd, it is [`LimitNOFILE` directive](http://serverfault.com/a/678861/126630).

go-apt-cacher does not require root privileges.  Users are strongly
advised to run go-apt-cacher with a non-root account.

Options
-------

| Option | Default | Description |
| ------ | ------- | ----------- |
| `-f`   | `/etc/go-apt-cacher.toml` | Configuration file path. |
| `-s`   | `:3142` | Listen address. |
| `-l`   | `info`  | Log level [`critical|error|warning|info|debug`] |

/etc/apt/sources.list
---------------------

For example, if `ubuntu` is mapped to `http://us.archive.ubuntu.com/ubuntu`
and `security` to `http://security.ubuntu.com/ubuntu`, you may edit
`/etc/apt/sources.list` as follows to make it use go-apt-cacher:

```
deb http://<go-apt-cacher hostname>/ubuntu trusty main restricted
deb http://<go-apt-cacher hostname>/ubuntu trusty-updates main restricted
deb http://<go-apt-cacher hostname>/security trusty-security main restricted
```

[TOML]: https://github.com/toml-lang/toml
[systemd]: https://www.freedesktop.org/wiki/Software/systemd/
[upstart]: http://upstart.ubuntu.com/
