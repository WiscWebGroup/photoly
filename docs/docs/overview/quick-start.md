---
title: Quick start
sidebar_position: 2
---

Run Photoly with Docker Compose.

## Steps

1. Copy `.env.example` to `.env`.
2. Set the three PostgreSQL passwords: administrator, migration and application.
3. Set `ADMIN_EMAIL` and `ADMIN_PASSWORD`.
4. From the repository root, run:

   ```sh
   docker compose up -d --build
   ```

5. Open `http://localhost:5200`.
6. Sign in with `ADMIN_EMAIL` and `ADMIN_PASSWORD`.

The `backend` container holds `photoly` and PostgreSQL. The `web` container serves the frontend.

The API is at `http://localhost:8384`. Swagger UI is at `/docs/`. The OpenAPI document is at `/openapi.json`.

## Next steps

- Add photos: [Uploads](../features/uploads.md).
- Turn on semantic search: [AI management](../administration/ai-management.md). AI is off by default.
- Connect existing storage: [Real folders](../features/storage/real-folders.md) and [External storage](../features/storage/external-storage.md).
- Check volumes and remote access: [Docker Compose](../install/docker-compose.md).
- Check environment variables: [Configuration](../install/configuration.md).
