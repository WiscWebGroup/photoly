---
title: License audit — November 5, 2026
---

## Result and scope

This audit covers the tracked Photoly application based on `cc6afe23` plus the
remediation in this change: the Vue application, documentation site, root npm
utilities, Go server and local SMB replacement, AI weights, shipped data and
runtime images. The untracked legacy `photoly_backend/` cache directory is not
part of the current application or this change. Historical published image
releases have not been retroactively remediated.

Photoly remains **AGPL-3.0-or-later**. Reviewed third-party grants retain their own
terms. No incompatible grant was identified among the reviewed packages actually
included in the current browser/worker output or linked Go modules. That is not a
claim that every installed development package, model provenance question, or
historical release is cleared. The outstanding evidence below remains explicit.

## Findings and remediation

| Finding | Evidence and disposition |
| --- | --- |
| Partial browser notices | Existing generators covered maps and ArtPlayer only. The new generator inventories the installed lockfile graph and preserves full notices, including ArtPlayer's embedded screenfull/Hint.css and Lucide's Feather attribution. Browser/worker output is checked against collected grants. |
| Missing source-code component attribution | `components.json` identifies shadcn-vue-generated UI. Its MIT license and Photoly modification notice are now included. The original generator revision was not recorded; the license evidence revision is explicitly distinguished from it. |
| SigLIP 2 licensing not distributed with downloads | The base-model Apache-2.0 text and a conversion provenance notice now accompany new and existing caches. Notice endpoints work without downloading weights. The model fingerprint and inference remain unchanged. |
| Native/browser ONNX Runtime notices incomplete | Version-bound upstream LICENSE and ThirdPartyNotices are included. Supplement files have upstream URLs and SHA-256 checks, without assuming all components in an umbrella notice are enabled. |
| Linked LGPL library | `github.com/cloudsoda/sddl` contains the LGPLv3 grant. Its notice, accompanying GPLv3 text, exact Go module source and rebuild instructions are now provided. |
| Modified SMB snapshot | Preserve upstream BSD-2-Clause and Go-derived BSD-3-Clause notices, including the source-file headers. Identify Photoly's AGPL modifications and include the actual replacement source. |
| Container source obligations | The backend image records installed packages, keeps their copyright and common-license files, and carries Go module and pgvector sources. Debian sources are available from the Debian archive or on request. FFmpeg's Debian GPL build is not labeled LGPL-only. |
| Frontend corresponding source | `/legal` links to the exact Photoly commit, which holds the frontend source and build instructions. |
| Missing public legal/source entry | `/legal` is public and linked from login, welcome, Library/account and administrator navigation. Frontend/backend revision and source addresses are independent; missing configuration is stated rather than falsely pointing to upstream main. Backend legal resources also remain available during maintenance. |
| Geographic attribution | Retain map-local OSM credit, link GeoNames' CC BY 4.0 terms, document the derived city snapshot and repair the stale Python-directory documentation link. |
| Contributor identity | Harold Ye confirmed that `Ye <Yixuan.Ye@Dell.com>` identifies his personal contributions unrelated to Dell work. The register records that clarification under his existing consent without rewriting history or inventing a new signature. |

The web runtime stays on `nginx:alpine`; both images list their installed system
packages. Debian package sources are provided from the Debian archive or on request
(see [Licenses and source](../overview/licenses.md)), not as per-image source archives.

## Inventory and remaining evidence

The macOS audit installation resolves 747 frontend/tool packages, 1,185 docs/tool
packages and one root utility package. Platform-specific optional packages differ
in Linux builds; each build emits its own exact inventory. There are 41 Go modules
linked into the server. `dependencies.json` records individual versions, grants,
source references, notice hashes and review status. The bundlers emit separate
lists for application and worker code; an installed dependency is not automatically
a shipped browser dependency.

The local frontend inventory still flags these **non-emitted** dependency records:
`@humanfs/types@0.15.0`, `@img/sharp-libvips-darwin-arm64@1.3.4`,
`boolbase@1.0.0`, `guid-typescript@1.0.9`, `is-node-process@1.2.0`, and
`natural-compare@1.4.0`. These have missing complete license evidence or need
additional license review. In particular, guid-typescript is used by ONNX Runtime's
legacy WebGL implementation; the selected WebGPU bundle's published source map
contains no guid-typescript sources. Photoly imports the WebGPU/WASM runtime.

The local docs inventory separately flags 14 non-emitted records, including the
native sharp bundle, `@jsonjoy.com/json-pointer`, the two `@pnpm` utilities,
`boolbase`, `cacheable-request`, `eastasianwidth`, `format`, `glob-to-regex.js`,
`keyv`, `svg-parser` and three WebSocket development packages. Exact versions and
reasons remain in the generated inventory. This does not authorize redistributing
those tool installations without further review. A future change that actually
bundles a flagged package fails the build. MPL code also requires a corresponding
source review before it may enter the browser output.

The SigLIP 2 ONNX repository identifies Google's Apache-licensed base model but
has no independent license declaration at the pinned revision. The supplied notice
records that evidence and limitation; it does not manufacture a separate grant
from the converter. Model-training-image rights, biometric processing, patents
and trademarks for a particular deployment have not been assessed.

Sample media retain their existing source/derivative manifests and CC BY/CC BY-SA
attribution; synthetic player fixtures contain no third-party audiovisual material.
They are not absorbed into Photoly's AGPL grant.

## Verification

- Frontend production build, typecheck and ESLint; all **346 unit tests** passed.
- **9 legal-page browser tests** passed across Chromium, WebKit and mobile, covering anonymous access, ordinary/admin navigation, real notice files, no weight downloads and viewport containment. All **12 video-player regression tests** also passed across those browser projects.
- Go notice regeneration/check, Docker build/vet, and focused tests for public resources, unsafe source URLs, model notices without network access and backfilling an existing cache passed.
- Docusaurus production build passed with emitted-module license validation.
- Generator tests reject absent/unknown/upgraded missing grants and check supplement hashes, duplicated project licenses and model links against pinned backend versions.

See [Licenses and source](../overview/licenses.md) for operator instructions and
`photoly_frontend_vue/licenses/README.md` for evidence-maintenance details.
