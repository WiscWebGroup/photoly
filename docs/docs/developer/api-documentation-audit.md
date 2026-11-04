---
title: API and documentation audit
---

This audit compares the current Go server and Vue interface with their documentation. It changes the contract and guides, not business permissions, database schema or deployed services.

## Route coverage

The previous specification described 234 operations. Actual registration and the maintenance controller expose **244 documented method/path operations** after normalizing parameter names and trailing-slash aliases. No removed business operation remained in the specification. Compatibility routes such as `/admin/pingMySQL` and `/admin/pingRedis` still exist; both now check PostgreSQL and remain documented as aliases.

Ten previously missing operations were added:

| Method | Path |
| --- | --- |
| POST | `/ai/search/vector` |
| POST | `/ai/index` |
| GET | `/ai/index/pending` |
| PUT | `/ai/photos/{photoId}/embedding` |
| PUT | `/ai/preference` |
| POST | `/admin/ai/models/{kind}/load` |
| POST | `/admin/ai/models/{kind}/unload` |
| PUT | `/uploads/{batch_id}/items/{item_id}/embedding` |
| GET | `/demo` |
| GET | `/models/{rev}/{model}/{file}` |

The last `file` parameter is a wildcard suffix that can contain subdirectories. The documentation HTML, its static assets and the OpenAPI JSON itself are explicitly excluded from the business-operation inventory. Implicit HEAD handling for GET routes is not counted as a separate operation.

## Contract corrections

- Replaced Flask/Redis readiness claims with Go/PostgreSQL and maintenance-controller behavior. Health responses include their envelope and maintenance state.
- Documented browser AI placement, fingerprints, model files, worker status and indexing-source counts. Corrected nullable runtime arrays, ONNX Runtime providers and float32 precision; removed unsupported Qwen creation choices.
- Added actual error envelopes for People, scan failures, resource monitoring and maintenance/backup. Preserved the distinction between HTTP status and legacy `msgCode`, including HTTP 202 with `msgCode: 200` for maintenance transitions.
- Corrected upload byte sums to decimal strings and fixed outdated photo-update, move and gallery examples.
- Enabled Swagger execution for GET, POST, PUT, PATCH and DELETE, keeping authorization in memory only.
- Updated the credential/resource generator so regenerating its contracts preserves these corrections.

## Guides and images

New guides cover galleries and descriptions, maps and locations, and video playback. Existing guides now explain pinned folders/default location, backup controls, browser AI choices and upload limits. Deployment pages no longer instruct users to run nonexistent Flask commands or separate migration/worker services. Older OCR measurements are identified as historical rather than measurements of the current implementation.

Seven screenshots show these key workflows with fictional accounts, generated media and schematic map tiles. No real account or personal library was accessed. Existing welcome and point-note images were also inspected. New PNGs contain no text, EXIF or location metadata.

## Validation and limits

The repeatable checks are described in [Development and tests](development.md#openapi-contract-checks). They cover both directions of the route inventory, reference/parameter consistency, generator reproducibility, 551 documented examples, 313 library/server response samples and 23 controller response samples. The contract remains OpenAPI 3.2.0.

The Go checks use a disposable PostgreSQL/pgvector database and temporary media. Browser checks verify the bundled Swagger UI, authorization and all five supported methods; operation execution in that browser test is intercepted, while Go tests exercise real handlers. Docs build/link checks and desktop/mobile previews cover the new guides and their images.

These checks do not exercise every possible request combination. Actual model loading/inference, real external storage services and a new full backup/restore round trip are outside this documentation audit; their existing integration suites remain available. No production deployment or publication is performed by the audit.
