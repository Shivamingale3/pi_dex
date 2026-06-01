from pidex.core.bus import EventBus
from pidex.pollers.base import BasePoller


class CpuPoller(BasePoller):
    def __init__(self, bus: EventBus, interval: int, warn: float, critical: float):
        super().__init__(bus, interval, warn, critical)
        self._prev = self._read_cpu()

    def read_value(self) -> float:
        curr = self._read_cpu()
        delta_total = curr["total"] - self._prev["total"]
        delta_idle = curr["idle"] - self._prev["idle"]
        self._prev = curr

        if delta_total == 0:
            return 0.0

        return (1.0 - delta_idle / delta_total) * 100.0

    @staticmethod
    def _read_cpu() -> dict:
        with open("/proc/stat") as f:
            parts = f.readline().split()
        fields = [int(v) for v in parts[1:]]
        total = sum(fields)
        idle = fields[3] + fields[4]
        return {"total": total, "idle": idle}
