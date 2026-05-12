# Blocked Production / Hardware / Provider Proof

| Area | Status | Evidence / next step |
|------|--------|----------------------|
| CI / Security / Production Proof on PR head `3fb3f28` | **PASS** | [PR #210](https://github.com/leduytuanvu/avf-vending-api/pull/210) — CI 25723652856, Security 25723652689, Production Proof 25723652868 |
| Security Release (Trivy published app/goose images) | **NOT_TRIGGERED** | Runs after merge + Build and Push on `develop`/`main` — not fired by `pull_request` alone |
| Production read-only HTTP smoke | **NOT_RUN** | Set `STAGING_BASE_URL` or `PRODUCTION_BASE_URL` and run `scripts/test/run-production-readonly-smoke.sh` |
| Production canary E2E | **BLOCKED** | Requires explicit canary env + write confirmation; see `reports/test/production-canary-e2e.md` |
| PSP / payment provider | **BLOCKED** | Sandbox or canary PSP credentials + webhook signing secret not in scope for this workstation proof |
| Physical vending / device ACK | **BLOCKED** | Real machine/simulator and OTA-safe test plan not attached; MQTT/hardware flows covered only via local broker + E2E harness |

Local **payment mock** paths exercised in `tests/e2e` (e.g. scenario 42) do **not** replace PSP production proof.
