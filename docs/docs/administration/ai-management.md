---
title: AI management
sidebar_position: 1
---

Photoly runs SigLIP2 (image and text) on ONNX Runtime. It runs either in the server's `ai-worker` child process or in the user's browser. Photoly downloads nothing until you enable AI.

## Where each model runs

Go to **Admin > AI Management > Where AI runs**. Set the placement for each model: photo indexing and search.

- **Server**
- **Browser**
- **Let users choose**. Each user then picks under **Account > AI on this device**.

### Server placement

- The server loads a model only while a user needs it there. It frees the model when all users choose the browser.
- The memory meter shows what fits.
- If a model may not fit, Photoly asks before you choose Server (`low-memory`). **Load Anyway** forces it.
- Photoly restarts a dead worker on the next request (`worker-died`).
- **Unload** frees a loaded model at once. It reloads on demand.

### Browser placement

- Photoly serves browser models from `/models/<revision>/`.
- The browser caches them. Users can remove them under **Account > AI on this device**.
- The browser search model is `small` (int8, 318 MB) or `full` (1.13 GB).

For indexing and search behavior, see [Search and similar media](../features/search.md).

## Operation

Commands:

- `photoly serve`
- `photoly migrate`
- `photoly ai-worker` (`serve` starts it)
- `photoly verify`
- `photoly health`

Storage and runtime:

- Weights are in `PHOTOLY_DATA`. This is the `photoly_models` volume.
- `ONNXRUNTIME_LIB` points to the ONNX Runtime library.
- CPU works everywhere. The CUDA provider needs a CUDA build of the image.

## Related

Sizes and upgrade notes are in [Media tools](media-tools.md), [Extract Text](extract-text.md) and [Smart Organization](smart-organization.md) operations.

## Model controls and API clients

The model cards expose separate load/unload actions for image, text and People models. A successful settings change is a desired configuration, not proof that a model is loaded; inspect the worker/model status. A low-memory load warning includes the estimated requirement and available memory. Loading anyway is an explicit override, not a persistent change to memory limits.

The current server uses ONNX Runtime. Device choices are CPU, CUDA and CoreML where the provider is available; the supported precision is float32. Provider initialization can fall back to CPU, reported by `device_used` and `device_note`. Qwen profile creation and the old MPS/PyTorch precision combinations are not supported by this server.

Integration clients should read `/ai/capabilities` for placement, the current browser fingerprint, model base URL and pinned files. See [API](../api.md) for indexing, vector submission and model load/unload contracts. Upload embedding success reports `state`; existing-photo embedding success reports `status`. Both can report a queued/waiting outcome instead of a stored vector.
