# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog], and this project adheres to
[Semantic Versioning].

<!-- references -->

[Keep a Changelog]: https://keepachangelog.com/en/1.0.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html

## Unreleased

### Added

- Added `Ingest()`, which reads events from an arbitrary channel and delivers
  them into the message bus.

## [0.3.0] - 2024-08-14

### Changed

- **[BC]** `Run()` now accepts a list of functions, instead of options.
- There is no longer any buffering on inbox channels.
- There is no longer a central "bus" channel. Instead, each function delivers
  the messages it publishes directly to the inbox channels of the functions that
  subscribe to them. This prevents deadlocks from unrelated functions saturating
  the buffers.

### Removed

- **[BC]** Removed `Option`, `WithFunc()` and `WithInboxSize()`.

## [0.2.1] - 2024-08-01

### Added

- `Subscribe()` now supports subscribing to interface types. The function will
  receive any messages that implement the interface.
- Added `runtime/trace` task and log annotations.

## [0.2.0] - 2024-07-31

This release abandons the "component" terminology and simply refers to the
functions executed by `Run()` as "functions".

### Changed

- **[BC]** Renamed `Start()` to `Ready()`
- **[BC]** Renamed `WithComponent()` to `WithFunc()`
- **[BC]** Renamed `WithBuffer()` to `WithInboxSize()`
- **[BC]** Renamed `RunOption` to `Option`

## [0.1.1] - 2024-07-30

### Fixed

- `Run()` now exits immediately when there are no components, instead of
  blocking until the context is canceled.
- Messages are no longer delivered back to the component that sent them.

## [0.1.0] - 2024-07-30

- Initial release

<!-- references -->

[Unreleased]: https://github.com/dogmatiq/minibus
[0.1.0]: https://github.com/dogmatiq/minibus/releases/tag/v0.1.0
[0.1.1]: https://github.com/dogmatiq/minibus/releases/tag/v0.1.1
[0.2.0]: https://github.com/dogmatiq/minibus/releases/tag/v0.2.0
[0.2.1]: https://github.com/dogmatiq/minibus/releases/tag/v0.2.1
[0.3.0]: https://github.com/dogmatiq/minibus/releases/tag/v0.3.0

<!-- version template
## [0.0.1] - YYYY-MM-DD

### Added
### Changed
### Deprecated
### Removed
### Fixed
### Security
-->
