from datetime import datetime

from pidex.core.event import Event


def test_event_creation():
    e = Event(
        source="ssh",
        event_type="SSH_LOGIN",
        severity="INFO",
        title="SSH Login",
        message="user logged in",
        timestamp=datetime(2026, 1, 1, 12, 0, 0),
    )
    assert e.source == "ssh"
    assert e.event_type == "SSH_LOGIN"
    assert e.severity == "INFO"
    assert e.title == "SSH Login"
    assert e.message == "user logged in"
    assert e.timestamp == datetime(2026, 1, 1, 12, 0, 0)


def test_event_default_timestamp():
    e = Event(
        source="test",
        event_type="TEST",
        severity="INFO",
        title="Test",
        message="test",
    )
    assert isinstance(e.timestamp, datetime)


def test_dedup_key():
    e1 = Event(
        source="ssh",
        event_type="SSH_LOGIN",
        severity="INFO",
        title="SSH Login",
        message="user logged in",
    )
    e2 = Event(
        source="ssh",
        event_type="SSH_LOGIN",
        severity="INFO",
        title="SSH Login",
        message="user logged in",
    )
    assert e1.dedup_key() == "ssh|SSH_LOGIN|SSH Login|user logged in"
    assert e1.dedup_key() == e2.dedup_key()


def test_dedup_key_excludes_timestamp():
    e1 = Event(
        source="ssh",
        event_type="SSH_LOGIN",
        severity="INFO",
        title="SSH Login",
        message="user logged in",
        timestamp=datetime(2026, 1, 1, 12, 0, 0),
    )
    e2 = Event(
        source="ssh",
        event_type="SSH_LOGIN",
        severity="INFO",
        title="SSH Login",
        message="user logged in",
        timestamp=datetime(2026, 1, 1, 12, 5, 0),
    )
    assert e1.dedup_key() == e2.dedup_key()


def test_dedup_key_differs_for_different_sources():
    e1 = Event(
        source="ssh",
        event_type="SSH_LOGIN",
        severity="INFO",
        title="SSH Login",
        message="user logged in",
    )
    e2 = Event(
        source="sudo",
        event_type="SSH_LOGIN",
        severity="INFO",
        title="SSH Login",
        message="user logged in",
    )
    assert e1.dedup_key() != e2.dedup_key()
