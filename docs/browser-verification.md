# Browser verification of the in-browser AI UI

Run in the Browser pane of the Claude desktop app (Chromium, WebGPU "apple metal-3", 8 threads), 1280x900.
Stack: `compose.go.yml` (project `photolybrowser`) with `AI_TEST_RUNTIME=1` (fake server embedder) and
`PHOTOLY_DATA` pointing at a copy of the real SigLIP2 ONNX files (revision `ba1f3b0843f2`, from the
local Hugging Face cache) so `/models/<rev>/siglip2-base-patch16-224/` served the real model. nginx web
service on :5200. Screenshots: `docs/screenshots/go-ai/`.

## Passed

| Check | Result | Screenshot |
|---|---|---|
| COOP/COEP | `Cross-Origin-Opener-Policy: same-origin`, `Cross-Origin-Embedder-Policy: credentialless` on `/`; `crossOriginIsolated === true`, `SharedArrayBuffer` available | - |
| Admin Inference card | Server / Browser / Let users choose per model; saved placements persist across reload; browser text model select; memory meter + Loaded / Fits / Doesn't fit chips; AI worker rows | 01 |
| Unload | Image model Unload: `POST .../image/unload` 200, row becomes "Not loaded · loads on demand", memory meter and chips update | 02 |
| Low-memory dialog | Real, not mocked: backend limited to 600 MB, choosing Server for Photo indexing returned 409 `low-memory`; dialog "needs about 0.9 GB, server has 0.6 GB free" with Keep in Browsers / Load Anyway. "Keep in Browsers" tested; "Load Anyway" not clicked | 03 |
| Account "AI on this device" | Saved-on-this-device list (Not downloaded), storage meter, GPU/threads detection, small/full model select; "Where AI runs for you" shows locked rows when admin fixes placement, and radio choices when "Let users choose" (choice persisted: `preference` in capabilities) | 04, 10 |
| Search model-loading card | "Downloading the search model", 0 -> 283 of 283 MB (real int8 text model), then results | 05 |
| Browser search, real model | After the real image vectors were produced in the browser, query "a white circle on a colorful gradient" returned all 3 photos from browser-computed text embedding | 08 |
| Upload row, model download | "Downloading the image model 372 of 372 MB" with "Keep this tab open..." note | 06 |
| Upload row, indexing | Real SigLIP2 vision model ran in the browser; 3 of 3 photos `ready` with 768-d vectors in `photo_embedding_state` | 07 |
| Fallback note | Truncated JPEG + good JPEG: "Indexed for search in this browser 2 of 2 - 1 photo will be indexed by the server instead." | 11 |
| Leaflet map under COEP | 20 of 20 `a.tile.openstreetmap.org` tiles loaded with credentialless COEP | 09 |
| Frontend checks | vitest 37 files / 220 tests pass, `pnpm typecheck` clean, `pnpm lint` clean (no source changes were needed) | - |

## Findings (no frontend code changed)

1. Model never reaches the Cache API in this browser pane. `Cache.put` of the 283 MB text model (and a plain
   streamed `cache.put(url, await fetch(url))` I tried by hand) fails with `UnknownError: Unexpected internal error`
   (transformers.js logs "Unable to add response to browser cache"). Config/tokenizer (<=34 MB) and the ORT wasm are
   cached. Consequence: every page load re-downloads the models and the card always says "Downloading", never
   "Loading from this device". This looks like a limit of the embedded browser, not of the app (the app's
   cache code path is unit-tested), but it was not confirmed in desktop Chrome/Safari. Worth a manual check there
   because the whole "download once" promise depends on it.
2. Service worker registration fails on every load: `Failed to register a ServiceWorker ... sw.js: An unknown error
   occurred when fetching the script`. `/sw.js` and the workbox file return 200 from nginx. Cause not determined
   (pane limitation vs COEP interaction); only console error besides the expected decode error of the truncated JPEG.
3. Admin page UX: pressing "Enable AI" while placement edits are unsaved bumps the config generation, so the
   draft is flagged stale ("The server configuration changed...") and Save is disabled until Discard. Existing
   behaviour, not specific to the new card. Also "Save AI settings" is enabled while the footer says "No unsaved changes".
4. A fresh AI-enabled admin still has no per-user AI access: Account/Search silently use the server path
   (`reason: not_enabled_for_user`) until the switch in User Management is turned on. Expected, but easy to trip on.

## Not exercised

- "Loading from this device" card and "Saved on this device" showing Downloaded/Remove (blocked by finding 1).
- The transient "Indexing in this browser - n of m" in-progress state: with 3 images it finished between screenshots;
  only the final "Indexed ... n of n" state was captured.
- Full-size text model (1.13 GB), fp16 vision, WebGPU path selection under load, WASM-only fallback, multi-thread timing.
- "Load Anyway" button, worker-died restart text, Server-side real ONNX inference (fake embedder only; real
  vectors came from the browser).
- Safari/Firefox, desktop Chrome, mobile viewport, dark mode, offline use, uploading ZIP/API (server-only indexing).
- Spot-check (1 in 100) re-verification of browser vectors by the server.
