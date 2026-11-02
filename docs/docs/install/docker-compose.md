---
title: Docker Compose
sidebar_position: 1
---

Docker Compose is the supported way to run Photoly. For the shortest path, see the [Quick start](../overview/quick-start.md).

## Data

`photoly serve` migrates the schema on start. It also seeds settings and the administrator. A repeated start keeps records, passwords and settings.

| Data | Location |
| --- | --- |
| PostgreSQL | `postgres_data` volume |
| Media | `./upload_data` |
| Model weights | `photoly_models` volume |

:::danger
Never run `docker compose down -v` on a production deployment. It deletes the database volume.
:::

## Access from another machine

1. Open `http://<server-address>:5200`.
2. Set `PUBLIC_BASE_URL` to an API URL that users can reach. Example: `http://<server-address>:5200/api`.

The frontend uses a same-origin `/api` proxy. Browser API requests follow the address in the address bar. Backend-generated share links use `PUBLIC_BASE_URL`.

## Enable AI

AI is off by default. To turn it on:

1. Open **AI Management**.
2. Enable AI.
3. For each model, choose server, browser or user choice.
4. Grant user access.
5. Activate a search profile.

For details, see [AI management](../administration/ai-management.md).
