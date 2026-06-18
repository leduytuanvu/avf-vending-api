# Market readiness — final report

## Executive summary

Backend **code and automated unit/OpenAPI/Postman gates pass** on this branch after fixing stale REST count (327→329). **Not fully market-ready for unattended real sales** without: merged perf/observability branch, operator production smoke with admin token, local/CI E2E with database, and controlled machine canary.

**Recommendation: READY WITH RISKS**

## Branch / commit

- Branch: `qa/market-readiness-full-flow-validation`
- Commit: (see `git log -1` after push)

## Inventory counts

| Surface | Count |
|---------|-------|
| REST (OpenAPI) | 329 |
| gRPC RPCs (proto scan) | 80 |
| MQTT channel defs | 16 |
| Worker processes | 7 |

Details: `docs/audits/api-grpc-mqtt-full-inventory.md`, `build/reports/api-grpc-mqtt-full-inventory.json`

## Canonical vs legacy

- REST legacy ~15% of routes (admin users alias, media flat paths, machine HTTP when enabled)
- gRPC: `avf.machine.v1` canonical; `avf.v1` protos not registered
- MQTT: enterprise layout preferred; legacy layout still subscribed

## Duplicate cleanup

See `docs/audits/duplicate-surface-cleanup-plan.md` — **no routes/protos/topics removed**.

## Performance

See `docs/audits/production-performance-audit.md`. Production health 200; admin latency not measured (no token).

## Tests

See `docs/audits/market-readiness-test-results.md`.

## CI / deploy

| Item | Status |
|------|--------|
| PR to develop | Pending push |
| CI | Pending |
| Production deploy | Not executed |
| Production smoke script | Added; full run needs bash + optional token |

## Remaining risks

1. Live VPS env may not match updated pool/Redis examples until deploy.
2. Full admin→planogram→publish not proven in one automated run without DB E2E.
3. Physical machine install/vend not verified on production in this session.
4. `perf/production-latency-observability-and-surface-cleanup` (PR #316) not merged into develop baseline used here.
5. Postman full suite historical docs still mention 327 in places — generator now uses 329.

## Intentionally not removed

- All legacy REST aliases
- `MachineAuthService` facade
- `MachineSaleService`
- Legacy MQTT topic subscriptions
- `proto/avf/v1` stubs

## Go-live recommendation

| Level | When |
|-------|------|
| **READY** | After CI green, merge, deploy, `smoke-market-readiness.sh` with token PASS, one test-machine catalog sync PASS |
| **READY WITH RISKS** | **Now** for code review + staging; production sales need post-deploy smoke |
| **NOT READY** | If post-deploy health/smoke fails or catalog sync fails on test machine |
