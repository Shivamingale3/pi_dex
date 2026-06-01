from pidex.core.event import Event


class DedupManager:
    def __init__(self):
        self._last_keys: dict[str, str] = {}

    def is_new(self, event: Event) -> bool:
        key = event.dedup_key()
        last = self._last_keys.get(event.source)
        return last != key

    def record(self, event: Event) -> None:
        self._last_keys[event.source] = event.dedup_key()

    def reset(self, source: str) -> None:
        self._last_keys.pop(source, None)
