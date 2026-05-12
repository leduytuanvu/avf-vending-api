# Production Proof Report

- **Production read-only smoke:** NOT_RUN (no `STAGING_BASE_URL` / `PRODUCTION_BASE_URL` in environment)
- **Reason:** Read-only staging/production probes require an explicit base URL; none was configured for this run.
- **Next action:** Export a read-only-safe URL and run  
  `BASE_URL="$STAGING_BASE_URL" bash scripts/test/run-production-readonly-smoke.sh`

## Local executable proof (this host)

- **Branch / commit:** `security/goose-otel-fix` @ `35e527857dfc2f42da41de9ab7e593728cacbbdb`
- **Go unit tests:** pass (`go test ./... -count=1` with clean DB `avf_vending_test_full_final`, DSN password redacted in docs)
- **Race (`-race`):** BLOCKED locally (no gcc); **CI:** Linux race gate in Production Proof workflow
- **govulncheck:** pass with `GOTOOLCHAIN=go1.25.10`
- **Trivy:** BLOCKED locally (binary not installed); **CI:** Security / image workflows
- **Full local E2E:** pass (23 passed, 0 failed, 0 skipped; Phase 8–42; session evidence under `reports/test/e2e-evidence/` — do not commit raw `.e2e-runs` dirs)
- **REST full live coverage:** `reports/test/rest-full-live-coverage.json` (conservative OpenAPI runner — many routes blocked/partial by design)
- **gRPC full coverage:** `reports/test/grpc-full-coverage.json`
- **MQTT full coverage:** `reports/test/mqtt-full-coverage.json`

## CI proof

- **Status:** pass (per `gh pr checks` on branch `security/goose-otel-fix`; includes Go CI Gates, Linux race + contract, vulnerability scan, security workflows — confirm on GitHub if HEAD changes)

## Secret audit

- **Status:** pass — no live tokens or private keys committed with these reports.
