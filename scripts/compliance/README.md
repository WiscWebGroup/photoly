# Distribution source archives

The publication workflow builds an exact runtime image locally, then builds
`sources.Dockerfile` from that image **before pushing it**. This collector never
connects to a running Photoly database or mounts media volumes.

```sh
docker buildx build --build-arg RUNTIME_IMAGE=your-image:exact-tag \
  --output type=local,dest=compliance-output \
  -f scripts/compliance/sources.Dockerfile scripts/compliance
```

Use the Docker engine's builder, which can resolve the local image (for example
`--builder default` in GitHub Actions or `--builder desktop-linux` on Docker
Desktop). Do not use an unrelated container-driver builder that cannot see it.

The collector snapshots **installed** dpkg packages before adding tools. Exact
source versions come from signed APT indexes, Debian snapshot when old versions
have rotated off the mirrors, or the official nginx.org source pool for nginx's
own packages. Source archives and Debian patches are checked against the `.dsc`
SHA-256 records. The runtime nginx image intentionally uses Debian trixie and refreshes
Debian security updates, so this process covers both runtime images.

For the frontend, the collector also fetches npm archives (verified against their
published integrity) and upstream Git source archives at the package commit or
peeled release tag for every package identified in browser/worker output. No
fallback to upstream HEAD is permitted. Prebundled libraries retain their package
notices; upstream source archives contain the source/build recipes rather than
only transpiled npm output. The backend supplies Go module sources and a
checksum-pinned pgvector source archive (pgvector is compiled into the base image,
not represented by a dpkg package). Models have separate pinned grants/provenance.

A failed fetch, missing exact revision, absent `.dsc`, or checksum mismatch stops
publication. `system/COMPLETE` and (for web) `system/javascript/COMPLETE` are created
only after collection succeeds. The final release archive adds the exact Photoly
source from `git archive`, immutable image identity, notices and source manifests.
Assets are split below GitHub's upload limit and accompanied by a checksum and
reassembly instructions. A rerun cannot overwrite different source bytes under an
existing asset name. Retain previously published sources when rebuilding a Git
revision against updated base packages.

This workflow does not retroactively supply sources for earlier image releases.
Their exact image digests and installed package graphs would need a separate
historical collection. The current audit does not claim those historical releases
have been remediated.
