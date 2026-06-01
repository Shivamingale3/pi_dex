import logging
import threading
from datetime import datetime

from pidex.core.bus import EventBus
from pidex.core.constants import (
    EVENT_CONTAINER_DIED,
    EVENT_CONTAINER_RESTARTED,
    EVENT_CONTAINER_STARTED,
    EVENT_CONTAINER_STOPPED,
    SEVERITY_CRITICAL,
    SEVERITY_INFO,
    SEVERITY_WARN,
    SOURCE_DOCKER,
)
from pidex.core.event import Event
from pidex.sources.base import BaseSource

_EVENT_MAP = {
    "start": (EVENT_CONTAINER_STARTED, SEVERITY_INFO, "Container Started"),
    "stop": (EVENT_CONTAINER_STOPPED, SEVERITY_WARN, "Container Stopped"),
    "die": (EVENT_CONTAINER_DIED, SEVERITY_CRITICAL, "Container Died"),
    "restart": (EVENT_CONTAINER_RESTARTED, SEVERITY_INFO, "Container Restarted"),
}

logger = logging.getLogger(__name__)


class DockerSource(BaseSource):
    def __init__(self, bus: EventBus, config: dict):
        super().__init__(bus, config)
        self._watch = config.get("containers", {}).get("watch", [])

    def run(self, stop_event: threading.Event) -> None:
        client = _docker_client()
        if client is None:
            return

        while not stop_event.is_set():
            try:
                for event in client.events(decode=True):
                    if stop_event.is_set():
                        return

                    parsed = _parse_event(event, self._watch)
                    if parsed is not None:
                        self._bus.publish(parsed)
            except Exception:
                logger.exception("Docker events stream error — reconnecting in 5s")
                stop_event.wait(5)


def _docker_client():
    try:
        import docker

        return docker.from_env()
    except ImportError:
        logger.error(
            "Docker SDK not installed — install: pip install docker "
            "(or: apt install python3-docker)"
        )
        return None
    except Exception:
        logger.exception(
            "Docker daemon unreachable — is Docker running and is your user in the docker group?"
        )
        return None


def _parse_event(raw: dict, watch: list[str]) -> Event | None:
    etype = raw.get("Type")
    action = raw.get("Action")
    if etype != "container":
        return None

    mapping = _EVENT_MAP.get(action)
    if mapping is None:
        return None

    name = raw.get("Actor", {}).get("Attributes", {}).get("name", "unknown")
    container_id = raw.get("id", "")[:12]

    if watch and name not in watch:
        return None

    event_type, severity, title = mapping
    ts = raw.get("time")
    timestamp = datetime.fromtimestamp(ts) if ts else datetime.now()
    return Event(
        source=SOURCE_DOCKER,
        event_type=event_type,
        severity=severity,
        title=title,
        message=f"Container '{name}' ({container_id})",
        timestamp=timestamp,
    )
