# Production latency and surface cleanup — final report

**Branch:** `perf/production-latency-observability-and-surface-cleanup`  
**Date:** 2026-05-23

## Summary

Non-breaking production performance and API-surface hygiene:

- Latency measurement scripts and operations runbooks
- Production-oriented Postgres pool defaults (env examples + code fallbacks when unset)
- HTTP in-flight and DB pool acquire metrics
- REST deprecation headers on documented legacy aliases
- Canonical API/gRPC/MQTT documentation
- Postman enterprise regen with **Legacy and Compatibility** folder routing
- App-node API CPU limit 0.75 → 1.50

**No REST routes, gRPC methods, or MQTT topics were removed.**

## Files changed (high level)

| Area | Paths |
|------|--------|
| Audit / ops docs | `docs/audits/production-latency-and-surface-audit.md`, `docs/operations/production-latency-runbook.md`, `production-resource-sizing.md`, `production-performance-deploy-checklist.md` |
| API surface docs | `docs/api/canonical-api-surface.md`, `grpc-canonical-surface.md`, `mqtt-contract.md` |
| Scripts | `scripts/production/measure-http-latency.sh`, `measure-production-stack.sh` |
| Config / DB | `internal/config/config.go`, `postgres_pool_defaults.go`, `internal/platform/db/postgres.go` |
| HTTP | `internal/httpserver/api_surface_deprecation.go`, `request_metrics.go`, `server.go` |
| Metrics | `internal/observability/dbpoolmetrics/dbpoolmetrics.go` |
| MQTT tests | `internal/platform/mqtt/router_test.go` |
| Deploy examples | `deployments/prod/.env.production.example`, `app-node/.env.app-node.example`, `docker-compose.app-node.yml` |
| Postman | `postman/production-enterprise/generate_enterprise_postman_project.py`, regenerated collection/env/zip |
| Gitignore | `.production-latency-runs/` |

## Tests run

| Check | Result |
|-------|--------|
| `go test ./...` | **PASS** |
| `python tools/openapi_verify_release.py` | **PASS** |
| `python postman/production-enterprise/generate_enterprise_postman_project.py` | **PASS** (264 REST items) |
| `python postman/suites/full-production-suite/validate_generated_assets.py` | **FAIL (pre-existing)** — OpenAPI/Postman REST count **329 vs expected 327** on `develop`; not introduced by this branch (see `manifest.json` / `06_COMPLETENESS_AUDIT.csv`) |
| `docker compose config` (app-node) | Requires `APP_IMAGE_REF` + mqtt client env (validated structurally after setting dummy refs) |

## Production pre-deploy baseline (current prod, not this branch)

| Endpoint | http_code | total (s) |
|----------|-----------|-----------|
| `/health/live` | 200 | ~0.36 |
| `/health/ready` | 200 | ~0.31 |

Measured from operator workstation via `curl.exe`; post-deploy comparison required after merge.

## Deploy / CI status

| Step | Status |
|------|--------|
| Branch | `perf/production-latency-observability-and-surface-cleanup` |
| PR | https://github.com/leduytuanvu/avf-vending-api/pull/316 |
| CI (2026-05-27) | **All required checks PASS** (Go CI Gates run `26500126148`, Linux race `26500126158`) |
| Merge → `develop` | **Blocked by branch policy** (review/approval required); `gh pr merge` returned not mergeable |
| Production deploy | **Not executed** — merge + release workflow + operator approval required |
| Post-deploy latency | Pending after deploy |

**Rollback:** use existing production rollback workflow / last-known-good `APP_IMAGE_REF` digest.

## Canonical vs legacy (unchanged mounts)

See `docs/api/canonical-api-surface.md` and `docs/api/grpc-canonical-surface.md`. Legacy REST aliases return `Deprecation: true` + `Link` successor template when matched.

## Remaining risks / uncertainties

1. **329 vs 327 REST** full-suite validator drift on `develop` — resolve in a dedicated OpenAPI/Postman parity PR (do not delete the two extra operations without inventory).
2. **Live VPS `.env`** may still use old pool sizes until redeployed — examples updated; runtime overrides explicit env wins.
3. **Supabase pooler limit** vs summed process pools — operator must validate before raising caps on multi-node fleets.
4. **`DATABASE_HEALTH_CHECK_PERIOD`** not implemented (pgx internal health only).
5. **Production deploy not verified** for this commit — health above reflects **current** production, not post-change image.

## Confirmation

- Secrets: none committed  
- `deployments/prod/.env.production`: not modified  
- gRPC proto compatibility: unchanged  
- MQTT enterprise + legacy subscriptions: unchanged  
- Postman enterprise coverage: regenerated, not reduced  
