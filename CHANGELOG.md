# Change Log

All notable changes to this project will be documented in this file.

## [Unreleased]

## [1.4.2] - 2020-12-23
### Changed
- Minor fixes
- Upgrade CI to go1.15

## [1.4.1] - 2018-11-16
### Changed
- Handle renaming of cybozu-go/cmd to [cybozu-go/well][well]
- Introduce support for Go modules

## [1.4.0] - 2018-03-02
### Changed
- No notable changes since RC1.

## [1.4.0rc1] - 2018-01-19
### Changed
- Do not consume too much memory when downloading large files (#14, #28).
- [mirror] could not find Release/InRelease if suites=["/"] (#25, #26).
- [cacher] create cache directories automatically (#29).  
  Contributed by @jacksgt.
- [cacher] prevent panic for URL whose path is a mapping prefix (#30).

## [1.3.2] - 2017-09-01
### Changed
- [mirror] file modes of by-hash indices were erroneously 0600.

## [1.3.1] - 2017-08-21
### Changed
- [mirror] failed to detect by-hash support in some cases (#21).

## [1.3.0] - 2017-08-02
### Added
- [mirror] support by-hash index acquisition (#15, #16).

### Changed
- [cacher] workaround for bad contents in Release (#13, #17).

## [1.2.2] - 2016-08-31
### Changed
- Check errors of wrong log configurations.

## [1.2.1] - 2016-08-24
### Changed
- Fix for the latest cybozu-go/cmd.

## [1.2.0] - 2016-08-21
### Added
- aptuitl now adopts [github.com/cybozu-go/cmd][cmd] framework.  
  As a result, commands implement [the common spec][spec].
- [cacher] added `listen_address` configuration parameter to specify listening address (#9).
- [cacher] added `log` configuration section to specify logging options.
- [mirror] added `log` configuration section to specify logging options.

### Changed
- aptutil now requires Go 1.7 or better.

### Removed
- [cacher] `-s` and `-l` command-line flags are gone.
- [mirror] `-s` command-line flag is gone.

## [1.1.0]
### Changed
- Update docs (kudos to @xipmix).
- [cacher] extend Release file check interval from 15 to 600 seconds (#8).

## [1.0.1]
### Changed
- [mirror] ignore Sources if `include_source` is not specified in mirror.toml.  
  This works as a workaround for some badly configured web servers.


[well]: https://github.com/cybozu-go/well
[cmd]: https://github.com/cybozu-go/cmd
[spec]: https://github.com/cybozu-go/cmd/blob/master/README.md#specifications
[Unreleased]: https://github.com/cybozu-go/aptutil/compare/v1.4.2...HEAD
[1.4.2]: https://github.com/cybozu-go/aptutil/compare/v1.4.1...v1.4.2
[1.4.1]: https://github.com/cybozu-go/aptutil/compare/v1.4.0...v1.4.1
[1.4.0]: https://github.com/cybozu-go/aptutil/compare/v1.4.0rc1...v1.4.0
[1.4.0rc1]: https://github.com/cybozu-go/aptutil/compare/v1.3.2...v1.4.0rc1
[1.3.2]: https://github.com/cybozu-go/aptutil/compare/v1.3.1...v1.3.2
[1.3.1]: https://github.com/cybozu-go/aptutil/compare/v1.3.0...v1.3.1
[1.3.0]: https://github.com/cybozu-go/aptutil/compare/v1.2.2...v1.3.0
[1.2.2]: https://github.com/cybozu-go/aptutil/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/cybozu-go/aptutil/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/cybozu-go/aptutil/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/cybozu-go/aptutil/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/cybozu-go/aptutil/compare/v1.0.0...v1.0.1
