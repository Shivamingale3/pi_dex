import logging
import threading
import time
from datetime import datetime

from pidex.core.bus import EventBus
from pidex.core.constants import EVENT_INTERFACE_DOWN, EVENT_INTERFACE_UP, SEVERITY_WARN, SOURCE_NETWORK
from pidex.core.event import Event
from pidex.sources.base import BaseSource

logger = logging.getLogger(__name__)


class NetworkSource(BaseSource):
    def run(self, stop_event: threading.Event) -> None:
        try:
            from pyroute2 import IPRoute

            ipr = IPRoute()
        except Exception:
            logger.error("pyroute2 not available — skipping network source")
            return

        try:
            ipr.bind()
            while not stop_event.is_set():
                for msg in ipr.get():
                    event = _parse_netlink(msg)
                    if event is not None:
                        self._bus.publish(event)
        except Exception:
            logger.exception("Netlink error")
        finally:
            ipr.close()


def _parse_netlink(msg: dict) -> Event | None:
    event = msg.get("event")
    if event == "RTM_DELLINK":
        ifindex = msg.get("index")
        attrs = {k: v for k, v in msg.get("attrs", [])}
        name = attrs.get("IFLA_IFNAME", f"if{ifindex}")
        ts = _extract_ts(msg)
        return Event(
            source=SOURCE_NETWORK,
            event_type=EVENT_INTERFACE_DOWN,
            severity=SEVERITY_WARN,
            title="Interface Removed",
            message=f"{name} removed",
            timestamp=ts,
        )

    if event != "RTM_NEWLINK":
        return None

    ifindex = msg.get("index")
    attrs = {k: v for k, v in msg.get("attrs", [])}
    name = attrs.get("IFLA_IFNAME", f"if{ifindex}")
    operstate = attrs.get("IFLA_OPERSTATE", "unknown")
    ts = _extract_ts(msg)

    if operstate == "UP":
        return Event(
            source=SOURCE_NETWORK,
            event_type=EVENT_INTERFACE_UP,
            severity=SEVERITY_WARN,
            title="Interface Up",
            message=f"{name} is UP",
            timestamp=ts,
        )
    elif operstate == "DOWN":
        return Event(
            source=SOURCE_NETWORK,
            event_type=EVENT_INTERFACE_DOWN,
            severity=SEVERITY_WARN,
            title="Interface Down",
            message=f"{name} is DOWN",
            timestamp=ts,
        )

    return None


def _extract_ts(msg: dict) -> datetime:
    ts = msg.get("timestamp")
    if ts is None:
        return datetime.now()
    return datetime.fromtimestamp(ts)
