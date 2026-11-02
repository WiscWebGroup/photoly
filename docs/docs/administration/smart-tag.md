---
title: Smart Tag operations
sidebar_position: 6
---

For rules and user controls, see [Smart Tag](../features/smart-tag.md).

## Deploy and enable

1. Rebuild and restart. Migration `0005_smart_organization` adds five companion tables and content and vector invalidation triggers. It does not rewrite existing vectors, media or catalogs.
2. Go to **Admin → AI → Settings and resource limits**. Enable global AI and **Enable Smart Tag**.
3. Grant the account AI access.
4. Start or resume the existing background scheduler.
5. In personal settings, enable **Smart Tag for my library**.

Both Smart Tag switches are off by default.

Personal settings show progress, wait reasons and failures. **Refresh and retry** requests another pass. It also retries failed metadata reads.

Deployment uses the existing Compose services and migration steps. See [AI management](ai-management.md).

### Gates

Photoly checks all four gates again before each automatic commit:

1. Global AI
2. Administrator Smart Tag setting
3. Account AI access
4. User opt-in

Processing pauses for:

- Maintenance
- Scheduler pause or stop
- CPU, RAM or disk pressure

An offline or unloaded model does not block EXIF work or existing-vector work. If you disable Smart Tag, generated results and manual edits stay. You can still restore a deleted item while automatic processing is off.

### Privacy

- No command sources `.env`.
- No external service receives location data.
- The city snapshot is fixed and ships with the application.

## Scheduling and limits

- Discovery checks every 60 seconds. It rotates through up to 25 opted-in users. It queues one deduplicated `smart` job per user. Pending jobs rotate between users.
- Each cooperative batch does at most 25 work units in about 5 seconds.
- Order in a batch: admin operations, then bounded Smart Tag work, then existing vector scheduling.
- EXIF subprocess deadlines use the time left in the batch.
- Three failed metadata reads mark the media failed. **Refresh** retries it.
- Missing EXIF is a normal skip.

Metadata inspection is incremental. Trip rebuilding streams cached facts in time order through durable phases: anchor, missing-GPS, item and membership. It never loads a user's whole vector collection.

- A user's grouping can need several bounded passes. This is likely after a late upload or a profile change.
- Progress is eventual. Photoly publishes gallery membership in batches.
- A metadata or vector upload during a pass raises its revision. Another pass follows.
- Persistent leases recover interrupted work after a restart.
- Memory admission is local. It does not depend on runtime or GPU availability.

## API

All routes need `HRD-Token`. They act only on the current user.

| Route | Purpose |
| --- | --- |
| `GET /ai/smart/settings` | Opt-in, availability, reason, check interval |
| `PATCH /ai/smart/settings` | `{ "enabled": true/false }` |
| `GET /ai/smart/status` | Inspected, total and failed counts, phase, pending state, wait reason |
| `POST /ai/smart/refresh` | Schedule a new evaluation and retry metadata failures |
| `GET /ai/smart/dismissed?after=ID` | Up to 50 suppressed items, with a continuation cursor |
| `POST /ai/smart/items/ID/restore` | Restore the dismissal lineage of the item |

Other changes:

- Admin settings accept `smart_tag_enabled` with the existing generation fence.
- Admin status adds `smartTag`.
- Tag and gallery lists, and photo association reads, accept `includeSmart=true`. Only then do they add a `smart` object, or `null` for an ordinary item. Without it, legacy response fields do not change.
- Existing edit, member and delete endpoints record smart overrides.
- The OpenAPI document includes these contracts.

## Validation, 2026-10-16

Deterministic tests cover:

- All 16 gate combinations
- User isolation and legacy response compatibility
- Device normalization
- Explicit and inferred time zones
- City snapshot checksums
- No-GPS and no-time cases
- The 36-hour and 14-day boundaries
- Hong Kong–Shenzhen continuity
- Split and merge dismissal inheritance
- Rename and cover preservation
- Explicit membership
- Immediate invalidation on overwrite and delete
- Offline scheduling
- Finite retry and manual refresh
- Permission revocation during EXIF reads

Both 768-dimension and 2048-dimension profiles test photo and video maximum-frame matching. Ambiguous ties are rejected. Comparison reads are limited to 20 anchors.

An isolated PostgreSQL test with **10,002 media** ran ten 25-item metadata batches:

- Mean: **0.140 s**
- Maximum: **0.149 s**
- No measured RSS growth

The test used stubbed metadata to isolate scheduling and database overhead. A separate test covers real EXIF descriptor reading. This measures bounded work. It does not predict real EXIF I/O or end-to-end time for a large library. After each batch, the same process goes on to its normal vector work.
