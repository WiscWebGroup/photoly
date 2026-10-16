from .model_base import TorchEmbedding
from .types import EmbeddingCapabilities, AIError


class Siglip2Embedding(TorchEmbedding):
    def capabilities(self):
        return EmbeddingCapabilities("siglip2", 768, 64)

    def load(self, profile, runtime_config):
        from transformers import AutoModel, AutoProcessor
        self.prepare(profile, runtime_config)
        self.processor = AutoProcessor.from_pretrained(profile["model_id"], use_fast=False, revision=profile["revision"], trust_remote_code=False)
        self.model = AutoModel.from_pretrained(profile["model_id"], revision=profile["revision"], torch_dtype=self.dtype,
                                              attn_implementation="sdpa", low_cpu_mem_usage=True, trust_remote_code=False).to(runtime_config.device).eval()

    def encode_texts(self, texts, *, role):
        import torch
        texts = [text.lower() for text in texts]
        lengths = self.processor.tokenizer(texts, padding=False, truncation=False)["input_ids"]
        if any(len(x) > 64 for x in lengths):
            raise AIError("text_token_limit", 413)
        inputs = self.processor(text=texts, padding="max_length", max_length=64, truncation=False, return_tensors="pt").to(self.config.device)
        with torch.inference_mode():
            return self.batch(self.model.get_text_features(**inputs))

    def encode_images(self, images, *, role):
        import torch
        inputs = self.processor(images=list(images), return_tensors="pt").to(device=self.config.device, dtype=self.dtype)
        with torch.inference_mode():
            return self.batch(self.model.get_image_features(**inputs))
