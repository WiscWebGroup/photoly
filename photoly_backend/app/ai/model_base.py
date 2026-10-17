import gc
from .devices import DEVICE_PRECISIONS
from .text_image_embedding_interface import TextImageEmbeddingInterface
from .types import AIError, EmbeddingBatch, normalized
from .registry import validate_spec


class TorchEmbedding(TextImageEmbeddingInterface):
    def download(self, profile, progress):
        from huggingface_hub import HfApi, hf_hub_download, snapshot_download
        # Fixed registry revisions only. Download inference assets, never remote code.
        progress("downloading")
        try:
            files = HfApi().list_repo_files(profile["model_id"], revision=profile["revision"])
        except Exception:
            # A complete cached snapshot remains usable without network access.
            try:
                self.model_path = snapshot_download(profile["model_id"], revision=profile["revision"], local_files_only=True)
            except Exception as error:
                raise AIError("model_download_failed", 503) from error
            return
        weights = ".safetensors" if any(f.endswith(".safetensors") for f in files) else ".bin"
        files = sorted(f for f in files if f.endswith((weights, ".json", ".txt", ".model", ".jinja", ".tiktoken")))
        progress("downloading", 0, len(files))
        for count, filename in enumerate(files, 1):
            try:
                hf_hub_download(profile["model_id"], filename, revision=profile["revision"])
            except Exception as error:
                raise AIError("model_download_failed", 503) from error
            progress("downloading", count, len(files))
        self.model_path = snapshot_download(profile["model_id"], revision=profile["revision"],
                                            local_files_only=True, allow_patterns=files)

    def prepare(self, profile, config):
        import torch
        validate_spec(profile)
        if DEVICE_PRECISIONS.get(config.device) != config.precision:
            raise AIError("unsupported_device_precision")
        if config.device == "cuda" and (not torch.cuda.is_available() or not torch.cuda.is_bf16_supported()):
            raise AIError("device_unavailable", 503)
        if config.device == "mps" and not torch.backends.mps.is_available():
            raise AIError("device_unavailable", 503)
        self.profile, self.config = profile, config
        self.dtype = getattr(torch, config.precision)
        torch.set_num_threads(config.cpu_threads)

    def batch(self, features):
        data = features.detach().float().cpu().numpy()
        return EmbeddingBatch(normalized(data, self.profile["dimension"]), self.profile["dimension"], self.profile["fingerprint"])

    def close(self):
        import torch
        self.model = self.processor = None
        gc.collect()
        device = getattr(getattr(self, "config", None), "device", None)
        if device == "cuda" and torch.cuda.is_available():
            torch.cuda.empty_cache()
        elif device == "mps" and torch.backends.mps.is_available():
            torch.mps.empty_cache()
