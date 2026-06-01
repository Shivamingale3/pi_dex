import threading
from abc import ABC, abstractmethod

from pidex.core.bus import EventBus


class BaseSource(ABC):
    def __init__(self, bus: EventBus, config: dict):
        self._bus = bus
        self._config = config
        self._thread: threading.Thread | None = None

    @abstractmethod
    def run(self, stop_event: threading.Event) -> None:
        ...

    def start(self, stop_event: threading.Event) -> None:
        self._thread = threading.Thread(
            target=self.run,
            args=(stop_event,),
            daemon=True,
            name=self.__class__.__name__,
        )
        self._thread.start()
