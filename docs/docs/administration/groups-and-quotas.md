---
title: Groups and quotas
sidebar_position: 2
---

**User Management** has a **Users** tab and a **Groups** tab.

## Groups

- Each account belongs to one group.
- **Max Storage** is a limit **per member**. It is not a shared pool.
- The protected default group starts at **50 GB (50,000,000,000 bytes)**. New accounts join it.
- You can rename the default group or change its quota.
- To delete another group, choose a replacement group for its members.

## Personal override

A personal override sets a finite limit or unlimited storage. Turn it off to inherit the group limit again.

## What counts

The limit includes:

- All local user files
- Local derivatives
- Avatars

The limit excludes:

- External local and WebDAV originals
- Personal S3 objects
- Remote cache
- Temporary sources
- PostgreSQL
- AI indexes

Administrators follow their own effective limits.

## Changes

- Changes to group membership and limits run in sequence with file changes. Concurrent uploads cannot go over a user's limit.
- If you reduce a quota, existing files stay.
- An over-quota account can read, download and delete. It can also replace content, if usage does not go up.

## API

Legacy photo and credential upload routes still work. They apply the same quotas.

New upload endpoints use the `{msgCode,t}` envelope. Check `msgCode`, even for HTTP 200.

| Route | Purpose |
| --- | --- |
| `/uploads` | Create and list batches |
| `/{batch_id}/items` | Register and list entries |
| `/{batch_id}/items/{item_id}/content` | Receive raw bytes |
| `finish`, `cancel`, `retry` | Act on a batch |

- Batch and item UUIDs make retries idempotent.
- All upload queries and actions need the owning login session.
- Admin group and personal quota operations are in Swagger UI at `/docs/`.
- `/user/getStorage` reports the effective account limit. `null` means unlimited. It also reports the source and the over-quota state.
