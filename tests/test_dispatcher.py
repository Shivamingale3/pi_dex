import threading
import time
from unittest.mock import MagicMock

from pidex.core.bus import EventBus
from pidex.core.cooldowns import CooldownManager
from pidex.core.dedup import DedupManager
from pidex.core.dispatcher import Dispatcher
from pidex.core.event import Event


def make_event(event_type="TEST_EVENT"):
    return Event(
        source="test",
        event_type=event_type,
        severity="INFO",
        title="Test",
        message="test",
    )


def test_dispatcher_sends_events():
    bus = EventBus()
    notifier = MagicMock()
    d = Dispatcher(bus=bus, notifier=notifier)
    stop = threading.Event()
    d.start(stop)
    e = make_event()
    bus.publish(e)
    time.sleep(0.2)
    notifier.send.assert_called_once_with(e)
    stop.set()


def test_dispatcher_drops_cooldowned_events():
    bus = EventBus()
    notifier = MagicMock()
    cooldowns = CooldownManager({"TEST_EVENT": 60})
    d = Dispatcher(bus=bus, notifier=notifier, cooldowns=cooldowns)
    stop = threading.Event()
    d.start(stop)
    e1 = make_event()
    bus.publish(e1)
    time.sleep(0.1)
    first_call_count = notifier.send.call_count
    bus.publish(e1)
    time.sleep(0.1)
    assert notifier.send.call_count == first_call_count
    stop.set()


def test_dispatcher_drops_duplicates():
    bus = EventBus()
    notifier = MagicMock()
    dedup = DedupManager()
    d = Dispatcher(bus=bus, notifier=notifier, dedup=dedup)
    stop = threading.Event()
    d.start(stop)
    e = make_event()
    bus.publish(e)
    time.sleep(0.1)
    bus.publish(e)
    time.sleep(0.1)
    notifier.send.assert_called_once_with(e)
    stop.set()


def test_dispatcher_sends_different_event_types():
    bus = EventBus()
    notifier = MagicMock()
    d = Dispatcher(bus=bus, notifier=notifier)
    stop = threading.Event()
    d.start(stop)
    e1 = make_event(event_type="EVENT_A")
    e2 = make_event(event_type="EVENT_B")
    bus.publish(e1)
    bus.publish(e2)
    time.sleep(0.2)
    assert notifier.send.call_count == 2
    stop.set()
