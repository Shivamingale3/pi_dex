from unittest.mock import patch, MagicMock

from pidex.pollers.disk import DiskPoller


def test_disk_read_value():
    mock_stat = MagicMock()
    mock_stat.f_frsize = 4096
    mock_stat.f_blocks = 1000000
    mock_stat.f_bavail = 250000
    mock_stat.f_bfree = 300000

    with patch("os.statvfs", return_value=mock_stat):
        poller = DiskPoller.__new__(DiskPoller)
        poller._mount = "/"
        poller.interval = 300
        poller.warn = 85
        poller.critical = 95
        poller._state = "ok"

        value = poller.read_value()
        expected = (1000000 * 4096 - 250000 * 4096) / (1000000 * 4096) * 100.0
        assert abs(value - expected) < 0.01


def test_disk_make_event_includes_mount():
    from pidex.core.bus import EventBus

    bus = EventBus()
    poller = DiskPoller(bus=bus, interval=300, warn=85, critical=95, mount="/data")

    e = poller._make_event("WARN", 90.0)
    assert "/data" in e.message
    assert "90.0%" in e.message
