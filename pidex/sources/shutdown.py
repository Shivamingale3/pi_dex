import logging
import sys
from datetime import datetime

from pidex.config.loader import load_config
from pidex.core.constants import EVENT_SHUTDOWN_STARTED, SEVERITY_WARN, SOURCE_SHUTDOWN
from pidex.core.event import Event
from pidex.notifiers.telegram import TelegramNotifier


def main() -> None:
    """Entry point for pidex-shutdown script (called by systemd on shutdown)."""
    logging.basicConfig(level=logging.INFO, format="%(message)s", stream=sys.stdout)

    cfg = load_config()

    if not cfg.telegram_token or not cfg.telegram_chat_id:
        print("pidex-shutdown: no Telegram credentials configured", file=sys.stderr)
        sys.exit(1)

    notifier = TelegramNotifier(bot_token=cfg.telegram_token, chat_id=cfg.telegram_chat_id)
    event = Event(
        source=SOURCE_SHUTDOWN,
        event_type=EVENT_SHUTDOWN_STARTED,
        severity=SEVERITY_WARN,
        title="System Shutdown",
        message="Server is powering off",

    )

    try:
        notifier.send(event)
        print("pidex-shutdown: notification sent")
    except Exception as e:
        print(f"pidex-shutdown: failed: {e}", file=sys.stderr)
        sys.exit(1)
