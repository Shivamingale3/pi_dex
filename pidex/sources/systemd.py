import fnmatch
import re
import time

from pidex.core.constants import (
    EVENT_SERVICE_FAILED,
    EVENT_SERVICE_RESTARTED,
    EVENT_SERVICE_STARTED,
    EVENT_SERVICE_STOPPED,
    SEVERITY_CRITICAL,
    SEVERITY_INFO,
    SEVERITY_WARN,
    SOURCE_SYSTEMD,
)
from pidex.core.event import Event

_STARTED_RE = re.compile(r"Started (.+\.service)")
_STOPPED_RE = re.compile(r"Stopped (.+\.service)")
_FAILED_RE = re.compile(r"Failed to start (.+\.service)")
_RESTARTED_RE = re.compile(r"Restarted (.+\.service)")


def make_parser(watch_patterns: list[str]) -> callable:
    def parse(entry: dict) -> Event | None:
        if not _is_systemd_entry(entry):
            return None

        message = entry.get("MESSAGE", "")
        service = _extract_service(message)
        if service is None:
            return None

        if not _is_watched(service, watch_patterns):
            return None

        if _STARTED_RE.match(message):
            return Event(
                source=SOURCE_SYSTEMD,
                event_type=EVENT_SERVICE_STARTED,
                severity=SEVERITY_INFO,
                title="Service Started",
                message=f"{service} started",
                timestamp=_ts(entry),
            )

        if _STOPPED_RE.match(message):
            return Event(
                source=SOURCE_SYSTEMD,
                event_type=EVENT_SERVICE_STOPPED,
                severity=SEVERITY_WARN,
                title="Service Stopped",
                message=f"{service} stopped",
                timestamp=_ts(entry),
            )

        if _FAILED_RE.match(message):
            return Event(
                source=SOURCE_SYSTEMD,
                event_type=EVENT_SERVICE_FAILED,
                severity=SEVERITY_CRITICAL,
                title="Service Failed",
                message=f"{service} failed to start",
                timestamp=_ts(entry),
            )

        if _RESTARTED_RE.match(message):
            return Event(
                source=SOURCE_SYSTEMD,
                event_type=EVENT_SERVICE_RESTARTED,
                severity=SEVERITY_INFO,
                title="Service Restarted",
                message=f"{service} was restarted",
                timestamp=_ts(entry),
            )

        return None

    return parse


def _is_systemd_entry(entry: dict) -> bool:
    return (
        entry.get("SYSLOG_IDENTIFIER") == "systemd"
        or (entry.get("_SYSTEMD_UNIT") or "").endswith(".service")
    )


def _extract_service(message: str) -> str | None:
    for pattern in (_STARTED_RE, _STOPPED_RE, _FAILED_RE, _RESTARTED_RE):
        m = pattern.match(message)
        if m:
            return m.group(1)
    return None


def _is_watched(service: str, patterns: list[str]) -> bool:
    if not patterns:
        return True
    for p in patterns:
        if fnmatch.fnmatch(service, p):
            return True
    return False


def _ts(entry: dict) -> float:
    return entry.get("__REALTIME_TIMESTAMP", time.time()) / 1_000_000
