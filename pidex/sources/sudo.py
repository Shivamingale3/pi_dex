import time

from pidex.core.constants import EVENT_SUDO_USED, SEVERITY_INFO, SOURCE_SUDO
from pidex.core.event import Event


def parse(entry: dict) -> Event | None:
    if entry.get("_COMM") != "sudo":
        return None

    message = entry.get("MESSAGE", "")
    parts = message.split(" ; ")
    if len(parts) < 4:
        return None

    user = parts[0].strip()
    command = parts[-1].replace("COMMAND=", "", 1).strip()

    return Event(
        source=SOURCE_SUDO,
        event_type=EVENT_SUDO_USED,
        severity=SEVERITY_INFO,
        title="Sudo Used",
        message=f"{user} ran sudo {command}",
        timestamp=_ts(entry),
    )


def _ts(entry: dict) -> float:
    return entry.get("__REALTIME_TIMESTAMP", time.time()) / 1_000_000
