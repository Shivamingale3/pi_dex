from abc import ABC, abstractmethod

from pidex.core.event import Event


class BaseNotifier(ABC):
    @abstractmethod
    def send(self, event: Event) -> None:
        ...
