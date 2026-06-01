from dataclasses import dataclass, field
from datetime import datetime


def entry_timestamp(entry: dict) -> datetime:
    micros = entry.get("__REALTIME_TIMESTAMP")
    if micros is not None:
        return datetime.fromtimestamp(int(micros) / 1_000_000)
    return datetime.now()


def entry_message(entry: dict) -> str:
    """Coerce a journal MESSAGE field to a single string.

    journalctl --output=json returns MESSAGE as:
    - a string (most common)
    - a list of strings (multi-line)
    - a list of bytes (binary or non-UTF8)
    - bytes (raw binary)
    """
    return _message_to_str(entry.get("MESSAGE", ""))


def _message_to_str(value) -> str:
    if value is None:
        return ""
    if isinstance(value, str):
        return value
    if isinstance(value, bytes):
        return value.decode("utf-8", errors="replace")
    if isinstance(value, list):
        return "\n".join(_message_to_str(item) for item in value)
    return str(value)


@dataclass
class Event:
    source: str
    event_type: str
    severity: str
    title: str
    message: str
    timestamp: datetime = field(default_factory=datetime.now)

    def dedup_key(self) -> str:
        return f"{self.source}|{self.event_type}|{self.title}|{self.message}"
