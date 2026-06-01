import os
import tempfile
import unittest
from unittest.mock import patch

from pidex.pollers.cpu import CpuPoller


def test_cpu_poller_zero_delta():
    stat_content = "cpu  100 0 50 8000 100 0 0 0 0 0\n"
    f = tempfile.NamedTemporaryFile(mode="w", delete=False, suffix=".txt")
    f.write(stat_content)
    f.close()
    try:
        with patch("builtins.open", unittest.mock.mock_open(read_data=stat_content)):
            poller = CpuPoller.__new__(CpuPoller)
            poller.interval = 15
            poller.warn = 80
            poller.critical = 95
            poller._state = "ok"

            poller._prev = poller._read_cpu()
            value = poller.read_value()
            assert isinstance(value, float)
    finally:
        os.unlink(f.name)

