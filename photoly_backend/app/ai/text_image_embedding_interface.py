from abc import ABC, abstractmethod
from typing import Literal, Sequence
from PIL.Image import Image
from .types import EmbeddingBatch, EmbeddingCapabilities, RuntimeConfig


class TextImageEmbeddingInterface(ABC):
    @abstractmethod
    def capabilities(self) -> EmbeddingCapabilities: ...
    @abstractmethod
    def load(self, profile: dict, runtime_config: RuntimeConfig) -> None: ...
    @abstractmethod
    def encode_texts(self, texts: Sequence[str], *, role: Literal["query", "document"]) -> EmbeddingBatch: ...
    @abstractmethod
    def encode_images(self, images: Sequence[Image], *, role: Literal["query", "document"]) -> EmbeddingBatch: ...
    @abstractmethod
    def close(self) -> None: ...
