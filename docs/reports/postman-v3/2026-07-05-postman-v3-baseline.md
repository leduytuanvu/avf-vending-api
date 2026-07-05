# Postman v3 baseline (2026-07-05)

Recorded before/after implementation on branch `main` @ `d8c7a053`.

## Commands

| Command | Result |
|---------|--------|
| `go test ./... -short` | **PASS** |
| `go vet ./...` | **PASS** |
| `python tools/build_postman_collection.py` | **PASS** |
| `python tools/check_postman_artifacts.py` | **PASS** |
| `python postman/production/generate_postman_from_manifest.py` | **PASS** (48 requests) |
| `python tests/e2e/production/scripts/validate_postman_shell_parity.py` | **POSTMAN_PARITY_OK** |
| `bash scripts/ci/verify_production_postman_parity.sh` | **SKIP** (bash/WSL unavailable on Windows host) |
| `make postman-generate` / `make postman-check` | **SKIP** (`make` not on Windows PATH; Python equivalents used) |
| `postman --version` (after `npm install -g postman-cli`) | **1.41.1** |
| `postman collection migrate --help` | `-o, --output <dir>` |

## Post-implementation validation

| Command | Result |
|---------|--------|
| `python tools/build_postman_v3_yaml.py --regen-json-skip` | **PASS** — 574 yaml requests, 328 env var entries |
| `python tools/check_postman_v3_artifacts.py` | **PASS** |
| `python scripts/postman/validate_v3_yaml.py` | **VALIDATION_PASS** |

## Notes

- Postman CLI migrate on Windows requires `TEMP`/`TMP` on the same drive as the repo (avoid `EXDEV` cross-device rename).
- Doc-only JSON item `README — Import and Safety` is excluded from v3 parity counts (Postman CLI omits it).
