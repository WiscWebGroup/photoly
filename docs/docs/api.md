---
title: API
sidebar_position: 6
---

Photoly has a REST API.

- Swagger UI: `/docs/`. Example: `http://localhost:8384/docs/`.
- OpenAPI document: `/openapi.json`.

## Try an authenticated operation

1. Open Swagger UI on your backend origin. The standard Compose mapping is `http://localhost:8384/docs/`; use your deployment's API origin or proxy prefix when different.
2. Call `POST /user/signIn` with your account email and password.
3. Copy the returned `token`, choose **Authorize**, and paste it into **SessionToken**. This sends `HRD-Token`; do not add `Bearer`.
4. Expand an operation, choose **Try it out**, supply its documented inputs, then **Execute**. GET, POST, PUT, PATCH and DELETE are supported. Write operations act on real data, including delete and restore operations.

Authorization is kept in memory and clears on reload. Swagger assets are bundled with the server. During instance maintenance, ordinary routes (including the documentation UI) can return HTTP 503; use the maintenance page to inspect status.

## Authentication and responses

| Credential | Where it goes | Purpose |
| --- | --- | --- |
| Account session | `HRD-Token` via SessionToken | Account and admin operations; admin role is checked separately |
| Scoped API token | `Authorization: Bearer YOUR_TOKEN` via ApiCredential | C/R/D integration operations only |
| Legacy path/query token | The parameter specified by that operation | Media/download sessions or share tokens; not interchangeable |

Legacy operations usually return HTTP 200 even for business failures. Check `msgCode` as well as HTTP status. AI routes and the newer scoped read APIs use actual HTTP error statuses. Some credential operations return raw text, arrays or binary content. A legacy `pageNum` means total pages, while newer cursor APIs return their documented cursor fields.

`GET /healthz` reports database/controller readiness, including maintenance state. `GET /demo` exposes only the intentionally public demo configuration. Browser AI uses `/ai/preference`, `/ai/index`, `/ai/index/pending`, `/ai/search/vector`, photo/upload embedding endpoints and pinned `/models/...` assets; the complete contracts are in Swagger. Never substitute an API credential for an account session on these routes.

## Feature endpoints

- Uploads and quotas: [Groups and quotas](administration/groups-and-quotas.md)
- Extract Text: [Processing and API](administration/extract-text.md)
- Smart Tag: [API](administration/smart-tag.md)
- External storage: [Storage deployment](administration/storage-deployment.md)
- Dates and folder details: [Dates and folder details](features/dates-and-folders.md)

## Scoped API tokens

Settings → API tokens creates account-owned credentials with **C** (upload), **R**
(read), and **D** (delete) permissions. These are distinct from login sessions and
public share tokens. API documents lists each operation, required permission,
parameters, and response. Copy address uses the deployment's actual API prefix;
Copy cURL defaults to `YOUR_TOKEN`. Selecting a token explicitly includes it in
the copied Authorization header. Replace folder IDs, media UUIDs and file paths
before executing a command.

The seven existing credential operations now also accept `Authorization: Bearer
YOUR_TOKEN` on their addresses without the final `/{token}` segment. Existing
path-token addresses remain compatible. Upload uses multipart `file` + `photo`;
batch upload uses repeated `files` + **`photosStr`**. Render is **GET**, returns the
original Content-Type and supports byte ranges. Successful legacy operations
retain their original plain-text or JSON-array response formats; header aliases
return HTTP authentication and business-error statuses.

New **R** APIs use real HTTP statuses and the `{msgCode,t}` envelope (except binary
thumbnails):

| Method and path | Result |
| --- | --- |
| `GET /cred/media` | Media page; optional `nsId`, `after`, `limit` |
| `GET /cred/media/{uuid}` | Media metadata and effective categories |
| `GET /cred/media/{uuid}/thumbnail` | Current static thumbnail |
| `GET /cred/categories` | Counts and at most three previews per category |
| `GET /cred/categories/{category}/media` | Category media page |

Pages are ordered by descending media ID, default to 100 items, and permit at
most 200. Pass `nextAfter` as `after` until it is null. Only owned media on enabled
sources is visible. The new metadata responses exclude Notes, People, share
credentials and physical storage paths. Invalid/revoked credentials return 401;
insufficient scope returns 403. Category names are `video`, `panorama`, `live`,
`animated`, `screenshot`, and `portrait`.

The catalog is maintained in `photoly_frontend_vue/src/lib/credentialEndpoints.json`.
After changing it, run `python3 scripts/update-credential-openapi.py`; contract
tests check the catalog against OpenAPI and the credential integration tests
exercise registered routes. Token management and admin APIs still require an
account session and are not available to API credentials.
