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

Sizes and upgrade notes are in [Media tools](media-tools.md), [Extract Text](extract-text.md) and [Smart Tag](smart-tag.md) operations.
