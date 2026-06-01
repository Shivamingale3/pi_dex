import time
from unittest.mock import patch

from pidex.core.cooldowns import CooldownManager
from pidex.core.event import Event


def make_event(event_type="TEST_EVENT"):
    return Event(
        source="test",
        event_type=event_type,
        severity="INFO",
        title="Test",
        message="test",
    )


def test_allows_new_event():
    cm = CooldownManager()
    assert cm.is_allowed(make_event())


def test_records_and_blocks():
    cm = CooldownManager({"TEST_EVENT": 60})
    e = make_event()
    assert cm.is_allowed(e)
    cm.record(e)
    assert not cm.is_allowed(e)


def test_allows_after_cooldown_expires():
    cm = CooldownManager({"TEST_EVENT": 1})
    e = make_event()
    cm.record(e)
    time.sleep(1.1)
    assert cm.is_allowed(e)


def test_zero_cooldown_always_allowed():
    cm = CooldownManager({"TEST_EVENT": 0})
    e = make_event()
    for _ in range(10):
        assert cm.is_allowed(e)
        cm.record(e)


def test_different_event_types_independent():
    cm = CooldownManager({"EVENT_A": 60, "EVENT_B": 0})
    a = make_event("EVENT_A")
    b = make_event("EVENT_B")
    cm.record(a)
    assert not cm.is_allowed(a)
    assert cm.is_allowed(b)


def test_update_duration():
    cm = CooldownManager({"TEST_EVENT": 60})
    e = make_event()
    cm.record(e)
    assert not cm.is_allowed(e)
    cm.update_duration("TEST_EVENT", 0)
    assert cm.is_allowed(e)


def test_overrides_defaults():
    cm = CooldownManager({"SSH_LOGIN": 120})
    e = Event(
        source="ssh",
        event_type="SSH_LOGIN",
        severity="INFO",
        title="SSH Login",
        message="user logged in",
    )
    cm.record(e)
    assert not cm.is_allowed(e)
