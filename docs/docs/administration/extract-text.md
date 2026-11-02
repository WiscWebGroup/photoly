---
title: Extract Text operations
sidebar_position: 5
---

For the user guide, see [Extract Text](../features/extract-text.md).

## Deployment

The backend image installs Tesseract 5 and these Debian `tessdata_fast` packages:

- `tesseract-ocr-eng`
- `tesseract-ocr-chi-sim`
- `tesseract-ocr-chi-tra`
- `tesseract-ocr-jpn`

Photoly installs language data at build time. It downloads nothing at runtime. The native MPS environment and the inference image get no OCR packages.

To upgrade, rebuild the images and run the migration service before you restart. From the repository root, for the base deployment:

```sh
docker compose stop backend
docker compose build backend migrate web
docker compose run --rm migrate
docker compose up -d backend web
```

For CPU, CUDA and MPS deployments:

- Use the same Compose files and profiles as your existing deployment.
- Rebuild and restart the active scheduler and maintenance services. They share the application image and check the current schema.
- Keep the existing inference runtime and model configuration.
- Do not use test Compose commands on production data.

### Migration

Migration `0004_photo_ocr` follows `0003_video_vectors`. It adds `photo_ocr`. It does not change existing vectors.

- A database trigger removes OCR results when `photo.content_version` changes.
- Foreign-key cascade removes them when a photo is deleted.
- Rename, move, tagging and gallery membership keep the text.
- A downgrade discards saved OCR text only.

### Check the languages

```sh
docker compose exec backend tesseract --version
docker compose exec backend tesseract --list-langs
```

The list must contain `chi_sim`, `chi_tra`, `eng` and `jpn`. The panel reports missing binaries and data. Photoly never falls back to another language silently.

If you run the Flask backend outside Docker, install the same Tesseract language data locally.

## Processing

- Language order is fixed: `chi_sim+chi_tra+eng+jpn`.
- Engine: LSTM (`--oem 1`). Page segmentation: automatic (`--psm 3`).
- CPU only, with `OMP_THREAD_LIMIT=1`.
- Image limit: **20 MiB** and **40 million pixels**.
- Decodable formats: JPEG, PNG, WebP, GIF, TIFF, BMP, ICO and PPM. The file must also have a supported photo extension.
- SVG, RAW and other encodings return a clear error.
- Photoly uses only the first animation frame or document page.

### Steps

1. The backend reads a stable snapshot of the original file.
2. It applies EXIF orientation.
3. It composites transparency onto white.
4. It scales down to a maximum side of 4096 px. It never enlarges.
5. It removes temporary files.

### Time and concurrency

- A disposable worker limits Pillow and Tesseract together to 45 seconds. Recognition has a separate 30-second timeout.
- On timeout, Photoly kills the worker process group.
- A PostgreSQL session advisory lock allows one OCR operation across all processes that share a database. A second request gets an immediate busy response. There is no queue.
- Recognition holds no long database transaction.

## API

All endpoints use `HRD-Token` and the `{msgCode, t}` envelope. HTTP 200 alone does not mean success.

| Endpoint | Behavior |
| --- | --- |
| `GET /photo/ocr?photoId=...` | Returns `photoId`, `contentVersion`, `revision`, `text`, `languages`, `engineVersion`, `updatedAt`. If nothing is saved: `revision: 0, text: null`. Saved empty text is `""`. |
| `POST /photo/ocr/recognize` | JSON: `photoId`, `contentVersion`, `revision`. Recognizes and saves atomically, only if both versions still match. |
| `PUT /photo/ocr` | Same version fields plus `text`. Accepts empty text and up to 100,000 characters. No null characters. |

Updates raise the revision.

Errors:

- `409`: conflict. The client keeps the draft and offers **Refresh saved version**. This reads new version data and keeps the draft. The user can then save it on purpose.
- `429`: busy.
- `504`: timeout.
- `503`: engine or languages unavailable.
- Other explicit errors: missing original, invalid image and limits.

For the full contract, read the OpenAPI definition.

### Stale requests

- When you switch images, the browser aborts requests and ignores late responses.
- A server recognition that is already running can still finish and save its own photo.
- Before saving, the server checks ownership, photo existence, content version and text revision again.
- A late response cannot overwrite stale image contents or a newer edit from another tab.

## Measured resources

Test setup, 2026-10-16:

- Apple M3, 16 GB unified memory
- Linux ARM64 Docker
- Tesseract **5.5.0**, CPU only
- Four language packs
- One operation at a time

| Fixture | Time | Peak summed process RSS | Recognition |
| --- | --- | --- | --- |
| Simplified Chinese, 1500 × 400 | 0.865 s | 237.2 MiB | Exact text |
| Traditional Chinese, 1500 × 400 | 0.922 s | 240.6 MiB | Exact text |
| English, 1500 × 400 | 0.890 s | 240.3 MiB | Exact text |
| Japanese, 1500 × 400 | 1.011 s | 242.8 MiB | Exact text |
| Mixed scripts, 1500 × 400 | 1.010 s | 242.3 MiB | English, Japanese and Traditional kept. The Simplified phrase on the mixed Chinese line was misread as `sera`. |
| Existing real rocket photograph | 0.829 s | 230.6 MiB | Empty result. Small scene text was not recovered. |

These are small-sample measurements. They are not memory caps or accuracy guarantees.

- RSS is sampled every 10 ms. It sums the disposable Python worker and its Tesseract child. Shared pages can count twice.
- It excludes the web server and databases.
- Larger originals need more decode memory.
- GPU VRAM use is zero. Apple Silicon CPU processes still use the shared memory pool.
- Natural scenes, tiny lettering and mixed-script lines are less reliable than clear screenshot-style text. Manual correction is by design.

The five screenshot-style fixtures were generated for this project. Photoly renders the text with the local Hiragino Sans GB font. It does not redistribute font files.

Frontend component tests and Chromium, WebKit and mobile tests cover:

- Explicit recognition
- Edits
- Clipboard behavior
- Reopening
- Independent photos
- History navigation
- Confirmation and retry paths
