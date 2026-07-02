# Pre-Commit Test Results

**Timestamp:** 20260702T184910Z

## avf-vending-api

| Check | Result |
|-------|--------|
| `go fmt` / `go vet` | PASS |
| `go test ./... -count=1` | PASS |
| `python tools/openapi_verify_release.py` | PASS |
| `python tools/enterprise_flow/*` validators | PASS |
| `sqlc generate` + drift check | PASS (regenerated gen/db) |
| `python tools/check_postman_artifacts.py` | PASS |
| `bash scripts/api-contract-check.sh` | SKIP (WSL bash unavailable locally; CI will run) |

## avf-vending-app

| Check | Result |
|-------|--------|
| `./gradlew test` | FAIL (3 pre-existing unit test modules; **zero app/src diff**) |
| `./gradlew assembleXyProductionRelease assembleTcnProductionRelease` | PASS |

App commit is docs/reports deletion only; release APK build succeeds.
