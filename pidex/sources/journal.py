import logging
import threading

from pidex.core.bus import EventBus
from pidex.core.event import Event
from pidex.sources.base import BaseSource

logger = logging.getLogger(__name__)


class JournalSource(BaseSource):
    def __init__(self, bus: EventBus, config: dict):
        super().__init__(bus, config)
        self._parsers: list[callable] = []

    def register(self, parser: callable) -> None:
        self._parsers.append(parser)

    def run(self, stop_event: threading.Event) -> None:
        reader = _open_journal()
        if reader is None:
            logger.error("journald not available — skipping journal source")
            return

        while not stop_event.is_set():
            for entry in reader:
                for parser in self._parsers:
                    try:
                        event = parser(entry)
                        if event is not None:
                            self._bus.publish(event)
                    except Exception:
                        logger.exception("Parser %s failed", parser.__name__)

            reader.wait(1.0)

        reader.close()


def _open_journal():
    try:
        from systemd import journal

        reader = journal.Reader()
        reader.seek_tail()
        reader.get_previous()
        return reader
    except ImportError:
        return None
    except Exception:
        logger.exception("Failed to open journald")
        return None
