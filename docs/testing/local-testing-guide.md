# Local testing guide

Use this checklist to bring up dependencies, migrate, seed credentials, and smoke-test **Admin REST**, **machine gRPC**, **Swagger**, **payment webhooks**, and **MQTT commands**. Deeper operational detail stays in linked runbooks—this page is the onboarding map.

## Prerequisites

- Go **1.25+**, **Docker** (for Compose stack), **`DATABASE_URL`** matching Compose Postgres ([`.env.example`](../../.env.example)).
- Optional: **`grpcurl`**, **`curl`** / PowerShell **`Invoke-WebRequest`**, Python **3** for OpenAPI tooling (`make swagger-check`).

## 1. Docker Compose

From the repo root:

```powershell
docker compose -f deployments/docker/docker-compose.yml up -d
```

Or `make dev-up` — see **[`../runbooks/local-dev.md`](../runbooks/local-dev.md)** and **[`../../deployments/docker/README.md`](../../deployments/docker/README.md)** for profiles (`broker` for EMQX, optional observability).

Default Postgres: host `localhost`, port **5432**, database **`avf_vending`**, user/password commonly `postgres`/`postgres` (see Compose env).

## 2. Environment

Copy **[`.env.example`](../../.env.example)** or **[`.env.local.example`](../../.env.local.example)** to **`.env`**. Typical local `DATABASE_URL`:

`postgres://postgres:postgres@localhost:5432/avf_vending?sslmode=disable`

For integration tests, set **`TEST_DATABASE_URL`** to the same.

Enable machine gRPC for kiosk smoke tests:

- **`GRPC_ENABLED=true`** or **`MACHINE_GRPC_ENABLED=true`**
- **`GRPC_ADDR=127.0.0.1:9090`** (or `:9090`)

Production-style enforcement differs (`APP_ENV=production` requires **`MACHINE_GRPC_ENABLED=true`** explicitly — see **`internal/config/deployment_env.go`**).

## 3. Migrations

```powershell
make dev-migrate
# or: make migrate-up   (with DATABASE_URL set)
```

Uses Goose against **`DATABASE_URL`**; see **`scripts/verify_database_environment.sh`** Notes in **`../runbooks/local-dev.md`**.

## 4. Seed a local admin user

There is no universal demo seed in-repo for every environment—follow **`../runbooks/local-dev.md`** and organization bootstrap flows your deployment uses (often first admin via **`POST /v1/auth/register`** where enabled, or ops-supplied SQL—check **`machine-activation.md`** / **`technician-setup.md`** for fleet provisioning vs kiosk activation).

Once you have **User JWT** credentials, store **`access_token`** / **`refresh_token`** for Bearer calls to **`/v1/admin/*`**.

## 5. Run the API

```powershell
go run ./cmd/api
# or: make run-api
```

Validate configuration anytime:

```powershell
go run ./cmd/cli -validate-config
```

## 6. gRPC smoke test (machine runtime)

Full examples: **[`../local/grpc-local-test.md`](../local/grpc-local-test.md)** — **`grpcurl`** against **`GRPC_ADDR`**, **`avf.machine.v1.MachineActivationService/ClaimActivation`**, **`MachineBootstrapService/GetBootstrap`** with Machine JWT, catalog/inventory samples.

Minimal sanity:

```bash
grpcurl -plaintext localhost:9090 grpc.health.v1.Health/Check
grpcurl -plaintext localhost:9090 list
```

Contract reference: **[`../api/machine-grpc.md`](../api/machine-grpc.md)** (auth, idempotency, error codes).

## 7. Swagger / OpenAPI

With **`HTTP_SWAGGER_UI_ENABLED=true`** (typical non-production defaults):

- UI: **`http://localhost:8080/swagger/index.html`** (adjust **`HTTP_ADDR`**).
- Raw JSON: **`/swagger/doc.json`**.

Drift check (repo root):

```powershell
python tools/build_openapi.py
python tools/openapi_verify_release.py
```

CI gate: **`make swagger-check`** (part of **`make api-contract-check`**).

## 8. Payment webhook (local)

Contract and security: **[`../api/payment-webhook-security.md`](../api/payment-webhook-security.md)**. Debugging steps: **[`../runbooks/payment-webhook-debug.md`](../runbooks/payment-webhook-debug.md)**. Reconciliation operator flows: **[`../runbooks/payment-reconciliation.md`](../runbooks/payment-reconciliation.md)**.

Exercise PSP-signed callbacks against your **`APP_BASE_URL`** route(s); never disable HMAC verification in production.

## 9. MQTT command / ingest (local)

Contract: **[`../api/mqtt-contract.md`](../api/mqtt-contract.md)**. Bring up the **`broker`** Compose profile when you need a local MQTT endpoint.

Operational debugging: **`../runbooks/mqtt-command-debug.md`**, stuck commands: **`../runbooks/mqtt-command-stuck.md`**.

---

## See also

| Topic | Doc |
| ----- | --- |
| Redis toggles locally | [`../runbooks/local-dev.md`](../runbooks/local-dev.md) |
| Production gRPC TLS | [`../runbooks/grpc-production.md`](../runbooks/grpc-production.md) |
| Field smoke script | [`../runbooks/field-smoke-tests.md`](../runbooks/field-smoke-tests.md) |
| Load / stress harness | [`load-test.md`](load-test.md) |
