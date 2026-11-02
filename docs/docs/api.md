---
title: API
sidebar_position: 6
---

Photoly has a REST API.

- Swagger UI: `/docs/`. Example: `http://localhost:8384/docs/`.
- OpenAPI document: `/openapi.json`.

Endpoints use the `HRD-Token` header for authentication. They respond in the `{msgCode, t}` envelope. HTTP 200 alone does not mean success. Check `msgCode`.

## Feature endpoints

- Uploads and quotas: [Groups and quotas](administration/groups-and-quotas.md)
- Extract Text: [Processing and API](administration/extract-text.md)
- Smart Tag: [API](administration/smart-tag.md)
- External storage: [Storage deployment](administration/storage-deployment.md)
- Dates and folder details: [Dates and folder details](features/dates-and-folders.md)
