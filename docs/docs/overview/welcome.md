---
title: Welcome
sidebar_position: 1
slug: /
---

Photoly is a self-hosted photo and video library.

You can organize media with folders, tags and galleries. You can share selected items and search names, metadata and visual content. A REST API is available for integrations.

![The Photoly welcome page](../../screenshots/welcome.webp)

## Highlights

- Upload files, folders or ZIPs into real folders on disk.
- Add tags and galleries. Share with links or QR codes.
- See EXIF data, GPS location on a map, and camera and lens details.
- Find media with [Smart Search](../features/search.md). Semantic search is optional.
- Read text in photos with [Extract Text](../features/extract-text.md).
- Group photos by camera, lens and trip with [Smart Tag](../features/smart-tag.md).
- Connect [S3, WebDAV, SMB and local mounts](../features/storage/external-storage.md).
- Install the web app as a PWA.

## Architecture

- One Go binary: `photoly`.
- PostgreSQL 17 with pgvector.
- SigLIP2 on ONNX Runtime for AI search. It runs in the server or in the browser. The administrator chooses.
- Frontend: Vue 3, TypeScript, Tailwind CSS and shadcn-vue.

## Where to next

- New to Photoly? Read the [Quick start](quick-start.md).
- Run a server? Read [Install](../install/docker-compose.md) and [Administration](../administration/ai-management.md).
- Build an integration? Read the [API](../api.md) and the [Developer](../developer/development.md) guide.
