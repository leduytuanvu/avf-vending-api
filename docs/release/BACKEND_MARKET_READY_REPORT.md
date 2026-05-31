# Backend market readiness report

**Generated:** 2026-05-31T20:17:48Z (UTC)  
**Repository:** `avf-vending-api`  
**Git SHA (main deployed candidate):** `51b93c55763426a4c2d049f1e262b5fd45ba8105`  
**Branch:** `develop` → promoted to `main` for production deploy  
**Production target:** `https://api.ldtv.dev`

> **Verdict policy:** This document does **not** claim fleet **GO** without **deployed** evidence. `/version.git_sha` must match the deployed **main** commit after env sync — stale `APP_GIT_SHA` on app-node blocks SHA proof until [PR #333](https://github.com/leduytuanvu/avf-vending-api/pull/333) merges and redeploys.

---

## Executive verdict

| Verdict | **GO-CANARY-ONLY** |
|---------|-------------------|

**Rationale:** Production deploy run [`26722922299`](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26722922299) rolled digest `sha256:11a11179…` built from main @ `51b93c55`. Strict readonly smoke [`20260531T201748Z`](../../reports/e2e/production-readonly-smoke/20260531T201748Z/REPORT.md) **PASS** with `READINESS_VERDICT=GO-CANARY-ONLY`, `payment_runtime.payment_mode=cash_only`, admin + gRPC strict probes green. **`/version.git_sha` still reports stale embed override `52a076e…` (fix pending PR #333 redeploy).**

**Blockers before fleet GO or MARKET_READY:**

1. `/version.git_sha` must equal main `51b93c55` after clearing stale `APP_GIT_SHA` on app-node
2. Signed app APKs at current app SHA + `validate-release-ref.sh market-launch`
3. Real-device canary matrix re-stamped at current SHAs (existing JSON is schema-valid but SHA-stale)

---

## Verdict definitions (strict)

| Verdict | When it applies |
|---------|-----------------|
| **GO** | Deployed full readonly smoke PASS **and** family canary cash sale PASS on a marked test machine (real hardware, inventory decrement) **and** higher-level fleet matrix (XY + TCN + TCN-HYBRID + bill evidence) **and** `/version.payment_runtime.payment_mode=cash_only` |
| **GO-CANARY-ONLY** | Deployed strict readonly smoke PASS (all required probes below) **and** `/version.payment_runtime` cash-only contract valid **but** canary live sale not yet run — proceed only with **guarded cash canary** on a marked machine |
| **NO-GO** | Failing tests, failing smoke probes, missing deployed `payment_runtime`, skipped strict canary probes, or canary attempted on non-canary machine |

`scripts/e2e/production-readonly-smoke.sh` **never** writes `READINESS_VERDICT=GO` — only `GO-CANARY-ONLY` or `NO-GO`.

`scripts/e2e/production-canary-live-sale.sh` emits **`FAMILY_CANARY_VERDICT=PASS`** for a successful real-hardware cash sale on one machine; it writes **`READINESS_VERDICT=NO-FLEET-GO`** (never fleet **GO**). Simulated or backend-only dry runs emit **`BACKEND-ONLY-NO-MARKET-GO`**. Fleet **GO** requires the higher-level matrix after XY + TCN + TCN-HYBRID + bill evidence pass.

**Strict canary gate (`PRODUCTION_SMOKE_STRICT_CANARY=true`, default for `https://api.ldtv.dev`):** the following probes must **PASS** (SKIP counts as failure):

- `version.payment_runtime` (+ cash-only contract sub-probe when mode is `cash_only`)
- `admin.auth`
- `grpc.bootstrap`, `grpc.bootstrap.payment_methods`, `grpc.catalog`, `grpc.media_manifest`, `grpc.inventory`, `grpc.planogram`

Non-strict developer smoke may emit `SMOKE_VERDICT=PASS_DEV_ONLY` when credentials or gRPC machine tokens are unset; **`READINESS_VERDICT` remains `NO-GO`** until strict probes pass.

---

## Tests run (repository)

| Command | Environment | Result | Notes |
|---------|-------------|--------|-------|
| `go test ./...` | Local | **PASS** | Required gate before merge/deploy |
| `internal/observability/version_payment_test.go` | Local | **PASS** | `/version.payment_runtime` for cash-only |
| `internal/config/deployment_env_test.go` | Local | **PASS** | Production cash-only + placeholder PSP rejection |
| `internal/platform/payments/production_payment_safety_test.go` | Local | **PASS** | QR/card hidden without wired PSP |
| `internal/grpcserver/machine_commerce_cash_only_test.go` | Local | **PASS** | CreatePaymentSession → `provider_unavailable`; ConfirmCashPayment works |
| `internal/grpcserver/machine_payment_runtime_test.go` | Local | **PASS** | Bootstrap payment_methods mapping |

---

## Production deploy verification (post-release checklist)

After **Build and Push** on `main` and **Deploy Production** (`deploy-prod.yml`):

```bash
curl -fsS https://api.ldtv.dev/version | jq .
```

| Check | Required | Evidence path |
|-------|----------|---------------|
| `payment_runtime` object present | **Yes** | `reports/e2e/production-readonly-smoke/20260531T201748Z/raw/version-payment-runtime.body` |
| `payment_runtime.payment_mode == "cash_only"` | **Yes** | Same artifact + strict smoke probe PASS |
| `git_sha` matches deployed main commit | **PARTIAL** | Deploy manifest `source_commit_sha=51b93c55`; live `/version.git_sha=52a076e` (stale `APP_GIT_SHA` override — fix in PR #333) |
| Strict readonly smoke PASS | **Yes** | [`reports/e2e/production-readonly-smoke/20260531T201748Z/`](../../reports/e2e/production-readonly-smoke/20260531T201748Z/REPORT.md) |
| Deploy manifest archived | **Yes** | GitHub Actions run `26722922299` artifact `production-deployment-manifest` |

**Deploy proof (2026-05-31):**

| Field | Value |
|-------|-------|
| Deploy run | [`26722922299`](https://github.com/leduytuanvu/avf-vending-api/actions/runs/26722922299) |
| Build run | `26722767982` |
| Security release | `26722859567` |
| App digest | `sha256:11a11179c6b0e4f18da1c54ca3106eeebd0d630ed6e1da3346161b2324679f26` |
| Main commit (manifest) | `51b93c55763426a4c2d049f1e262b5fd45ba8105` |
| Live `/version.git_sha` (2026-05-31T20:17Z) | `52a076e340a15a69dad7787cad54d7e3000fcafe` (**mismatch**) |

---

## Verification matrix (code — pre-deploy)

### Payment provider status

| Check | Status | Evidence |
|-------|--------|----------|
| Path B cash-only documented | **PASS** | [`docs/payments/PRODUCTION_PAYMENT_PROVIDER_STATUS.md`](../payments/PRODUCTION_PAYMENT_PROVIDER_STATUS.md) |
| Production requires `PAYMENT_ENV=live` or `cash_only` | **PASS** | `internal/config/deployment_env.go` |
| Placeholder PSPs blocked when `PAYMENT_ENV=live` | **PASS** | Config validation + `internal/platform/payments/` |
| `cash_only` forbids `COMMERCE_PAYMENT_PROVIDER` | **PASS** | `deployment_env.go` |
| Bootstrap `payment_methods` | **PASS** | `GetBootstrapResponse` in `machine_grpc_services.go` |
| `/version.payment_runtime` in code | **PASS** | `internal/observability/version.go` |
| `/version.payment_runtime` on deployed prod | **PENDING DEPLOY** | Must be verified post-release; absent field = **NO-GO** for GO claims |

### Cash-only commerce gRPC

| Check | Status | Evidence |
|-------|--------|----------|
| `CreatePaymentSession` → `provider_unavailable` when QR/card disabled | **PASS** | `machine_commerce_cash_only_test.go` |
| `ConfirmCashPayment` end-to-end | **PASS** | `machine_commerce_cash_only_test.go` |
| Bootstrap `cash_enabled=true`, `qr_card_enabled=false` | **PASS** | `machine_commerce_cash_only_test.go` |

### Build metadata

| Check | Status | Evidence |
|-------|--------|----------|
| Docker image link-time `version.Commit` + `version.BuildTime` | **PASS** | `deployments/prod/Dockerfile` + `_reusable-build.yml` build-args |
| Runtime must not override with stale `APP_GIT_SHA` | **PASS** | `deployments/prod/app-node/.env.app-node.example` documents unset |

---

## Remaining blockers

1. **Deploy to production** — Merge to `main`, build images with git SHA/build time embed, run Deploy Production workflow.
2. **Post-deploy `/version` proof** — Confirm `payment_runtime.payment_mode=cash_only`; save artifact under `reports/e2e/production-readonly-smoke/<timestamp>/raw/`.
3. **Full production readonly smoke** — Re-run with `GRPC_ADDR`, machine token, `TEST_MACHINE_ID`, admin credentials.
4. **Production canary cash sale** — Run `production-canary-live-sale.sh` on a canary-marked machine with operator present.
5. **Live PSP** — Not required for cash-only pilot; blocks QR/card fleet launch.

---

## GO / NO-GO decision record

| Criterion | Result |
|-----------|--------|
| All unit/integration tests pass locally | **GO** (code) |
| Production HTTP health | **Unknown until smoke** |
| Production `/version.payment_runtime` visible with `cash_only` | **NO-GO until post-deploy curl** |
| Production gRPC + bootstrap verified live | **NO-GO** (not run / not deployed) |
| Canary live sale evidence | **NO-GO** (not run) |
| Live PSP ready | **NO-GO** (by design for pilot) |
| Cash-only path tested in code | **GO** (code) |

### Final verdict: **GO-CANARY-ONLY** (code ready; **deployed evidence required**)

Proceed with guarded cash-only canary **only after** deployed `/version.payment_runtime` passes. **Do not** declare general market-ready or fleet QR/card until live PSP wiring, full readonly smoke, and canary artifacts are green.

---

## References

- Machine gRPC: [`docs/api/machine-grpc-production-contract.md`](../api/machine-grpc-production-contract.md)
- MQTT: [`docs/api/mqtt-contract.md`](../api/mqtt-contract.md)
- Payments: [`docs/payments/PRODUCTION_PAYMENT_PROVIDER_STATUS.md`](../payments/PRODUCTION_PAYMENT_PROVIDER_STATUS.md)
- E2E runbook: [`docs/testing/PRODUCTION_E2E_CANARY_RUNBOOK.md`](../testing/PRODUCTION_E2E_CANARY_RUNBOOK.md)
