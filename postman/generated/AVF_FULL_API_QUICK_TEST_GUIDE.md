# AVF Full API Quick Test Guide

## 1. Files generated

- `postman/generated/API_INVENTORY_CANONICAL.json`
- `postman/generated/rest/AVF_REST_FULL.postman_collection.json`
- `postman/generated/rest/AVF_REST_* .postman_environment.json`
- `postman/generated/grpc/*`
- `postman/generated/mqtt/*`

## 2. Import order into Postman

1. Import REST environment (LOCAL or CANARY)
2. Import `AVF_REST_FULL.postman_collection.json`
3. gRPC/MQTT: follow README in `grpc/` and `mqtt/` (manual proto/MQTT setup)

## 3. Environment setup

Set `adminEmail`, `adminPassword` locally (never commit). Run health checks first.

## 4. Required variables

See environment files: `baseUrl`, `accessToken`, `machineId`, `siteId`, `mqttTopicPrefix`, etc.

## 5. REST all-in-one collection

Run folder `00_Health_System` then `01_Auth` login, then domain folders in order.

## 6. Token capture

Login/refresh tests set `accessToken` and `refreshToken`. Gated writes capture IDs when gates enabled.

## 7–11. Business flow order

Health → Auth → Admin/RBAC → Catalog → Media → Products → Sites → Machine provisioning → Config → Telemetry → Inventory → Planogram → Orders → Payments → Refunds → Promotions → Finance → Incidents → OTA → Audit → Webhooks

## 12. Companion scripts

- `grpc/AVF_GRPCURL_SMOKE.sh list|dry-run|run`
- `mqtt/AVF_MQTT_SMOKE.sh list|dry-run|run`

## 13. Troubleshooting

- **401/403:** refresh token or check RBAC role
- **GATED writes:** set `canaryMode=true` or `readiness=true`
- **gRPC TLS:** set `GRPC_TLS=true` when listener uses TLS
- **MQTT auth:** verify broker ACL + topic prefix

## 14. Coverage summary

- REST: 327
- gRPC: 86
- MQTT: 28

## Live smoke (Phase 9)

Not run (optional Phase 9).
