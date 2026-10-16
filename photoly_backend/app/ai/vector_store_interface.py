from abc import ABC, abstractmethod


class VectorStoreInterface(ABC):
    @abstractmethod
    def search(self, session, user_id, profile, vector, *, limit=20, namespace_id=None, gallery_id=None, exclude=None, timeout_ms=5000, min_score=None): ...
