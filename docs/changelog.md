# Changelog

## v1.2.3 (2026-06-06)

- Remove `requireRoot()` from `pidex run` — daemon runs as `pidex` user via systemd
- Fix `ensureGroups()` to add groups individually (comma list fails if any group missing)
- Fix install.sh group assignment: add `systemd-journal`, `adm`, `docker` individually
- `systemd-journal` group is required for journalctl access on Debian/Pi OS

## v1.2.2 (2026-06-06)

- Add `ensureGroups()` to `pidex update` — auto-repairs `adm`/`docker` group membership
- Install script adds `pidex` user to `adm` and `docker` groups

## v1.2.1 (2026-06-06)

- Fix cross-device rename on `pidex update` (temp file to `/usr/local/bin`)
- Bump `core.Version` to 1.2.1

## v1.2.0 (2026-06-06)

- Add `pidex update` command for self-updates via GitHub releases
- Rewrite install.sh to prefer pre-built binaries, fall back to Go build
- Simplify README install instructions

## v1.1.1 (2026-06-03)

- Add `requireRoot()` check to `run`, `setup`, `uninstall`, `test` (not dry-run)
- Simplify install.sh back to plain `command -v go` check

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
