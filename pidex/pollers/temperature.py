from datetime import datetime

from pidex.core.bus import EventBus
from pidex.core.event import Event
from pidex.pollers.base import BasePoller


class TemperaturePoller(BasePoller):
    def read_value(self) -> float:
        temp_millidegrees = self._read_temp()
        return temp_millidegrees / 1000.0

    @staticmethod
    def _read_temp() -> int:
        with open("/sys/class/thermal/thermal_zone0/temp") as f:
            return int(f.read().strip())

    def _make_event(self, severity: str, value: float) -> Event:
        return Event(
            source=self.source_name,
            event_type=f"{self.source_name.upper()}_{severity}",
            severity=severity,
            title=f"Temperature {severity}",
            message=f"CPU temperature at {value:.1f}°C (warn={self.warn}°C, crit={self.critical}°C)",
            timestamp=datetime.now(),
        )
