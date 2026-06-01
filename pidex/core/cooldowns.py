import time

from pidex.core.constants import (
    DEFAULT_COOLDOWN_SSH_LOGIN,
    DEFAULT_COOLDOWN_SSH_LOGOUT,
    DEFAULT_COOLDOWN_SSH_BRUTEFORCE,
    DEFAULT_COOLDOWN_SUDO_USED,
    DEFAULT_COOLDOWN_CPU_HIGH,
    DEFAULT_COOLDOWN_CPU_RECOVERED,
    DEFAULT_COOLDOWN_TEMP_WARN,
    DEFAULT_COOLDOWN_TEMP_CRITICAL,
    DEFAULT_COOLDOWN_DISK_WARN,
    DEFAULT_COOLDOWN_DISK_CRITICAL,
    DEFAULT_COOLDOWN_RAM_HIGH,
    EVENT_SSH_LOGIN,
    EVENT_SSH_LOGOUT,
    EVENT_SSH_BRUTEFORCE,
    EVENT_SUDO_USED,
    EVENT_CPU_HIGH,
    EVENT_CPU_RECOVERED,
    EVENT_TEMP_WARN,
    EVENT_TEMP_CRITICAL,
    EVENT_DISK_WARN,
    EVENT_DISK_CRITICAL,
    EVENT_RAM_HIGH,
)
from pidex.core.event import Event


DEFAULT_COOLDOWNS: dict[str, float] = {
    EVENT_SSH_LOGIN: DEFAULT_COOLDOWN_SSH_LOGIN,
    EVENT_SSH_LOGOUT: DEFAULT_COOLDOWN_SSH_LOGOUT,
    EVENT_SSH_BRUTEFORCE: DEFAULT_COOLDOWN_SSH_BRUTEFORCE,
    EVENT_SUDO_USED: DEFAULT_COOLDOWN_SUDO_USED,
    EVENT_CPU_HIGH: DEFAULT_COOLDOWN_CPU_HIGH,
    EVENT_CPU_RECOVERED: DEFAULT_COOLDOWN_CPU_RECOVERED,
    EVENT_TEMP_WARN: DEFAULT_COOLDOWN_TEMP_WARN,
    EVENT_TEMP_CRITICAL: DEFAULT_COOLDOWN_TEMP_CRITICAL,
    EVENT_DISK_WARN: DEFAULT_COOLDOWN_DISK_WARN,
    EVENT_DISK_CRITICAL: DEFAULT_COOLDOWN_DISK_CRITICAL,
    EVENT_RAM_HIGH: DEFAULT_COOLDOWN_RAM_HIGH,
}


class CooldownManager:
    def __init__(self, overrides: dict[str, float] | None = None):
        self._cooled_until: dict[str, float] = {}
        self._durations: dict[str, float] = {**DEFAULT_COOLDOWNS}
        if overrides:
            self._durations.update(overrides)

    def is_allowed(self, event: Event) -> bool:
        now = time.time()
        deadline = self._cooled_until.get(event.event_type, 0)
        return now >= deadline

    def record(self, event: Event) -> None:
        duration = self._durations.get(event.event_type, 0)
        if duration > 0:
            self._cooled_until[event.event_type] = time.time() + duration

    def update_duration(self, event_type: str, seconds: float) -> None:
        self._durations[event_type] = seconds
