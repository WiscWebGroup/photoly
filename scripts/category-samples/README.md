# Category samples

Thirty logical media items, five per category. The generated set has 35 files because each Live Photo has a still and a MOV. The samples belong in a separate, private `Category Samples` folder. No media binaries or user photos are committed here.

## Reproduce

Use Python 3.9+, FFmpeg (H.264, GIF, APNG and WebP encoders) and ExifTool. Alternatively use an existing Photoly backend Docker image, which contains the tools:

```sh
python3 scripts/category-samples/prepare.py --output /absolute/path/to/samples --docker-image photoly-preview-backend:YOUR_VERSION
python3 scripts/category-samples/import.py --directory /absolute/path/to/samples --base-url http://localhost:8384 --session-file /private/path/session.json
```

The session file contains a `token` JSON property or the raw session token. It is read locally and sent only to the specified Photoly server, never printed. Keep it outside the repository. Do not use a public server URL unless you intend to upload the samples there.

The importer verifies file hashes, uses the normal resumable upload API, waits for pairing, and verifies exactly 30 media IDs. A deterministic batch ID and local receipt make repeated imports idempotent. Keep the generated manifest and receipt with the sample directory. `prepare.py` pins source downloads to `sources.json`; a checksum mismatch requires reviewing the new source, not silently accepting it. The maintainer-only `--record-source-hashes` flag records a reviewed initial selection.

## Attribution and modifications

`sources.json` records each original file's author, source page, download URL, license URL and SHA-256. `manifest.json` records each derivative's source, transformations and checksums. CC BY and CC BY-SA works retain their respective licenses, including ShareAlike for modified panoramas. Source images marked Public Domain identify their provenance on the linked Commons description page; screenshots and the 360-degree fractal are CC0. Big Buck Bunny trailer excerpts and derivatives are © 2008 Blender Foundation, licensed CC BY 3.0: <https://peach.blender.org/about/>. Preserve this attribution and the manifests when redistributing the sample set.

- Panoramas: four real wide photographs; one 2:1, rendered 360-degree fractal with explicit GPano metadata. JPEG conversion/resizing is recorded.
- Screenshots: five openly licensed real screenshots. Three use a Screenshot filename; two use an ordinary filename and EXIF UserComment.
- Portrait Mode: five public-domain NASA photographs with **synthetic Apple capture metadata**. These validate classification; they are not original portrait-mode camera captures and have no depth or portrait matte data. Names explicitly say `SYNTHETIC_METADATA_TEST`.
- Videos and animations: short, resized excerpts of the licensed Blender trailer; animations cover GIF, APNG and animated WebP.
- Live Photos: five **synthetic Photoly JPEG/MOV pairs**, with matching content identifiers and distinct scenes. They validate Photoly pairing and playback. They do not reproduce Apple's native timed still-image metadata track and are not iPhone originals. Names explicitly say `SYNTHETIC_PAIR`.

The importer does not manually override any category. Membership must follow the normal classifier and pairing pipeline. The prepared workspace, including downloaded sources, is capped at 300 MiB.
