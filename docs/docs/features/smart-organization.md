---
title: Smart Organization
sidebar_position: 6
---
Smart Organization organizes existing photos and videos. You can turn on each feature on its own: camera and lens tags, and trip galleries from capture time and location.
Smart Organization organizes existing photos and videos. It makes camera and lens tags. It also makes galleries from capture time and location.

Smart Organization runs in the existing `ai-scheduler` Python process. It uses the existing PostgreSQL job leases. It adds no container, model, inference request, online geocoder or daemon.

OCR, search, avatars and video recommendations keep their own paths.

To turn it on, see [Smart Organization operations](../administration/smart-organization.md).

## Organization rules

### Device tags

- A device tag needs a real EXIF model.
- Photoly trims whitespace and repeated make prefixes. Comparison ignores case.
- `Apple iPhone 13` shows as `iPhone 13`.
- Camera and lens sources stay separate. Ordinary tags with the same name stay separate.
- Photoly never reads exposure aperture as a lens specification.

### Capture time

Photoly picks the first reliable time in this order:

1. Original capture time, including subseconds
2. Creation date
3. EXIF creation time
4. Media creation time

Photoly does not use file modification or upload dates.

For the time zone, Photoly uses these in order:

1. An explicit offset
2. The IANA time zone of the nearest city
3. The `TIMEZONE` setting

Photoly stores and shows the inferred source. It skips nonexistent DST times and invalid dates. For an ambiguous local time, it uses the first occurrence.

### Trip galleries

- Photoly sorts GPS anchors by capture time, then media ID.
- A gap longer than **36 hours** starts a new group.
- A group spans at most **14 days**.
- A gallery needs **5 media** and **2 GPS anchors**.
- A daily activity can qualify.
- A collection with no GPS never forms a trip alone.

### Media with time but no GPS

Photoly compares these media with the nearest GPS members. It uses at most **20** members within **6 hours**.

- It reads only the ready vectors of the current search profile.
- A photo has one vector. For a video, it compares all stored sampled frames and takes the highest pairwise cosine.
- A match needs a score of **0.80** or more.
- The score must lead every other candidate gallery by **0.05** or more.
- If vectors are incomplete, the media waits for indexing.
- Photoly calls no model.
- A profile change schedules a new evaluation.

### Names

Gallery names use `Times in CITY and CITY · START – END`.

- A name has at most two main cities, then `+ N places`.
- All cities stay in the basis.
- If the location is unknown, the name is `Travel moments`.
- City assignments are approximate. The limit is 100 km.
- Photoly uses a fixed [GeoNames snapshot](https://github.com/WiscWebGroup/photoly/blob/main/photoly_server/internal/ai/smartdata/README.md).

## What Photoly stores

| Table | Content |
| --- | --- |
| `smart_preference` | Opt-in per feature (`devices_enabled`, `trips_enabled`), dirty and completed revisions, resumable phase checkpoint |
| `smart_media` | EXIF facts with content version, UTC capture time, GPS and time-zone evidence |
| `smart_item` | Stable identity, source, generated basis, custom name and cover, dismissal |
| `smart_member` | Automatic membership, and explicit include and exclude choices |
| `smart_lineage` | Merged and split group relations |

Public tags, galleries and link tables stay as they were. Smart Search finds smart names and members with no ranking change.

In the UI, smart tags have a purple outline. Galleries have teal accents. Both show a sparkle icon and the text **Smart**. They also show Camera or Lens labels and trip details.

## Your edits

- Renames and cover changes keep your custom values.
- Manual additions stay pinned. Manual removals stay excluded.
- When choices conflict during a merge, exclusions win.
- Deleting a smart item saves a dismissal. A date change, a name change or a later upload does not bypass it.
- Overlapping groups inherit dismissals and manual overrides when they split or merge.
- **Deleted smart items → Restore** restores the related lineage. An ancestor dismissal cannot hide it again.

## When content changes

- Replacing content clears cached EXIF and automatic links at once. Explicit pins and exclusions stay.
- Deleting media removes its member records.
- Automatic writes check ownership, eligibility, current profile and content version again. They run under the per-user mutation lock.
- Photoly reads files through a pinned file descriptor. A replaced original cannot swap in during metadata extraction.
