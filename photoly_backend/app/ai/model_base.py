import gc
from .text_image_embedding_interface import TextImageEmbeddingInterface
from .types import AIError, EmbeddingBatch, normalized
from .registry import validate_spec


class TorchEmbedding(TextImageEmbeddingInterface):
    def prepare(self, profile, config):
        import torch
        validate_spec(profile)
        if (config.device, config.precision) not in {("cpu", "float32"), ("cuda", "bfloat16")}:
            raise AIError("unsupported_device_precision")
        if config.device == "cuda" and (not torch.cuda.is_available() or not torch.cuda.is_bf16_supported()):
            raise AIError("device_unavailable", 503)
        self.profile, self.config = profile, config
        self.dtype = torch.float32 if config.device == "cpu" else torch.bfloat16
        torch.set_num_threads(config.cpu_threads)

    def batch(self, features):
        data = features.detach().float().cpu().numpy()
        return EmbeddingBatch(normalized(data, self.profile["dimension"]), self.profile["dimension"], self.profile["fingerprint"])

    def close(self):
        import torch
        self.model = self.processor = None
        gc.collect()
        if torch.cuda.is_available():
            torch.cuda.empty_cache()
