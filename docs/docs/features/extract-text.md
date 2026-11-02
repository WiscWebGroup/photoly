---
title: Extract Text
sidebar_position: 5
---

Extract Text reads the text in a photo. It uses Tesseract on the backend CPU.

## Use it

1. Open a photo.
2. Expand **Extract Text**. It is below **Common Metadata**.
3. Click **Recognize Text**.
4. Edit the text if needed.
5. Click **Save**.

Opening the panel shows saved text only. It does not start recognition. Photoly saves a result only after a successful run.

**Copy** copies the current draft, even before you save. If the clipboard fails, the text stays selected so you can copy it by hand.

## Recognize again

**Recognize Again** asks before it replaces saved text and drafts. If recognition fails, Photoly keeps both.

## Unsaved changes

Photoly asks you to save, discard or cancel when you have unsaved text and you:

- Close the panel.
- Open a similar image.
- Return through viewer history.
- Enter the image editor.

If a save fails, the draft stays. On browser unload, the browser shows its own prompt. When you log out, Photoly clears local state and cancels client requests.

## Privacy and scope

- Only the photo owner can see the text. This is true even when the image is shared.
- OCR does not need AI search. It does not load embedding models.
- Not included: video OCR, batch processing, bounding boxes and search integration.

For deployment, limits and API, see [Extract Text operations](../administration/extract-text.md).
