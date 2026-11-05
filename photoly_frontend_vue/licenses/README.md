# License evidence

`supplements.json` binds each supplemental license/notice to the exact npm package
version, source URL and SHA-256. Files are copied verbatim from upstream. An npm
`gitHead` may identify an annotated tag object; use the peeled commit for raw URLs.
These records are deliberately not wildcards: upgrading the package requires review.

The full installed dependency graph is inventoried, including peers and optional
packages installed on the build platform. Platform binaries absent from this build
are recorded as skipped. Inventory records are not a compatibility certification.
`needs-review` means the package metadata or text needs more evidence. Browser and
worker bundles fail if they actually contain such a package; Node-only/build-tool
records remain visible in the inventory without pretending they ship to browsers.

ONNX Runtime's LICENSE and ThirdPartyNotices are copied from the exact native/web
release or package commit. Its umbrella notices may include disabled components;
they are retained conservatively and do not establish binary linkage. ArtPlayer's
nested screenfull/Hint.css grants and Lucide's embedded Feather notices are retained.

`require-like@0.1.2` has no SPDX metadata, but the published `License` file contains
the full MIT grant and Felix Geisendörfer's copyright. Its reviewed override is MIT.

Generated `public/legal/` is build output, not a hand-maintained list. Run
`pnpm run licenses` and build after changing the lockfile. The build checks actual
bundled modules too; do not silence a missing grant by adding a package to an ignore
list. Offline builds require already installed dependencies and checked-in evidence.
