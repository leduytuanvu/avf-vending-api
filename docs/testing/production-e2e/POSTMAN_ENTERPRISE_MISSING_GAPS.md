# Postman enterprise missing gaps

Generated: 2026-05-26T20:08:42Z

## 1. REST missing from Enterprise Postman

- None (all runnable production routes represented or explicitly skipped).

## 2. REST present in Postman but not in OpenAPI

- `POST /v1/admin/machines/{machineId}/operator-sessions/start`

## 7. gRPC method missing from docs/catalog

- None.

## 10. MQTT topic missing from docs/catalog

- None.

## 13. Skipped/excluded without reason

- All skips have explicit reasons in matrix/overrides.

## 14–15. Production validation

- Newman: **PENDING_OPERATOR_CREDENTIALS** (no local `*LOCAL*.postman_environment.json` in repo)
- Import parity: **PENDING** until Newman run
