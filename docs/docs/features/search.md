---
title: Search and similar media
sidebar_position: 2
---

## Smart Search

Smart Search is the default search mode. It has one text box and returns up to 100 results.

- Matches on names, tags and galleries show first.
- Semantic results join them when available.
- If semantic search fails, text results stay.

## Similar photos and videos

Scroll down in the viewer to load **Similar photos and videos**.

- Each item has its own results and scroll state.
- If you return within 30 seconds, Photoly restores the list. After that, it starts collapsed.
- If you keep viewing an item, its list does not expire.
- **Back to top** collapses the list and clears the cache for that item.

## Choose where AI runs

Open account settings to inspect the available AI placement choices. When the administrator permits user choice, image indexing and text search can run in the browser or on the server. If a choice is fixed by the administrator, a user preference cannot override it.

Browser processing downloads the pinned model assets when needed and requires a compatible browser with enough memory. Keep the app open while it indexes existing images. Closing the browser stops local work; it does not delete already stored vectors. Browser pending lists contain still images; video sampling remains a server task.

Use the indexing action in the AI controls to request processing of existing media. Progress distinguishes browser-pending items from the server queue. Changing placement does not guarantee that every old item has been indexed already.

## How indexing works

Photoly embeds photos and videos after upload. The server queue or the uploading browser does this.

- The server stores browser vectors with `source = 'client'`.
- When idle, the server re-embeds a random 1% of them.
- The server drops a user's browser vectors if their cosine similarity is below 0.98.
- If no allowed side can embed a photo, it stays `pending`. A browser or the server embeds it later.

## How search works

Search uses the active profile.

- A browser sends its query vector to `POST /ai/search/vector`.
- With server placement, the server embeds the text.
- If you change the model or preprocessing, Photoly builds a new profile in the background. Search keeps the old profile until the new one is ready.

For model placement and limits, see [AI management](../administration/ai-management.md).
