import queue

import pytest

from pidex.core.event import Event


def test_publish_and_get(bus):
    e = Event(source="test", event_type="T", severity="INFO", title="T", message="m")
    bus.publish(e)
    retrieved = bus.get(timeout=1)
    assert retrieved is e


def test_get_timeout_raises_empty(bus):
    with pytest.raises(queue.Empty):
        bus.get(timeout=0.1)


def test_qsize_empty(bus):
    assert bus.qsize == 0


def test_qsize_after_publish(bus):
    e = Event(source="test", event_type="T", severity="INFO", title="T", message="m")
    bus.publish(e)
    assert bus.qsize == 1


def test_multiple_events(bus):
    for i in range(5):
        e = Event(
            source="test",
            event_type=f"T{i}",
            severity="INFO",
            title="T",
            message="m",
        )
        bus.publish(e)
    assert bus.qsize == 5
    for i in range(5):
        retrieved = bus.get(timeout=1)
        assert retrieved.event_type == f"T{i}"
