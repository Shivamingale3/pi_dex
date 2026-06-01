import html
import logging
import time as time_module

import requests

from pidex.core.event import Event
from pidex.notifiers.base import BaseNotifier

logger = logging.getLogger(__name__)

TELEGRAM_API = "https://api.telegram.org/bot{token}/sendMessage"
_MAX_RETRIES = 3


class TelegramNotifier(BaseNotifier):
    def __init__(self, bot_token: str, chat_id: str):
        self._api_url = TELEGRAM_API.format(token=bot_token)
        self._chat_id = chat_id
        self._session = requests.Session()

    def send(self, event: Event) -> None:
        text = self._format_message(event)
        payload = {
            "chat_id": self._chat_id,
            "text": text,
            "parse_mode": "HTML",
            "disable_web_page_preview": True,
        }
        last_exc = None
        for attempt in range(_MAX_RETRIES):
            try:
                resp = self._session.post(self._api_url, json=payload, timeout=10)
                resp.raise_for_status()
                logger.info("Sent %s notification", event.event_type)
                return
            except requests.RequestException as e:
                last_exc = e
                logger.warning(
                    "Telegram send failed (attempt %d/%d): %s",
                    attempt + 1, _MAX_RETRIES, e,
                )
                if attempt < _MAX_RETRIES - 1:
                    time_module.sleep(2 ** attempt)
        logger.exception("Failed to send Telegram message for %s", event.event_type)
        raise last_exc

    @staticmethod
    def _format_message(event: Event) -> str:
        severity_icon = {
            "INFO": "\u2139\ufe0f",
            "WARN": "\u26a0\ufe0f",
            "CRITICAL": "\U0001f6a8",
            "RECOVERED": "\u2705",
        }.get(event.severity, "")

        safe_message = html.escape(event.message)
        lines = [
            f"{severity_icon} <b>{html.escape(event.title)}</b>",
            f"<code>{safe_message}</code>",
            f"\U0001f4c5 {event.timestamp.strftime('%Y-%m-%d %H:%M:%S')}",
            f"\U0001f3e0 {html.escape(event.source)}",
        ]
        return "\n".join(lines)
