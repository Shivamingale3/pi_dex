from datetime import datetime

from pidex.core.event import Event, entry_message


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


def test_entry_message_string():
    assert entry_message({"MESSAGE": "Started nginx.service"}) == "Started nginx.service"


def test_entry_message_bytes():
    assert entry_message({"MESSAGE": b"Started nginx.service"}) == "Started nginx.service"


def test_entry_message_list_of_strings():
    assert entry_message({"MESSAGE": ["line one", "line two"]}) == "line one\nline two"


def test_entry_message_list_of_bytes():
    assert entry_message({"MESSAGE": [b"line one", b"line two"]}) == "line one\nline two"


def test_entry_message_mixed_list():
    assert entry_message({"MESSAGE": ["line one", b"line two"]}) == "line one\nline two"


def test_entry_message_bytes_with_invalid_utf8():
    assert entry_message({"MESSAGE": b"\xff\xfe\xfd"}) == "\ufffd\ufffd\ufffd"


def test_entry_message_missing_returns_empty():
    assert entry_message({}) == ""


def test_entry_message_none_returns_empty():
    assert entry_message({"MESSAGE": None}) == ""
