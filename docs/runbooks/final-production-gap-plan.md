# Final production-readiness gap plan (AVF vending API)

**Scope:** Audit-only document. **No runtime behavior** is changed by this file.

**North star:** Same boundaries as [`docs/architecture/production-final-contract.md`](../architecture/production-final-contract.md) and [`docs/architecture/transport-boundary.md`](../architecture/transport-boundary.md).

---

## P0 — Blockers before production go-live

These items violate the **target architecture** or create **unmitigated money/replay risk** if shipped without a documented compensating control.

| # | Gap | Exact locations | Risk | Recommended fix | Tests |
| --- | --- | --- | --- | --- | --- |
| P0-1 | **No server-side PSP `CreatePaymentSession` on the machine path** — persistence is `StartPaymentWithOutbox` with **caller-built** JSON; outbound `PaymentProvider.CreatePaymentSession`.
 | `internal/platform/payments/provider.go` (interface + “not all providers implement”); `internal/platform/payments/mock_provider.go`; `internal/app/commerce/service.go` (`StartPaymentWithOutbox`); `internal/grpcserver/machine_commerce_grpc.go` (`CreatePaymentSession` — no PSP adapter call) | Integrator or malicious client can supply arbitrary **QR / payment URLs** in outbox payload while backend records a “session”; fails **backend-owned PSP session** and weakens fraud accountability | Wire a real provider adapter: API creates payment server-side, persists **provider_reference** from PSP response, puts **only server-generated** checkout hints in outbox envelope; restrict or drop client-supplied URL fields for non-cash | Unit/integration tests: gRPC payment session with mock PSP returning fixed URL; assert DB payment + outbox match PSP output; reject client-supplied URLs when policy=ON |
| P0-2 | **Client-supplied `outbox_payload_json` drives `QrPayloadOrUrl`** — machine can echo **qr_url / payment_url / checkout_url** from its own JSON into the RPC response | `internal/grpcserver/machine_commerce_grpc.go` (`CreatePaymentSession`, `extractQRPayloadFromOutboxJSON`); `internal/app/commerce/payment_session_contract.go` (`extractHostedCheckoutURLs` — HTTP parity) | Phishing, **amount/UI mismatch** vs webhook, training users to trust machine-supplied payment surfaces | Same as P0-1: server-only payload construction; optional signing of display hints; proto/doc deprecation of “Bring your own payload” for card flows | Contract tests: tampered `out-box` keys rejected; golden tests for allowed envelope schema |
| P0-3 | **JetStream internal outbox has publisher in `cmd/worker` but no in-repo consumer loop** for `avf.internal.outbox.>` | `internal/platform/nats/doc.go` (explicit); `internal/platform/nats/publisher_outbox.go`; `cmd/worker/main.go` (publisher wiring); `internal/platform/nats/consumer.go` (`EnsureOutboxPullConsumer` / `BindOutboxPull` — **no call sites** in `cmd/*`) | Revenue/compliance **integration events** never delivered unless an **external** consumer exists — silent loss of downstream automation | Either implement `cmd/outbox-consumer` (or worker mode) with handlers + DLQ + replay **or** contractually mandate and verify an external consumer in staging/prod | Consumer integration tests; staging soak with message drain; runbook for DLQ replay (`docs/runbooks/outbox-dlq-debug.md`) |
| P0-4 | **Promotion / catalog fingerprint ≠ checkout pricing** — catalog version uses a **placeholder** for promotions while checkout uses **slot `price_minor` only** | `internal/app/salecatalog/fingerprint.go` (`PromotionsFingerprintPlaceholder`); `internal/modules/postgres/commerce_sale_line.go` (`ResolveSaleLine` / `saleLineFromRow`); `internal/app/commerce/service.go` (`CreateOrder` → `ResolveSaleLine`) | Customers see **promo or catalog UX** that **cannot** match charged totals; reconciliation and trust break when promotions go live | Unify: fold **effective promotion-adjusted line totals** into both runtime catalog projection and `ResolveSaleLine`, and mix real promo material into `RuntimeSaleCatalogFingerprint` | Golden tests: catalog item price == `CreateOrder` totals for promo fixtures; fingerprint bump on promo change |
| P0-5 | **Legacy machine REST and commerce HTTP still compiled in** — production safety depends on **env flags** (`MACHINE_REST_LEGACY_ENABLED`, `MACHINE_GRPC_ENABLED`, webhook secrets) | `internal/httpserver/server.go` (groups using `machineLegacyRESTGuard`); `internal/httpserver/transport_legacy_guard.go`; `internal/httpserver/commerce_http.go`; `internal/config/config.go` (`TransportBoundary` / `MachineRESTLegacy*`); `docs/api/machine-grpc.md` | Misconfiguration re-enables **non-target** HTTP machine plane or leaves **unsigned webhooks** acceptable if unsafe flags leak | CI/config guard: **fail** `docker-compose`/helm if production profile has legacy machine REST or unsafe webhook flags; production smoke **requires** gRPC + HMAC | `internal/config/config_test.go` forbidden combinations; deploy template tests; `internal/httpserver/openapi_spec_test.go` (machine HTTP deprecation contract) |

**Staging proof P0:** Before go-live, run an end-to-end scenario on **staging-like env flags**: native gRPC only (`MACHINE_GRPC_ENABLED=true`, `MACHINE_REST_LEGACY_ENABLED=false`), TLS’d MQTT, HMAC webhooks only, NATS required if outbox publisher required — with **no break-glass unsafe secrets**.

---

## Audit trace — requirement checklist

### 1. Server-owned payment session creation

| Pri | Gap | Files | Risk | Fix | Tests |
| --- | --- | --- | --- | --- | --- |
| P0 | See **P0-1** | Same as P0-1 | Backend does not own PSP session | Implement real adapter path; document which providers are supported | PSP adapter integration tests |

### 2. Client-supplied `providerReference` / payment URL / QR risks

| Pri | Gap | Files | Risk | Fix | Tests |
| --- | --- | --- | --- | --- | --- |
| P0 | Machine **`CreatePaymentSession`** accepts **free-form** `outbox_payload_json` and returns **embedded URL/QR hint** | `internal/grpcserver/machine_commerce_grpc.go` | User directed to attacker-controlled payment surface | Server-generated payload only; schema validation | See P0-2 |
| P1 | **`provider_reference` trust model** on webhooks — correct for provider-asserted events, but must not be confused with machine-supplied refs | `internal/app/commerce/service.go` (`ApplyPaymentProviderWebhook`); `internal/modules/postgres/commerce_webhook.go` | Ops assumes machine provided ref | Docs: only webhook establishes provider ref for settlement | Webhook idempotency / mismatch tests (existing — extend for machine path) |
| P1 | HTTP **`StartPaymentWithOutbox`** input surfaces for admins/machines — same “caller-supplied provider/outbox” pattern | `internal/httpserver/commerce_http.go` | Same as gRPC if integrator passes rich JSON | Align HTTP with gRPC policy once PSP path exists | HTTP handler tests with disallowed keys |

### 3. Legacy machine HTTP routes

| Pri | Gap | Files | Risk | Fix | Tests |
| --- | --- | --- | --- | --- | --- |
| P1 | **Large HTTP surface** remains under `machineLegacyRESTGuard`: bootstrap, sale catalog, shadow, telemetry **GETs**, device bridge, operator sessions, **commerce** | `internal/httpserver/server.go`; `activation_http.go` (setup); `sale_catalog_http.go`; `telemetry_http.go`; `device_http.go`; `machine_runtime_http.go`; `operator_http.go`; `commerce_http.go` | Long **strangler** tail; accidental enablement in prod | Force **metrics + 404** in prod; delete deprecated routes after fleet cutover | Route-mount tests; integration with guard off |
| P2 | **Command poll HTTP fallback** documented as degraded mode | `internal/httpserver/router.go` (comments); `internal/httpserver/device_http.go` | Ops confuses fallback with primary | Runbook prominence; alert if MQTT publish healthy but poll dominates | Metrics differentiation |

### 4. Machine gRPC-only final contract

| Pri | Gap | Files | Risk | Fix | Tests |
| --- | --- | --- | --- | --- | --- |
| P1 | **Dual stack** (HTTP + gRPC) until fleet fully migrated | `docs/api/machine-grpc.md`; `internal/grpcserver/machine_grpc_services.go` | Client ships wrong stack | Field checklist + version gate on app | gRPC integration tests (existing suites) |
| P2 | **Proto drift** — clients must track `proto/avf/machine/v1/*.proto` | `proto/avf/machine/v1/machine_runtime.proto` | Breaking changes unnoticed | Buf breaking change checks in CI | Already in `proto/buf.yaml` / CI — verify enabled on release branches |

### 5. Pricing / promotion / order / payment amount consistency

| Pri | Gap | Files | Risk | Fix | Tests |
| --- | --- | --- | --- | --- | --- |
| P0 | See **P0-4** | Same | Catalog/checkout mismatch | Single pricing authority | Golden vector tests |
| P1 | **Webhook amount vs order** handling — needs sustained alerting | `internal/modules/postgres/commerce_webhook.go`; `deployments/prod/observability/prometheus/alerts.yml` (partial coverage via reconciler) | Silent undercharges/overcharges | Ensure `webhook_amount_currency_mismatch` (or equivalent) has **critical** path + runbook | extend `internal/modules/postgres/*integration_test.go`; alert unit tests |

### 6. MQTT TLS / ACL / ACK / QoS / reconnect

| Pri | Gap | Files | Risk | Fix | Tests |
| --- | --- | --- | --- | --- | --- |
| P1 | **TLS optional via env**; **InsecureSkipVerify** exists | `internal/platform/mqtt/config.go`; `internal/platform/mqtt/publisher.go` (`applySecurity`) | MITM if mis-set | Production profile forbids skip-verify; mTLS rollout per EMQX | Config validation tests in `internal/config` / `internal/platform/mqtt/config_test.go` |
| P2 | **Publisher uses QoS 1 + auto-reconnect** — good baseline; **ACL** and device identity live in broker config | `internal/platform/mqtt/publisher.go`; `deployments/prod/emqx/base.hocon`; `deployments/staging/emqx/base.hocon` | Broker mis-ACL leaks commands | EMQX review checklist; least-privilege users | Staging smoke TLS + subscribe denial tests |
| P1 | **Application ACK** vs transport ACK — contract enforcement is cross-cutting | `docs/api/mqtt-contract.md`; `testdata/telemetry/valid_command_ack.json`; `internal/app/telemetryapp/*` | Device deletes local work too early | Field runbook + firmware review | Contract tests on sample payloads |

### 7. Outbox / NATS / JetStream / DLQ / replay

| Pri | Gap | Files | Risk | Fix | Tests |
| --- | --- | --- | --- | --- | --- |
| P0 | See **P0-3** | `internal/platform/nats/*`; `cmd/worker/main.go` | Downstream starvation | Consumer process or external SLA | Consumer tests |
| P1 | **Worker** ensures streams/consumers for **telemetry**; **outbox pull consumer** helper unused | `cmd/worker/main.go`; `internal/platform/nats/telemetry_consumer.go` | Asymmetric maturity | Wire or delete dead helper | Lint for dead code policy |
| P1 | **DLQ** semantics for outbox publish side | `internal/platform/nats/outbox_dead_letter.go`; `internal/app/background/worker.go` | Poison messages stuck | Operational replay documented | `internal/app/background/worker_outbox_test.go` extensions |

### 8. Media — object storage / HTTPS / local cache

| Pri | Gap | Files | Risk | Fix | Tests |
| --- | --- | --- | --- | --- | --- |
| P1 | **Upload + presign** path must match **machine catalog** URL strategy | `internal/app/mediaadmin/*`; `internal/platform/objectstore/store.go`; `internal/grpcserver/machine_media_grpc.go` | Broken images offline | Align cache keys with epoch headers | `internal/app/mediaadmin/*_test.go`; field smoke |
| P2 | **CDN / long TTL** outside repo | `docs/runbooks/product-media-cache-invalidation.md` | Stale branding | CDN invalidation playbook tied to media epoch | Manual runbook QA |

### 9. Observability / alerts / runbooks

| Pri | Gap | Files | Risk | Fix | Tests |
| --- | --- | --- | --- | --- | --- |
| P1 | **Alerts exist** for outbox lag, telemetry lag, webhook HMAC spikes, paid-no-vend — but **not every money edge** has a dedicated alert | `deployments/prod/observability/prometheus/alerts.yml`; `docs/runbooks/observability-alerts.md` | Blind spots | Add rules for webhook unsigned rejections / config unsafe mode if exposed via metrics | Promtool CI |
| P2 | **Dashboard coverage** vs new commerce/MQTT metrics | `deployments/prod/observability/grafana/provisioning/dashboards/json/*.json` | Slower incidents | Extend dashboards for machine gRPC error codes | — |

### 10. Production deployment / secrets safety

| Pri | Gap | Files | Risk | Fix | Tests |
| --- | --- | --- | --- | --- | --- |
| P0 | **`COMMERCE_PAYMENT_WEBHOOK_UNSAFE_*` / unsigned paths** must never be default in prod | `internal/config/config.go`; `internal/httpserver/commerce_webhook_public.go`; `docs/api/payment-webhook-security.md` | Fraudulent capture events | Hard deny in prod profile | `internal/config/config_test.go` |
| P1 | **Secret matrix** spread across compose examples | `.env.production.example`; `deployments/prod/**/*example*`; `docs/contracts/deployment-secrets-contract.yml` | Wrong secret plane | Single source doc + preflight CLI | Config load tests |
| P1 | **Two-VPS / split topology** operational maturity | `docs/operations/two-vps-rolling-production-deploy.md`; `deployments/prod/shared/README.md` | Wrong failover | Game days | Checklist signoff |

---

## Commands (requested)

### `git diff --name-only`

At the time of this audit, the working tree reported **many** modified paths (application code, `db/`, `deployments/`, `.github/workflows/`, docs, etc.). **Do not treat that snapshot as release-ready** — re-run the command at merge/tag time and ensure only intended paths ship.

### `go test ./... -count=1`

**Result:** **PASS** (full module, 2026-04-29).

---

## Final merge sequence (recommended)

Land work in **dependency order** so each PR restores a contract before building on it:

1. **Merge A — P0 payment truthfulness**  
   Server-owned PSP session adapter + **remove/neutralize client-supplied payment surfaces** (gRPC + HTTP) behind feature flags if needed; extend tests.

2. **Merge B — P0 outbox consumer story**  
   In-repo consumer **or** locked external consumer SLO + verification harness; DLQ/replay runbooks exercised once.

3. **Merge C — P0 pricing/promotion consistency**  
   Single authoritative line totals + fingerprint inputs; migration for any catalog rows.

4. **Merge D — P0 production config guardrails**  
   Fail fast on unsafe webhook + legacy machine REST in prod templates; CI asserts env matrices.

5. **Merge E — P1 operational hardening**  
   MQTT TLS/mTLS checklist execution, alert gaps, Grafana updates, EMQX ACL review.

6. **Merge F — P2 deprecation / cleanup**  
   Remove dead HTTP routes after fleet metrics show zero use; OpenAPI polish; documentation-only tidy-ups.

---

## Document control

| Version | Date | Notes |
| --- | --- | --- |
| 2.0 | 2026-04-29 | Full audit: payment session ownership, client URL/payload risks, legacy HTTP inventory, JetStream outbox consumer gap, promo/fingerprint mismatch, MQTT/NATS items, observability/secrets; `go test ./... -count=1` PASS. Added [`production-final-contract.md`](../architecture/production-final-contract.md). |
| 1.0 | 2026-04-29 | Prior draft (superseded by 2.0). |

---

## Acceptance checklist (this phase)

- [x] **`docs/runbooks/final-production-gap-plan.md`** updated (this file).
- [x] **`docs/architecture/production-final-contract.md`** created.
- [x] **No runtime code** or generated/swagger changes in this phase.
- [x] **P0 blockers** called out **before** operational P1/P2 items.
- [x] Each gap lists **concrete files**, **risk**, **fix**, **tests**, **priority**.
