from datetime import datetime
from unittest.mock import MagicMock

import pytest

from pidex.core.bus import EventBus
from pidex.core.event import Event


@pytest.fixture
def bus():
    return EventBus()


@pytest.fixture
def sample_event():
    return Event(
        source="test",
        event_type="TEST_EVENT",
        severity="INFO",
        title="Test Event",
        message="This is a test",
        timestamp=datetime(2026, 1, 1, 12, 0, 0),
    )


@pytest.fixture
def mock_notifier():
    notifier = MagicMock()
    notifier.send = MagicMock()
    return notifier


@pytest.fixture
def mock_journal_entry():
    return {
        "_COMM": "sshd",
        "SYSLOG_IDENTIFIER": "sshd",
        "MESSAGE": "Accepted password for leadows from 192.168.1.100 port 22 ssh2",
        "__REALTIME_TIMESTAMP": 1735689600000000,
    }
