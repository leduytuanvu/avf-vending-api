# Production E2E missing inputs (audit only)

**Scope:** Read-only audit of `tests/e2e/.env.production.destructive.local` against `https://api.ldtv.dev`. No destructive calls, no login, no MQTT publish, no DB access.

**Machine-readable:** [`production-e2e-input-audit.json`](production-e2e-input-audit.json)

**Checklist:** [`production-e2e-input-checklist.md`](production-e2e-input-checklist.md)

---

## Overall readiness

| Flow | Ready? | Reason |
|------|--------|--------|
| Production read-only (health/version) | **Yes** | `/health/live`, `/health/ready`, `/version` returned **200** at audit time. |
| Web Admin full destructive | **No** | No `ADMIN_TOKEN`; email/password path incomplete (`ADMIN_EMAIL`/`ADMIN_PASSWORD` empty) and **`E2E_ORGANIZATION_ID` absent** from local env. |
| Vending REST-equivalent | **No** | Depends on admin setup data; `MACHINE_ID` / `MACHINE_TOKEN` / `E2E_ACTIVATION_CODE` not ready. |
| gRPC machine | **No** | Machine auth inputs missing; **`GRPC_PROTO_ROOT`** empty; harness uses **grpcurl `-plaintext`** only — **TLS on `:443`** likely incompatible until addressed. |
| MQTT | **No** | **`MQTT_HOST` empty** in `.local` (example suggests `mqtt.ldtv.dev`); credentials empty. |
| Payment (mock / signed webhook) | **No** | `E2E_ALLOW_REAL_PAYMENT=false` (correct). Signed webhook tests need **`COMMERCE_PAYMENT_WEBHOOK_SECRET`** (or HMAC alias) in **process env** for scenario 42 — not set in local file. |
| Direct DB seed | **N/A** | Standard harness does not use `DATABASE_URL`; backend canonical var is **`DATABASE_URL`** (`internal/config`). |

---

## Core flags (destructive template)

From `tests/e2e/.env.production.destructive.local`:

| Variable | Expected | Status |
|----------|----------|--------|
| `BASE_URL` | `https://api.ldtv.dev` | OK |
| `E2E_TARGET` | `production` | OK |
| `E2E_ENABLE_FLOW_REVIEW` | `true` | OK |
| `E2E_ALLOW_WRITES` | `true` | OK |
| `E2E_ALLOW_DESTRUCTIVE` | `true` | OK |
| `E2E_ALLOW_DESTRUCTIVE_CLEANUP` | `true` | OK |
| `E2E_PRODUCTION_WRITE_CONFIRMATION` | `I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION` | OK |
| `E2E_PRODUCTION_DESTRUCTIVE_CONFIRMATION` | `I_UNDERSTAND_DB_WILL_BE_RESET_AFTER_TEST` | OK |

**Irreversible flags (must stay false unless explicitly enabled):**

| Variable | Value in file | Effect |
|----------|---------------|--------|
| `E2E_ALLOW_REAL_PAYMENT` | `false` | Live PSP flows **blocked** (correct for safety). |
| `E2E_ALLOW_REAL_DISPENSE` | `false` | Real hardware dispense **blocked**. |
| `E2E_ALLOW_REAL_MACHINE_COMMANDS` | `false` | Dangerous machine commands **blocked**. |
| `E2E_ALLOW_EXTERNAL_NOTIFICATIONS` | `false` | External email/SMS **blocked**. |

---

## Missing required values

| Variable | Required for | Current status | Where to get it | Cursor can derive? | Safe default / note |
|----------|--------------|----------------|-----------------|-------------------|---------------------|
| `ADMIN_TOKEN` | Web Admin REST, Phase 4 | **empty** | IdP / prior session / admin UI | No | N/A — secret |
| `ADMIN_EMAIL` | Optional login path | **empty** | Operator | No | — |
| `ADMIN_PASSWORD` | Optional login path | **empty** | Operator | No | — |
| `E2E_ORGANIZATION_ID` | `POST /v1/auth/login` body (email path); token path if org not in data | **absent** in `.local` | Org UUID from DB/admin | No | Must add to env or `test-data.json` |
| `MACHINE_ID` | Machine REST, gRPC meta, MQTT topics | **empty** | After admin setup or reuse | Yes **after** admin run | — |
| `MACHINE_TOKEN` | Authenticated machine REST/gRPC | **empty** | `secrets.private.json` reuse or activation | Yes **if** `E2E_ACTIVATION_CODE` set | — |
| `E2E_ACTIVATION_CODE` | Claim path (scenarios 02 / 20) | **absent** in `.local` | Operator-issued code | No | Optional if token reused |
| `MQTT_HOST` | MQTT scenarios | **empty** in `.local` | Ops / `docs/runbooks/production-2-vps.md` (**mqtt.ldtv.dev**) | Yes (hostname from doc) | Set host; creds still manual |
| `MQTT_USERNAME` / `MQTT_PASSWORD` | Broker auth | **empty** | EMQX/ops | No | — |
| `GRPC_PROTO_ROOT` | grpcurl import when `GRPC_USE_REFLECTION=false` | **empty** | Local checkout | Yes | Repo: `proto` (relative to repo root) |

**Login endpoint (no call performed):** `POST /v1/auth/login` under **`/v1/auth`** — see `internal/httpserver/server.go` + `internal/httpserver/auth_http.go`; harness: `tests/e2e/scenarios/01_web_admin_setup.sh`.

---

## Values Cursor / repo can derive

- **`GRPC_PROTO_ROOT`:** `proto` directory at repository root (contains `avf/machine/v1`).
- **Admin login path:** `POST /v1/auth/login` (JSON body per OpenAPI / setup scenario).
- **Production MQTT hostname:** `mqtt.ldtv.dev` per `docs/runbooks/production-2-vps.md` (still need credentials).
- **Machine token:** from **`E2E_ACTIVATION_CODE`** + claim flows once code exists.
- **`machineId` / `organizationId`:** from **`01_web_admin_setup.sh`** once admin auth works.

---

## Values operator must provide (names only)

- `ADMIN_TOKEN` **or** `ADMIN_EMAIL` + `ADMIN_PASSWORD` + `E2E_ORGANIZATION_ID`
- `E2E_ORGANIZATION_ID` (if using password login or token without org in reuse data)
- `MACHINE_TOKEN` **or** `E2E_ACTIVATION_CODE` (+ device fingerprint envs if applicable)
- MQTT **username/password** (and optional client TLS material)
- **`COMMERCE_PAYMENT_WEBHOOK_SECRET`** (or legacy HMAC alias) in shell env if running **signed** webhook E2E against prod
- Any **grpcurl TLS** flags / harness changes for `api.ldtv.dev:443` (today: **`-plaintext`** in `tests/e2e/lib/e2e_grpc.sh`)

---

## Dangerous command types (policy)

Until `E2E_ALLOW_REAL_MACHINE_COMMANDS` / `E2E_ALLOW_REAL_DISPENSE` are intentionally **true**, treat as **forbidden for real execution**: dispense, reboot, unlock, door/cashbox operations, firmware/update campaigns, and any MQTT `command_type` that maps to hardware motion. `MachineCommandService` uses generic `command_type` + `Struct` payload (`proto/avf/machine/v1/command.proto`).

---

## Exact next steps for operator

1. Merge updates from **`tests/e2e/.env.production.destructive.example`** into **`tests/e2e/.env.production.destructive.local`** (add `E2E_ORGANIZATION_ID`, `E2E_ACTIVATION_CODE`, `MQTT_HOST`, etc. — keep secrets local-only).
2. Fill **admin** credentials (`ADMIN_TOKEN` or email/password + org UUID).
3. Fill **machine** identity (`MACHINE_ID`, `MACHINE_TOKEN` or activation code).
4. Set **`GRPC_PROTO_ROOT`** to your checkout’s `proto` folder; resolve **TLS vs plaintext** for prod gRPC before relying on results.
5. Set **`MQTT_HOST`** (e.g. `mqtt.ldtv.dev`) and broker credentials.
6. Export webhook HMAC secret in the shell if testing **signed** Phase 8 payment webhooks.

---

## Rerun audit (static, no writes)

```bash
E2E_ENV_FILE=tests/e2e/.env.production.destructive.local \
  E2E_TARGET=production E2E_ALLOW_WRITES=false E2E_ENABLE_FLOW_REVIEW=true \
  ./tests/e2e/run-flow-review.sh --static-only
```

After secrets are filled, start with read-only smoke:

```bash
E2E_ENV_FILE=tests/e2e/.env.production.destructive.local \
  E2E_TARGET=production E2E_ALLOW_WRITES=false E2E_ENABLE_FLOW_REVIEW=true \
  ./tests/e2e/run-rest-local.sh --readonly
```

---

## Git safety

- **`tests/e2e/.env.production.destructive.local`** is **gitignored** — do not commit.
- **`.e2e-runs/`** is **gitignored**.

---

## Local env file note

The checked-in audit reflects **`tests/e2e/.env.production.destructive.local`** as it existed on disk; its first lines still match the **tracked template comment** — replace the header with a clear **local-only / never commit** comment when you next edit the file (optional hygiene).
