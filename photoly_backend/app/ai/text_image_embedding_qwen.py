"""Qwen's public message template and last-valid-token pooling, image/text only.
Reference: https://github.com/QwenLM/Qwen3-VL-Embedding (Apache-2.0).
No remote code, paths, video readers or URL fetchers are exposed by this adapter.
"""
from .model_base import TorchEmbedding
from .types import EmbeddingCapabilities, AIError


class QwenEmbedding(TorchEmbedding):
    def capabilities(self):
        return EmbeddingCapabilities("qwen3_vl", 2048, 8192)

    def load(self, profile, runtime_config):
        from transformers import AutoProcessor
        from transformers.models.qwen3_vl.modeling_qwen3_vl import Qwen3VLPreTrainedModel, Qwen3VLModel
        # Preserve the checkpoint's model.* prefix without allocating a language-model head.
        class EmbeddingModel(Qwen3VLPreTrainedModel):
            _checkpoint_conversion_mapping = {}
            def __init__(self, config):
                super().__init__(config)
                self.model = Qwen3VLModel(config)
                self.post_init()
            def forward(self, **inputs):
                return self.model(**inputs, use_cache=False)
        self.prepare(profile, runtime_config)
        self.processor = AutoProcessor.from_pretrained(profile["model_id"], use_fast=False, revision=profile["revision"], padding_side="right", trust_remote_code=False)
        self.model = EmbeddingModel.from_pretrained(profile["model_id"], revision=profile["revision"], torch_dtype=self.dtype,
                                                   attn_implementation="sdpa", low_cpu_mem_usage=True, trust_remote_code=False).to(runtime_config.device).eval()

    def encode(self, values, role, images=False):
        import torch
        from qwen_vl_utils import process_vision_info
        conversations = []
        for value in values:
            content = {"type": "image", "image": value, "min_pixels": 4096, "max_pixels": self.profile["max_pixels"]} if images else {"type": "text", "text": value}
            conversations.append([
                {"role": "system", "content": [{"type": "text", "text": self.profile[role + "_instruction"]}]},
                {"role": "user", "content": [content]},
            ])
        texts = self.processor.apply_chat_template(conversations, add_generation_prompt=True, tokenize=False)
        picture_inputs, _ = process_vision_info(conversations, image_patch_size=16)
        inputs = self.processor(text=texts, images=picture_inputs, padding=True, truncation=False, do_resize=False, return_tensors="pt")
        if inputs.input_ids.shape[1] > self.profile["max_tokens"]:
            raise AIError("text_token_limit", 413)
        inputs = inputs.to(self.config.device)
        with torch.inference_mode():
            output = self.model(**inputs).last_hidden_state
            mask = inputs.attention_mask
            columns = mask.shape[1] - mask.flip(dims=[1]).argmax(dim=1) - 1
            features = output[torch.arange(output.shape[0], device=output.device), columns]
            return self.batch(features)

    def encode_texts(self, texts, *, role):
        return self.encode(texts, role)

    def encode_images(self, images, *, role):
        return self.encode(images, role, images=True)
