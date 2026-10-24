# VectorChord vs pgvector for Photoly: feasibility

## Verdict

**Defer.** It is technically feasible and low-risk to try, because VectorChord reuses pgvector's `vector`/`halfvec` types and `<=>` operator. But Photoly's profile (thousands to ~1M photos, 768-dim, per-user filtering) does not have the problem VectorChord solves (very large indexes, slow or memory-heavy builds). Switching would add a heavier image, a license question, and a filtered-search tuning rewrite for little gain. Revisit if libraries grow well past ~1M vectors or HNSW build memory becomes a real complaint.

## How pgvector is used today

- Column type: `halfvec` with no fixed dimension, cast to `halfvec(768)` at query time and in the index expression (`photoly_server/internal/migrations/sql/00001_baseline.sql:378` state table, `:744` video frames).
- Index: one **partial HNSW expression index per profile and table**, built `CONCURRENTLY` by the maintenance job: `photoly_server/internal/ai/scheduler.go:819` (`USING hnsw ((embedding::halfvec(dim)) halfvec_cosine_ops) WHERE profile_id=.. AND status='ready'`). Index names are built at `scheduler.go:756`. The Python equivalent is `photoly_backend/app/scheduler/maintenance.py:49`.
- Search: `photoly_server/internal/ai/store.go:104` (`<=>` on `::halfvec(768)`), filter builder at `store.go:83-103` (user_id, profile, status, content_version, namespace, gallery, exclude). Python twin: `photoly_backend/app/ai/vector_store_pgvector.py:39,24-35`.
- Filtered-search strategy: if the user has more than 5000 ready rows, set `hnsw.iterative_scan=strict_order`, `hnsw.ef_search=1000`, `hnsw.max_scan_tuples=20000`, take candidate photo ids from the index, rank them exactly, and fall back to an exact full scan if the candidates come up short (`store.go:241-250`; Python `vector_store_pgvector.py:82-98`). Under that threshold it just does an exact scan. Similar-photo paging (`Page`, `store.go:302`; `search_page`, Python `:100`) is always exact.
- Writes use `pgvector-go` `HalfVector` (`photoly_server/internal/ai/jobs.go:328-366`, `go.mod:7`).
- Extension install and version pin: `database/postgres-init/01-photoly.sh:15` (`CREATE EXTENSION vector VERSION '0.8.7'`). Version reporting: `photoly_server/internal/admin/admin.go:356`, and the Python `app/infrastructure/schema.py:18`.
- Images: `pgvector/pgvector:0.8.7-pg17-bookworm@sha256:...` in `docker-compose.yml:82`, `compose.go.yml:49`, `compose.test.yml:14`, `poc/single-binary/compose.yml:25`.
- The code already relies on pgvector index behavior in tests: `photoly_backend/tests/test_ai.py:321-360` (HNSW recall and generic plan), `tests/test_video_vectors.py:269-297`, `tests/test_postgresql.py:105-162`.

## Comparison

| Aspect | pgvector 0.8.7 HNSW (today) | VectorChord 1.1.x `vchordrq` |
|---|---|---|
| Index type | HNSW graph | IVF-style lists + RaBitQ quantization |
| Types / operators | `vector`, `halfvec`, `<=>` | Same types and `<=>`; **depends on pgvector being installed**. Adds `<<=>>` for similarity-range queries |
| halfvec cosine | `halfvec_cosine_ops` | `halfvec_cosine_ops` (supported) |
| Build time / memory | Slow and memory-hungry at large N (vendor: 1M GIST in ~16x longer) | Vendor claims much faster builds and low memory; fastest builds use external k-means centroids |
| Small tables (<100k) | Fine | `lists=[]` default, no partitioning, comparable |
| Filtered search | `iterative_scan`, `ef_search`, `max_scan_tuples` (what Photoly uses) | No iterative scan equivalent. Has `vchordrq.prefilter=on` and `vchordrq.max_scan_tuples`. Needs re-tuning and recall tests |
| Recall tuning | `ef_search` | `probes`, `epsilon`, optional rerank, `rabitq8/4` types (not usable with `residual_quantization`) |
| Insert behavior | Online, incremental | Online, but quality depends on centroids chosen at build; may need rebuild as data grows |
| License | PostgreSQL license | **AGPLv3 or Elastic License v2 (dual)**. Fine for self-hosted use, a concern if Photoly is ever offered as a service or bundled |
| PG versions | 13-18 | 14-18 (images for pg17 and pg18) |
| Docker | `pgvector/pgvector` (multi-arch) | `ghcr.io/tensorchord/vchord-postgres:pg18-v1.1.1`, `tensorchord/vchord-suite:pg17-latest`; arm64 listed as tested for packages, image multi-arch not confirmed |
| Managed hosting | Widely available (RDS, Supabase, Neon, ...) | No managed availability confirmed in the sources checked; self-host only |
| Go driver | `pgvector-go` | Same `pgvector-go` works (same wire types) |

## Migration steps and code touchpoints

1. Switch the image and bootstrap: `docker-compose.yml:82`, `compose.go.yml:49`, `compose.test.yml:14`, `poc/single-binary/compose.yml:25`. In `database/postgres-init/01-photoly.sh:15` add `CREATE EXTENSION vchord CASCADE` (pulls in `vector`) and `shared_preload_libraries='vchord'` via postgres command args.
2. Index DDL: replace `USING hnsw (...) halfvec_cosine_ops` with `USING vchordrq (... halfvec_cosine_ops) WITH (options = $$ build.internal.spherical_centroids = true $$)` (add `lists` for large tables), at `photoly_server/internal/ai/scheduler.go:819` and `photoly_backend/app/scheduler/maintenance.py:49`. Partial and expression indexes are plain PostgreSQL, so the existing predicate layout can stay. Index names/suffix `_hnsw` at `scheduler.go:756` and tests should be renamed.
3. Search tuning: replace the three `hnsw.*` SETs at `photoly_server/internal/ai/store.go:243` and `vector_store_pgvector.py:83-85` with `vchordrq.probes`, `vchordrq.epsilon`, `vchordrq.prefilter`, `vchordrq.max_scan_tuples`. The candidate-then-exact-rank design can stay, but the "index returns too few rows under a selective user filter" case is where recall must be re-measured.
4. Admin/version display: `admin.go:356` and `schema.py:18` check the `vector` extension; add `vchord` version reporting if desired.
5. Tests: update the HNSW-specific tests listed above (index name, `EXPLAIN` assertions, recall check).
6. Existing deployments: `DROP INDEX` the HNSW indexes and let the maintenance job recreate them; no data conversion is needed since the column types do not change.

## Risks

- **Per-user filtering is the weak spot.** Photoly's core query is `user_id = ?` on a table shared by many users. pgvector's iterative scan was added for this. vchordrq relies on prefilter/probes, and the docs' prefilter guidance (benefit at ~1% selectivity, overhead otherwise) means recall must be benchmarked, not assumed.
- AGPL/ELv2 licensing for a project that may be redistributed in images.
- Heavier operational surface: preload library, version-matched image tags, extension upgrade scripts, a dependency on both `vector` and `vchord`.
- Image availability: only third-party/vendor images; no pinned official Docker Official Image; arm64 image support unverified. The pgvector image is already pinned by digest and works on Apple Silicon.
- Vendor benchmarks only (VectorChord and EDB marketing). No independent 768-dim halfvec comparison at Photoly's scale was found.
- Does not help the single-binary goal: the Go process stays the same, and Postgres memory is dominated by shared_buffers, not the vector index at <1M rows.

## Recommendation

**Defer.** Keep pgvector HNSW as the default. If large-library users ask, add VectorChord as an **optional backend** behind the existing `VectorStoreInterface` / index-DDL seams (steps 1-3), gated by an env flag, and only after benchmarking recall with realistic per-user filters on a 1M-row 768-dim set. Quick low-risk experiment: run the existing `test_ai.py` recall test against `vchord-postgres:pg17`.

## Sources

- https://github.com/tensorchord/VectorChord (license, images, pgvector dependency, vendor cost claims)
- https://docs.vectorchord.ai/vectorchord/usage/indexing.html (vchordrq options, probes, epsilon, halfvec ops, max_scan_tuples)
- https://docs.vectorchord.ai/vectorchord/usage/prefilter.html (prefilter)
- https://docs.vectorchord.ai/vectorchord/benchmark/pgvector.html (vendor build-time benchmark)
- https://pgxn.org/dist/vchord/ (release 1.1.x, AGPL-3.0)
- https://hub.docker.com/r/tensorchord/vchord-postgres (images, laion-768 figure)
- https://docs.vectorchord.ai/vectorchord/getting-started/installation.html (platforms)
- https://www.enterprisedb.com/pgvector-vs-vectorchord-benchmark (vendor benchmark)
