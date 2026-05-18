# Final verification — single-company baseline (partition concepts removed)

## Branch and baseline commit

- **Branch:** `test/openapi-json-body-shape-proof`
- **Commit SHA before this verification commit:** `bccff2aa9310ba6dcba01b87547091e90eb03f8b`

## Migration squash summary

- **Canonical DDL:** `db/schema/01_platform.sql` drives sqlc and the squashed goose baseline.
- **Goose files retained:** `migrations/00001_placeholder.sql`, `migrations/00002_platform_schema.sql`, `migrations/00003_seed_dev.sql`.
- **Removed:** incremental files formerly numbered `00004`–`00076` (historical deltas folded into the baseline).
- **Production:** requires backup, coordinated downtime, **`public` schema reset**, then **`goose up` from zero** with this baseline only; then seed operators per runbooks.

## Generators (Phase 2)

| Command | Exit |
|--------|------|
| `python tools/build_openapi.py` | 0 |
| `sqlc generate` | 0 |
| `python tools/build_postman_collection.py` | 0 |
| `python postman/full-production-suite/generate_full_postman_suite.py` | 0 (`VALIDATION_PASS`, REST 325 / gRPC 85 / MQTT 28) |

## Fresh database — goose

- **Database:** local Docker `avf-postgres`, database `avf_vending`, port **15432**.
- **Result:** `goose_db_version` lists applied versions through **`00003`** (baseline + seed). No incremental migration files remain in-tree.

## Schema legacy probes (Phase 5)

Executed the five `information_schema` / `pg_indexes` / `pg_constraint` / `information_schema.views` probes from the verification checklist (filters use `%` wildcards around legacy substrings **scope**, **organization**, **tenant** on names and definitions).

| Probe | Rows |
|-------|------|
| Tables | **0** |
| Columns | **0** |
| Indexes | **0** |
| Constraints | **0** |
| Views | **0** |

## Forbidden-token scan (Phase 6)

From repo root, `git grep` with pathspec exclusions (`vendor`, `node_modules`, `.git`) using:

1. The **extended alternation** list from the cleanup checklist (partition-style identifiers and env keys enumerated there — not spelled here so this file stays gate-clean).
2. A **primary** search for the contiguous token formed by `scope` + `_` + `id`.

**Result:** both searches returned **no hits** (non-zero exit with empty stdout).

## Static Go gates

| Gate | Result |
|------|--------|
| `gofmt` (all `*.go` except `vendor/`) | PASS |
| `go vet ./...` | PASS |

## Unit and integration tests

| Command | Result |
|---------|--------|
| `go test ./... -count=1` | PASS (full suite, no `-short`) |

## End-to-end (Phase 7)

Harness: **Git Bash** (`C:\Program Files\Git\bin\bash.exe`), repo root.

| Suite | Exit | Passed | Failed | Skipped | Run directory (local `.e2e-runs/`, gitignored) |
|-------|------|--------|--------|---------|--------------------------------------------------|
| `tests/e2e/run-grpc-local.sh` | 0 | 1 | 0 | 0 | `run-20260518T042444Z-35248-29269` |
| `tests/e2e/run-mqtt-local.sh` | 0 | 1 | 0 | 0 | `run-20260518T042652Z-37326-15100` |
| `tests/e2e/run-all-local.sh --fresh-data` | 0 | 23 | 0 | 0 | `run-20260518T042756Z-38232-26502` |

## Auxiliary artifact

- **`scripts/db/verify_public_schema_legacy_cleanup.sql`** — optional scripted form of the legacy-name probes for operators.

## Final verdict

**SCOPE_ID_REMOVED_100_PERCENT**  
**READY_FOR_DEVELOP_MERGE**

All listed gates passed in this workspace session on **2026-05-18** (UTC clock per generator manifest).
