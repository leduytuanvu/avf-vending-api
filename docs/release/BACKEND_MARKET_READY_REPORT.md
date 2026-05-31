# Backend market readiness report

**Generated:** 2026-05-31 (UTC)  
**Repository:** `avf-vending-api`  
**Git SHA (working tree baseline):** see `git rev-parse HEAD` at commit time  
**Branch:** `develop` → promoted to `main` for production deploy  
**Production target:** `https://api.ldtv.dev`

> **Verdict policy:** This document does **not** claim fleet **GO** without **deployed** evidence. Repository code/tests can pass while production still lags — `/version.payment_runtime` on the live API is the gate for cash-only canary readiness.

---

## Executive verdict

| Verdict | **GO-CANARY-ONLY** (until deployed proof) |
|---------|-------------------------------------------|

**Rationale:** Backend code supports `PAYMENT_ENV=cash_only`, production config rejects unwired placeholder PSP keys, `/version` exposes `payment_runtime`, machine bootstrap exposes `payment_methods` (cash/QR flags, mode, provider status/reason), and commerce gRPC blocks QR/card sessions while allowing `ConfirmCashPayment`. Local `go test ./...` passes.

**Deployed production is not yet verified in this report** until post-deploy curl confirms:

1. `GET /version` includes `payment_runtime.payment_mode == "cash_only"`
2. `git_sha` in `/version` matches the **main** commit SHA of the deployed image (link-time embed; do not override with stale `APP_GIT_SHA` on app nodes)
3. Full readonly smoke (HTTP + gRPC + admin) and canary cash sale artifacts are archived under `reports/e2e/`

**Do not claim general fleet GO** until all three are green. **Do not claim GO-CANARY-ONLY** from code alone if deployed `/version` lacks `payment_runtime`.

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
| `payment_runtime` object present | **Yes** | `reports/e2e/production-readonly-smoke/<timestamp>/raw/version-payment-runtime.body` |
| `payment_runtime.payment_mode == "cash_only"` | **Yes** | Same artifact + jq |
| `git_sha` matches deployed main commit | **Yes** | Compare to promotion manifest / image-metadata |
| `build_time` non-empty | **Yes** | Link-time inject via `deployments/prod/Dockerfile` build-args |

**Prior production state (pre-deploy):** readonly smoke artifacts showed `version.payment_runtime` **SKIP** (field absent). That state is **NO-GO** for canary claims regardless of HTTP health — strict mode now **FAIL**s this case and never emits `GO-CANARY-ONLY`.

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
