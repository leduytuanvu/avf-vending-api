package httpserver

// Swag-style operation documentation for the HTTP API.
//
// These declarations are not executed at runtime; they exist so tools/build_openapi.py can
// parse @Router and related annotations and emit docs/swagger/swagger.json (OpenAPI 2.0).
// Run: make swagger
//
// Auth contract: `/v1/*` uses BearerAccessTokenMiddlewareWithValidator — missing/invalid JWT → 401 JSON
// `{"error":{"message":"..."}}` (no `code` field). Role/scope denials → same shape with 403.
// Misconfigured auth (e.g. JWKS) → 503 with the same minimal JSON body.
// Most handler JSON errors use writeAPIError → `{"error":{"code":"...","message":"..."}}`.
//
// Keep each func body empty: func DocOpXxx() {}

// --- Health ---

// DocOpHealthLive godoc
// @Summary Liveness probe
// @Description Returns 200 with body `ok` when the process is running. No authentication. Response echoes `X-Request-ID` / `X-Correlation-ID` when sent.
// @Tags Health
// @Produce text/plain
// @Success 200 {string} string "ok"
// @Router /health/live [get]
func DocOpHealthLive() {}

// DocOpHealthReady godoc
// @Summary Readiness probe
// @Description Returns 200 with body `ok` when readiness checks pass. When READINESS_STRICT and dependencies fail, returns **503** with plain text body `not ready` (not the JSON error envelope).
// @Tags Health
// @Produce text/plain
// @Success 200 {string} string "ok"
// @Failure 503 {string} string "not ready"
// @Router /health/ready [get]
func DocOpHealthReady() {}

// DocOpMetrics godoc
// @Summary Prometheus metrics scrape
// @Description Exposed only when METRICS_ENABLED=true; otherwise the route is not registered (clients may observe connection-level 404).
// @Tags Reliability
// @Produce text/plain
// @Success 200 {string} string "Prometheus text exposition format"
// @Router /metrics [get]
func DocOpMetrics() {}

// --- OpenAPI / Swagger UI (no Bearer auth) ---

// DocOpSwaggerDocJSON godoc
// @Summary OpenAPI 2.0 document (embedded)
// @Description Served when HTTP_SWAGGER_UI_ENABLED=true. Same JSON the UI loads; no `Authorization` header required.
// @Tags Documentation
// @Produce application/json
// @Success 200 {object} object "swagger 2.0 document root"
// @Router /swagger/doc.json [get]
func DocOpSwaggerDocJSON() {}

// DocOpSwaggerIndex godoc
// @Summary Swagger UI (HTML)
// @Description Browser UI entrypoint when Swagger is enabled; loads `/swagger/doc.json`.
// @Tags Documentation
// @Produce text/html
// @Success 200 {string} string "Swagger UI HTML"
// @Router /swagger/index.html [get]
func DocOpSwaggerIndex() {}

// --- Admin (platform_admin or org_admin) ---

// DocOpV1AdminMachinesList godoc
// @Summary List machines (admin)
// @Description Postgres-backed fleet listing when Fleet service is wired. Returns **501** with `not_implemented` when fleet is nil or misconfigured (`v1.admin.machines.list`). Platform admin must supply tenant scope (organization_id on principal) or receives **400** `tenant_scope_required`.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} V1ListViewEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 501 {object} V1NotImplementedError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines [get]
func DocOpV1AdminMachinesList() {}

// DocOpV1AdminTechniciansList godoc
// @Summary List technicians (admin)
// @Description Returns **501** with `not_implemented` and capability `v1.admin.technicians.list` until a repository is wired.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Success 501 {object} V1NotImplementedError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/technicians [get]
func DocOpV1AdminTechniciansList() {}

// DocOpV1AdminAssignmentsList godoc
// @Summary List technician assignments (admin)
// @Description Returns **501** with capability `v1.admin.assignments.list`.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Success 501 {object} V1NotImplementedError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/assignments [get]
func DocOpV1AdminAssignmentsList() {}

// DocOpV1AdminCommandsList godoc
// @Summary List commands (admin)
// @Description Returns **501** with capability `v1.admin.commands.list`.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Success 501 {object} V1NotImplementedError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/commands [get]
func DocOpV1AdminCommandsList() {}

// DocOpV1AdminOTAList godoc
// @Summary List OTA artifacts (admin)
// @Description Returns **501** with capability `v1.admin.ota.list`.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Success 501 {object} V1NotImplementedError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ota [get]
func DocOpV1AdminOTAList() {}

// --- Artifacts (feature-gated: API_ARTIFACTS_ENABLED) ---

// DocOpV1AdminArtifactsReserve godoc
// @Summary Reserve artifact id
// @Description Routes are **not mounted** unless API_ARTIFACTS_ENABLED=true and object store is configured. Requires org_admin for orgId or platform_admin. Uses sensitive-write rate limiting when HTTP_RATE_LIMIT_SENSITIVE_WRITES_ENABLED=true → **429** `rate_limited`.
// @Tags Artifacts
// @Security BearerAuth
// @Param orgId path string true "Organization UUID"
// @Accept json
// @Produce json
// @Success 201 {object} object "{artifact_id,upload_path}"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/organizations/{orgId}/artifacts [post]
func DocOpV1AdminArtifactsReserve() {}

// DocOpV1AdminArtifactsList godoc
// @Summary List artifacts
// @Tags Artifacts
// @Security BearerAuth
// @Param orgId path string true "Organization UUID"
// @Produce json
// @Success 200 {object} object "{items:[artifact objects]}"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/organizations/{orgId}/artifacts [get]
func DocOpV1AdminArtifactsList() {}

// DocOpV1AdminArtifactsGet godoc
// @Summary Get artifact metadata
// @Tags Artifacts
// @Security BearerAuth
// @Param orgId path string true "Organization UUID"
// @Param artifactId path string true "Artifact UUID"
// @Produce json
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/organizations/{orgId}/artifacts/{artifactId} [get]
func DocOpV1AdminArtifactsGet() {}

// DocOpV1AdminArtifactsDownloadURL godoc
// @Summary Presigned download URL
// @Tags Artifacts
// @Security BearerAuth
// @Param orgId path string true "Organization UUID"
// @Param artifactId path string true "Artifact UUID"
// @Produce json
// @Success 200 {object} object "{method,url,headers,expires_at}"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/organizations/{orgId}/artifacts/{artifactId}/download [get]
func DocOpV1AdminArtifactsDownloadURL() {}

// DocOpV1AdminArtifactsPutContent godoc
// @Summary Upload artifact bytes
// @Description Request body is raw bytes. Required headers: **Content-Length**; optional **Content-Type**, **X-Artifact-SHA256**, **X-Artifact-Filename**. Sensitive-write rate limit may return **429**.
// @Tags Artifacts
// @Security BearerAuth
// @Param orgId path string true "Organization UUID"
// @Param artifactId path string true "Artifact UUID"
// @Accept octet-stream
// @Produce json
// @Success 200 {object} object "{status,artifact_id}"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/organizations/{orgId}/artifacts/{artifactId}/content [put]
func DocOpV1AdminArtifactsPutContent() {}

// DocOpV1AdminArtifactsDelete godoc
// @Summary Delete artifact
// @Tags Artifacts
// @Security BearerAuth
// @Param orgId path string true "Organization UUID"
// @Param artifactId path string true "Artifact UUID"
// @Produce json
// @Success 200 {object} object "{status,artifact_id}"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/organizations/{orgId}/artifacts/{artifactId} [delete]
func DocOpV1AdminArtifactsDelete() {}

// --- Operator insights ---

// DocOpV1OperatorInsightsTechnicianAttributions godoc
// @Summary List action attributions for a technician
// @Description Mounted only when operator service is configured. **organization_id** query is required when the caller is platform_admin without tenant org on the JWT.
// @Tags Operator
// @Security BearerAuth
// @Param technicianId path string true "Technician UUID"
// @Param organization_id query string false "Required for platform_admin without org scope"
// @Param limit query int false "Default 50, max 500"
// @Produce json
// @Success 200 {object} V1OperatorListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/operator-insights/technicians/{technicianId}/action-attributions [get]
func DocOpV1OperatorInsightsTechnicianAttributions() {}

// DocOpV1OperatorInsightsUserAttributions godoc
// @Summary List action attributions for a user principal
// @Description **user_principal** query parameter is required. Same organization_id rules as technician insights.
// @Tags Operator
// @Security BearerAuth
// @Param organization_id query string false "Required for platform_admin without org scope"
// @Param user_principal query string true "User subject / principal string"
// @Param limit query int false "Default 50, max 500"
// @Produce json
// @Success 200 {object} V1OperatorListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/operator-insights/users/action-attributions [get]
func DocOpV1OperatorInsightsUserAttributions() {}

// --- Tenant commerce lists (not implemented) ---

// DocOpV1PaymentsList godoc
// @Summary List payments for organization
// @Description Requires Bearer JWT and RequireOrganizationScope. **501** `not_implemented` with capability `v1.payments.org_list` until implemented. Non-platform callers without org on JWT get **403** from middleware (minimal JSON, no `code`).
// @Tags Commerce
// @Security BearerAuth
// @Produce json
// @Success 501 {object} V1NotImplementedError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/payments [get]
func DocOpV1PaymentsList() {}

// DocOpV1OrdersList godoc
// @Summary List orders for organization
// @Description **501** with capability `v1.orders.org_list`. Same auth and 403 behavior as payments list.
// @Tags Commerce
// @Security BearerAuth
// @Produce json
// @Success 501 {object} V1NotImplementedError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/orders [get]
func DocOpV1OrdersList() {}

// --- Fleet / device read ---

// DocOpV1MachineShadowGet godoc
// @Summary Get machine shadow JSON
// @Description Requires Bearer JWT and RequireMachineURLAccess(machineId). Invalid UUID in path → **400** from middleware or handler (`invalid_machine_id`). Missing shadow row → **404** `machine_shadow_not_found`.
// @Tags Fleet
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Produce json
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/shadow [get]
func DocOpV1MachineShadowGet() {}

// DocOpV1MachineTelemetrySnapshot godoc
// @Summary Current machine telemetry snapshot (projected)
// @Description Read-only `machine_current_snapshot` row (rollups + shadow projection). **404** when no snapshot exists yet. Not a raw MQTT history API.
// @Tags Fleet
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Produce json
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/telemetry/snapshot [get]
func DocOpV1MachineTelemetrySnapshot() {}

// DocOpV1MachineTelemetryIncidents godoc
// @Summary Recent persisted machine incidents
// @Description Returns deduped incident rows from Postgres (fed by `incident.*` telemetry via JetStream workers). Optional `limit` query (default 50, max 500).
// @Tags Fleet
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Param limit query int false "Default 50, max 500"
// @Produce json
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/telemetry/incidents [get]
func DocOpV1MachineTelemetryIncidents() {}

// DocOpV1MachineTelemetryRollups godoc
// @Summary Telemetry rollup buckets (1m / 1h)
// @Description Aggregated `telemetry_rollups` only — not raw high-frequency streams. Query `from`/`to` as RFC3339 (default last 24h), `granularity` (`1m` default, `1h`).
// @Tags Fleet
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Param from query string false "RFC3339 lower bound"
// @Param to query string false "RFC3339 upper bound"
// @Param granularity query string false "1m or 1h"
// @Produce json
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/telemetry/rollups [get]
func DocOpV1MachineTelemetryRollups() {}

// --- Device remote commands ---

// DocOpV1MachineCommandReceipts godoc
// @Summary List recent command receipts
// @Description Mounted when RemoteCommands service is configured. If app wiring is nil, handler returns **500** `internal` (not 503). Optional **limit** query (default 50, max 500).
// @Tags Device
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Param limit query int false "Default 50, max 500"
// @Produce json
// @Success 200 {object} V1OperatorListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/commands/receipts [get]
func DocOpV1MachineCommandReceipts() {}

// DocOpV1MachineCommandStatus godoc
// @Summary Get command dispatch status by sequence
// @Description **sequence** must be a non-negative integer. Unknown sequence → **404** `command_not_found`.
// @Tags Device
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Param sequence path string true "Ledger sequence (non-negative integer)"
// @Produce json
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/commands/{sequence}/status [get]
func DocOpV1MachineCommandStatus() {}

// DocOpV1MachineCommandDispatch godoc
// @Summary Dispatch remote MQTT command
// @Description Requires **Idempotency-Key** or **X-Idempotency-Key**. Org/platform admin only. When MQTT publisher is missing → **503** `capability_not_configured` (`mqtt_command_dispatch`). Nil RemoteCommands wiring → **500**. Rate limiter may return **429** `rate_limited`.
// @Tags Device
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Accept json
// @Produce json
// @Param body body object true "command_type, payload, desired_state, optional correlation_id, operator_session_id"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Failure 503 {object} V1CapabilityNotConfiguredError
// @Router /v1/machines/{machineId}/commands/dispatch [post]
func DocOpV1MachineCommandDispatch() {}

// --- Operator sessions (machine-scoped) ---

// DocOpV1OperatorSessionCurrent godoc
// @Summary Get current operator session
// @Description Routes not mounted when operator service is nil. Response is `{"active_session":null}` or `{"active_session":{...}}` (optional `technician_display_name`).
// @Tags Operator
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Produce json
// @Success 200 {object} V1OperatorCurrentEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/operator-sessions/current [get]
func DocOpV1OperatorSessionCurrent() {}

// DocOpV1OperatorSessionHistory godoc
// @Summary List session history
// @Tags Operator
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Param limit query int false "Default 50, max 500"
// @Produce json
// @Success 200 {object} V1OperatorListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/operator-sessions/history [get]
func DocOpV1OperatorSessionHistory() {}

// DocOpV1OperatorSessionAuthEvents godoc
// @Summary List auth events
// @Tags Operator
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Param limit query int false "Default 50, max 500"
// @Produce json
// @Success 200 {object} V1OperatorListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/operator-sessions/auth-events [get]
func DocOpV1OperatorSessionAuthEvents() {}

// DocOpV1OperatorSessionActionAttributions godoc
// @Summary List action attributions for machine
// @Tags Operator
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Param limit query int false "Default 50, max 500"
// @Produce json
// @Success 200 {object} V1OperatorListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/operator-sessions/action-attributions [get]
func DocOpV1OperatorSessionActionAttributions() {}

// DocOpV1OperatorSessionTimeline godoc
// @Summary Combined operator timeline
// @Tags Operator
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Param limit query int false "Default 50, max 500"
// @Produce json
// @Success 200 {object} V1OperatorListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/operator-sessions/timeline [get]
func DocOpV1OperatorSessionTimeline() {}

// DocOpV1OperatorSessionLogin godoc
// @Summary Start or resume operator session
// @Description Actor (TECHNICIAN vs USER) comes from JWT claims only. Optional **force_admin_takeover** (org/platform admin). Conflicts → **409** `active_session_exists`. Technician without assignment checker returns **503** with JSON `code` **assignment_checker_misconfigured** (standard envelope, not capability_not_configured). Rate limit → **429**.
// @Tags Operator
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Accept json
// @Produce json
// @Param body body object false "auth_method (defaults oidc), expires_at, client_metadata, force_admin_takeover"
// @Success 200 {object} V1OperatorSessionEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 503 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/operator-sessions/login [post]
func DocOpV1OperatorSessionLogin() {}

// DocOpV1OperatorSessionLogout godoc
// @Summary End operator session
// @Description **final_status** empty or ENDED (default) or REVOKED (org/platform admin only). **session_id** may be omitted when an ACTIVE session exists for the machine.
// @Tags Operator
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Accept json
// @Produce json
// @Param body body object true "session_id, ended_reason, auth_method, optional final_status (ENDED|REVOKED)"
// @Success 200 {object} V1OperatorSessionEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/operator-sessions/logout [post]
func DocOpV1OperatorSessionLogout() {}

// DocOpV1OperatorSessionHeartbeat godoc
// @Summary Session activity heartbeat
// @Tags Operator
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Param sessionId path string true "Session UUID"
// @Produce json
// @Success 200 {object} V1OperatorSessionEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/operator-sessions/{sessionId}/heartbeat [post]
func DocOpV1OperatorSessionHeartbeat() {}

// --- Commerce ---

// DocOpV1CommerceCreateOrder godoc
// @Summary Create order and initial vend session
// @Description Commerce routes not mounted when commerce service nil. Requires org on JWT for non-platform users. **Idempotency-Key** or **X-Idempotency-Key** required. Not configured → **503** `capability_not_configured` (`v1.commerce.persistence`). Rate limit → **429**.
// @Tags Commerce
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body object true "machine_id, product_id, slot_index, currency, subtotal_minor, tax_minor, total_minor"
// @Success 201 {object} V1CommerceCreateOrderResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 503 {object} V1CapabilityNotConfiguredError
// @Failure 500 {object} V1StandardError
// @Router /v1/commerce/orders [post]
func DocOpV1CommerceCreateOrder() {}

// DocOpV1CommercePaymentSession godoc
// @Summary Start payment with outbox row
// @Description When commerce outbox env (topic/event/aggregate) is unset → **503** `capability_not_configured` (`v1.commerce.payment_session.outbox`). Requires idempotency header.
// @Tags Commerce
// @Security BearerAuth
// @Param orderId path string true "Order UUID"
// @Accept json
// @Produce json
// @Param body body object true "provider, payment_state, amount_minor, currency, outbox_payload_json"
// @Success 200 {object} object "{payment_id,payment_state,outbox_event_id,replay}"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 503 {object} V1CapabilityNotConfiguredError
// @Failure 500 {object} V1StandardError
// @Router /v1/commerce/orders/{orderId}/payment-session [post]
func DocOpV1CommercePaymentSession() {}

// DocOpV1CommerceGetOrder godoc
// @Summary Checkout status for order
// @Description Optional **slot_index** query (default 0). Response nests `order`, `vend`, and `payment` (nullable).
// @Tags Commerce
// @Security BearerAuth
// @Param orderId path string true "Order UUID"
// @Param slot_index query int false "Slot index (default 0)"
// @Produce json
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 503 {object} V1CapabilityNotConfiguredError
// @Failure 500 {object} V1StandardError
// @Router /v1/commerce/orders/{orderId} [get]
func DocOpV1CommerceGetOrder() {}

// DocOpV1CommerceReconciliationSnapshot godoc
// @Summary Reconciliation snapshot wrapper
// @Description Returns `{kind, status}` where **status** matches checkout status JSON.
// @Tags Commerce
// @Security BearerAuth
// @Param orderId path string true "Order UUID"
// @Param slot_index query int false "Slot index"
// @Produce json
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 503 {object} V1CapabilityNotConfiguredError
// @Failure 500 {object} V1StandardError
// @Router /v1/commerce/orders/{orderId}/reconciliation [get]
func DocOpV1CommerceReconciliationSnapshot() {}

// DocOpV1CommercePaymentWebhook godoc
// @Summary Apply provider webhook
// @Tags Commerce
// @Security BearerAuth
// @Param orderId path string true "Order UUID"
// @Param paymentId path string true "Payment UUID"
// @Accept json
// @Produce json
// @Param body body object true "provider, provider_reference, event_type, normalized_payment_state, payload_json, ..."
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 503 {object} V1CapabilityNotConfiguredError
// @Failure 500 {object} V1StandardError
// @Router /v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks [post]
func DocOpV1CommercePaymentWebhook() {}

// DocOpV1CommerceVendStart godoc
// @Summary Advance vend to in_progress
// @Description Idempotency header required for write contract; value not yet used for dedupe on this transition. Illegal state → **409** `illegal_transition`.
// @Tags Commerce
// @Security BearerAuth
// @Param orderId path string true "Order UUID"
// @Accept json
// @Produce json
// @Param body body object true "slot_index"
// @Success 200 {object} object "{vend_state,slot_index}"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 503 {object} V1CapabilityNotConfiguredError
// @Failure 500 {object} V1StandardError
// @Router /v1/commerce/orders/{orderId}/vend/start [post]
func DocOpV1CommerceVendStart() {}

// DocOpV1CommerceVendSuccess godoc
// @Summary Finalize vend success
// @Description Requires captured payment or **409** `payment_not_settled`.
// @Tags Commerce
// @Security BearerAuth
// @Param orderId path string true "Order UUID"
// @Accept json
// @Produce json
// @Param body body object true "slot_index"
// @Success 200 {object} object "{order_id,order_status,vend_state}"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 503 {object} V1CapabilityNotConfiguredError
// @Failure 500 {object} V1StandardError
// @Router /v1/commerce/orders/{orderId}/vend/success [post]
func DocOpV1CommerceVendSuccess() {}

// DocOpV1CommerceVendFailure godoc
// @Summary Finalize vend failure
// @Tags Commerce
// @Security BearerAuth
// @Param orderId path string true "Order UUID"
// @Accept json
// @Produce json
// @Param body body object true "slot_index, failure_reason"
// @Success 200 {object} object "{order_id,order_status,vend_state}"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 503 {object} V1CapabilityNotConfiguredError
// @Failure 500 {object} V1StandardError
// @Router /v1/commerce/orders/{orderId}/vend/failure [post]
func DocOpV1CommerceVendFailure() {}
