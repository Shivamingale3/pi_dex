from pidex.core.bus import EventBus
from pidex.pollers.base import BasePoller


class RamPoller(BasePoller):
    def read_value(self) -> float:
        total, available = self._read_meminfo()
        if total == 0:
            return 0.0
        return (total - available) / total * 100.0

    @staticmethod
    def _read_meminfo() -> tuple[int, int]:
        total = 0
        available = 0
        with open("/proc/meminfo") as f:
            for line in f:
                if line.startswith("MemTotal:"):
                    total = int(line.split()[1])
                elif line.startswith("MemAvailable:"):
                    available = int(line.split()[1])
        return total, available
