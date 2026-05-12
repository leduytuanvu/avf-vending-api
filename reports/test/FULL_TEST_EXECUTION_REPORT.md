# FULL TEST EXECUTION REPORT

- Branch: `security/goose-otel-fix`
- Commit: `35a699603143b5048d01b2d8e5defc279472488d`
- PR: https://github.com/leduytuanvu/avf-vending-api/pull/210 → `develop`
- API URL: `http://127.0.0.1:18080`
- gRPC addr: `:9090`
- MQTT broker: `127.0.0.1:1883`
- REST summary: **partial** — 365 ops, **6** pass, **359** blocked (`rest-full-live-coverage.json`)
- gRPC summary: `{'generated_at': '2026-05-12T08:23:55.203480+00:00', 'grpc_addr': '127.0.0.1:9090', 'server_status': 'reachable', 'server_reason': 'grpcurl list succeeded', 'total_methods': 85, 'passed': 0, 'failed': 0, 'partial': 54, 'blocked': 31}`
- MQTT summary: `{'generated_at': '2026-05-12T08:24:38.836871+00:00', 'mqtt_host': '127.0.0.1', 'mqtt_port': '1883', 'mqtt_topic_prefix': 'avf-dev/devices', 'broker_status': 'reachable', 'broker_reason': 'publish/subscribe round-trip evidence captured', 'total_flows': 12, 'passed': 1, 'failed': 0, 'partial': 9, 'blocked': 2}`
- Production smoke: `NOT_RUN`
- Production canary: `BLOCKED`
- PSP: `BLOCKED: PSP sandbox/canary credentials not configured in this run`
- Hardware: `BLOCKED: real canary vending hardware/simulator not attached in this run`
- **Merge:** **NOT_MERGED** — `REVIEW_REQUIRED` / branch policy (`gh pr merge 210 --merge` rejected).
- CI: **PASS** on `35a6996` — CI [25725682062](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25725682062), Security [25725682070](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25725682070), Production Proof [25725682100](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25725682100). Post-merge Build, Security Release, published image Trivy: **NOT_RUN**.

## Commands Run

| Label | Exit | Duration ms | Command |
|---|---:|---:|---|
| `phase0_baseline_secret_scan` | 0 | 3500 | `git status/branch/rev/diff checks + PowerShell secret scan` |
| `python_syntax` | 0 | 1000 | `python -m py_compile scripts/test/rest_full_live_coverage.py scripts/test/generate-full-backend-reports.py` |
| `bash_syntax` | 0 | 1200 | `find scripts tests -name '*.sh' -print0 | xargs -0 -n1 bash -n` |
| `rest_full_live_local` | 0 | 5100 | `python scripts/test/rest_full_live_coverage.py --mode local --base-url http://127.0.0.1:18080 --timeout 1` |
| `full_report_generation` | 0 | 5100 | `python scripts/test/generate-full-backend-reports.py` |
| `gofmt` | 0 | 1000 | `gofmt -l .` |
| `go_vet` | 0 | 8000 | `go vet ./...` |
| `go_test` | 0 | 49000 | `go test ./... -count=1` |
| `python_compileall` | 0 | 3000 | `python -m compileall scripts tools tests` |
| `docker_compose_config` | 0 | 1000 | `docker compose -f deployments/docker/docker-compose.yml config` |
| `race_test` | 1 | 39000 | `CGO_ENABLED=1 go test -race ./... -count=1` |
| `govulncheck_host_toolchain` | 3 | 24000 | `govulncheck ./...` |
| `govulncheck_pinned_toolchain` | 0 | 29500 | `GOTOOLCHAIN=go1.25.10 govulncheck ./...` |
| `trivy_local` | 127 | 1000 | `trivy --version && trivy fs ...` |
| `local_e2e_bash` | 0 | 600000 | `bash tests/e2e/run-all-local.sh --fresh-data` |
| `grpc_full_bash` | 0 | 5000 | `bash scripts/test/run-grpc-full-coverage.sh (PYTHON=/c/Python314/python.exe, GRPC_ADDR=127.0.0.1:9090)` |
| `mqtt_full_bash` | 0 | 7000 | `bash scripts/test/run-mqtt-full-coverage.sh (mosquitto + PYTHON on PATH)` |
| `production_readonly_smoke` | 1 | 0 | `bash scripts/test/run-production-readonly-smoke.sh` |

## Final Claim

**BLOCKED:** PR CI/Security/Production Proof **passed** on head `35a6996`, but **merge to develop did not occur** (required review). Post-merge Build / Security Release / image Trivy **not executed**. Production read-only smoke **NOT_RUN** (URL env unset). Canary/PSP/hardware blocked. REST remains **6/365 partial** — no full live REST claim.
