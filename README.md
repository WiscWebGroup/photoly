![example workflow](https://github.com/WiscWebGroup/photoly/actions/workflows/codeql-analysis.yml/badge.svg)
# Photoly

Photoly is a self-hosted photo and video library. Organize media with folders, tags and galleries, share selected items, and search names, metadata and visual content. A documented REST API supports integrations.

Photoly是一款非常美丽的图床网页应用。随着人们对可靠的 自助图片托管应用需求增加，Photoly应运而生。使用 Photoly，您可以轻松管理图片和视频，并使用标签、图库和文件夹对图片和视频进行分类；利用搜索功能，充分探索您的媒体库。此外，Photoly 还提供 API，方便您将我们的功能嵌入到其他应用程序中。

## Functionalities
- Upload Photo / Video, and store them in folders
- Attach a Photo / Video with a Gallery or a Tag
- Share a Photo / Video using a share link, or a QR code
- Automatic record photo's Exif information, including GPS location, Camera Model, Lens Model, ISO, Shutter, Aperture, etc.
- Display a Photo Location Map
- Smart Search combines photo/video names, tags and galleries with optional semantic search; existing search modes remain available
- Similar photos and videos, including five-frame video indexing and per-media recommendation state
- On-demand multilingual text recognition with editable, saved results and copy support (Tesseract CPU)
- Optional Smart Tag camera/lens labels and time/location-based trip galleries
- Circular avatar crop preview with square 512 × 512 JPEG output
- Create API to operate your photo using REST API

## Screenshots
![The Photoly welcome page: “Your photos, on your terms” beside a photo grid with a search-by-meaning demo](docs/screenshots/welcome.webp)

## Specifications

One Go binary (`photoly`) plus PostgreSQL 17 with pgvector 0.8.7. AI search uses SigLIP2 on ONNX Runtime, in the server or in the user's browser (the administrator chooses where each model runs).

Frontend: Vue 3 + TypeScript + Tailwind CSS + shadcn-vue (installable PWA)

## How to Run?

1. Copy `.env.example` to `.env` and set the three PostgreSQL passwords (administrator, migration, application) and the administrator account.
2. Run `docker compose up -d --build` from this directory. The `backend` container holds `photoly` and PostgreSQL; `web` serves the frontend.
3. Open `http://localhost:5200`. The API is available at `http://localhost:8384`, with interactive [Swagger UI](http://localhost:8384/docs/) and an [OpenAPI document](http://localhost:8384/openapi.json).
4. Sign in with `ADMIN_EMAIL` and `ADMIN_PASSWORD` from `.env`.

`photoly serve` migrates the schema and seeds settings and the administrator on start; repeated starts preserve records, passwords and settings. PostgreSQL data lives in the `postgres_data` volume, media in `./upload_data`, downloaded model weights in `photoly_models`. Never use `down -v` on a production deployment.

For access from another machine, open `http://<server-address>:5200`. The frontend uses its same-origin `/api` proxy, so browser API requests follow the address used to open the site. Set `PUBLIC_BASE_URL` to an externally reachable API URL for backend-generated share links (for example, `http://<server-address>:5200/api`).

### AI

AI starts disabled. In **AI Management**, enable it, choose per model whether it runs on the server, in browsers, or by user choice, grant user access and activate a search profile. See [AI operations](docs/ai-backend-operations.md).

### Upgrading an existing PostgreSQL deployment

Back up the database and media together, rebuild and restart. A database created by the earlier Python release must be at Alembic `0008_upload_activity` before the first start; `photoly serve` adopts it without changing the schema.

### Development and tests

Frontend: `docker compose -f compose.dev.yml up -d --build`, then `pnpm dev` in `photoly_frontend_vue` (see [frontend development](photoly_frontend_vue/README.md)). Server: Go 1.24 in `photoly_server`; `go test ./...` needs `TEST_DATABASE_URL` for the database-backed tests and the ONNX Runtime library for the AI worker test. Database access goes through sqlc: edit `photoly_server/internal/db/queries/*.sql` and run `devbox run -- sqlc generate` in `photoly_server` (CI fails if `internal/db/gen` is stale).

## License
See [LICENSE](LICENSE).


## Search, recognition and smart organization

**Smart Search** is the default search mode: one text box, up to 100 results, with name/tag/gallery matches shown first and semantic results fused when available. Semantic failure preserves text results. See [AI operations](docs/ai-backend-operations.md) for indexing, quotas, model lifecycle and mixed photo/video search.

Scroll down in the viewer to load **Similar photos and videos**. Each item has independent results and scroll state. Returning within 30 seconds of leaving restores it; after that it starts collapsed. Viewing an item continuously does not expire it. **Back to top** collapses recommendations and clears that item's cache.

**Extract Text** runs only when requested. Tesseract recognizes simplified/traditional Chinese, English and Japanese on CPU, independently of AI permissions or the model runtime. Results are saved and can be edited or copied. See [OCR limits and operation](docs/ocr.md).

**Smart Tag** requires global AI, administrator Smart Tag, user AI permission and personal Smart Tag to be enabled. Both Smart Tag switches default off. It runs inside the existing scheduler process, with no extra container or model inference. Turning it off retains editable results. See [rules, controls and offline city attribution](docs/smart-tag.md).

### Real folders and personal S3

The local library mirrors each user’s actual UUID directory. External changes are scanned automatically; users can connect one personal S3 bucket as a cloud folder alongside local content. This release requires a fresh library and refuses old flat-layout media. See [layout, synchronization, S3, downloads and deployment](docs/real-folders.md).

### Bulk uploads and storage groups

Files, folders and ZIPs can be uploaded in unlimited-count batches. The global Upload activity panel stays available while browsing. Each account inherits a default 50 GB group quota, with administrator-managed groups and personal overrides. See [uploads, storage, migration and worker operations](docs/uploads-storage.md).

### Dates and folder details

**Organize → Dates** brings together the signed-in user's photos across all folders. Latest/Oldest
uses EXIF capture time, falling back to upload time; records with neither appear last under Undated.
Year and month headings show photo totals. Years start expanded; only the first month in the chosen
sort order opens automatically. Each month card shows up to ten thumbnail previews (up to five on phones), loaded only near the viewport; collapsed months do not fetch full photo pages. Multiple months can stay open. Photos load near the viewport in
100-image pages per day; offscreen pages retain their layout but release their rendered cards.
The ordinary page cache is bounded (20 pages, with visible pages retained while in use).
The floating photo calendar shows monthly and daily counts. Choose a year/month and click a day to
jump directly to it without loading earlier photos. Hide the calendar to leave a small restore button;
this preference is remembered on the device. Undated photos remain in a separate expandable section.
Changing the sort, editing a photo or completing an upload refreshes the counts and loaded content.
Selection survives page recycling; Select All applies to loaded photos, not the entire library.
The photo viewer's **Open folder** link returns to the containing folder.

Right-click a folder and choose **Details** to view recursive photo,
video and subfolder counts. Storage separates original files from thumbnails and excludes avatars
and temporary files. Missing expected files are reported as incomplete totals. Places are approximate
nearby towns derived from GPS; cameras and lenses come from photo metadata, with counts per photo.

The new owner-only endpoints are `GET /photo/getDateSummary`, `GET /photo/getByDate`, `GET /photo/getMonthPreview` and `GET /namespace/getDetails`; see `/docs`
for parameters, pagination and response schemas. Dates itself requires no schema changes. Media tools use migration `0007_media_tools`; persistent upload activity controls use `0008_upload_activity`.

### Media tools

Right-click an image and choose **Format Convert** to save a JPEG, PNG or GIF copy in its actual folder. GIFs animate in the viewer; newly uploaded Apple Live Photo pairs have a LIVE playback control and original-pair ZIP download. Upload pickers accept pasted images with Cmd+V / Ctrl+V. Administrators can enable **Online Download** in Settings; the download icon appears immediately left of Upload. See [format support, pairing, security limits and upgrade instructions](docs/media-tools.md).

See [external local mounts and WebDAV](docs/external-storage.md) for multi-source setup, Docker mounts, scanning and file retention.

Personal Storage supports multiple WebDAV and SMB connections, plus administrator-assigned local mounts. Folder pages show indexing progress and offer recursive folder refresh. See [external storage deployment and testing](docs/external-storage.md).
