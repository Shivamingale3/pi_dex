import re

from pidex.core.constants import EVENT_SUDO_USED, SEVERITY_INFO, SOURCE_SUDO
from pidex.core.event import Event, entry_message, entry_timestamp

_SUDO_RE = re.compile(
    r"^\s*(\S+)\s*;.*?;\s*COMMAND=(.*)"
)


def parse(entry: dict) -> Event | None:
    if entry.get("_COMM") != "sudo" and entry.get("SYSLOG_IDENTIFIER") != "sudo":
        return None

    message = entry_message(entry)
    m = _SUDO_RE.match(message)
    if not m:
        return None

    user = m.group(1)
    command = m.group(2).strip()

    return Event(
        source=SOURCE_SUDO,
        event_type=EVENT_SUDO_USED,
        severity=SEVERITY_INFO,
        title="Sudo Used",
        message=f"{user} ran sudo {command}",
        timestamp=entry_timestamp(entry),
    )
