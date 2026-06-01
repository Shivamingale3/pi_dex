import os

from pidex.core.bus import EventBus
from pidex.core.event import Event
from pidex.pollers.base import BasePoller


class DiskPoller(BasePoller):
    def __init__(self, bus: EventBus, interval: int, warn: float, critical: float, mount: str = "/"):
        super().__init__(bus, interval, warn, critical)
        self._mount = mount

    def read_value(self) -> float:
        s = os.statvfs(self._mount)
        total = s.f_frsize * s.f_blocks
        free = s.f_frsize * s.f_bfree
        if total == 0:
            return 0.0
        return (total - free) / total * 100.0

    def _make_event(self, severity: str, value: float) -> Event:
        return Event(
            source=self.source_name,
            event_type=f"{self.source_name.upper()}_{severity}",
            severity=severity,
            title=f"Disk {severity}",
            message=f"Mount '{self._mount}' at {value:.1f}% (warn={self.warn}%, crit={self.critical}%)",
        )
