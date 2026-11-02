# AI operations

SigLIP2 (image and text) runs on ONNX Runtime, either in the server's `ai-worker` child process or in the user's browser. Nothing is downloaded until AI is enabled.

## Where each model runs

**Admin > AI Management > Where AI runs** sets placement per model (photo indexing, search): Server, Browser, or Let users choose. With "Let users choose", each user picks under Account > AI on this device.

- The server loads a model only while some user needs it there, and frees it when everyone has chosen the browser.
- The memory meter shows what fits. Choosing Server for a model that may not fit asks first (`low-memory`); **Load Anyway** forces it. A dead worker is restarted on the next request (`worker-died`).
- **Unload** frees a loaded model at once; it reloads on demand.
- Browser models are served from `/models/<revision>/`, cached in the browser, and removable under Account > AI on this device. The browser search model is `small` (int8, 318 MB) or `full` (1.13 GB).

## Indexing and search

Photos and videos are embedded after upload, by the server queue or by the uploading browser. Browser vectors are validated and stored with `source = 'client'`; when idle, the server re-embeds a random 1% and drops a user's browser vectors whose cosine similarity falls below 0.98. Photos no allowed side can embed stay `pending` until a browser or the server can.

Search runs against the active profile. A browser-computed query vector is sent to `POST /ai/search/vector`; server placement embeds the text on the server. Changing the model or preprocessing creates a new profile that builds in the background while search keeps using the old one.

## Operation

- `photoly serve|migrate|ai-worker|verify|health`; `ai-worker` is started by `serve`.
- Weights are stored under `PHOTOLY_DATA` (the `photoly_models` volume); `ONNXRUNTIME_LIB` points at the ONNX Runtime library.
- Devices: CPU everywhere; the CUDA provider needs a CUDA build of the image.
- Sizes and upgrade notes for video frames, OCR and Smart Tag are in [media-tools.md](media-tools.md), [ocr.md](ocr.md) and [smart-tag.md](smart-tag.md).
