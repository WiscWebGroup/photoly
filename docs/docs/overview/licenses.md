---
title: Licenses and source
---

Photoly is distributed under **AGPL-3.0-or-later**. Dependencies, models, geographic data and sample media retain their own licenses. Read the [full Photoly license](/legal/AGPL-3.0.txt).

## In the application

**Licenses & source** is available without signing in at `/legal`, from the sign-in page, Library sidebar, account page and administrator navigation. It links to complete notices and to the corresponding frontend and backend source independently. The two components can run different revisions.

A package name or a link to its homepage does not replace its required copyright, license or NOTICE text. Photoly ships those texts with the frontend, Go binary and containers. Map attribution remains visible on the map; moving it solely to an About page would be insufficient.

## Software and model notices

- [Documentation-site dependency notices](/legal/third-party-notices.txt) and [inventory](/legal/dependencies.json). These are generated for this documentation build, not the running application.
- The application provides its own frontend notices, Go notices and model licenses. Inventory entries marked `needs-review` are evidence gaps, not approvals. Installed inventories conservatively include build tools and Node-only packages. The bundlers separately check packages actually used in browser and worker output.
- **SigLIP 2:** Google's base model card at revision `75de2d55ec2d0b4efc50b3e9ad70dba96a7b2fa2` declares Apache-2.0. The ONNX conversion at revision `ba1f3b0843f24bc5417d38e19c37b287d719b2f4` identifies that base model but does not declare a separate license. Photoly supplies the base grant and explicitly records this provenance limitation in its model NOTICE. fp32, fp16 and int8 files remain unchanged and checksum-pinned.
- **YuNet:** MIT; **SFace:** Apache-2.0, as supplied by the pinned OpenCV Zoo revision. License files remain alongside cached weights and are available without downloading weights.
- **ONNX Runtime:** preserve both the MIT license and the matching `ThirdPartyNotices.txt`. The notice collection can describe components not enabled in a particular binary; inclusion in that collection does not establish that every named component is linked.
- **cloudsoda/sddl:** LGPLv3, linked into the Go backend through the SMB client. Its LGPL text, accompanying GPLv3 text and source are supplied. Photoly's source and build instructions permit rebuilding the application with a modified library; no restriction on reverse engineering for debugging such modifications is imposed.
- The Debian FFmpeg build enables GPL components. Its package copyright file describes GPLv2-or-later binaries and GPLv3-or-later library combinations; it must not be described simply as LGPL. FFmpeg, libheif, libvips, ExifTool, Tesseract, PostgreSQL, pgvector and their installed dependencies are inventoried from each runtime image.

## Geographic data and samples

Approximate city names use [GeoNames](https://www.geonames.org/), under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/). Photoly converts the October 16, 2026 cities15000 snapshot into an indexed database of selected fields, omitting alternate names. The source, hashes and transformations are recorded in `photoly_server/internal/ai/smartdata/`.

Maps display [© OpenStreetMap contributors](https://www.openstreetmap.org/copyright); OSM data is licensed under the ODbL. Tile-service usage policies apply separately from data licensing.

Development/evaluation media have separate provenance records in `scripts/category-samples/` and the frontend test fixtures. CC BY / CC BY-SA assets and their derivatives retain their attribution and applicable ShareAlike terms. They are not relicensed under AGPL merely by appearing in this repository.

## Deploying and redistributing

For modified network deployments, offer users the corresponding source of the version they are using, including build instructions and modifications. An upstream homepage or an unrelated main-branch snapshot is not that source. Read [AGPL section 13](https://www.gnu.org/licenses/agpl-3.0.html#section13), [Apache redistribution conditions](https://www.apache.org/licenses/LICENSE-2.0), and the relevant component licenses.

Build both Dockerfiles with `PHOTOLY_REVISION` and `PHOTOLY_SOURCE_URL`. The frontend embeds these at build time. The backend also accepts a runtime `PHOTOLY_SOURCE_URL` override. Use an HTTP(S) URL providing your actual source. Without configuration, the UI clearly identifies a development build or missing source URL instead of claiming it matches upstream. Set these explicitly when using a custom Compose configuration.

Published images are built from a specific commit. Their `/legal` page links to that exact commit, and the image labels record it. The backend image also carries the Go module and pinned pgvector sources under `/usr/share/photoly/sources`, and a list of installed Debian packages with their copyright files under `/usr/share/photoly/licenses`. Corresponding source for the Debian packages (including the GPL FFmpeg build) comes from the Debian archive, or on request from the maintainers through the project issue tracker, for at least three years after distribution. The web image uses `nginx:alpine` and lists its installed Alpine packages the same way.

Runtime container notices are under `/usr/share/photoly/licenses`; Go dependency source archives are under `/usr/share/photoly/sources` in the backend image.

To rebuild with a modified LGPL SMB dependency, unpack the backend's
`dependencies/go-modules.tar.gz`, copy the `github.com/cloudsoda/sddl@...` directory
into `photoly_server/third_party/sddl`, and run
`go mod edit -replace github.com/cloudsoda/sddl=./third_party/sddl` in
`photoly_server`. Modify that source, regenerate the notices, and rebuild the
Dockerfile with the documented database build context. The Dockerfile already
copies `third_party` before resolving modules. Retain the LGPL/GPL notices and
provide the modified corresponding source when redistributing the result.

## Updating dependencies

1. Install with the lockfile frozen. Review the exact upstream grant, bundled code and NOTICE files when updating a model, package, native library or base image.
2. Run `pnpm run licenses` and build the frontend/docs.
3. In `photoly_server`, run `go mod download`, `go run ./scripts/licenses`, then `go run ./scripts/licenses --check`. Commit regenerated Go notices with the dependency change.
4. Manual npm supplements live in `photoly_frontend_vue/licenses/supplements.json`; every file is bound to an exact package version, upstream URL and SHA-256. Do not copy an old exception to a new version without review.
5. Check that the final image contains the notices under `/usr/share/photoly/licenses`.

This audit addresses licensing evidence and distribution engineering. It does not certify rights to every model-training image, biometric-data processing, codec patents or trademarks for a particular deployment.
