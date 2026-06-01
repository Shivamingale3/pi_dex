# Development Changelog

## 2026-06-01 — Fixes after first run

- Added `docker>=7.0` to `requirements.txt` and `pyproject.toml` (was missing)
- Improved error messages in `docker.py` — distinguish "SDK not installed" vs
  "daemon unreachable"
- Improved error message in `journal.py` — tells user `apt install python3-systemd`
- Fixed `timestamp` type mismatch: all `Event.timestamp` values now properly
  use `datetime` objects (was passing `time.time()` floats in several places,
  causing `AttributeError: 'float' object has no attribute 'strftime'`)
- Updated `deploy/install.sh` to install `python3-systemd` via apt

## 2026-06-01 — Phase 1: Core + Telegram

- Created project structure (pidex/ package layout)
- Defined `Event` dataclass in `core/event.py`
- Implemented `EventBus` in `core/bus.py` (thread-safe queue wrapper)
- Implemented `Dispatcher` in `core/dispatcher.py`
- Implemented `CooldownManager` in `core/cooldowns.py`
- Implemented `DedupManager` in `core/dedup.py`
- Defined `BaseNotifier` ABC in `notifiers/base.py`
- Implemented `TelegramNotifier` in `notifiers/telegram.py`
- Created `core/constants.py` with all default values
- Created `pidex.py` CLI entry point with `run`, `test`, `version` subcommands
- Wrote architecture doc, decision log, and changelog

## 2026-06-01 — Phase 2: Fake events + dry-run

- Built into `test` subcommand: generates synthetic events (ssh-login, ssh-fail,
  sudo-used, docker-down, reboot)
- `--dry-run` flag prints event details without sending to Telegram

## 2026-06-01 — Phase 3: Config system

- `config/loader.py` — TOML config loader with `load_config()` and `apply_config()`
- `config/loader.py` — `get_cooldown_overrides()`, `get_telegram_config()` helpers
- `config/config.toml` — User-facing configuration template
- `pidex.py` — Integrated config into `run` and `test` commands
- Config overrides `constants.py` values at startup via `apply_config()`

## 2026-06-01 — Phase 4: Event sources

- `sources/base.py` — `BaseSource` ABC with `start()` thread lifecycle
- `sources/journal.py` — Single journald listener thread, dispatches to parsers
- `sources/ssh.py` — SSH journal parser (login, logout, bruteforce detection)
- `sources/sudo.py` — Sudo journal parser
- `sources/systemd.py` — Systemd parser factory (service start/stop/fail/restart)
- `sources/docker.py` — Docker Events API listener (container lifecycle)
- `sources/network.py` — pyroute2 netlink listener (interface up/down)
- `sources/shutdown.py` — Shutdown event helper + standalone entry point
- All sources wired into `pidex run` with per-source toggle from config

## 2026-06-01 — Phase 5: Pollers

- `pollers/base.py` — `BasePoller` ABC with state machine (ok → warn → critical → recover)
- `pollers/cpu.py` — Reads `/proc/stat`, delta-based CPU usage calculation
- `pollers/ram.py` — Reads `/proc/meminfo` (MemTotal / MemAvailable)
- `pollers/disk.py` — Reads `os.statvfs()` for disk usage percentage
- `pollers/temperature.py` — Reads `/sys/class/thermal/thermal_zone0/temp`
- All pollers wired into `pidex run` with configurable intervals and thresholds

## 2026-06-01 — Phase 6: Packaging + Deployment

- `pyproject.toml` — Modern Python packaging with entry points
- `deploy/pidex.service` — systemd unit for the main daemon
- `deploy/pidex-shutdown.service` — systemd unit for shutdown notification
- `deploy/install.sh` — Automated installation script
- Package installs via `pip install -e .` or `pip install .`
- Console scripts: `pidex` (daemon CLI), `pidex-shutdown` (shutdown helper)
