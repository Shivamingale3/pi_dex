# Changelog

## v2.0.0 (2026-10-28)

Boot independence
- `pidex.service` is ordered `After=network.target` only; no dependency on
  `network-online.target`, `tailscaled.service` or `docker.service`
- Removed the DNS wait before `DAEMON_START`; startup events go through the offline queue
- `pidex update` installs the current systemd units and runs `systemctl daemon-reload`
- `install.sh` no longer overwrites an existing `/etc/pidex/config.toml`

Persistent offline queue
- Undeliverable notifications are stored in `/var/lib/pidex/queue.jsonl` (atomic rewrite)
- Flushed oldest first every 30s, stopping at the first failure; new events queue behind older ones
- Delivered late messages carry `Delayed: originally at <time>`
- Capped at 500 entries (oldest dropped); a single `N alerts dropped while offline` summary is sent
- Survives restarts and reboots; the shutdown event is persisted when offline
- Messages Telegram rejects outright (HTTP 400/413) are not queued; HTTP 429 honours `retry_after`

Systemd watch fix
- `[services] watch` entries without `.service` (e.g. `"cloudflared"`) now match; globs still work
- Unit names come from the journal `UNIT` field when present (handles Description-only messages)
- New SERVICE_FAILED triggers: `<unit>: Failed with result '...'` and `Dependency failed for ...`

Health checks (new)
- `[[health_checks]]` of type http, tcp, redis, systemd, docker, command, interface
- DOWN alert after N consecutive failures, RECOVERED with downtime; no repeats
- Diagnosis in the alert: raw error, unit state, container state, interface/IP presence,
  which address the port is actually listening on, and the last 10 log lines
- `pidex setup` -> 11. Manage health checks (list, add, edit, delete, test now)
- `pidex test --health <name>` runs a check once and prints the diagnosis (no Telegram)

Other
- Cooldowns are tracked per source and event type, so checks do not suppress each other
- Telegram messages are truncated to fit the 4096 character limit
- New config files are created with mode 0640; installer sets `config.toml` to `root:pidex 0640`
- `config/config.toml` is no longer tracked by git (only `config.toml.example`)
- First unit tests (`go test ./...`)

## v1.2.5 (2026-06-06)

- Fix SSH parser: accept `sshd-session` in addition to `sshd`
- Fix `isRelevant()` filter to let `sshd-session` through
- Add DNS readiness wait before publishing DAEMON_START

## v1.2.4 (2026-06-06)

- Fix `LoadConfig` to check env vars even when config.toml is unreadable
- Fix file permissions: install.sh sets `root:pidex` ownership on env and config.toml
- `ensureGroups()` now repairs env/config.toml permissions on update

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
