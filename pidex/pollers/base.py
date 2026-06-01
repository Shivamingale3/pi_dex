import logging
import threading
from abc import ABC, abstractmethod
from datetime import datetime

from pidex.core.bus import EventBus
from pidex.core.event import Event

logger = logging.getLogger(__name__)

STATE_OK = "ok"
STATE_WARN = "warn"
STATE_CRITICAL = "critical"


class BasePoller(ABC):
    def __init__(self, bus: EventBus, interval: int, warn: float, critical: float):
        self._bus = bus
        self.interval = interval
        self.warn = warn
        self.critical = critical
        self._state = STATE_OK
        self._thread: threading.Thread | None = None

    @abstractmethod
    def read_value(self) -> float:
        ...

    @property
    def source_name(self) -> str:
        return self.__class__.__name__.replace("Poller", "").lower()

    def _check(self) -> Event | None:
        try:
            value = self.read_value()
        except Exception:
            logger.exception("%s read failed", self.__class__.__name__)
            return None

        if self._state == STATE_OK and value >= self.warn:
            self._state = STATE_WARN
            return self._make_event("WARN", value)

        if self._state == STATE_WARN:
            if value >= self.critical:
                self._state = STATE_CRITICAL
                return self._make_event("CRITICAL", value)
            if value < self.warn:
                self._state = STATE_OK
                return self._make_event("RECOVERED", value)

        if self._state == STATE_CRITICAL:
            if value < self.warn:
                self._state = STATE_OK
                return self._make_event("RECOVERED", value)

        return None

    def _make_event(self, severity: str, value: float) -> Event:
        return Event(
            source=self.source_name,
            event_type=f"{self.source_name.upper()}_{severity}",
            severity=severity,
            title=f"{self.__class__.__name__.replace('Poller', '')} {severity}",
            message=f"{self.source_name.upper()} usage at {value:.1f}% (warn={self.warn}%, crit={self.critical}%)",
            timestamp=datetime.now(),
        )

    def run(self, stop_event: threading.Event) -> None:
        while not stop_event.is_set():
            event = self._check()
            if event is not None:
                self._bus.publish(event)
                logger.info("%s: %s", self.source_name, event.severity)
            stop_event.wait(self.interval)

    def start(self, stop_event: threading.Event) -> None:
        self._thread = threading.Thread(
            target=self.run,
            args=(stop_event,),
            daemon=True,
            name=self.__class__.__name__,
        )
        self._thread.start()
