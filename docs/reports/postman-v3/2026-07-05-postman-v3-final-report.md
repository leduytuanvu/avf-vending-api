# Postman v3 implementation final report (2026-07-05)

## Root cause

Postman v12 Local Mode / Native Git requires **collection schema 3.0.0** (YAML folder layout). The repo only tracked **v2.1 JSON**, causing the Local Mode upgrade error.

## Solution

Dual-output pipeline:

- **JSON v2.1** preserved for Newman/CI (`postman/collections/`, `postman/environments/`, suites, production E2E).
- **YAML v3** added under `postman/v3/` via `postman collection migrate` + deterministic environment YAML export.

## Files changed (high level)

| Area | Files |
|------|-------|
| Generators | `tools/build_postman_v3_yaml.py`, `tools/postman_v3_environment.py`, `tools/postman_v3_parity.py`, `scripts/postman/generate_v3_yaml.py`, `scripts/postman/postman_openapi_lib.py`, `scripts/postman/gfs_import.py` |
| Validators | `tools/check_postman_v3_artifacts.py`, `scripts/postman/validate_v3_yaml.py` |
| Makefile | Split `postman-generate-json/v3`, `postman-check-json/v3` |
| Docs | `postman/v3/README.md`, `docs/postman/POSTMAN_V3_LOCAL_MODE_IMPORT_GUIDE.md`, runbooks, audit/baseline reports |
| JSON source fix | `tools/build_postman_collection.py` (stale path text) |

## Generated artifacts

### JSON (preserved)

- `postman/collections/avf-vending-api.postman_collection.json` (33 requests)
- `postman/collections/avf-vending-api-function-path.postman_collection.json` (33)
- `postman/environments/avf-{local,staging,production}.postman_environment.json`
- `postman/suites/production-full/avf-vending-production.full.*` (461 JSON items; 460 HTTP + doc README)
- `postman/production/avf-production-e2e.*` (48)

### YAML v3 (new)

- `postman/v3/collections/avf-vending-api/`
- `postman/v3/collections/avf-vending-api-function-path/`
- `postman/v3/collections/avf-production-e2e/`
- `postman/v3/suites/production-full/avf-vending-production-full/` (460 requests)
- `postman/v3/environments/*.environment.yaml` (5 files)
- `postman/v3/manifest.json`

## Counts (manifest)

| Metric | Value |
|--------|-------|
| JSON requests (parity scope) | 574 |
| YAML requests | 574 |
| Environment variable entries | 328 |

## Validation results

| Check | Result |
|-------|--------|
| `go test ./... -short` | PASS |
| `go vet ./...` | PASS |
| `python tools/check_postman_artifacts.py` | PASS |
| `python tools/check_postman_v3_artifacts.py` | PASS |
| `python scripts/postman/validate_v3_yaml.py` | VALIDATION_PASS |
| `postman collection lint postman/v3/collections/avf-vending-api` | No issues (33 scanned) |
| Production E2E parity (Python) | POSTMAN_PARITY_OK |

## Commands

```bash
make postman-generate-json
make postman-generate-v3      # requires Postman CLI
make postman-generate
make postman-check
```

## Import guide

[`docs/postman/POSTMAN_V3_LOCAL_MODE_IMPORT_GUIDE.md`](../postman/POSTMAN_V3_LOCAL_MODE_IMPORT_GUIDE.md)

## Known limitations

1. **Postman CLI required** for collection v3 generation (`npm install -g postman-cli`).
2. **Windows**: set `TEMP`/`TMP` to repo-local `.tmp-postman-migrate/` if migrate fails with `EXDEV`.
3. **Newman** cannot run v3 YAML — CI continues using JSON.
4. Doc-only JSON request `README — Import and Safety` is excluded from v3 parity (CLI omits it).
5. Environment v3 uses deterministic YAML exporter (no official `postman environment migrate`).

## Safety confirmations

- [x] No secrets committed (blank placeholders enforced)
- [x] JSON CI assets preserved
- [x] YAML v3 assets generated and parity-validated
- [x] Postman Local Mode issue addressed via `postman/v3/`
- [x] Production gated writes remain in collection scripts
- [x] No production deploy run
- [x] No production DB mutation run
- [x] No destructive production e2e run
