---
title: Upgrading
sidebar_position: 3
---

## Standard upgrade

1. Back up the database and the media together.
2. Rebuild and restart:

   ```sh
   docker compose up -d --build
   ```

A database from the earlier Python release must be at Alembic `0008_upload_activity` before the first start. `photoly serve` adopts it. The schema does not change.

## Release notes

- Text recognition image and migration: [Extract Text operations](../administration/extract-text.md).
- Media tasks and migration `0007_media_tools`: [Media tools operations](../administration/media-tools.md).
- Smart Tag migration: [Smart Tag operations](../administration/smart-tag.md).
- External storage migration 9: [Local mounts, WebDAV and SMB](../features/storage/external-storage.md).
- S3 migration 12: [Personal S3](../features/storage/personal-s3.md).

## Old flat layout

Libraries from the old flat layout cannot upgrade in place. Use a fresh library.

Photoly refuses to start with old flat-layout media. `photoly migrate` applies the schema to a fresh library. `photoly serve` also migrates on start.
