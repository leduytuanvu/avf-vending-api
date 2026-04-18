package httpserver

// Route registration for the public API is implemented in server.go (NewHTTPServer → mountV1).
//
// JSON error envelope for most /v1 JSON handlers: {"error":{"code":"...","message":"..."}}.
// List handlers that intentionally omit persistence return *api.CapabilityError → HTTP 501 with:
// {"error":{"code":"not_implemented","message":"...","capability":"...","implemented":false}}.
// Optional runtime wiring that is missing in this process (MQTT publisher, commerce store, etc.) uses
// writeCapabilityNotConfigured → HTTP 503 with code "capability_not_configured" (retryable / ops action).
// Prefer branching on error.code; message is diagnostic text and may change.
//
// Request tracing: middleware.RequestID runs on the root router. Clients may send X-Request-ID and
// X-Correlation-ID; both are echoed on the response. When X-Correlation-ID is omitted, it defaults
// to the request ID. Operator session login/logout persists correlation where the domain supports it.
// Bearer auth rejections log request_id and correlation_id (see internal/platform/auth/middleware.go).
// Optional per-IP rate limits on sensitive writes: HTTP_RATE_LIMIT_SENSITIVE_WRITES_* (middleware_rate_limit.go).
//
// Backend artifacts (Bearer JWT + org_admin or platform_admin; S3 when API_ARTIFACTS_ENABLED=true):
// POST/GET/PUT/DELETE under /v1/admin/organizations/{orgId}/artifacts — see artifacts_http.go.
//
// Commerce endpoints (Bearer JWT + RequireOrganizationScope) are mounted under:
//
//	/v1/commerce/orders
//	/v1/commerce/orders/{orderId}/payment-session
//	/v1/commerce/orders/{orderId}
//	/v1/commerce/orders/{orderId}/reconciliation
//	/v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks
//	/v1/commerce/orders/{orderId}/vend/start
//	/v1/commerce/orders/{orderId}/vend/success
//	/v1/commerce/orders/{orderId}/vend/failure
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
// OpenAPI: every mounted path above must stay in sync with tools/build_openapi.py REQUIRED_OPERATIONS
// and the DocOp* stubs in swagger_operations.go (generation fails on drift).
const pathV1MachineOperatorSessions = "/v1/machines/{machineId}/operator-sessions"
