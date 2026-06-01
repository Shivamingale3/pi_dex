from unittest.mock import mock_open, patch

from pidex.pollers.ram import RamPoller


def test_ram_reads_values():
    meminfo = """MemTotal:       8000000 kB
MemFree:        1000000 kB
MemAvailable:   4000000 kB
Buffers:         500000 kB
Cached:         2500000 kB
"""
    with patch("builtins.open", mock_open(read_data=meminfo)):
        total, available = RamPoller._read_meminfo()
        assert total == 8000000
        assert available == 4000000
