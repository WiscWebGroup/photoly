![example workflow](https://github.com/WiscWebGroup/photoly/actions/workflows/codeql-analysis.yml/badge.svg)
# Photoly
<img src="https://raw.githubusercontent.com/Harold-y/photoly/main/Logo.png" height="160px" width="160px" />
<br>

This is a picture host application. There are demands for the self-host and reliable picture-host applications. Photoly provides a solution just right for that. Using Photoly, you can manage your pictures and photos with easy clicks, and categorize them using tags, galleries, and namespaces. We are also sure that the search functionality could make it easier to see the photos you are interested in. Moreover, Photoly provides API for you to access our functionalities for embedding in other applications.

Photoly是一款非常美丽的图床网页应用。随着人们对可靠的 自助图片托管应用需求增加，Photoly应运而生。使用 Photoly，您可以轻松管理图片和视频，并使用标签、图库和文件夹对图片和视频进行分类；利用搜索功能，充分探索您的媒体库。此外，Photoly 还提供 API，方便您将我们的功能嵌入到其他应用程序中。

## Functionalities
- Upload Photo / Video, and store them in folders
- Attach a Photo / Video with a Gallery or a Tag
- Share a Photo / Video using a share link, or a QR code
- Automatic record photo's Exif information, including GPS location, Camera Model, Lens Model, ISO, Shutter, Aperture, etc.
- Display a Photo Location Map
- Search Photos based on various conditions
- Create API to operate your photo using REST API

## Screenshots
<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/3304e591-7b8a-49d2-a483-61e04206e8dd" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/bd6cc12e-dc89-4e98-9cf7-2eaf35ba95b7" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/1518dfcc-eee2-4c5e-b686-aa1c5cdb57ae" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/4219d76f-5e9b-4587-ae4b-11379107fb35" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/44d94933-d7e7-4617-9fa5-4b2cc972da9c" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/23ae3e12-e465-4c3b-b642-c9b2de01c5a5" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/72e5adf6-d4bb-4f20-bd20-d6c7ea7921c8" style="width: 800px;" />

<img src="https://github.com/WiscWebGroup/photoly/assets/68500948/7acbc34e-30c0-4115-8607-9cfc8c9396ca" style="width: 800px;" />

## Specifications

DB: PostgreSQL 17 + pgvector 0.8.7 + Redis

Backend: Python 3.12 + Flask (application factory, Controller / Service / Entity)

Frontend: Vue 3 + TypeScript + Tailwind CSS + shadcn-vue (installable PWA)

## How to Run?

The backend has been migrated from Spring Boot to Flask. This version supports **fresh deployments**; it does not migrate legacy AES passwords or Redis sessions. The original Java implementation is available in Git at `e6619ab`.

### Docker

1. Copy `.env.example` to `.env` and set database, Redis and administrator passwords. Use separate PostgreSQL administrator, migration and application passwords. The Redis password must be URL-safe because it enters a connection URL.
2. Run `docker compose up -d --build` from this directory.
3. Open `http://localhost:5200`. The API is available at `http://localhost:8384`, with interactive [Swagger UI](http://localhost:8384/docs/) and an [OpenAPI 3.2 document](http://localhost:8384/openapi.json).
4. Sign in with `ADMIN_EMAIL` and `ADMIN_PASSWORD` from `.env`.

PostgreSQL bootstrap installs pgvector and creates separate schema-owner and application roles. The one-shot `migrate` service runs Alembic and seeds settings/admin before Gunicorn starts. The Web process only checks the schema and cannot execute DDL. Repeated startup preserves records, passwords and settings. PostgreSQL uses `postgres_data`; media remains in `./upload_data`. This is a fresh deployment, with no MySQL data import tool.

The administrator dashboard displays PostgreSQL, pgvector and the schema revision using `/admin/pingDatabase` and `/admin/databaseInfo`.

For access from another machine, set `PUBLIC_BASE_URL` to the externally reachable API URL and update the frontend `VITE_APP_BASE_URL` build setting before rebuilding. `localhost` is only appropriate for local access.

### Development and tests

See [backend setup and architecture](photoly_backend/README.md) and [API compatibility notes](photoly_backend/COMPATIBILITY.md).

Database tests require real PostgreSQL and an Alembic-created schema. Run from the repository root:

```sh
docker compose -p photoly-test -f compose.test.yml build tests backend migrate smoke ai-runtime ai-scheduler ai-maintenance
docker compose -p photoly-test -f compose.test.yml run --rm tests
docker compose -p photoly-test -f compose.test.yml run --rm smoke
docker compose -p photoly-test -f compose.test.yml down -v --remove-orphans
```

The integration database is `photoly_test`; the HTTP server uses `photoly_http_test`. Both are temporary and isolated from production. The final HTTP test deliberately stops its own Gunicorn server. Only run cleanup with the explicit test project and Compose file above.

## License
See [LICENSE](LICENSE).


## AI search and administration

See [deployment and API operations](docs/ai-backend-operations.md) for CPU/CUDA images, model lifecycle, user eligibility, search contracts, capacity and recovery. AI starts disabled. Open **AI Management** in the administrator navigation to configure CUDA/CPU inference, user access, profiles and background resource limits. Search supports natural-language queries. Scroll below a photo in the viewer to discover similar images, with paginated recommendations and same-viewer navigation. Administrators can set an optional similarity threshold; it is disabled by default. The image-upload search API remains available.
