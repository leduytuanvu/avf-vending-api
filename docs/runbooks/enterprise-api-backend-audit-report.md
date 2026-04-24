# Enterprise API / backend / CI-CD audit report

> **Historical snapshot only (pre-P0 HTTP completion).** For current readiness gates use [final-enterprise-audit.md](./final-enterprise-audit.md), [production-release-readiness.md](./production-release-readiness.md), and `make verify-enterprise-release`. P0 routes listed below as “missing” in sections A–D are **superseded** by the current `internal/httpserver/server.go` tree and `docs/swagger/swagger.json`.

**Scope:** Repository state as audited (AVF `avf-vending-api`). **Method:** code and workflow inspection; docs treated as non-authoritative when they disagree with route registration or tests.

**Auditor notes:** `internal/auth/*` is not present; auth lives in `internal/platform/auth/*`. Remaining **future** HTTP surfaces belong in [roadmap.md](../api/roadmap.md) only until mounted.

---

## A. Executive verdict

| Gate | Verdict |
| --- | --- |
| **READY_FOR_PILOT** | **Conditional yes** — only if pilot accepts: manual provisioning/JWT issuance, no dedicated runtime sale-catalog HTTP, manual refunds/settlement, MQTT-first telemetry with **no** application-level reconcile API, and org-admin **cross-tenant machine UUID** risk at HTTP edge until DB-bound middleware lands. |
| **READY_FOR_100_MACHINES** | **No** (or **high-risk yes** with same mitigations and ops bandwidth). |
| **READY_FOR_500_MACHINES** | **No.** |
| **READY_FOR_1000_MACHINES** | **No.** |
| **READY_FOR_ENTERPRISE_PRODUCTION_RELEASE** | **No** at claimed feature completeness; **possible** for a **narrow** pilot with explicit gap register and storm evidence for the **declared** scale tier. |

**Reasons (strict):**

1. **Missing HTTP APIs** for activation/provisioning, runtime **sale catalog**, **cashbox/settlement** sessions, **refund/cancel** CRUD, **catalog write** CRUD, and **device-scoped telemetry reconcile/ACK** — not in Chi router / OpenAPI required set.
2. **P0 product gap** acknowledged in code: `internal/app/telemetryapp/critical_telemetry_ack_doc_test.go` — no device-scoped HTTP/MQTT reconcile proving critical rows persisted; PUBACK ≠ business ACK.
3. **Tenant isolation:** `internal/platform/auth/principal.go` `CanAccessMachineRead` allows **any** `org_admin` with org claim through HTTP for **any** `machineId` before DB check; `internal/app/api/shadow_sql.go` `GetShadow` does not assert principal org vs `machines.organization_id`.
4. **Device bridge:** `internal/httpserver/device_http.go` requires **platform_admin/org_admin**, not machine token — kiosk cannot use `/v1/device/...` without admin JWT unless product intentionally uses commerce HTTP only.
5. **Commerce refunds:** `EvaluateRefundEligibility` exists (`internal/app/commerce/service.go`); **no** dedicated refund HTTP routes found in httpserver; vend failure path exists; duplicate refund prevention not fully audited without refund API tests.
6. **Storm / scale:** Staging workflow `.github/workflows/telemetry-storm-staging.yml` supports **100x100, 500x200, 1000x500** scenarios; production deploy `.github/workflows/deploy-prod.yml` has **fleet_scale_target** and evidence inputs — **PASS** still depends on running suite and attaching artifacts; not a substitute for missing business APIs.
7. **Catalog:** `internal/httpserver/admin_catalog_http.go` is **read-only** (no POST/PUT for product CRUD in grep); runtime sale catalog route **absent**.
8. **Healthcheck `|| true`:** `deployments/prod/scripts/healthcheck_prod.sh` uses `|| true` in diagnostic paths by design; `check_monitoring_readiness.sh` documents intentional `|| true` with aggregated failure — **not** a silent bypass if final exit fails the script (verify locally for your wrapper).

---

## B. Đã đạt (completed / implemented with evidence)

| Area | Evidence |
| --- | --- |
| Technician **auth** | `POST /v1/auth/login`, `refresh`, `me`, `logout` — `internal/httpserver` + `REQUIRED_OPERATIONS` |
| **Operator session** | `/v1/machines/{machineId}/operator-sessions/*` — `server.go`, `operator_http.go`, Swagger |
| **Bootstrap** | `GET /v1/setup/machines/{machineId}/bootstrap` |
| **Topology / planogram / sync** | `PUT .../topology`, `PUT .../planograms/draft`, `POST .../planograms/publish`, `POST .../sync` |
| **Slots / inventory** | `GET .../slots`, `GET .../inventory`, `GET .../inventory-events`, `POST .../stock-adjustments` (idempotent) |
| **Cash sale** | `POST /v1/commerce/cash-checkout` |
| **Wallet / PSP path** | `POST /v1/commerce/orders`, `.../payment-session`, `.../webhooks` (HMAC) |
| **Vend** | `POST .../vend/start`, `.../success`, `.../failure` |
| **Device vend bridge** | `POST /v1/device/machines/{machineId}/vend-results` |
| **HTTP command poll fallback** | `POST /v1/device/machines/{machineId}/commands/poll` |
| **Check-in / config apply** | `POST /v1/machines/{machineId}/check-ins`, `.../config-applies` |
| **Shadow + telemetry reads** | `GET .../shadow`, `GET .../telemetry/snapshot|incidents|rollups` |
| **Reporting** | `/v1/reports/*` four routes |
| **Admin fleet lists** | `/v1/admin/machines`, technicians, assignments, commands, ota |
| **Health / version** | `GET /health/live`, `/health/ready`, `/version` |
| **Metrics** | `GET /metrics` when enabled — `internal/httpserver/server.go` |
| **Swagger** | Generated; **production server first** asserted in `tools/build_openapi.py` (~L2069–L2110) |
| **MQTT ingest + dedupe** | `internal/platform/mqtt`, `docs/api/mqtt-contract.md`, `testdata/telemetry/*.json` |
| **JetStream / worker** | `internal/platform/nats/*`, `internal/app/telemetryapp/*` |
| **DB pool per process** | `internal/config/config.go` `PostgresConfig`, overrides `API_DATABASE_MAX_CONNS`, `WORKER_*`, tests in `config_test.go` |
| **CI gates** | `Makefile` `ci-gates` = fmt, vet, placeholders, wiring, migrations, sqlc-check, swagger-check |
| **Production deploy single canonical** | `.github/workflows/deploy-production.yml` is **pointer only**; real deploy `deploy-prod.yml` |
| **Digest-pinned / rollback inputs** | `deploy-prod.yml` workflow_dispatch inputs |
| **Storm staging workflow** | `telemetry-storm-staging.yml` scenarios **100x100, 500x200, 1000x500** |
| **Commerce reconciliation read** | `GET /v1/commerce/orders/{orderId}/reconciliation` |
| **Refund eligibility (service)** | `EvaluateRefundEligibility` — `internal/app/commerce/service.go` |
| **Inventory idempotency from telemetry** | `telemetry_store.go` `AppendInventoryEventFromDeviceTelemetry` requires idempotency_key |
| **Kiosk docs** | `docs/api/kiosk-app-flow.md`, `kiosk-app-implementation-checklist.md`, `api-surface-audit.md` |

---

## C. Chưa đạt (missing or partial)

| Flow | Status | Notes |
| --- | --- | --- |
| First install / **activation / provisioning** | **MISSING** | No routes in `httpserver`; no tests grep match |
| **Runtime sale catalog** (price, stock, image, configVersion) | **MISSING** | No `sale-catalog` route; bootstrap catalog only at setup |
| **Product/brand/category/tag CRUD** | **MISSING** (HTTP) | Admin catalog handlers are **GET**-only |
| **Product image binding** | **MISSING** (HTTP) | No `PUT .../image` in router |
| **Cancel unpaid order** | **PARTIAL / unverified** | No dedicated cancel route in required ops; may be internal only |
| **Refund after paid vend failure** | **PARTIAL** | Eligibility in service; **no** public refund API in httpserver audit |
| **Critical telemetry **application** ACK / reconcile** | **MISSING** | Explicit P0 gap in `critical_telemetry_ack_doc_test.go` |
| **Cashbox / recycler settlement** | **MISSING** (HTTP) | DB tables may exist; **no** admin cashbox/collection routes in server |
| **`/v1/device/*` for machine JWT** | **PARTIAL** | Requires **admin** role today (`device_http.go` L33–36) |
| **Cross-tenant machine read hardening** | **PARTIAL** | `CanAccessMachineRead` + handlers like `shadow_sql.go` without org join |
| **Activation rate limit** | **MISSING** | No activation endpoint to rate-limit |

---

## D. API thiếu (missing APIs)

| P | Missing API | Why needed | Expected route (indicative) | Client | Acceptance criteria |
| --- | --- | --- | --- | --- | --- |
| **P0** | Machine **activation / claim** | Fleet scale provisioning | `POST /v1/.../activate`, `POST /v1/.../claim` | Kiosk, Admin | Idempotent claim; rate limit; tests |
| **P0** | **Runtime sale catalog** | Kiosk UX, cache-friendly | `GET /v1/machines/{id}/sale-catalog` | Kiosk | configVersion, price, stock, availability, image hash/URL |
| **P0** | **Telemetry reconcile / ACK** | Business durability vs PUBACK | `POST /v1/machines/{id}/events/reconcile` (+ status GET) | Kiosk | Duplicate-safe; aligns with OLTP projection |
| **P0** | **Refund / cancel** HTTP | Paid + vend failure compliance | `POST /v1/commerce/orders/{id}/refunds`, cancel unpaid | Kiosk, Admin | Idempotent; no double refund; amount ≤ capture |
| **P1** | **Cashbox settlement** | Field ops | `GET .../cashbox`, `POST .../cash-collections`, `close` | Admin | Operator session; idempotent close |
| **P1** | **Catalog CRUD + image** | Self-serve merchandising | `/v1/admin/products` POST/PATCH, `PUT .../image`, tags | Admin | sqlc + tests + Swagger |
| **P2** | **Device bridge with machine token** | HTTP fallback without admin JWT | Policy change on `/v1/device/...` | Kiosk integration | Security review + tests |

---

## E. API thừa / dư / trùng (excess / duplication / risk)

| Endpoint / area | Issue | Action | Risk if unchanged |
| --- | --- | --- | --- |
| `GET .../telemetry/*` vs MQTT firehose | Overlap in **observability**; HTTP is projection | **document** (MQTT primary) | Clients misuse HTTP for volume |
| `POST .../commands/poll` vs MQTT dispatch | **Fallback** | **document / mark** in OpenAPI extensions | Ops treats poll as primary |
| `GET .../bootstrap` vs future **sale-catalog** | Will **overlap** catalog data | **merge** conceptual model when sale-catalog ships | Conflicting cache sources |
| `GET .../shadow` | Broad machine read + weak org edge | **restrict** + DB tenant middleware | Cross-tenant read |
| Commerce **GET** lists `/v1/orders`, `/v1/payments` | Broad admin back-office | **keep**; kiosk should not call | Accidental data exposure if token wrong |
| **Swagger** public | Intentional when enabled | **document** | Fingerprinting |
| **/metrics** unauthenticated | Default when enabled | **restrict** network | Scraping / DoS |

---

## F. Rủi ro còn lại (remaining risks)

| P | Risk | Flow | Files | Mitigation |
| --- | --- | --- | --- | --- |
| **P0** | No **application-level** critical ACK | Offline replay / storm reconnect | `critical_telemetry_ack_doc_test.go`, `mqtt-contract.md` | Ship reconcile API + device outbox policy |
| **P0** | **org_admin** + guessed **machineId** | Shadow / telemetry HTTP reads | `principal.go`, `shadow_sql.go`, `server.go` | DB-bound `machines.organization_id` middleware |
| **P0** | **Refund** path incomplete at HTTP | Vend failure + PSP | `commerce/service.go`, httpserver gap | Refund idempotent API + tests |
| **P1** | **Device bridge** admin-only | HTTP vend-results / poll | `device_http.go` | Machine-scoped policy or document admin-only proxy |
| **P1** | **EMQX reconnect storm** | 1000 machines online | `docs/runbooks/*`, ops | Broker sizing, client jitter docs (checklist exists) |
| **P1** | **DB pool** aggregate limits | Scale-out | `config.go`, deploy docs | Calculate Σ process MaxConns vs `max_connections` |
| **P2** | **healthcheck_prod.sh** `|| true` | Diagnostics | `deployments/prod/scripts/healthcheck_prod.sh` | Ensure final exit code still fails smoke |
| **P2** | Docs claim **complete** APIs | Android checklist vs code | `docs/api/kiosk-app-implementation-checklist.md` | Version-gate “confirm OpenAPI” rows |

---

## G. Historical action list (superseded — verify current tree)

**Note:** This table captured gaps at audit time. The items below are now **implemented** in-repo (tenant middleware, commerce refund/cancel, reconcile HTTP, sale catalog, catalog CRUD, cash settlement, OpenAPI alignment). Use [api-surface-audit.md](../api/api-surface-audit.md) and `make verify-enterprise-release` as the source of truth.

| P | File | Change (was) | Test |
| --- | --- | --- | --- |
| P0 | `internal/httpserver/*` | Tenant-bound middleware for machine-scoped routes | `httpserver` + integration |
| P0 | `internal/httpserver/commerce_http.go` | Refund/cancel routes | Commerce integration |
| P0 | Telemetry reconcile handlers | Machine-scoped POST + status | telemetry integration |
| P1 | `internal/httpserver/admin_catalog_http.go` | CRUD + image | catalog tests |
| P1 | `internal/httpserver/server.go` | Sale catalog GET | handler tests |
| P1 | `internal/httpserver/device_http.go` | HTTP device bridge policy | auth tests |
| P1 | `tools/build_openapi.py` + `swagger_operations.go` | OpenAPI annotations | `make swagger-check` |
| P2 | `docs/api/*` | Align checklist vs shipped | manual review |

---

## H. 0–1000 machine readiness

**Offline storm:** Many machines offline buffer critical events in **device outbox** (documented). On reconnect, replay must use **dedupe_key** / identity (`mqtt-contract.md`); without **application ACK**, operators cannot distinguish “broker accepted” vs “OLTP committed.”

**Reconnect together:** Risk of **publish storm**; need **jitter 0–300s** (or equivalent) and **per-machine rate** caps on client; server-side **JetStream** retention and **worker concurrency** must stay within **DB pool** budget (`config.go` overrides).

**Server behavior:** mqtt-ingest enforces **critical identity** or rejects; duplicates suppressed in projection (`telemetry_idempotency_integration_test.go`).

**Bottlenecks:** Postgres write path, JetStream consumer lag, EMQX connection churn, **missing business APIs** causing manual ops load.

**Evidence before scale:** Run **staging** storm suite (`.github/workflows/telemetry-storm-staging.yml`) for target tier; attach **telemetry-storm-result** to **deploy-prod** scale gate; run `deployments/prod/shared/scripts/validate_production_scale_storm_*` as documented in prod runbooks.

---

## I. Release checklist (commands)

```bash
# Local / CI
go test ./...
make swagger
make swagger-check
make verify-enterprise-release

# Staging storm (GitHub Actions: "Staging telemetry storm suite")
# Inputs: scenario_mode = 100x100 | 500x200 | 1000x500 | all
# Artifact: telemetry-storm-result.json

# Production (GitHub Actions: "Deploy Production" — deploy-prod.yml only)
# workflow_dispatch: fleet_scale_target pilot | scale-100 | scale-500 | scale-1000
# Attach storm evidence or explicit bypass reason per workflow inputs
```

---

## J. Final recommendation

**Do not** claim **enterprise production release** for **1000 machines** with **full** feature parity until: **(1)** runtime sale catalog, **(2)** activation, **(3)** refund/cancel HTTP, **(4)** cash settlement (if cash-heavy market), **(5)** **telemetry application reconcile**, **(6)** tenant-bound machine middleware, and **(7)** **passing** storm evidence for the **target** tier are all **implemented, tested, and in Swagger**.

**Can deploy now?** **Pilot-only**, with written acceptance of gaps above and MQTT-first operations.

**Fleet scale:** Without storm evidence and missing P0 APIs, cap at **small pilot**; **do not** assert 500/1000 readiness.

---

*End of report.*
