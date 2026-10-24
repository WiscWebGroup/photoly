# Go acceptance report (phase 0)

Run: `COMPOSE_PROJECT_NAME=photolygoacc docker compose -f compose.test.yml --profile go up -d --build photoly`, then
`... run --rm --no-deps smoke-go pytest -q tests/test_ai_http.py tests/test_upload_http.py tests/test_http.py -m http`
(Go stack only, no Flask; a distinct project name is needed because other agents share the default `photoly` project and `down` each other's containers).
Fresh DB/volumes. Last run: **4 passed, 3 failed**. (Starting point was 1 passed, 6 failed.)

## Infra fixes made (Dockerfile / compose / test infra)

| File | Problem | Fix |
|---|---|---|
| photoly_server/Dockerfile | Binary lived at `/photoly`, same path as the `UPLOAD_FOLDER=/photoly` volume mount: container failed to start ("mounting ... not a directory") | Binary now `/usr/local/bin/photoly` (ENTRYPOINT + HEALTHCHECK updated) |
| photoly_server/Dockerfile | Named/anon volume at `/photoly` was root-owned, uid 65532 got `mkdir /photoly/.staging: permission denied` -> every upload 500 | `/out/data` (chown 65532) is now copied to `/photoly` (was `/photoly-data`), so fresh volumes inherit ownership |
| photoly_server/internal/server/server.go | Not infra, but blocked all tests: see row 1 below. Fixed in place (small, outside ai/admin/photo/uploads) | Strip trailing slash before mux (Flask `strict_slashes=False`) |

Image builds (arm64), `go vet` passes inside the build, `compose.go.yml` validates. Note: `test_media_tools.py` HTTP parts do not exist (it drives Flask in-process via `test_uploads_storage`); `tests/contracts/*` are consumed by in-process Flask tests plus `test_http_openapi_and_upload_contract`. Probe against the Go server: all 90 legacy routes in `spring_routes.json` and every path in Go `/openapi.json` are answered by Go (none fall through to the Flask proxy); `/openapi.json` path set equals the expected set exactly. Only `/docs` is not ported.

Also note: `test_http_z_admin_stop_terminates_all_workers` makes `photoly` exit (code 0) by design; the `photoly` service has no restart policy in compose.test.yml, which matches the Flask backend.

## Failure triage

| # | Test / route | Expected vs actual | Root cause (file) | Status |
|---|---|---|---|---|
| 1 | all tests, `POST /user/signUp/`, `/namespace/getRoot/`, ... | 200 vs 503 (fell to Flask proxy; no Flask running) | Go 1.22 `ServeMux` treats `/x/` != `/x`; Flask accepts both. `internal/server/server.go` New() | FIXED (trailing slash stripped) |
| 2 | test_http_full_workflow, test_upload_http (`POST /photo/insert`, `/uploads`) | non-JSON 500 `internal error` | `mkdir /photoly/.staging: permission denied` - volume ownership (Dockerfile) | FIXED (now pass) |
| 3 | test_http_openapi_and_upload_contract, `GET /docs`, `/docs/`, `/docs/assets/vendor/swagger-ui-bundle.js`, `/docs/assets/vendor/swagger-ui.css`, `/docs/assets/swagger-init.js` | 200 + HTML containing `swagger-ui-bundle.js` vs 503 (proxied to absent Flask) | Swagger UI page/assets not ported to Go; only `/openapi.json` is registered (`internal/server/server.go` registers just healthz+openapi). Needs a handler + embedded assets (copy `photoly_backend/app/docs` vendor files). Everything after line 97 in that test (spec validation, path set, `/photo/inserts`, deleteUser) untested yet; the path-set part verified by probe | OPEN, owner of server skeleton |
| 4 | test_ai_http_services_and_lifecycle, `POST /admin/ai/profiles` then `index_status` never `ready` (line 44, 45s timeout) | `indexStatus == "ready"` vs stays building | `internal/ai/scheduler.go:~818` maintenanceTick runs `CREATE INDEX CONCURRENTLY` on the app-role pool: `must be owner of table photo_embedding_state (42501)`. Flask ai-maintenance used the migrator credentials (`POSTGRES_MIGRATION_PASSWORD`, see compose.test.yml ai-maintenance). Go maintenance needs a pool/conn opened with `cfg` migration URL | OPEN (ai owner) |
| 5 | same test, follow-on | maintenance cycle logs `inconsistent types deduced for parameter $3 (42P08)` after the index failure | `failJob` UPDATE in `internal/ai/store.go`/`jobs.go`: `status=$3 ... CASE WHEN $3='failed'` needs `$3::text` (already changed in working tree by another agent after my image build; re-verify). Earlier build also logged scheduler `could not determine data type of parameter $1 (42P18)` every 2s, apparently also fixed in tree | VERIFY on next rebuild |
| 6 | test_simplified_controls_across_real_services (`ai.enable` then `/admin/ai/status` `startup.stage == 'ready'`, line 95) | `ready` vs never | Same chain as 4 (profile index never ready) plus the Python suite relies on `AI_TEST_RUNTIME=1` deterministic fake adapter with no weights; Go has no equivalent fake provider/env, and the `photoly` service in compose.test.yml has no model files. Not verified individually after fixing 4 | OPEN (needs 4 first, then a test-mode embedder in `internal/ai`, e.g. `AI_TEST_RUNTIME`) |
| 7 | test_ai_http_services_and_lifecycle on a reused DB | `cfg["enabled"] is False` fails (True) | Not a bug: tests need a fresh DB; run `down -v` between runs | N/A |

## Top root causes
1. Trailing-slash route matching (Go mux vs Flask strict_slashes=False): fixed.
2. Image layout: binary shadowed by volume mount and volume ownership: fixed.
3. AI maintenance uses the app DB role for DDL (needs migrator role) and failJob SQL typing: blocks both AI HTTP tests.
4. `/docs` Swagger UI not ported.
5. No deterministic fake AI runtime in Go for tests.

Not committed.
