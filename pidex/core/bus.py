import queue
from pidex.core.event import Event


class EventBus:
    def __init__(self):
        self._queue: queue.Queue = queue.Queue()

    def publish(self, event: Event) -> None:
        self._queue.put_nowait(event)

    def get(self, timeout: float | None = None) -> Event:
        return self._queue.get(timeout=timeout)

    @property
    def qsize(self) -> int:
        return self._queue.qsize()
