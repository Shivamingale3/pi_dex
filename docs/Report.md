# PiDex Code Review Report

**Project:** PiDex v0.1.0 — Event-driven home server watchman  
**Language:** Python 3.11+  
**Total Source Lines:** ~1,330 across 22 substantive files  
**Review Date:** 2026-06-01  
**Reviewer:** Automated code review

---

## 1. Architecture

### Overview

PiDex is an event-driven home server watchman for Raspberry Pi / Linux. It listens for OS-level events (SSH logins, Docker container changes, systemd service failures, network interface changes, resource thresholds) and forwards them as Telegram notifications via a Bot API.

### Data Flow

```
Linux / Docker / Systemd / Kernel
                │
                ▼
        Event Sources (10 threads)
   ┌──────┬──────┬──────┬──────┬──────┬──────┬──────┬──────┐
  SSH  sudo  systemd Docker Network Shutdown CPU   RAM   Temp/Disk
  (journald parsers)
                │
                ▼
          Event Bus (queue.Queue)
                │
                ▼
          Dispatcher (1 thread)
        (cooldown → dedup → notify)
                │
                ▼
       TelegramNotifier (HTTP POST)
```

### Concurrency Model

- **Threading** (not asyncio) — each source and poller runs in its own daemon thread
- All threads write `Event` objects into a shared `queue.Queue` (no locks needed)
- A single `Dispatcher` thread reads from the queue, applies cooldowns & dedup, then notifies
- Thread count: ~10 daemon threads at peak (1 main, 1 dispatcher, 1 journal, 1 Docker, 1 network, 4 pollers, optionally 1 shutdown)

### Strengths

- **Clean layered design**: sources → bus → dispatcher → notifier with clear separation of concerns
- **Thread-safe by design**: single-writer pattern for sources (write-only to queue), single-reader for dispatcher (reads from queue). No shared mutable state accessed concurrently
- **Graceful degradation**: all third-party dependencies (docker SDK, pyroute2, systemd.journal) are optional — if unavailable, the corresponding source logs an error and the daemon continues
- **ABCs/interfaces**: `BaseSource`, `BasePoller`, `BaseNotifier` provide clear contracts for extension

### Weaknesses

- No `stop()` / `join()` on `BaseSource` — threads cannot be cleanly stopped
- `ShutdownSource` class is defined but **never instantiated** anywhere (dead code at `shutdown.py:13-33`)
- Configuration via mutation of global module constants (`loader.py:apply_config`) rather than dependency injection — brittle and untestable
- No graceful shutdown of threads (they are daemon threads, terminated abruptly)

---

## 2. Code Cleanliness

### What's Good

- **Type hints** used consistently across the entire codebase (Python 3.11+ style with `str | None`)
- **Constants centralized** in `pidex/core/constants.py` — no magic numbers scattered
- **Small files** — average ~40 LOC per file, single responsibility
- **Logging** is consistent (structured logger names, proper levels)
- **Naming conventions** are clear and consistent (snake_case for functions, PascalCase for classes)
- **No commented-out code** or dead imports

### What Needs Improvement

- **Zero tests** — the `tests/` directory contains only an empty `__init__.py`. This is the single biggest quality issue
- **`apply_config()`** mutates global constants at runtime — a side-effect that makes testing and debugging harder
- **`ssh.py` uses module-level mutable state** (`_bruteforce_tracker` dict) that accumulates IPs indefinitely
- **Inconsistent import style**: `docker.py` imports function inside method for graceful degradation, but `systemd.py` and `network.py` use different patterns
- **`_ts()` helper** is duplicated across `ssh.py`, `sudo.py`, `systemd.py` — should be a shared utility
- **Empty `__init__.py` files** everywhere — fine for namespace packages, but no package-level exports
- **Event timestamps**: `docker.py:94` uses `datetime.now()` instead of extracting the timestamp from Docker event payload, losing original event timing
- **No config validation** — if user sets a string where number is expected, downstream crashes at runtime
- **`fnmatch` vs exact match inconsistency**: systemd source uses glob patterns (`fnmatch`), Docker source uses exact string matching (`name not in watch`)

---

## 3. Resource Consumption

### Memory

| Component | Estimated Memory |
|-----------|-----------------|
| Python interpreter | ~5 MB |
| Imports + code | ~3 MB |
| Event queue | ~0 (empty) to ~few KB (backed up) |
| Journal subprocess buffer | Up to 65 KB per read |
| Bruteforce tracker (ssh.py) | Grows unboundedly with unique IPs |
| Cooldown/dedup state | Negligible (<1 KB) |
| **Total estimate (idle)** | **~10–15 MB** |

Well within the stated target of <50 MB.

### CPU

- Almost all threads spend most of their time blocking on I/O (`queue.get()`, `journal.wait()`, Docker socket, netlink, `stop_event.wait()`)
- Poller threads wake at their intervals (CPU: 15s, RAM: 30s, Temp: 30s, Disk: 300s) for ~1ms reads
- **Target of ~0% at idle is achievable**

### Network

- Only active during Telegram notification sends (HTTP POST to `api.telegram.org`)
- Journal subprocess and Docker communicate over local sockets only

### Concerns

- **`_bruteforce_tracker` memory leak** in `ssh.py:68-77`: IP keys in the module-level dict are never removed. After a large-scale attack with many unique IPs, memory grows unbounded
- **`_SubprocessWrapper._buf`** in `journal.py:108`: bytearray buffer could grow if a partial line never terminates (e.g., journalctl produces corrupt output). No maximum size enforcement
- **No backpressure mechanism** — poller threads continuously check regardless of system load

---

## 4. Optimization

### Low-Hanging Fruit

| Issue | Impact | Effort |
|-------|--------|--------|
| Dedup checked after cooldown — events during cooldown are dropped anyway | Low | Trivial (reorder) |
| `_is_bruteforce` list comprehension recreates full list each call | Medium | Trivial (use deque) |
| `_SubprocessWrapper.__next__` loops on partial line | Low | Low |
| `RamPoller._read_meminfo` iterates entire meminfo file (~50 lines) | Negligible | Low (early break) |
| `shutdown.py`: `stop_event.wait(0.5)` tight loop | Low | Trivial (no timeout) |

### Architectural Optimizations

- **Pre-filter journal entries** in `_run_loop` by checking `_COMM` before dispatch to parsers — currently every journal entry is sent to every parser
- **Use `collections.deque`** for `_bruteforce_tracker` values instead of list + list comprehension
- **Connection pooling** already used (requests.Session) — good
- **No unnecessary logging** in hot paths — good

### Not Worth Optimizing

- The daemon is I/O-bound and spends >99.9% of time sleeping. Micro-optimizations in pollers are irrelevant
- Python threading overhead for ~10 threads is negligible on any modern Linux system

---

## 5. Bugs

### Critical

| # | File:Line | Description |
|---|-----------|-------------|
| 1 | `ssh.py:68-77` | **Bruteforce tracker never prunes IP keys** — module-level `_bruteforce_tracker` dict grows unboundedly. Under sustained attack from many IPs, memory grows without bound |
| 2 | `disk.py:16` | **Uses `f_bfree` instead of `f_bavail`** — counts root-reserved blocks (typically 5%) as used. Disk usage reported is inflated by ~5%, causing false warnings |

### Moderate

| # | File:Line | Description |
|---|-----------|-------------|
| 3 | `shutdown.py:13-33` | **`ShutdownSource` class is dead code** — defined but never instantiated. Shutdown event in `cli.py:133` is published directly via `bus.publish()` |
| 4 | `telegram.py:28` | **No retry logic** — Telegram API 5xx/429 responses cause event loss. Exception propagates to dispatcher which logs and continues |
| 5 | `telegram.py:24,44-50` | **HTML parse mode without escaping** — event messages containing `<`, `>`, `&` will be interpreted as HTML, potentially breaking message rendering |
| 6 | `docker.py:94` | **Timestamps always `datetime.now()`** — ignores Docker event timestamps. Events reflect when PiDex processed them, not when Docker generated them |

### Minor

| # | File:Line | Description |
|---|-----------|-------------|
| 7 | `sudo.py:12` | **Fragile sudo log parsing** — splits on `" ; "`. Different sudo configurations/locales produce different formats |
| 8 | `network.py:38` | **Only handles `RTM_NEWLINK`** — interface removal (`RTM_DELLINK`) is not handled |
| 9 | `constants.py:77` | **`DEFAULT_COOLDOWN_SSH_LOGOUT` defined but unused** — not included in `DEFAULT_COOLDOWNS` in `cooldowns.py` |
| 10 | `cli.py:141` | **Race condition on shutdown** — `time.sleep(0.5)` gives dispatcher time to process. If dispatcher is slow (Telegram API timeout), event is lost |
| 11 | `journal.py:117-129` | **`_SubprocessWrapper.__next__`** could loop infinitely if partial line never terminates |

### Configuration

| # | File:Line | Description |
|---|-----------|-------------|
| 12 | `loader.py:28-45` | **`apply_config` modifies global constants** — components that import constants before vs after `apply_config` see different values |

---

## 6. Security Concerns

### 🔴 CRITICAL: Hardcoded Telegram Credentials

**File:** `config/config.toml:2-3`

```toml
[telegram]
bot_token = "8978504260:AAELXT1FZA5KDqoh_N5ec1zMA6FEnpSKlAM"
chat_id = "632720215"
```

These appear to be **real, active Telegram bot credentials** committed to the repository. Anyone with access to this repo can:
- Send messages via this bot (spam, phishing)
- If the bot has group access, read group messages
- The chat ID reveals a target chat/user

**Immediate actions required:**
1. **Revoke the bot token** via [@BotFather](https://t.me/botfather) on Telegram immediately
2. **Remove credentials from git history** (`git filter-branch` or `git rebase` to purge)
3. **Add `config/config.toml` to `.gitignore`** and use environment variables for credentials
4. **Store `config.toml.example`** in the repo instead of the real config

### Other Security Issues

| # | Severity | File:Line | Issue |
|---|----------|-----------|-------|
| 2 | Medium | `shutdown.py:46-77` | `pidex-shutdown` entry point loads config with credentials, sends to Telegram, then exits |
| 3 | Low | `deploy/install.sh:36` | Installation script copies config.toml to `/etc/pidex/` with default permissions |
| 4 | Low | `deploy/install.sh:60` | `pidex` user needs Docker group access (effectively root-equivalent via Docker socket) |

### Recommendations

- **Use environment variables** for secrets (`TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`)
- **Load credentials from env** first, with config file as fallback
- **Store `config/config.toml.example`** in the repo, add `config/config.toml` to `.gitignore`
- **Consider `python-dotenv`** for `.env` file support

---

## 7. Long-Term Effects on the System

### Positive

- **Minimal resource footprint** — ~10-15 MB RAM, near-zero CPU when idle, no disk writes beyond logs
- **No database** — zero disk usage for state (all in-memory)
- **systemd integration** — proper service management, restart on failure, resource limits
- **Graceful degradation** — missing dependencies don't crash the daemon

### Potential Concerns

| Concern | Description |
|---------|-------------|
| **Docker socket access** | The `pidex` user needs Docker group membership, granting effective root-equivalent access to the Docker daemon |
| **Netlink access** | `pyroute2` requires `CAP_NET_ADMIN` or root. The network source silently fails without it |
| **Journald load** | Following the journal adds a client to systemd-journald. Negligible for home server use |
| **No log rotation** | Daemon logs to stdout/stderr, captured by journald. Standard systemd pattern |
| **Memory leak (bruteforce tracker)** | Under sustained SSH brute force from many IPs, memory grows unbounded |
| **No data persistence** | All state (dedup keys, cooldowns, poller state) lost on restart. First event of each type sent after restart |

### Compatibility

- **Python 3.11+ required** (tomllib) — Debian 12, Ubuntu 23.04+, Raspberry Pi OS Bookworm ship 3.11
- **Linux-only** — requires `/proc/stat`, `/proc/meminfo`, `/sys/class/thermal`, journald, netlink
- **No Windows/macOS** — appropriate for the target use case

---

## 8. Future Enhancements

### Near-Term (Technical Debt)

| Priority | Enhancement | Rationale |
|----------|-------------|-----------|
| 🔴 P0 | **Write tests** | Zero coverage. Unit tests for parsers, integration tests for pollers, end-to-end for dispatcher |
| 🔴 P0 | **Fix credential leak** | Revoke token, purge from git, use env vars |
| 🟡 P1 | **Replace `apply_config` with DI** | Pass config values explicitly to constructors instead of mutating globals |
| 🟡 P1 | **Fix bruteforce tracker leak** | Use LRU cache (`OrderedDict` with max size) or periodic cleanup |
| 🟡 P1 | **Add config schema validation** | Type checking in `apply_config` or use `pydantic` |
| 🟢 P2 | **Fix disk poller `f_bfree` → `f_bavail`** | Accurate non-root available space |
| 🟢 P2 | **Add HTTP retry/backoff** | Exponential backoff on Telegram API 429/5xx |
| 🟢 P2 | **Fix HTML escaping** | `html.escape()` or switch to MarkdownV2 parse mode |
| 🟢 P2 | **Extract `_ts()` into shared utility** | Eliminate duplication across ssh.py, sudo.py, systemd.py |

### Medium-Term Features

| Feature | Description | Effort |
|---------|-------------|--------|
| **Additional notifiers** | Discord webhook, email (SMTP), Slack, generic webhook | Medium |
| **Health check endpoint** | Unix socket or HTTP endpoint for `systemctl health` | Low |
| **SIGHUP reload** | Re-read config without restart | Medium |
| **Per-source dedup window** | Time-based dedup instead of "last event only" | Low |
| **Poller pause on suspend** | Detect system suspend/resume to avoid false alerts | Low |
| **Persistent dedup state** | JSON file across restarts | Low |

### Long-Term Vision

| Feature | Description | Effort |
|---------|-------------|--------|
| **Plugin system** | Third-party source/poller/notifier plugins via entry points | High |
| **Metrics endpoint** | Prometheus counters per event type | Medium |
| **CLI subcommand `status`** | Show active sources, thread health, last N events | Medium |
| **Watchdog** | Alert if dispatcher queue grows beyond threshold | Low |
| **Rewrite in Go/Rust** | Per README, if performance becomes an issue | Very high |

---

## Summary

| Category | Rating | Key Finding |
|----------|--------|-------------|
| **Architecture** | 🟢 Excellent | Clean layered design, thread-safe by construction, graceful degradation |
| **Code Cleanliness** | 🟡 Good | Type hints, small files, consistent naming. **Zero tests** is the biggest gap |
| **Resource Consumption** | 🟢 Excellent | Well within <50 MB target, near-zero CPU idle |
| **Optimization** | 🟢 Good | I/O-bound design, no performance bottlenecks. Minor improvements available |
| **Bugs** | 🟡 Moderate | 12 issues found. 2 critical (memory leak, wrong disk metric), 3 moderate, 7 minor |
| **Security** | 🔴 **CRITICAL** | Hardcoded Telegram credentials committed to repo. Must be revoked immediately |
| **Long-Term Effects** | 🟢 Positive | Minimal system impact, proper systemd integration |
| **Future Potential** | 🟢 High | Well-structured for extension, clear ABCs, modular design |

### Overall Assessment

PiDex is a **well-architected, cleanly implemented** v0.1.0 project. The architecture is sound, the data flow is simple and correct, and the threading model is appropriate for the problem domain. The code is readable, consistently styled, and follows Python best practices (type hints, ABCs, centralized constants, logging).

However, it has **two critical issues** that must be addressed before production deployment:

1. **🔴 SECURITY: Hardcoded Telegram credentials** in `config/config.toml` — revoke immediately
2. **🔴 QUALITY: Zero test coverage** — the project has no automated tests despite being structurally testable

With these two issues resolved, PiDex is a solid, production-ready home server monitoring tool.
