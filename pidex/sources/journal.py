import json
import logging
import os
import select
import subprocess
import threading

from pidex.core.bus import EventBus
from pidex.core.event import Event
from pidex.sources.base import BaseSource

logger = logging.getLogger(__name__)


class JournalSource(BaseSource):
    def __init__(self, bus: EventBus, config: dict):
        super().__init__(bus, config)
        self._parsers: list[callable] = []

    def register(self, parser: callable) -> None:
        self._parsers.append(parser)

    def run(self, stop_event: threading.Event) -> None:
        reader = _open_journal()
        if reader is None:
            return
        _run_loop(reader, stop_event, self._bus, self._parsers)


def _run_loop(reader, stop_event, bus, parsers) -> None:
    while not stop_event.is_set():
        for entry in reader:
            if stop_event.is_set():
                break
            for parser in parsers:
                try:
                    event = parser(entry)
                    if event is not None:
                        bus.publish(event)
                except Exception:
                    logger.exception("Parser %s failed", parser.__name__)

        reader.wait(1.0)

    reader.close()


# ── Native path (systemd.journal C module) ──────────────────────────


def _open_native():
    try:
        from systemd import journal

        reader = journal.Reader()
        reader.seek_tail()
        reader.get_previous()
        logger.info("journald: using native systemd.journal")
        return _NativeWrapper(reader)
    except ImportError:
        return None
    except Exception:
        logger.exception("Failed to open native journald")
        return None


class _NativeWrapper:
    def __init__(self, reader):
        self._reader = reader
        self._iter = iter(reader)

    def __iter__(self):
        return self

    def __next__(self):
        return next(self._iter)

    def wait(self, timeout: float) -> None:
        self._reader.wait(timeout)

    def close(self) -> None:
        self._reader.close()


# ── Subprocess path (journalctl -f) ─────────────────────────────────


def _open_subprocess():
    try:
        proc = subprocess.Popen(
            ["journalctl", "-f", "--output=json", "--since=now"],
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
        )
        logger.info("journald: using journalctl subprocess")
        return _SubprocessWrapper(proc)
    except FileNotFoundError:
        logger.error("journald: journalctl not found on this system")
        return None
    except Exception:
        logger.exception("Failed to start journalctl subprocess")
        return None


class _SubprocessWrapper:
    def __init__(self, proc: subprocess.Popen):
        self._proc = proc
        self._buf = bytearray()
        self._fd = proc.stdout.fileno()
        self._poller = select.poll()
        self._poller.register(self._fd, select.POLLIN)
        os.set_blocking(self._fd, False)

    def __iter__(self):
        return self

    def __next__(self) -> dict:
        while True:
            idx = self._buf.find(b"\n")
            if idx >= 0:
                line = self._buf[:idx]
                del self._buf[: idx + 1]
                if line.strip():
                    return json.loads(line.decode("utf-8"))
                continue

            self._drain()
            if not self._buf:
                raise StopIteration

    def _drain(self) -> None:
        try:
            chunk = os.read(self._fd, 65536)
            self._buf.extend(chunk)
        except BlockingIOError:
            pass

    def wait(self, timeout: float) -> None:
        if timeout > 0:
            self._poller.poll(int(timeout * 1000))
        self._drain()

    def close(self) -> None:
        self._proc.kill()
        self._proc.wait(timeout=5)


# ── Factory ─────────────────────────────────────────────────────────


def _open_journal():
    native = _open_native()
    if native is not None:
        return native

    logger.info("native systemd.journal unavailable — trying journalctl subprocess")
    sub = _open_subprocess()
    if sub is not None:
        return sub

    logger.error("no journald backend available — install: apt install python3-systemd")
    return None
