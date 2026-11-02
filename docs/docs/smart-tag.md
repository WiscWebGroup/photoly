# Smart Tag

Smart Tag organizes existing photos and videos into camera/lens tags and capture-time/location galleries. It runs cooperatively in the **existing `ai-scheduler` Python process**, uses the existing PostgreSQL job leases, and adds no container, model, inference request, online geocoder or background daemon. OCR, search, avatars and video/similar recommendations continue to use their existing paths.

## Deploy and enable

1. Rebuild and restart. Migration `0005_smart_organization` adds five companion tables and content/vector invalidation triggers. No existing photo vectors, media or ordinary catalogs are rewritten.
2. In **Admin → AI → Settings and resource limits**, enable global AI and **Enable Smart Tag**, grant the account AI access, and start/resume the existing background scheduler.
3. In personal settings, enable **Smart Tag for my library**. Both Smart Tag switches default off. Progress, resource/maintenance waiting reasons and failures appear in settings; **Refresh and retry** requests another pass and retries failed metadata reads.

All four gates are rechecked before each automatic commit: global AI, administrator Smart Tag setting, account AI access, user opt-in. Maintenance, scheduler pause/stop and CPU/RAM/disk pressure pause processing. An offline or unloaded model does not block EXIF or existing-vector work. Disabling preserves generated results and manual edits. Restoring a deleted item is also available while automatic processing is disabled.

No commands source `.env`, no external service receives location data, and the fixed city snapshot ships with the application. Deployment uses the existing Compose services and migration procedure described in [AI operations](ai-backend-operations.md).

## Organization rules

- Device tags require a real EXIF model. Whitespace and repeated make prefixes are normalized; canonical comparison is case-insensitive. `Apple iPhone 13` displays as `iPhone 13`. Camera and lens sources remain distinct, and ordinary tags with the same name remain separate. Exposure aperture is never turned into a lens specification.
- Reliable time precedence is original capture time (including subseconds), creation date, EXIF creation time, then media creation time. Filesystem modification/upload dates are not used. Explicit offsets win; otherwise the nearest city's IANA time zone is used, falling back to application `TIMEZONE`. The inferred source is stored and shown. Nonexistent DST wall times and invalid dates are skipped; ambiguous local times use the zone's first occurrence.
- GPS anchors are sorted by capture instant and media ID. Gaps **greater than 36 hours** split groups; each group spans at most **14 days**. Each automatic gallery requires **5 media and 2 GPS anchors**. A daily activity can qualify. No-GPS-only collections never form a trip by themselves.
- For media with time but no GPS, consider at most the **20 nearest-in-time GPS members within 6 hours**. Read only the current search profile's ready, current-content **document** vectors. Photos have one vector; videos compare all stored sampled frames, taking the maximum pairwise cosine. Admission requires score **≥ 0.80**, and a lead **≥ 0.05** over another candidate gallery. Incomplete candidate vectors wait for indexing. No model is called. A profile change schedules reevaluation.
- Names use `Times in CITY and CITY · START – END`, at most two main cities plus `+ N places`; all cities remain in the basis. Unknown locations use `Travel moments`. City assignments are approximate, limited to 100 km, from the fixed [GeoNames snapshot](https://github.com/WiscWebGroup/photoly/blob/main/photoly_backend/app/smart/data/README.md).

## Persistence and user edits

`smart_preference` stores opt-in, dirty/completed revisions and a resumable phase checkpoint. `smart_media` stores EXIF facts with content version, reliable UTC capture time, GPS and time-zone evidence. `smart_item` stores stable identity, source, generated basis, custom name/cover and dismissal. `smart_member` separates automatic membership from explicit include/exclude choices. `smart_lineage` relates merged/split groups.

Public tags, galleries and their link tables remain the existing structures, so Smart Search automatically finds smart names and members without ranking changes. Smart tags have a purple outline; galleries have teal accents. Both include a sparkle icon and **Smart** text, with Camera/Lens source labels and trip details.

Renaming and changing covers preserve custom values. Manual additions remain pinned; manual removals remain excluded. When merging conflicting inherited choices, exclusions take precedence. Deleting a smart item saves a dismissal tombstone; changing the date/name or later uploads do not bypass it. Overlapping groups inherit dismissal and manual overrides when split or merged. **Deleted smart items → Restore** restores its related lineage, avoiding a surviving ancestor dismissal hiding it again.

Replacing content invalidates cached EXIF and automatic links immediately; explicit pins/exclusions persist. Deleting media cascades member records. Automatic writes recheck ownership, eligibility, current profile and content version under the existing per-user mutation lock. Files are read through a pinned file descriptor, so replacement cannot substitute another original during metadata extraction.

## Scheduling and limits

Discovery checks every 60 seconds, rotates through up to 25 opted-in users, and queues one deduplicated `smart` job per user. Pending jobs rotate between users. Each cooperative batch performs at most 25 work units with a roughly 5-second elapsed budget; admin operations run first, followed by bounded Smart Tag work and existing vector scheduling. EXIF subprocess deadlines use the remaining batch budget. Three failed metadata reads mark the media failed; Refresh retries them. Missing EXIF is a normal skip.

Metadata inspection is incremental. Trip rebuilding streams cached facts chronologically through durable anchor, missing-GPS, item and membership phases. It never loads a user's complete vector collection. An affected user's grouping can require several bounded passes, especially after a late upload or a profile change; progress is eventual, and gallery membership is published in batches. Ordinary metadata/vector uploads during a pass bump its revision so another pass follows. Persistent leases recover interrupted work after restart. Memory admission is evaluated locally, independently of runtime/GPU availability.

## API

All routes require `HRD-Token` and operate only on the current user:

| Route | Purpose |
|---|---|
| `GET /ai/smart/settings` | Opt-in, availability, reason, check interval |
| `PATCH /ai/smart/settings` | `{ "enabled": true/false }` |
| `GET /ai/smart/status` | Inspected/total/failed counts, phase, pending state and wait reason |
| `POST /ai/smart/refresh` | Schedule reevaluation and retry metadata failures |
| `GET /ai/smart/dismissed?after=ID` | Up to 50 suppressed items, continuation cursor |
| `POST /ai/smart/items/ID/restore` | Restore the item's inherited dismissal lineage |

Admin settings accept `smart_tag_enabled` with the existing generation fence. Admin status adds `smartTag`. Tag/gallery all/page lists and photo association reads accept `includeSmart=true`; only that opt-in adds a `smart` object or `null` for an ordinary item. Without it, legacy response fields remain unchanged. Existing edit/member/delete endpoints record smart overrides. The OpenAPI document includes these contracts.

## Validation, 2026-10-16

Deterministic tests cover all 16 gate combinations, user isolation, legacy response compatibility, device normalization, explicit/inferred time zones, city snapshot checksums, no-GPS/no-time cases, 36-hour and 14-day boundaries, Hong Kong–Shenzhen continuity, split/merge dismissal inheritance, rename/cover preservation, explicit membership, immediate overwrite/delete invalidation, offline scheduling, finite retry/manual refresh, and permission revocation during EXIF reads. Both 768- and 2048-dimensional profiles exercise photo/video maximum-frame matching. Ambiguous ties are rejected and comparison reads are limited to 20 anchors.

An isolated PostgreSQL test with **10,002 media** ran ten 25-item metadata batches: mean **0.140 s**, max **0.149 s**, no measured RSS growth. Metadata was stubbed to isolate scheduling/database overhead; real EXIF descriptor reading is tested separately. This is a bounded-work measurement, not a promise about real EXIF I/O or end-to-end large-library completion time. The same process still proceeds to its normal vector work after each batch.

