from unittest.mock import mock_open, patch

from pidex.pollers.temperature import TemperaturePoller


def test_temperature_reads_millidegrees():
    with patch("builtins.open", mock_open(read_data="75000\n")):
        poller = TemperaturePoller.__new__(TemperaturePoller)
        poller.interval = 30
        poller.warn = 75
        poller.critical = 85
        poller._state = "ok"

        value = poller.read_value()
        assert abs(value - 75.0) < 0.01
