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

DB: MySQL + Redis

Backend: Python 3.12 + Flask (application factory, Controller / Service / Entity)

Frontend: Vue 3 + TypeScript + Tailwind CSS + shadcn-vue (installable PWA)

## How to Run?

The backend has been migrated from Spring Boot to Flask. This version supports **fresh deployments**; it does not migrate legacy AES passwords or Redis sessions. The original Java implementation is available in Git at `e6619ab`.

### Docker

1. Copy `.env.example` to `.env` and set database, Redis and administrator passwords. Use URL-safe alphanumeric database/Redis passwords because they are embedded in connection URLs.
2. Run `docker compose up -d --build` from this directory.
3. Open `http://localhost:5200`. The API is available at `http://localhost:8384`, with interactive [Swagger UI](http://localhost:8384/docs/) and an [OpenAPI 3.2 document](http://localhost:8384/openapi.json).
4. Sign in with `ADMIN_EMAIL` and `ADMIN_PASSWORD` from `.env`.

The backend waits for MySQL and Redis, initializes missing tables/settings/admin once, then serves Flask with Gunicorn. Repeated startup preserves existing records and does not reset passwords. MySQL uses the Compose `mysql_data` named volume; media stays in `./upload_data`. This Compose does not mount or reset the old `database/mysql-data` directory. The frontend API calls are unchanged.

For access from another machine, set `PUBLIC_BASE_URL` to the externally reachable API URL and update the frontend `VITE_APP_BASE_URL` build setting before rebuilding. `localhost` is only appropriate for local access.

### Development and tests

See [backend setup and architecture](photoly_backend/README.md) and [API compatibility notes](photoly_backend/COMPATIBILITY.md).

```sh
cd photoly_backend
uv sync --python 3.12
uv run pytest -q --cov=app
```

The local suite uses SQLite and fakeredis. Install ExifTool first (`brew install exiftool` on macOS, `apt install libimage-exiftool-perl` on Debian). For real MySQL/Redis and HTTP/Gunicorn acceptance tests, run from the repository root:

```sh
docker compose -p photoly-test -f compose.test.yml build tests backend smoke
docker compose -p photoly-test -f compose.test.yml run --rm tests
docker compose -p photoly-test -f compose.test.yml run --rm smoke
docker compose -p photoly-test -f compose.test.yml down -v --remove-orphans
```

Run integration and smoke tests sequentially: integration tests reset the isolated test database, and the final smoke test intentionally stops its disposable backend. These commands never mount production database or upload directories.

## License
See [LICENSE](LICENSE).
