package httpserver

// Route registration for the public API is implemented in server.go (NewHTTPServer → mountV1).
//
// JSON error envelope for /v1 JSON handlers and auth middleware:
// {"error":{"code":"...","message":"...","details":{...},"requestId":"..."}}.
// Optional features that are not wired return *api.CapabilityError → HTTP 501 with
// code "not_implemented" and details { "capability": "...", "implemented": false }.
// Optional runtime wiring that is missing in this process (MQTT publisher, commerce store, etc.) uses
// writeCapabilityNotConfigured → HTTP 503 with code "capability_not_configured" and the same details keys.
// Prefer branching on error.code; message is diagnostic text and may change.
//
// Request tracing: middleware.RequestID runs on the root router. Clients may send X-Request-ID and
// X-Correlation-ID; both are echoed on the response. When X-Correlation-ID is omitted, it defaults
// to the request ID. Operator session login/logout persists correlation where the domain supports it.
// Bearer auth rejections log request_id and correlation_id (see internal/platform/auth/middleware.go).
// Optional token-bucket limits on mutating routes: HTTP_RATE_LIMIT_SENSITIVE_WRITES_* (middleware_rate_limit.go),
// including public POST /v1/auth/login|refresh, /v1/setup/activation-codes/claim, and payment webhooks when enabled.
// Fixed-window abuse limits (RATE_LIMIT_*): auth_login/auth_refresh/activation_claim/webhook buckets in abuse_protection.go.
//
// Backend artifacts (Bearer JWT + org_admin or platform_admin; S3 when API_ARTIFACTS_ENABLED=true):
// POST/GET/PUT/DELETE under /v1/admin/organizations/{orgId}/artifacts — see artifacts_http.go.
//
// Commerce: provider webhooks are mounted without Bearer JWT (HMAC-only); other commerce routes use
// Bearer JWT + RequireOrganizationScope. Paths:
//
//	/v1/commerce/cash-checkout
//	/v1/commerce/orders
//	/v1/commerce/orders/{orderId}/payment-session
//	/v1/commerce/orders/{orderId}
//	/v1/commerce/orders/{orderId}/reconciliation
//	/v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks
//	/v1/commerce/orders/{orderId}/vend/start
//	/v1/commerce/orders/{orderId}/vend/success
//	/v1/commerce/orders/{orderId}/vend/failure
//
// Machine setup bootstrap (Bearer JWT + RequireMachineURLAccess("machineId")):
//
//	GET /v1/setup/machines/{machineId}/bootstrap
//
// Machine runtime writes (Bearer JWT + RequireMachineURLAccess("machineId") + sensitive-write rate limit when enabled):
//
//	POST /v1/machines/{machineId}/check-ins
//	POST /v1/machines/{machineId}/config-applies
//
// Admin machine setup writes (Bearer JWT + platform_admin or org_admin + sensitive-write rate limit when enabled):
//
//	PUT  /v1/admin/machines/{machineId}/topology
//	PUT  /v1/admin/machines/{machineId}/planograms/draft
//	POST /v1/admin/machines/{machineId}/planograms/publish  (requires Idempotency-Key header)
//	POST /v1/admin/machines/{machineId}/sync               (requires Idempotency-Key header)
//
// Admin machine directory (Bearer JWT + platform_admin or org_admin):
//
//	GET /v1/admin/machines
//	GET /v1/admin/machines/{machineId}   (optional organization_id query for platform_admin)
//
// Admin inventory reads and stock writes (Bearer JWT + platform_admin or org_admin; stock-adjustments uses writeRL + Idempotency-Key):
//
//	GET  /v1/admin/inventory/low-stock (velocity_days, site_id, machine_id, product_id, urgency, days_threshold; organization_id for platform_admin)
//	GET  /v1/admin/inventory/refill-suggestions (same filters; all slotted products)
//	GET  /v1/admin/machines/{machineId}/slots (cabinetCode, cabinetIndex, slotCode; legacy-only machines default cabinet CAB-A)
//	POST /v1/admin/machines/{machineId}/stock-adjustments
//	GET  /v1/admin/machines/{machineId}/inventory (optional cabinetCode/cabinetIndex when unambiguous)
//	GET  /v1/admin/machines/{machineId}/inventory-events (append-only ledger; optional from/to, limit, offset)
//	GET  /v1/admin/machines/{machineId}/refill-suggestions (per-machine refill forecast)
//
// Feature flags + staged machine config rollouts (fleet.read reads; fleet.write mutates; organization_id for platform_admin):
//
//	GET/PATCH/POST /v1/admin/feature-flags ... ; PUT /v1/admin/feature-flags/{flagId}/targets
//	GET/POST /v1/admin/machine-config/rollouts ; GET /v1/admin/machine-config/rollouts/{rolloutId}
//
// Telemetry reads (Bearer JWT + RequireMachineURLAccess("machineId")):
//
//	GET /v1/machines/{machineId}/telemetry/snapshot
//	GET /v1/machines/{machineId}/telemetry/incidents
//	GET /v1/machines/{machineId}/telemetry/rollups
//
// Remote MQTT command dispatch (Bearer JWT + RequireMachineURLAccess("machineId") + org/platform admin):
//
//	POST /v1/machines/{machineId}/commands/dispatch
//	GET  /v1/machines/{machineId}/commands/{sequence}/status
//	GET  /v1/machines/{machineId}/commands/receipts
//
// Device commerce bridge (Bearer JWT + RequireMachineURLAccess("machineId") + org/platform admin; same write rate limit group as command dispatch):
//
//	POST /v1/device/machines/{machineId}/vend-results   (requires Idempotency-Key)
//	POST /v1/device/machines/{machineId}/commands/poll  (HTTP fallback when MQTT is degraded)
//
// Machine operator session endpoints (Bearer JWT + RequireMachineURLAccess("machineId")) are
// mounted under:
//
//	/v1/machines/{machineId}/operator-sessions
//
// List endpoints under that prefix accept optional query `limit` (positive int, default 50, max 500)
// and return {"items":[...],"meta":{"limit":N,"returned":M}}.
//
// See mountOperatorSessionRoutes in operator_http.go for the concrete paths (login, logout, current, …).
//
// Platform transactional outbox ops (Bearer JWT + platform_admin only):
//
//	GET  /v1/admin/ops/outbox
//	POST /v1/admin/ops/outbox/{outboxId}/retry
//
// Data retention system APIs (Bearer JWT + platform_admin only):
//
//	GET  /v1/admin/system/retention/stats
//	POST /v1/admin/system/retention/dry-run
//	POST /v1/admin/system/retention/run
//
// Reporting reads (Bearer JWT + platform_admin or org_admin):
//
//	GET /v1/reports/sales-summary
//	GET /v1/reports/payments-summary
//	GET /v1/reports/fleet-health
//	GET /v1/reports/inventory-exceptions
//	GET /v1/admin/organizations/{organizationId}/reports/sales
//	GET /v1/admin/organizations/{organizationId}/reports/payments
//	GET /v1/admin/organizations/{organizationId}/reports/refunds
//	GET /v1/admin/organizations/{organizationId}/reports/cash
//	GET /v1/admin/organizations/{organizationId}/reports/inventory-low-stock
//	GET /v1/admin/organizations/{organizationId}/reports/machine-health
//	GET /v1/admin/organizations/{organizationId}/reports/failed-vends
//	GET /v1/admin/organizations/{organizationId}/reports/reconciliation-queue
//	GET /v1/admin/organizations/{organizationId}/reports/vends
//	GET /v1/admin/organizations/{organizationId}/reports/inventory
//	GET /v1/admin/organizations/{organizationId}/reports/machines
//	GET /v1/admin/organizations/{organizationId}/reports/products
//	GET /v1/admin/organizations/{organizationId}/reports/reconciliation
//	GET /v1/admin/organizations/{organizationId}/reports/commands
//	GET /v1/admin/organizations/{organizationId}/reports/fills
//	GET /v1/admin/organizations/{organizationId}/reports/export
//
// OpenAPI: every mounted path above must stay in sync with tools/build_openapi.py REQUIRED_OPERATIONS
// and the DocOp* stubs in swagger_operations.go (generation fails on drift).
const pathV1MachineOperatorSessions = "/v1/machines/{machineId}/operator-sessions"
