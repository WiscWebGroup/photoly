from abc import ABC, abstractmethod


class SchedulerInterface(ABC):
    @abstractmethod
    def tick(self) -> None: ...
    @abstractmethod
    def run(self) -> None: ...
