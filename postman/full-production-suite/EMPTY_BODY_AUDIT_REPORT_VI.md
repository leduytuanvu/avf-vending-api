# EMPTY_BODY_AUDIT_REPORT_VI

## Thời điểm audit

- **Timestamp (UTC):** 2026-05-19T05:34:28.816317+00:00
- **Nhánh git:** feature/product-media-offline-cache
- **Commit:** 9e859d5ea156379669c6e93b5559685a15e70fbb

## Đếm OpenAPI / Postman

| Chỉ số | Giá trị |
|---------|--------|
| Tổng operations OpenAPI | 327 |
| Operations có `requestBody` | 110 |
| Operations có JSON `requestBody` (`application/json`) | 110 |
| Request Postman có raw JSON body không rỗng | 110 |
| JSON `requestBody` còn thiếu/sai body trong Postman | **0** |

## `operationId` chịu ảnh hưởng `schema.type=string` + `example` (swagger)

- (không có)

## GET / không có `requestBody` — body trống là OK

- Các thao tác GET/HEAD/OPTIONS không khai báo `requestBody` giữ body trống.

## POST/PUT/PATCH không có `requestBody` trong swagger (đúng spec — OK)

- DocOpV1AdminActivationCodeCatalogRevoke — POST /v1/admin/activation-codes/{codeId}/revoke
- DocOpV1AdminArtifactsPutContent — PUT /v1/admin/artifacts/{artifactId}/content
- DocOpV1AdminArtifactsReserve — POST /v1/admin/artifacts
- DocOpV1AdminAuthUsersActivate — POST /v1/admin/auth/users/{accountId}/activate
- DocOpV1AdminAuthUsersDeactivate — POST /v1/admin/auth/users/{accountId}/deactivate
- DocOpV1AdminAuthUsersRevokeSessions — POST /v1/admin/auth/users/{accountId}/revoke-sessions
- DocOpV1AdminCommandCancel — POST /v1/admin/commands/{commandId}/cancel
- DocOpV1AdminCommandRetry — POST /v1/admin/commands/{commandId}/retry
- DocOpV1AdminFeatureFlagDisablePost — POST /v1/admin/feature-flags/{flagId}/disable
- DocOpV1AdminFeatureFlagEnablePost — POST /v1/admin/feature-flags/{flagId}/enable
- DocOpV1AdminInventoryAnomalyResolve — POST /v1/admin/inventory/anomalies/{anomalyId}/resolve
- DocOpV1AdminMachineArchive — POST /v1/admin/machines/{machineId}/archive
- DocOpV1AdminMachineDiagnosticRequest — POST /v1/admin/machines/{machineId}/diagnostics/requests
- DocOpV1AdminMachineDisable — POST /v1/admin/machines/{machineId}/disable
- DocOpV1AdminMachineEnable — POST /v1/admin/machines/{machineId}/enable
- DocOpV1AdminMachineInventoryReconcile — POST /v1/admin/machines/{machineId}/inventory/reconcile
- DocOpV1AdminMachineMarkCompromised — POST /v1/admin/machines/{machineId}/mark-compromised
- DocOpV1AdminMachineResume — POST /v1/admin/machines/{machineId}/resume
- DocOpV1AdminMachineRetire — POST /v1/admin/machines/{machineId}/retire
- DocOpV1AdminMachineRevokeCredentials — POST /v1/admin/machines/{machineId}/revoke-credentials
- DocOpV1AdminMachineRevokeSessions — POST /v1/admin/machines/{machineId}/revoke-sessions
- DocOpV1AdminMachineRevokeToken — POST /v1/admin/machines/{machineId}/revoke-token
- DocOpV1AdminMachineRotateCredential — POST /v1/admin/machines/{machineId}/rotate-credential
- DocOpV1AdminMachineRotateCredentials — POST /v1/admin/machines/{machineId}/rotate-credentials
- DocOpV1AdminMachineRotateTokenVersion — POST /v1/admin/machines/{machineId}/rotate-token-version
- DocOpV1AdminMachineSuspend — POST /v1/admin/machines/{machineId}/suspend
- DocOpV1AdminOTACampaignApprove — POST /v1/admin/ota/campaigns/{campaignId}/approve
- DocOpV1AdminOTACampaignCancel — POST /v1/admin/ota/campaigns/{campaignId}/cancel
- DocOpV1AdminOTACampaignPause — POST /v1/admin/ota/campaigns/{campaignId}/pause
- DocOpV1AdminOTACampaignPublish — POST /v1/admin/ota/campaigns/{campaignId}/publish
- DocOpV1AdminOTACampaignResume — POST /v1/admin/ota/campaigns/{campaignId}/resume
- DocOpV1AdminOTACampaignStart — POST /v1/admin/ota/campaigns/{campaignId}/start
- DocOpV1AdminOperationalAnomalyIgnore — POST /v1/admin/anomalies/{anomalyId}/ignore
- DocOpV1AdminOperationalAnomalyResolve — POST /v1/admin/anomalies/{anomalyId}/resolve
- DocOpV1AdminOutboxRetry — POST /v1/admin/ops/outbox/{outboxId}/retry
- DocOpV1AdminPriceBookActivate — POST /v1/admin/price-books/{priceBookId}/activate
- DocOpV1AdminPriceBookArchive — POST /v1/admin/price-books/{priceBookId}/archive
- DocOpV1AdminPriceBookDeactivate — POST /v1/admin/price-books/{priceBookId}/deactivate
- DocOpV1AdminPromotionActivate — POST /v1/admin/promotions/{promotionId}/activate
- DocOpV1AdminPromotionArchive — POST /v1/admin/promotions/{promotionId}/archive
- DocOpV1AdminPromotionDeactivate — POST /v1/admin/promotions/{promotionId}/deactivate
- DocOpV1AdminPromotionPause — POST /v1/admin/promotions/{promotionId}/pause
- DocOpV1AdminRolloutCancel — POST /v1/admin/rollouts/{rolloutId}/cancel
- DocOpV1AdminRolloutPause — POST /v1/admin/rollouts/{rolloutId}/pause
- DocOpV1AdminRolloutResume — POST /v1/admin/rollouts/{rolloutId}/resume
- DocOpV1AdminRolloutRollback — POST /v1/admin/rollouts/{rolloutId}/rollback
- DocOpV1AdminRolloutStart — POST /v1/admin/rollouts/{rolloutId}/start
- DocOpV1AdminSiteArchive — POST /v1/admin/sites/{siteId}/archive
- DocOpV1AdminSiteDisable — POST /v1/admin/sites/{siteId}/disable
- DocOpV1AdminSystemOutboxReplayPost — POST /v1/admin/system/outbox/{eventId}/replay
- DocOpV1AdminSystemRetentionDryRunPost — POST /v1/admin/system/retention/dry-run
- DocOpV1AdminSystemRetentionRunPost — POST /v1/admin/system/retention/run
- DocOpV1AdminTechnicianAssignmentCancel — POST /v1/admin/technician-assignments/{assignmentId}/cancel
- DocOpV1AdminTechnicianDisable — POST /v1/admin/technicians/{technicianId}/disable
- DocOpV1AdminTechnicianEnable — POST /v1/admin/technicians/{technicianId}/enable
- DocOpV1AdminUsersDisable — POST /v1/admin/users/{userId}/disable
- DocOpV1AdminUsersEnable — POST /v1/admin/users/{userId}/enable
- DocOpV1AdminUsersRevokeSessions — POST /v1/admin/users/{userId}/revoke-sessions
- DocOpV1AuthMFAEnroll — POST /v1/auth/mfa/totp/enroll
- DocOpV1OperatorSessionHeartbeat — POST /v1/machines/{machineId}/operator-sessions/{sessionId}/heartbeat

## Auth kiểm tra mục tiêu

| Kiểm tra | Kết quả |
|-----------|---------|
| `DocOpV1AuthLogin` có `{{adminEmail}}`, `{{adminPassword}}` trong raw body | **PASS** |
| `DocOpV1AuthMe` không có body JSON không rỗng | **PASS** |
| `DocOpV1AuthMe` có `Authorization: Bearer {{accessToken}}` | **PASS** |

## Lệnh validation đã chạy

```text
python postman/full-production-suite/generate_full_postman_suite.py
python postman/full-production-suite/validate_generated_assets.py
python -m json.tool postman/full-production-suite/AVF_REST_365_FULL.postman_collection.json
python -m json.tool postman/full-production-suite/AVF_PRODUCTION.postman_environment.json
python -m json.tool postman/full-production-suite/manifest.json
python -m json.tool postman/full-production-suite/grpc/grpc_request_templates.json
python -m json.tool postman/full-production-suite/mqtt/mqtt_request_templates.json
```

## Kết quả validation (snapshot)

```text
VALIDATION_PASS
openapi_operations: 327
postman_requests: 327
grpc_templates: 86
mqtt_templates: 28
manifest_finalStatus: PASS_IMPORT_ASSETS_COMPLETE
openapi_idempotency_ops: 91
```

## Quét secret (validator)

- **Kết quả:** PASS

## Danh sách `operationId` JSON body vẫn rỗng/sai (phải rỗng sau sửa)

- *(không có)*

## Khẳng định cuối (theo audit này)

**PASS_AFTER_FIXES**

> Nội dung: chỉ phản ánh **đầy đủ body import Postman + validator**; **không** tuyên bố PASS runtime production.
