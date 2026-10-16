from dataclasses import dataclass
from typing import Literal, Sequence
import numpy as np


class AIError(Exception):
    def __init__(self, code, status=400):
        super().__init__(code)
        self.code, self.status = code, status


@dataclass(frozen=True)
class RuntimeConfig:
    device: str = "cpu"
    precision: str = "float32"
    cpu_threads: int = 4

    def __post_init__(self):
        if (self.device, self.precision) not in (("cpu", "float32"), ("cuda", "bfloat16")):
            raise AIError("unsupported_device_precision")
        if type(self.cpu_threads) is not int or not 1 <= self.cpu_threads <= 64:
            raise AIError("invalid_cpu_threads")


@dataclass(frozen=True)
class EmbeddingCapabilities:
    provider: str
    dimension: int
    text_tokens: int
    modalities: tuple = ("text", "image")
    devices: tuple = ("cpu", "cuda")


@dataclass(frozen=True)
class EmbeddingBatch:
    vectors: np.ndarray
    dimension: int
    fingerprint: str


def normalized(vectors, dimension):
    data = np.asarray(vectors, dtype=np.float32)
    if data.ndim != 2 or data.shape[1] != dimension or not np.isfinite(data).all():
        raise AIError("invalid_embedding", 502)
    norms = np.linalg.norm(data, axis=1, keepdims=True)
    if (norms <= 0).any():
        raise AIError("zero_embedding", 502)
    return data / norms
