import logging
import queue
import threading

from pidex.core.bus import EventBus
from pidex.core.cooldowns import CooldownManager
from pidex.core.dedup import DedupManager
from pidex.core.constants import DISPATCHER_QUEUE_TIMEOUT
from pidex.notifiers.base import BaseNotifier

logger = logging.getLogger(__name__)


class Dispatcher:
    def __init__(
        self,
        bus: EventBus,
        notifier: BaseNotifier,
        cooldowns: CooldownManager | None = None,
        dedup: DedupManager | None = None,
    ):
        self._bus = bus
        self._notifier = notifier
        self._cooldowns = cooldowns or CooldownManager()
        self._dedup = dedup or DedupManager()
        self._thread: threading.Thread | None = None

    def start(self, stop_event: threading.Event) -> None:
        self._thread = threading.Thread(
            target=self._run,
            args=(stop_event,),
            daemon=True,
            name="dispatcher",
        )
        self._thread.start()

    def _run(self, stop_event: threading.Event) -> None:
        while not stop_event.is_set():
            try:
                event = self._bus.get(timeout=DISPATCHER_QUEUE_TIMEOUT)
            except queue.Empty:
                continue

            if not self._cooldowns.is_allowed(event):
                logger.debug("Cooldown active for %s, dropping", event.event_type)
                continue

            if not self._dedup.is_new(event):
                logger.debug("Duplicate %s from %s, dropping", event.event_type, event.source)
                continue

            self._cooldowns.record(event)
            self._dedup.record(event)

            try:
                self._notifier.send(event)
            except Exception:
                logger.exception("Failed to send event %s", event.event_type)
