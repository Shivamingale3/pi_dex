import logging

import requests

from pidex.core.event import Event
from pidex.notifiers.base import BaseNotifier

logger = logging.getLogger(__name__)

TELEGRAM_API = "https://api.telegram.org/bot{token}/sendMessage"


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
        try:
            resp = self._session.post(self._api_url, json=payload, timeout=10)
            resp.raise_for_status()
            logger.info("Sent %s notification", event.event_type)
        except requests.RequestException:
            logger.exception("Failed to send Telegram message for %s", event.event_type)
            raise

    @staticmethod
    def _format_message(event: Event) -> str:
        severity_icon = {
            "INFO": "\u2139\ufe0f",
            "WARN": "\u26a0\ufe0f",
            "CRITICAL": "\U0001f6a8",
            "RECOVERED": "\u2705",
        }.get(event.severity, "")

        lines = [
            f"{severity_icon} <b>{event.title}</b>",
            f"<code>{event.message}</code>",
            f"\U0001f4c5 {event.timestamp.strftime('%Y-%m-%d %H:%M:%S')}",
            f"\U0001f3e0 {event.source}",
        ]
        return "\n".join(lines)
