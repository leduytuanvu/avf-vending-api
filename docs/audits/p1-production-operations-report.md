# P1 production operations report

Concise record of **P1** hardening (post-P0): cash settlement clarity, catalog image validation, metrics scrape auth on the ops listener, payment webhook provider binding + timestamp errors, storm evidence schema checks, and doc alignment.

## Operational endpoints

- **Cash settlement:** Responses now expose **`expectedCloudCashMinor`** (GET cashbox) and, on collections, **`countedPhysicalCashMinor`**, **`expectedCloudCashMinor`**, **`varianceMinor`**, **`reviewState`** alongside legacy snake_case fields. Disclosures state ledger vs physical count (no hardware command).
- **Catalog:** `PUT /v1/admin/products/{productId}/image` validates **HTTPS** artifact URLs, **64-hex** `contentHash` (optional `sha256:` prefix), and **image/*** MIME allowlist before bind.
- **Metrics:** When **`METRICS_SCRAPE_TOKEN`** is set, **`GET /metrics`** on **`HTTP_OPS_ADDR`** requires **`Authorization: Bearer`**, same as public `/metrics` when exposed. **`/health/live`** and **`/health/ready`** on the ops mux stay open for probes.
- **Payment webhook:** Stale/invalid timestamp → **400** `webhook_timestamp_skew`; bad HMAC → **401**. Body **`provider`** must match the payment row → **403** `webhook_provider_mismatch` (integration test: `commerce_webhook_provider_mismatch_integration_test.go`).

## Storm / evidence

- **`telemetry_storm_load_test.sh`:** Result payload keeps **`critical_expected`**; **`critical_accepted`** is emitted as an integer when Prometheus delta rounds cleanly.
- **`validate_production_scale_storm_evidence.py`:** Scale evidence must include **`critical_expected`** and **`critical_accepted`** keys (in addition to existing strict gates: `critical_lost`, `duplicate_critical_effects`, `db_pool_result`, `health_result`, `restart_result`, `completed_at_utc`, etc.).

## Swagger / OpenAPI

- Regenerated via **`python tools/build_openapi.py`**: cash examples (exact match, variance, pending review), cash close **409** example, webhook **400/403** examples, product image **`contentHash`** example fixed to valid SHA-256 hex.

## Tests added/updated

- `internal/httpserver/server_newhttpserver_test.go` — ops `/metrics` bearer when token set; health on ops unaffected.
- `internal/httpserver/catalog_image_validation_test.go` — URL/hash/MIME validation.
- `internal/httpserver/admin_cash_map_test.go` — cash collection DTO mapping / `reviewState`.
- `internal/modules/postgres/commerce_webhook_provider_mismatch_integration_test.go` — provider mismatch rejected.

## Commands (maintainer machine)

| Command | Notes |
| --- | --- |
| `go test ./...` | Full suite; integration tests need Postgres when not `-short`. |
| `make swagger` / `python tools/build_openapi.py` | Regenerate `docs/swagger/*`. |
| `make swagger-check` | Fails if swagger artifacts drift vs repo. |
| `make verify-enterprise-release` | Bash/docker/python per runbook; CI-shaped gate. |

**Runs on this change (Windows dev host):** `go test ./internal/httpserver/... ./internal/observability/... ./internal/modules/postgres/...`; `python tools/build_openapi.py`; `python tools/openapi_verify_release.py` — all **PASS**. `make` targets were not executed here (no `make` in PATH).
