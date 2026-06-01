from datetime import datetime
from unittest.mock import MagicMock, patch

import pytest
import requests

from pidex.core.event import Event
from pidex.notifiers.telegram import TelegramNotifier


@pytest.fixture
def notifier():
    return TelegramNotifier(bot_token="test:token", chat_id="123")


def test_format_message_info():
    e = Event(
        source="ssh",
        event_type="SSH_LOGIN",
        severity="INFO",
        title="SSH Login",
        message="user logged in",
        timestamp=datetime(2026, 1, 1, 12, 0, 0),
    )
    text = TelegramNotifier._format_message(e)
    assert "SSH Login" in text
    assert "user logged in" in text
    assert "ssh" in text
    assert "2026-01-01" in text
    assert "<b>" in text
    assert "<code>" in text


def test_format_message_escapes_html():
    e = Event(
        source="test",
        event_type="TEST",
        severity="WARN",
        title="Test",
        message="value < 100 & cost > $50",
    )
    text = TelegramNotifier._format_message(e)
    assert "value &lt; 100" in text
    assert "&amp;" in text


def test_send_success(notifier):
    with patch.object(notifier._session, "post") as mock_post:
        mock_response = MagicMock()
        mock_response.raise_for_status.return_value = None
        mock_post.return_value = mock_response

        e = Event(
            source="test",
            event_type="TEST",
            severity="INFO",
            title="Test",
            message="test",
        )
        notifier.send(e)
        mock_post.assert_called_once()
        url = mock_post.call_args[0][0]
        assert "test:token" in url
        payload = mock_post.call_args[1]["json"]
        assert payload["chat_id"] == "123"
        assert payload["parse_mode"] == "HTML"


def test_send_failure_logs_and_raises(notifier):
    with patch.object(notifier._session, "post") as mock_post:
        mock_post.side_effect = requests.RequestException("connection error")
        e = Event(
            source="test",
            event_type="TEST",
            severity="INFO",
            title="Test",
            message="test",
        )
        with pytest.raises(requests.RequestException):
            notifier.send(e)
