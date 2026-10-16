# Extract Text

Open a photo and expand **Extract Text**, immediately below **Common Metadata**. Opening the panel reads saved text but does not run recognition. **Recognize Text** uses Tesseract on the backend CPU and saves successful results. Edit the multiline text and click **Save**; **Copy** copies the current draft even before saving. Clipboard permission failures leave the text selectable for manual copying.

**Recognize Again** asks before replacing saved text and drafts. Failed recognition keeps both. Closing, opening a similar image, returning through viewer history, and entering the image editor prompt to save, discard, or cancel when text has unsaved changes. A failed save keeps the draft. Browser unload uses the browser's native unsaved-change prompt. Logging out clears the local state and cancels client requests.

Text is private to the photo owner, even when the image itself is shared. OCR does not require AI search to be enabled and does not load embedding models. No video OCR, automatic batch processing, OCR bounding boxes, or search integration is included.

## Deployment

The backend application image installs Tesseract 5 and Debian's compact `tessdata_fast` language packages: `tesseract-ocr-eng`, `tesseract-ocr-chi-sim`, `tesseract-ocr-chi-tra`, and `tesseract-ocr-jpn`. No OCR packages are added to the native MPS environment or the inference image. Language data is installed while building, with no runtime download.

Rebuild the application images and run the existing migration service before restarting application services. From the repository root, for the base deployment:

```sh
docker compose stop backend
docker compose build backend migrate web
docker compose run --rm migrate
docker compose up -d backend web
```

For CPU/CUDA/MPS deployments, use the same Compose files and profiles as the existing deployment, and rebuild/restart the active scheduler and maintenance services too: their shared application image checks the current schema. Keep the existing inference runtime and model configuration. Do not use test Compose commands against production data.

Migration `0004_photo_ocr` follows `0003_video_vectors`. It adds `photo_ocr` without modifying existing vectors. A database trigger removes OCR results when `photo.content_version` changes; foreign-key cascading removes them on photo deletion. Renaming, moving, tagging, and gallery membership preserve text. Downgrading this migration discards saved OCR text only.

Check installed languages:

```sh
docker compose exec backend tesseract --version
docker compose exec backend tesseract --list-langs
```

The list must contain `chi_sim`, `chi_tra`, `eng`, and `jpn`. Missing binaries/data are reported in the panel; no silent language fallback occurs. Developers running the Flask backend outside Docker must install those same Tesseract language resources locally.

## Processing and API

The language order is fixed at `chi_sim+chi_tra+eng+jpn`, with LSTM (`--oem 1`) and automatic page segmentation (`--psm 3`). Only CPU inference is used, with `OMP_THREAD_LIMIT=1`. Images must be at most **20 MiB / 40 million pixels**. JPEG, PNG, WebP, GIF, TIFF, BMP, ICO and PPM content can be decoded; the media must also have a supported photo extension. SVG, RAW and other unsupported encodings return a clear error. Only the first animation frame or document page is used.

The backend reads a stable original-file snapshot, applies EXIF orientation, composites transparency onto white, and scales down to a maximum side of 4096 pixels without enlarging. Temporary files are always removed. A disposable worker bounds Pillow and Tesseract together to 45 seconds, with a separate 30-second recognition timeout. Timeout kills the worker process group. A PostgreSQL session advisory lock on a separate autocommit connection allows one OCR operation across application processes sharing a database, with immediate busy responses rather than a waiting queue. Recognition does not hold a long database transaction.

All endpoints use `HRD-Token` and the legacy `{msgCode, t}` envelope. HTTP 200 alone does not indicate success.

| Endpoint | Behavior |
| --- | --- |
| `GET /photo/ocr?photoId=...` | Returns `photoId`, `contentVersion`, `revision`, `text`, `languages`, `engineVersion`, `updatedAt`. No saved result has `revision: 0, text: null`; saved empty text is `""`. |
| `POST /photo/ocr/recognize` | JSON: `photoId`, `contentVersion`, `revision`. Recognizes and atomically saves only if both versions still match. |
| `PUT /photo/ocr` | Same version fields plus `text`; accepts empty text, up to 100,000 characters, excluding null characters. |

Updates increment the revision. Conflicts (`409`) keep the current draft and offer **Refresh saved version**, which reads new version metadata without discarding the draft; the user can then deliberately save it. Other explicit errors cover busy (`429`), timeout (`504`), unavailable engine/languages (`503`), missing original, invalid image and limits. Read the OpenAPI definition for the complete request/response contract.

Switching images aborts browser requests and ignores late responses. An already-running server recognition may still complete and save its own photo. Before committing, the server checks ownership, photo existence, content version and text revision again. Neither stale image contents nor another tab's newer edits can be overwritten by a late response.

## Verification and measured resources

2026-10-16, Apple M3 host with 16 GB unified memory, Linux ARM64 Docker, Tesseract **5.5.0**, CPU-only, four language packs, one operation at a time:

| Fixture | Time | Peak summed process RSS | Recognition |
| --- | --- | --- | --- |
| Simplified Chinese, 1500 × 400 | 0.865 s | 237.2 MiB | Exact text |
| Traditional Chinese, 1500 × 400 | 0.922 s | 240.6 MiB | Exact text |
| English, 1500 × 400 | 0.890 s | 240.3 MiB | Exact text |
| Japanese, 1500 × 400 | 1.011 s | 242.8 MiB | Exact text |
| Mixed scripts, 1500 × 400 | 1.010 s | 242.3 MiB | English/Japanese/Traditional retained; Simplified phrase on the mixed Chinese line was misrecognized as `sera` |
| Existing real rocket photograph | 0.829 s | 230.6 MiB | Empty result; small scene text was not recovered |

These are small-sample measurements, not memory caps or accuracy guarantees. RSS is sampled every 10 ms and summed for the disposable Python worker and its Tesseract child; shared pages may be counted twice. It excludes the web server and databases. Larger originals need more decode memory. Extra GPU VRAM usage is zero; Apple Silicon CPU processes still use the shared system memory pool. Natural scenes, tiny lettering, and mixed-script lines remain less reliable than clear screenshot-style text; manual correction is intentional.

The five screenshot-style fixtures were generated for this project (text rendered with the local Hiragino Sans GB font); no font files are redistributed. The existing real-photo fixture's attribution is in `photoly_backend/tests/fixtures/ai/manifest.json`. To repeat OCR measurements without a database:

```sh
docker compose -f compose.test.yml build tests
docker compose -f compose.test.yml run --rm --no-deps tests python scripts/ocr_smoke.py
```

Backend tests use the isolated PostgreSQL/Redis environment in `compose.test.yml`. Coverage includes persistence, ownership, optimistic conflicts, content overwrite/delete races, migration rollback, global concurrency, timeout termination, cleanup, limits, first-frame behavior, orientation, transparency and real multilingual recognition. Frontend component and Chromium/WebKit/mobile tests cover explicit recognition, edits, clipboard behavior, reopening, independent photos, history navigation, confirmation and retry paths.
