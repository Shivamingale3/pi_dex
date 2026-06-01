from pidex.core.dedup import DedupManager
from pidex.core.event import Event


def make_event(source="test", event_type="T", title="T", message="m"):
    return Event(
        source=source,
        event_type=event_type,
        severity="INFO",
        title=title,
        message=message,
    )


def test_first_event_is_new():
    dm = DedupManager()
    assert dm.is_new(make_event())


def test_same_event_is_duplicate():
    dm = DedupManager()
    e = make_event()
    dm.record(e)
    assert not dm.is_new(e)


def test_different_event_is_new_after_record():
    dm = DedupManager()
    dm.record(make_event(event_type="A"))
    assert dm.is_new(make_event(event_type="B"))


def test_reset():
    dm = DedupManager()
    e = make_event()
    dm.record(e)
    assert not dm.is_new(e)
    dm.reset("test")
    assert dm.is_new(e)


def test_different_sources_independent():
    dm = DedupManager()
    e1 = make_event(source="src1", event_type="T", title="T", message="m")
    e2 = make_event(source="src2", event_type="T", title="T", message="m")
    assert dm.is_new(e1)
    assert dm.is_new(e2)
    dm.record(e1)
    assert not dm.is_new(e1)
    assert dm.is_new(e2)


def test_consecutive_alternating_events():
    dm = DedupManager()
    a = make_event(event_type="A")
    b = make_event(event_type="B")
    dm.record(a)
    dm.record(b)
    assert dm.is_new(a)
