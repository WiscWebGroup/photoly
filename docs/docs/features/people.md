---
title: People
sidebar_position: 8
---

People finds faces in photos and groups them into private, named identities. Enable it in **Settings → People**, after an administrator enables AI, People and your account's AI access. Administrator and personal People switches default to off.

On desktop, open **Library → People**. On a phone, open **Albums → People**. The main list shows named people and unnamed people found in at least two different photos. **To organize** contains single-photo identities. Counts are distinct photos, even when a person appears more than once in a photo.

Open a person to browse their photos across your whole library using the usual viewer. Search names, rename the person, or open **Manage faces** to choose a cover and correct a face. **This isn't this person** lets you create a new identity or select an existing one. **Not a face** removes a false detection. Select multiple cards, including cards across People and To organize, and choose **Merge**; choose which identity and name to retain. Old person links resolve to the retained identity.

**Hide person** removes that person from ordinary lists, Gallery people and photo people. Open **Hidden** to restore them. Names and identities survive deletion of their last photo. Corrections take precedence over automatic retries.

The viewer's **People in this photo** button is independent of Find Similar. It includes singleton faces and distinguishes waiting, processing, failed and successfully processed photos with no visible faces. A Gallery's **People** button includes people found anywhere in that Gallery; opening a person takes you to their full-library photos. Browser Back returns to the previous Gallery or view.

People processes still photos, the static image of Live Photos and the first frame of animations. It does not process videos. Public shares do not show people and share tokens cannot access People endpoints.

## Processing controls

- **Browser** downloads the pinned models when needed and processes while Photoly is open. WebGPU is preferred; unsupported or failed WebGPU execution falls back to browser CPU (WASM). The current device is shown in settings. Closing the page, losing connectivity or sleeping the device leaves completed progress intact. No automatic fallback to server inference occurs.
- **Server** processes in the background through the AI worker. The administrator can require one location or let each user choose; personal default is Browser.
- **Pause processing** stops automatic work for your account on every device, preserving browsing and editing. **Retry failed photos** restarts exhausted jobs.
- **Delete all face data** requires confirmation, removes names, embeddings and corrections, and disables People. Photos remain. Turning a switch off without deleting retains the data.

## Recognition limits

YuNet + SFace is a first model profile. Small, blurred, obscured or strongly turned faces can be missed or split into separate people. Similar-looking people can still be grouped incorrectly. Review automatic groups before relying on names. The release smoke evaluation uses two identities and transformed copies; it is **not** a measured false-match rate for family albums or a demographic accuracy study. See [operations and evaluation](../administration/people.md).
