# Blocked Production / Hardware / Provider Proof

| Area | Status | Evidence / next step |
|------|--------|----------------------|
| CI / Security / Production Proof on PR head `d7d28ec` | **PASS** | [PR #210](https://github.com/leduytuanvu/avf-vending-api/pull/210) — CI [25726935875](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25726935875), Security [25726935841](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25726935841), Production Proof [25726935862](https://github.com/leduytuanvu/avf-vending-api/actions/runs/25726935862) |
| Merge to `develop` | **NOT_MERGED** | Required review; **also** `mergeStateStatus` **BEHIND** (sync `develop`); `gh pr merge` rejected when out of date |
| Security Release (Trivy published app/goose images) | **NOT_RUN** | **Build and Push Images** on `develop` not triggered because merge did not land |
| Production read-only HTTP smoke | **NOT_RUN** | Set `STAGING_BASE_URL` or `PRODUCTION_BASE_URL` and run `scripts/test/run-production-readonly-smoke.sh` |
| Production canary E2E | **BLOCKED** | Requires explicit canary env + write confirmation; see `reports/test/production-canary-e2e.md` |
| PSP / payment provider | **BLOCKED** | Sandbox or canary PSP credentials + webhook signing secret not in scope for this workstation proof |
| Physical vending / device ACK | **BLOCKED** | Real machine/simulator and OTA-safe test plan not attached; MQTT/hardware flows covered only via local broker + E2E harness |

Local **payment mock** paths exercised in `tests/e2e` (e.g. scenario 42) do **not** replace PSP production proof.
