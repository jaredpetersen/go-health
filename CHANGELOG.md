# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2021-10-14
### Added
- Stable release.

## [0.1.0] - 2021-10-14
### Added
- Godoc examples.
- Static code analysis with [staticcheck](https://staticcheck.io/).

### Changed
- `Monitor()` function on `Monitor` now takes variadic `Check` arguments, allowing you to pass multiple checks in at
once. This does not break backwards compatability.

### Fixed
- README example that didn't compile.

## [0.0.0] - 2021-10-13
### Added
- Initial development release.
