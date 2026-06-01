# Design Decisions

## ADR-001: Threading over asyncio

**Date:** 2026-06-01

**Context:** The app needs to listen to multiple blocking event sources
(journald, Docker socket, netlink) simultaneously.

**Decision:** Use threading + `queue.Queue` rather than `asyncio`.

**Rationale:**
- All target libraries (systemd.journal, docker-py, pyroute2) are synchronous
- Threads map 1:1 to sources — each thread blocks independently
- Shared state (cooldowns, dedup) is trivial to protect since only the
  dispatcher mutates it; sources are write-only
- Simpler mental model than async/await + run_in_executor
- Thread overhead is acceptable for ~10 threads on a server

---

## ADR-002: Single journald listener with multiple parsers

**Date:** 2026-06-01

**Context:** SSH, sudo, and systemd events all come from journald.

**Decision:** One `journal.py` thread follows journald, then dispatches entries
to parser functions in `ssh.py`, `sudo.py`, and `systemd.py`.

**Rationale:**
- Single connection to journald (saves resources)
- Each parser is independently testable
- Adding a new journald-based source = adding one parser file + registering it

---

## ADR-003: Polling over netlink for v1 (reversed)

**Date:** 2026-06-01

**Context:** Network interface up/down events.

**Decision:** Use `pyroute2` netlink listener (event-driven).

**Rationale:** User explicitly chose event-driven approach over polling.
pyroute2 is the standard Python netlink interface.

---

## ADR-004: requests library for Telegram HTTP

**Date:** 2026-06-01

**Context:** Need to POST messages to Telegram API.

**Decision:** Use `requests` library.

**Rationale:** Cleaner API than stdlib urllib. Added dependency is minimal and
well-justified for maintainability.

---

## ADR-005: All hardcoded values in constants.py

**Date:** 2026-06-01

**Context:** Thresholds, intervals, cooldowns, event type strings, severity
strings — all hardcoded values.

**Decision:** Place every hardcoded value in `pidex/core/constants.py`. Config
loader overrides these values at startup once config system is built.

**Rationale:**
- Single source of truth for defaults
- Components import from constants; they automatically use config-overridden
  values when the config loader has run
- Prevents scattered magic numbers throughout the codebase

---

## ADR-006: stdout/stderr logging

**Date:** 2026-06-01

**Context:** Where should the daemon write logs?

**Decision:** stdout/stderr only.

**Rationale:** When running as a systemd service, journald captures stdout
automatically. No need for a separate log file or log rotation setup.

---

## ADR-007: Dedup hash = source + event_type + title + message

**Date:** 2026-06-01

**Context:** Prevent repeated identical alerts.

**Decision:** Hash is computed from `source + event_type + title + message`.

**Rationale:** Catches exact duplicate alerts. Excluding timestamp means the
same event from different times is still deduplicated. Per-source tracking
allows different sources to have independent dedup state.

---

## ADR-008: Configurable subset of services/containers

**Date:** 2026-06-01

**Context:** Docker & systemd sources — monitor all or a subset?

**Decision:** Monitor a user-configured subset defined in `config.toml`.

**Rationale:** Less noise. The user knows which services matter to them.

---

## ADR-009: Shutdown via SIGTERM handler + dedicated systemd service

**Date:** 2026-06-01

**Context:** Need to notify before the machine goes offline.

**Decision:** The main daemon catches SIGTERM and sends a SHUTDOWN_STARTED
event. Additionally, a `pidex-shutdown.service` (runs before shutdown target)
acts as a safety net.

**Rationale:** SIGTERM covers the normal case (systemctl stop pidex). The
separate service covers hard shutdown/reboot where the main daemon may not
get a clean signal.

---

## ADR-010: Poller thresholds

**Date:** 2026-06-01

**Context:** Define WARN and CRITICAL thresholds for CPU, RAM, Disk, Temperature.

**Decision:**

| Metric | WARN | CRITICAL |
|--------|------|----------|
| CPU    | 80%  | 95%      |
| RAM    | 85%  | 95%      |
| Disk   | 85%  | 95%      |
| Temp   | 75°C | 85°C     |

**Rationale:** Conservative defaults suitable for a Raspberry Pi home server.
All values are configurable via `constants.py` (and eventually `config.toml`).
