How to configure and run go-apt-mirror
======================================

Synopsis
--------

```
go-apt-mirror [options]
```

go-apt-mirror is a console application.  
Run it in your shell, or use `sudo -u USER` to run it as USER.

Configuration
-------------

go-apt-mirror reads configurations from a [TOML][] file.  
The default location is `/etc/apt/mirror.toml`.

A sample configuration file is available [here](mirror.toml).

Options
-------

| Option | Default | Description |
| ------ | ------- | ----------- |
| `-f`   | `/etc/apt/mirror.toml` | Configurations |
| `-l`   | `info`  | Log level [`critical|error|warning|info|debug`] |


[TOML]: https://github.com/toml-lang/toml
