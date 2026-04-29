# Production smoke tests (enterprise)

**Scope (P0 CI / operator):** Describes **automated** **`smoke_prod.sh`** tiers attached to deploy workflows. These checks are **GET-only** and **zero side-effect by design** — they **do not** replace **[`../testing/field-test-cases.md`](../testing/field-test-cases.md)** (hardware/PSP/command/mutating rows) or **[`../runbooks/field-smoke-tests.md`](../runbooks/field-smoke-tests.md)** local/pilot mutating scripts.

**Transport contract:** Kiosk **gRPC + MQTT** vs Admin **REST** is defined in **[`../architecture/production-final-contract.md`](../architecture/production-final-contract.md)** — smoke scripts only touch the **public HTTP** edge, not machine gRPC.

**P2 note:** Claiming **fleet-scale** readiness still requires **[`../runbooks/production-release-readiness.md`](../runbooks/production-release-readiness.md)** storm thresholds + field matrix — not this file alone.

Production deploys use [`deployments/prod/scripts/smoke_prod.sh`](../../deployments/prod/scripts/smoke_prod.sh) in **tiers** so safety is not based on `/health/ready` alone. JSON under `--json` is assembled by [`scripts/smoke/emit_production_smoke_json.py`](../../scripts/smoke/emit_production_smoke_json.py) for CI evidence.

## Smoke levels (`SMOKE_LEVEL`)

| Level | What runs |
| --- | --- |
| `health` | Public base URL, `/health/ready`, `/health/live`, `/version` (build metadata) |
| `business-readonly` | `health` + at least one **read-only** business signal (default for production) |
| `business-safe-synthetic` or `full` | Above + **optional** synthetic GET (only when `SMOKE_ENABLE_BUSINESS_SYNTHETIC=1` and a vetted path is set) |

**GitHub `deploy-prod.yml`:** per-node smokes use `SMOKE_LEVEL=business-readonly`. The final public run uses `business-readonly` or `business-safe-synthetic` when the workflow input **enable_business_synthetic_smoke** is true.

## Side-effect policy

- The script only uses **GET** (never POST/PUT/PATCH/DELETE) against the public base URL.
- It **must not** call routes that perform real **payment capture**, **dispense**, **inventory mutation**, or **MQTT / device commands**.
- **`zero_side_effects_claim`** in JSON is an operator-facing signal: the script can truthfully claim no writes from its own operations; a misconfigured **GET** that triggers work on the server is still a configuration error—**vet every path** before enabling it in production.
- The **synthetic** tier is **skipped with a reason** (not a pass) when no safe path is configured—this repository does not ship a public unauthenticated “dry run order” endpoint by default.

## Business-readonly tier

**Goal:** prove catalog/API machinery beyond ops probes (e.g. OpenAPI, authenticated list endpoints).

1. **Explicit (recommended for locked-down OpenAPI):** set **`SMOKE_BUSINESS_READONLY_SPECS`**:  
   `name|path|http_status_list|body_regex` entries separated by `;;`.  
   Example (paths illustrative):  
   `product-list|/v1/admin/products?limit=1|200|items;;fleet|/v1/reports/fleet-health|200|machines`  
   Use with **`SMOKE_BUSINESS_READONLY_BEARER_TOKEN`** for Bearer GETs. Regex must not contain the delimiter `|`.

2. **Default discovery:** if `SMOKE_BUSINESS_READONLY_SPECS` is **unset**, the script tries **`GET /swagger/doc.json`**. **HTTP 404** is treated as *skip* (OpenAPI not mounted), not a pass—you must then provide **`SMOKE_DB_READ_PATH`** and match regex, and/or **SPECS**, and/or enable OpenAPI JSON on the edge per [`internal/httpserver/swagger.go`](../../internal/httpserver/swagger.go).

3. **Legacy:** `SMOKE_DB_READ_PATH` + `SMOKE_DB_READ_MATCH_REGEX` for an optional **DB-backed read** (still GET-only), as before.

**Failure** if no successful readonly probe is recorded when the tier runs.

## Business-safe-synthetic tier

- Enable with **`SMOKE_ENABLE_BUSINESS_SYNTHETIC=1`** and raise **`SMOKE_LEVEL`** to `business-safe-synthetic` (or `full`).
- Set **`SMOKE_SYNTHETIC_GET_PATH`** to a **single** vetted path (e.g. future operator dry-run or sandbox route). Optional **`SMOKE_SYNTHETIC_BEARER_TOKEN`** and **`SMOKE_SYNTHETIC_MATCH_REGEX`**.
- If synthetic is **eligible** and enabled but the path is **empty**, the tier is **skipped** with a clear reason; the deploy does **not** treat that as success of synthetic checks.

## JSON evidence (`--json` / `SMOKE_JSON=1`)

Includes (among others): `level`, `started_at_utc`, `completed_at_utc`, `health_result`, `business_readonly_result`, `business_synthetic_result`, `skipped_reasons`, `checks` (per-endpoint), `final_result` / `overall_status`, `zero_side_effects_claim`, plus legacy fields such as `critical_read_result` and `optional_db_read_result`.

## How to add a new **safe** smoke endpoint

1. Prefer **GET** that is strictly read-only and idempotent, authenticated if needed.
2. Add it to **`SMOKE_BUSINESS_READONLY_SPECS`** (or a dedicated var in org settings) with a **tight** body regex.
3. Run `bash deployments/prod/scripts/smoke_prod.sh --level business-readonly` against staging or a canary with the same env.
4. **Never** add POST/PUT/DELETE, webhook replays, machine commands, or “force dispense” style routes to smoke.

## What must **never** be done in production smoke

- Real payment **capture** or refund execution “to verify PSP”.
- Machine **dispense** or slot tests against production hardware.
- **Inventory** adjustments, stock **mutations**, or cash collection **close** operations.
- **MQTT publish** to production device topics from the workflow runner.
- Disabling auth “just to get a 200” on a sensitive write route.

See also: [two-vps-rolling-production-deploy.md](two-vps-rolling-production-deploy.md) for when per-node vs final public smoke runs.
