import logging
import threading
import time

from pidex.core.bus import EventBus
from pidex.core.constants import EVENT_REBOOT_STARTED, EVENT_SHUTDOWN_STARTED, SEVERITY_WARN, SOURCE_SHUTDOWN
from pidex.core.event import Event
from pidex.sources.base import BaseSource

logger = logging.getLogger(__name__)


class ShutdownSource(BaseSource):
    def __init__(self, bus: EventBus, config: dict):
        super().__init__(bus, config)
        self._triggered = False

    def run(self, stop_event: threading.Event) -> None:
        while not stop_event.is_set():
            stop_event.wait(0.5)

        if self._triggered:
            return
        self._triggered = True

        logger.info("Sending shutdown notification")
        self._bus.publish(Event(
            source=SOURCE_SHUTDOWN,
            event_type=EVENT_SHUTDOWN_STARTED,
            severity=SEVERITY_WARN,
            title="Shutdown Initiated",
            message="PiDex daemon is shutting down",
            timestamp=time.time(),
        ))


def send_reboot_event(bus: EventBus) -> None:
    bus.publish(Event(
        source=SOURCE_SHUTDOWN,
        event_type=EVENT_REBOOT_STARTED,
        severity=SEVERITY_WARN,
        title="Reboot Initiated",
        message="System rebooting",
        timestamp=time.time(),
    ))


def main() -> None:
    """Entry point for pidex-shutdown script (called by systemd on shutdown)."""
    import sys

    from pidex.config.loader import get_telegram_config, load_config
    from pidex.notifiers.telegram import TelegramNotifier

    logging.basicConfig(level=logging.INFO, format="%(message)s", stream=sys.stdout)

    cfg = load_config()
    token, chat_id = get_telegram_config(cfg)

    if not token or not chat_id:
        print("pidex-shutdown: no Telegram credentials configured", file=sys.stderr)
        sys.exit(1)

    notifier = TelegramNotifier(bot_token=token, chat_id=chat_id)
    event = Event(
        source=SOURCE_SHUTDOWN,
        event_type=EVENT_SHUTDOWN_STARTED,
        severity=SEVERITY_WARN,
        title="System Shutdown",
        message="Server is powering off",
        timestamp=time.time(),
    )

    try:
        notifier.send(event)
        print("pidex-shutdown: notification sent")
    except Exception as e:
        print(f"pidex-shutdown: failed: {e}", file=sys.stderr)
        sys.exit(1)
