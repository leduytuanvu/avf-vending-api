# P0 / P1 / P2 implementation roadmap and checklist

This roadmap translates the **enterprise transport model** into phased work items and **likely touch points** in this repository. It is a planning artifact for **Phase P0.0**; adjust priorities as product scope changes.

**Rules (recap):**

- Reuse **`internal/app/*`** and **`internal/modules/postgres`**; thin adapters only at transport edges.
- **Postgres** = SoR; **Redis/NATS/MQTT** via existing **`internal/platform/*`** patterns.
- Every **mutation** supports **auditability**; commerce, inventory, payment, command, activation, and machine-runtime mutations remain **idempotent**.
- **Machine runtime gRPC** → **Machine JWT**; **Admin REST** → **User JWT** / RBAC.
- **Payment webhook** stays **REST**, **HMAC**, idempotent.
- **No media binaries over gRPC**; **MQTT** remains command channel (no gRPC streaming replacement in this program phase).

For boundaries and flows, see [`enterprise-target-model.md`](enterprise-target-model.md) and [`transport-boundary.md`](transport-boundary.md).

---

## P0 — Transport correctness, credentials, and parity

**Theme:** Lock trust boundaries, avoid logic duplication, and align machine runtime with **Machine JWT** on new gRPC where introduced; keep **Admin** on **OpenAPI REST**.

| # | Deliverable | Likely packages / files |
| - | ----------- | ---------------------- |
| P0.1 | **Machine JWT** issuance, validation, and claims for **runtime gRPC** (distinct from admin user tokens) | `internal/platform/auth/*`, `internal/bootstrap/api.go`, `internal/grpcserver/*`, proto `proto/avf/v1/*` |
| P0.2 | **gRPC machine runtime** surface (activation, catalog read models, commerce steps, telemetry writes) delegating to existing app services | `internal/grpcserver/*`, `internal/app/*` (commerce, device, setupapp, catalog, telemetry), `cmd/api/main.go` |
| P0.3 | **HTTP machine/device** endpoints preserved; shared use cases with gRPC (no divergent rules) | `internal/httpserver/*` (e.g. `activation_http.go`, `commerce_http.go`, `device_http.go`, `machine_runtime_http.go`, `sale_catalog_http.go`) |
| P0.4 | **MQTT command ledger + ingest** idempotency review; ACK path unchanged | `internal/app/device`, `internal/platform/mqtt`, `cmd/mqtt-ingest`, `internal/modules/postgres` (commands/receipts queries) |
| P0.5 | **Webhook** HMAC + idempotency regression coverage | `internal/httpserver/commerce_webhook_*.go`, `internal/modules/postgres/commerce_webhook.go`, `internal/app/commerce` |
| P0.6 | **Audit hooks** on new mutations | `internal/app/audit`, `internal/app/api/audit_hooks.go`, enterprise audit SQL / gen |
| P0.7 | **OpenAPI** updates **only** for REST changes; **proto** + `make proto` for gRPC | `internal/httpserver/swagger_operations.go`, `tools/build_openapi.py`, `proto/` |
| P0.8 | Tests per behavior change | `*_test.go` adjacent to handlers and services; integration tests under `internal/modules/postgres` |

---

## P1 — Enterprise operations, media scale, outbox hardening

**Theme:** Strengthen async plane, reporting, media distribution, and fleet-scale concerns without breaking P0 boundaries.

| # | Deliverable | Likely packages / files |
| - | ----------- | ---------------------- |
| P1.1 | **NATS outbox** consumer strategy (in-repo or documented external) for worker-published subjects | `internal/app/reliability`, `internal/platform/nats`, `cmd/worker` |
| P1.2 | **Media** pipeline consistency: object keys, signed URLs, catalog admin | `internal/platform/objectstore`, `internal/app/catalogadmin`, `internal/app/artifacts` |
| P1.3 | **Telemetry** retention and JetStream resilience alignment | `internal/modules/postgres/telemetry_retention.go`, `docs/runbooks/telemetry-jetstream-resilience.md`, config |
| P1.4 | **Finance / daily close** and reporting exports if product requires | `internal/app/finance`, `internal/app/reporting`, `internal/httpserver/admin_*` |
| P1.5 | **Feature flags / rollouts** integration with machine runtime (HTTP + gRPC read same projection) | `internal/app/featureflags`, fleet admin HTTP, future gRPC read models |
| P1.6 | **OTA admin** and device campaign lifecycle (metadata over gRPC; blobs via HTTPS) | `internal/app/otaadmin`, `internal/httpserver/ota_admin_http.go`, storage |

---

## P2 — Split readiness and advanced enterprise

**Theme:** Prepare for service extraction **only** when justified; keep modular monolith default.

| # | Deliverable | Likely packages / files |
| - | ----------- | ---------------------- |
| P2.1 | **Internal gRPC** boundaries documented per bounded context; no public admin exposure | `docs/api/internal-grpc.md`, `internal/grpcserver` |
| P2.2 | **Temporal** workflow expansion where durable orchestration beats in-process | `cmd/temporal-worker`, `internal/platform/temporal`, `internal/app/workfloworch` |
| P2.3 | **ClickHouse** or analytics sink expansion beyond worker mirror | `internal/platform/clickhouse`, worker pipelines |
| P2.4 | **Multi-region / DR** documentation and deployment patterns | `deployments/prod/*`, `docs/runbooks/*` |

---

## Ambiguity handling

If product behavior is unclear (e.g. token exchange between technician and machine, exact Machine JWT claims, or PSP-specific webhook ordering), **do not guess**: add a **TODO** in code with explanation and link to an issue or doc section—per repository working agreement.

---

## Verification commands (before merge)

From repository root (also see root `README.md` and `Makefile`):

```powershell
go test ./...
go build ./cmd/api
go build ./cmd/cli
```

**OpenAPI / Swagger** (regenerate and commit when REST contract changes):

```powershell
make swagger
# CI-style check:
make swagger-check
```

**Proto / gRPC** (when `.proto` files change):

```powershell
make proto
```

Optional full static gates (no DB tests): `make ci-gates`. Integration tests: set `TEST_DATABASE_URL` and run `go test ./...`.
