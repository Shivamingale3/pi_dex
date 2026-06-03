# Changelog

## v1.1.0 (2026-06-03)

- Rename `SeverityWarn` from `WARN` to `WARNING`
- Replace hardcoded severity strings with `core.Severity*` constants
- Redesign Telegram notification format with consistent template and hostname
- Include source/poller counts in daemon start message

## v0.2.0 (2026-06-03)

- Complete migration from Python to Go
- Remove all Python source files and artifacts
- Add `pidex setup` interactive wizard (9 options)
- Add `pidex test <event>` with 5 event types + `--dry-run`
- Add `pidex uninstall` command
- Add `pidex help` command
- Add `ConfigToMap()` and `SaveConfig()` for writing TOML
- Update CI workflows (`test.yml`, `release.yml`) for Go
- All builds pass `go build ./... && go vet ./...`
- Fix sudo parser regex for `user : TTY=... ; COMMAND=...` format
- Fix dispatcher drain loop: replace `QSize()` polling with `sync.WaitGroup`
- Deploy and test: SSH, sudo, CPU, shutdown notifications confirmed working

## v0.1.0 — Python (2026-06-01)

- Initial Python implementation
- Core abstractions: Event, EventBus, Dispatcher, Cooldowns, Dedup
- Telegram notification backend
- Journald source with SSH, sudo, systemd parsers
- Docker events, network netlink sources
- CPU, RAM, disk, temperature pollers
- TOML configuration system
- Systemd service files and install script
