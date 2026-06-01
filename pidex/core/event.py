from dataclasses import dataclass, field
from datetime import datetime


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
