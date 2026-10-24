# ONNX (Go worker) vs PyTorch SigLIP2: vector parity

Measured 2026-10-24 with `photoly_server/scripts/onnx_vs_pytorch.sh` (see that file for the exact method).

- ONNX side: `photoly_server/internal/ai` code path (`prepareImage` + `EmbedImage`, `EmbedText`), fp32,
  ONNX Runtime 1.29 CPU inside the `build` stage of `photoly_server/Dockerfile` (linux/arm64), pinned
  `onnx-community/siglip2-base-patch16-224-ONNX@ba1f3b0`.
- PyTorch side: the existing profile (`google/siglip2-base-patch16-224@75de2d5`, HF `AutoProcessor` use_fast=False,
  float32 CPU, Pillow decode via `app.ai.preprocessing.decode_image`, text lowercased, padded to 64), torch 2.8.0.
- Inputs: 62 photos (4 PNG/JPG in `photoly_backend/tests/fixtures/ai` + 58 JPEGs in `upload_data`; photos over the
  app's 20 MB / 40 MP limits are excluded because both paths refuse them) and 24 queries (English and Chinese).
  Both sides were already cached locally; nothing was downloaded.

## Results

| Metric | min | mean | p1 |
| --- | --- | --- | --- |
| Image vector cosine (62 photos) | 0.998881 | 0.999813 | 0.998970 |
| Text vector cosine (24 queries) | 1.000000 | 1.000000 | - |

Lowest image cosines: 0.998881, 0.999027, 0.999437, 0.999553, 0.999568 (all upload_data JPEGs). The PNG fixtures
are ~1.000000 (identical pixels after preprocessing), so the gap comes from JPEG decoding (Go `image/jpeg` vs libjpeg)
and resize rounding, not from the model.

Search set (62 images, 24 queries, top-10 vs the all-PyTorch ranking):

- All ONNX (new image vectors and new query vectors): mean overlap 0.975, worst query 0.9.
- ONNX query vectors against the existing PyTorch image vectors: overlap 1.0 for every query.

## Verdict (docs/go-rewrite-plan.md section 4)

Threshold is min cosine >= 0.999 for every photo. Measured min is 0.99888, so the strict rule is **not met**:
re-index under the new profile (build, activate) as planned. The miss is small (0.0001 below the bar, mean 0.9998)
and limited to JPEG-decoder noise, and text vectors are exact. Searching old PyTorch image vectors with ONNX query
vectors gives identical top-10 lists here, so "keep existing vectors" would be safe in practice if the
threshold is relaxed to about 0.998 or the Go JPEG path is made closer to libjpeg; that is a decision, not a measured pass.

## If "keep existing vectors" is chosen: fingerprint fields that must match

Vectors are tied to the fingerprint = sha256 of the canonical JSON of the spec. The Go profile must emit exactly the
Python spec (`photoly_backend/app/ai/registry.py::profile_spec("siglip2")`):
`provider=siglip2`, `model_id=google/siglip2-base-patch16-224`, `revision=75de2d55ec2d0b4efc50b3e9ad70dba96a7b2fa2`,
`dimension=768`, `query_instruction=""`, `document_instruction=""`, `max_tokens=64`, `max_pixels=50176`,
`preprocessing="exif-rgb-white-first-frame-v1"`, `metric=cosine`, `normalization=l2-fp32`, and **no other keys**.
Current Go fingerprint `cce33d93...` differs from Python `1d101e06...` because Go changes `preprocessing`
(`exif-white-rgb-224-bilinear-v1`) and adds `runtime`, `onnx_repo`, `onnx_revision`, `dtype`, `vision_sha256`,
`text_sha256`. Runtime details would then have to live outside the fingerprint (e.g. in the profile row's metadata).
The browser path shares the same preprocessing string, so it must follow the same decision.
