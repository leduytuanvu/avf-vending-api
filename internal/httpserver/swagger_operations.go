package httpserver

// Swag-style operation documentation for the HTTP API.
//
// These declarations are not executed at runtime; they exist so tools/build_openapi.py can
// parse @Router and related annotations and emit docs/swagger/swagger.json (OpenAPI 3.0).
// Run: make swagger
//
// Auth contract: `/v1/*` uses BearerAccessTokenMiddlewareWithValidator plus route-level RBAC
// (RequireAnyRole, RequireOrganizationScope, RequireMachineURLAccess). All JSON errors—including
// bearer, scope, and handler failures—use the same envelope:
// `{"error":{"code":"...","message":"...","details":{},"requestId":"..."}}`.
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
// @Summary OpenAPI 3.0 document (embedded)
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

// --- Auth (session APIs) ---

// DocOpV1AuthLogin godoc
// @Summary Exchange email/password for JWT session tokens
// @Description Authenticates an **API account** for a specific organization. Returns access + refresh tokens and resolved roles. On failure returns **401** with `unauthenticated` or credential-specific codes. Rate limiting may apply when `HTTP_RATE_LIMIT_SENSITIVE_WRITES_ENABLED=true` (sensitive-write bucket).
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body V1AuthLoginRequest true "Login credentials"
// @Success 200 {object} V1AuthLoginResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 429 {object} V1StandardError "rate_limited when sensitive-write rate limit trips"
// @Failure 500 {object} V1StandardError
// @Router /v1/auth/login [post]
func DocOpV1AuthLogin() {}

// DocOpV1AuthRefresh godoc
// @Summary Rotate access token using a refresh token
// @Description Validates a non-revoked refresh token and returns a new access/refresh pair. Refresh token may be rotated server-side; clients must persist the latest refresh token from the response body.
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body V1AuthRefreshRequest true "Refresh token"
// @Success 200 {object} V1AuthRefreshResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/auth/refresh [post]
func DocOpV1AuthRefresh() {}

// DocOpV1AuthMe godoc
// @Summary Current authenticated principal
// @Description Returns account id, organization id (when scoped), email, and roles for the Bearer access token. Requires `Authorization: Bearer` on `/v1/auth/*` bearer group.
// @Tags Auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} V1AuthMeResponse
// @Failure 401 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/auth/me [get]
func DocOpV1AuthMe() {}

// DocOpV1AuthLogout godoc
// @Summary Revoke refresh token(s)
// @Description Optionally revokes a single refresh token or all refresh tokens for the account when `revokeAll` is true. Access tokens remain valid until expiry; clients should discard local copies after logout.
// @Tags Auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body V1AuthLogoutRequest false "Optional body (omit for default revoke current refresh)"
// @Success 204 {string} string "empty body"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/auth/logout [post]
func DocOpV1AuthLogout() {}

// --- Admin catalog (read-only) ---

// DocOpV1AdminProductsList godoc
// @Summary List products (admin catalog)
// @Description Paginated product directory for an organization. **platform_admin** must pass **organization_id** query; **org_admin** is scoped to JWT organization. Supports `q` substring search on sku/name, `active_only` boolean, and standard **limit**/**offset** pagination (default 50, max 500).
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param q query string false "Search substring (sku / name)"
// @Param active_only query bool false "When true, only active products"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminProductListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/products [get]
func DocOpV1AdminProductsList() {}

// DocOpV1AdminProductGet godoc
// @Summary Get product by id (admin catalog)
// @Description Returns full product attributes including JSON `attrs`, allergen codes, and merchandising metadata. **platform_admin** must pass **organization_id** matching the product's organization.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param productId path string true "Product UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminProduct
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/products/{productId} [get]
func DocOpV1AdminProductGet() {}

// DocOpV1AdminPriceBooksList godoc
// @Summary List price books (admin catalog)
// @Description Operational pricing tables for the organization (effective windows, default flag, scope). **platform_admin** requires **organization_id** query.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminPriceBookListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/price-books [get]
func DocOpV1AdminPriceBooksList() {}

// DocOpV1AdminPlanogramsList godoc
// @Summary List planograms (admin catalog)
// @Description Planogram revisions for slot layouts (draft/published/archived). **platform_admin** requires **organization_id** query.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminPlanogramListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/planograms [get]
func DocOpV1AdminPlanogramsList() {}

// DocOpV1AdminPlanogramGet godoc
// @Summary Get planogram detail with slots
// @Description Returns planogram header plus ordered slot rows (product assignment, max quantity, joined sku/name when configured). **platform_admin** requires **organization_id** matching the planogram organization.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param planogramId path string true "Planogram UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminPlanogramDetail
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/planograms/{planogramId} [get]
func DocOpV1AdminPlanogramGet() {}

// --- Admin inventory (read-only) ---

// DocOpV1AdminMachineSlots godoc
// @Summary List live slot inventory for a machine
// @Description Joins `machine_slot_state` to planograms/slots/products for the machine's current fill levels, price projection, and low-stock heuristics. **platform_admin** must pass **organization_id** for tenant pick; machine must belong to that organization.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminMachineSlotListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/slots [get]
func DocOpV1AdminMachineSlots() {}

// DocOpV1AdminMachineInventory godoc
// @Summary Aggregate inventory by product for a machine
// @Description Rolls up slot quantities per product for refill planning (totals, slot coverage, low-stock flag). Same scoping rules as slot list.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminMachineInventoryEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/inventory [get]
func DocOpV1AdminMachineInventory() {}

// --- Reporting (read-only analytics) ---

// DocOpV1ReportsSalesSummary godoc
// @Summary Sales rollup and trend breakdown
// @Description Aggregates `orders` for an organization in a half-open time window **[from, to)** (RFC3339, required). **platform_admin** must pass **organization_id**; maximum span **366 days**. `group_by` controls the breakdown dimension: **day** (default), **site**, **machine** (top 500 by revenue), **payment_method** (joins authorized/captured payments), or **none** (summary only). Amounts are integer minor units as stored in OLTP.
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param group_by query string false "day | site | machine | payment_method | none (default day)"
// @Success 200 {object} V1ReportingSalesSummaryResponse
// @Failure 400 {object} V1StandardError "invalid_query / organization_id_required / invalid date span"
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/reports/sales-summary [get]
func DocOpV1ReportsSalesSummary() {}

// DocOpV1ReportsPaymentsSummary godoc
// @Summary Payment outcomes and method/status breakdown
// @Description Aggregates `payments` joined to `orders` for the organization in **[from, to)** on `payments.created_at`. Counts authorized/captured/failed/refunded plus amount sums per state bucket. `group_by` selects **day** (default), **payment_method** (provider roll-up), **status** (state roll-up), or **none** (summary only). Requires same org scoping rules as other reports.
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param group_by query string false "day | payment_method | status | none (default day)"
// @Success 200 {object} V1ReportingPaymentsSummaryResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/reports/payments-summary [get]
func DocOpV1ReportsPaymentsSummary() {}

// DocOpV1ReportsFleetHealth godoc
// @Summary Machine posture and incident rollups
// @Description `machineSummary` buckets interpret `machines.status` (online/offline/maintenance/provisioning/retired). Incident sections filter `incidents.opened_at` and `machine_incidents.opened_at` to **[from, to)** for operational correlation. Machine counts themselves are current state (not time-filtered). **group_by** is not supported on this route.
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Success 200 {object} V1ReportingFleetHealthResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/reports/fleet-health [get]
func DocOpV1ReportsFleetHealth() {}

// DocOpV1ReportsInventoryExceptions godoc
// @Summary Slots needing refill or restock attention
// @Description Current-state scan of `machine_slot_state` vs planogram `slots`: **out_of_stock** (`current_quantity <= 0`) and **low_stock** (filled below 15% of configured `max_quantity` when max > 0). `exception_kind` filters rows: **all** (default), **low_stock**, or **out_of_stock**. **from/to** are validated for consistency with other reporting routes but do not filter rows in this version. Pagination uses **limit**/**offset** (default 50, max 500).
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param from query string true "RFC3339 (validated; reserved for future time filtering)"
// @Param to query string true "RFC3339 (validated; reserved for future time filtering)"
// @Param exception_kind query string false "all | low_stock | out_of_stock (default all)"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1ReportingInventoryExceptionsResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/reports/inventory-exceptions [get]
func DocOpV1ReportsInventoryExceptions() {}

// --- Admin (platform_admin or org_admin) ---

// DocOpV1AdminMachinesList godoc
// @Summary List machines (admin)
// @Description Read-only operational list of machines for an organization. **platform_admin** must pass **organization_id** query (tenant pick). **org_admin** uses JWT organization scope. Optional filters: **site_id**, **machine_id**, **status** (machine.status), **from** / **to** on `updated_at` (RFC3339), **search** is ignored for this resource. Pagination: **limit** (default 50, max 500), **offset**.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param site_id query string false "Filter by site UUID"
// @Param machine_id query string false "Filter to a single machine UUID"
// @Param status query string false "Filter by machine status (e.g. online, offline)"
// @Param from query string false "Inclusive lower bound for updated_at (RFC3339)"
// @Param to query string false "Inclusive upper bound for updated_at (RFC3339)"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset for pagination"
// @Success 200 {object} V1AdminMachinesListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines [get]
func DocOpV1AdminMachinesList() {}

// DocOpV1AdminTechniciansList godoc
// @Summary List technicians (admin)
// @Description Directory of technicians for the organization. **platform_admin** requires **organization_id** query. Optional **technician_id**, **search** (matches display_name or email), **from** / **to** on `created_at`, pagination **limit** / **offset**.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param technician_id query string false "Filter to one technician UUID"
// @Param search query string false "Case-insensitive substring on name or email"
// @Param from query string false "created_at lower bound (RFC3339)"
// @Param to query string false "created_at upper bound (RFC3339)"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminTechniciansListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/technicians [get]
func DocOpV1AdminTechniciansList() {}

// DocOpV1AdminAssignmentsList godoc
// @Summary List technician assignments (admin)
// @Description Joins technician, machine, and assignment rows for the organization. **platform_admin** requires **organization_id**. Optional **technician_id**, **machine_id**, **from** / **to** on assignment `created_at`, pagination **limit** / **offset**.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param technician_id query string false "Filter by technician UUID"
// @Param machine_id query string false "Filter by machine UUID"
// @Param from query string false "created_at lower bound (RFC3339)"
// @Param to query string false "created_at upper bound (RFC3339)"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminAssignmentsListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/assignments [get]
func DocOpV1AdminAssignmentsList() {}

// DocOpV1AdminCommandsList godoc
// @Summary List machine commands (admin)
// @Description Operational view of `command_ledger` joined to machines and latest `machine_command_attempts` status. **platform_admin** requires **organization_id**. Optional **machine_id**, **status** (filters latest attempt status; pending used when no attempts yet), **from** / **to** on command `created_at`, pagination **limit** / **offset**.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param machine_id query string false "Filter by machine UUID"
// @Param status query string false "Latest attempt status (pending, sent, completed, …)"
// @Param from query string false "created_at lower bound (RFC3339)"
// @Param to query string false "created_at upper bound (RFC3339)"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminCommandsListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/commands [get]
func DocOpV1AdminCommandsList() {}

// DocOpV1AdminOTAList godoc
// @Summary List OTA campaigns (admin)
// @Description Read-only list of `ota_campaigns` with joined artifact metadata. **platform_admin** requires **organization_id**. Optional **status** (draft, active, paused, completed), **from** / **to** on campaign `created_at`, pagination **limit** / **offset**.
// @Tags Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param status query string false "Campaign status filter"
// @Param from query string false "created_at lower bound (RFC3339)"
// @Param to query string false "created_at upper bound (RFC3339)"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminOTAListResponse
// @Failure 400 {object} V1StandardError
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

// --- Tenant commerce operational lists ---

// DocOpV1PaymentsList godoc
// @Summary List payments for organization
// @Description Read-only operational list: each row joins `payments` to its parent `orders` for machine and order status context. **platform_admin** must pass **organization_id** query; **org_admin** uses JWT organization scope (optional `organization_id` must match). Filters: **status** (payment.state enum), **payment_method** (exact provider string, e.g. stripe), **machine_id** (UUID on parent order), **search** (substring on payment id, order id, or idempotency key), **from** / **to** inclusive bounds on `payments.created_at` (defaults to wide internal bounds when omitted — prefer explicit windows for large tenants). Pagination: **limit** default 50, max **500**, **offset** for pages. Response is typed `items` + `meta.total` for UI virtualization.
// @Tags Commerce
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param status query string false "Payment state filter (created, authorized, captured, failed, refunded)"
// @Param payment_method query string false "Provider filter (e.g. stripe)"
// @Param machine_id query string false "Filter by order.machine_id"
// @Param search query string false "Substring search on ids / idempotency key"
// @Param from query string false "created_at lower bound (RFC3339)"
// @Param to query string false "created_at upper bound (RFC3339)"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1PaymentsListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/payments [get]
func DocOpV1PaymentsList() {}

// DocOpV1OrdersList godoc
// @Summary List orders for organization
// @Description Read-only `orders` rows for reconciliation and support dashboards. **platform_admin** requires **organization_id** query; **org_admin** is JWT-scoped. Filters: **status** (order.status lifecycle), **machine_id** (UUID), **search** (substring on order id or idempotency key), **from** / **to** inclusive on `orders.created_at` (defaults apply when omitted). Pagination: **limit** default 50, max **500**, **offset**. Typed `items` + `meta.total` envelope matches payments list for shared client parsing.
// @Tags Commerce
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param status query string false "Order status filter"
// @Param machine_id query string false "Filter by machine UUID"
// @Param search query string false "Substring search on order id or idempotency key"
// @Param from query string false "created_at lower bound (RFC3339)"
// @Param to query string false "created_at upper bound (RFC3339)"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1OrdersListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/orders [get]
func DocOpV1OrdersList() {}

// --- Fleet / device read ---

// DocOpV1MachineShadowGet godoc
// @Summary Get machine shadow JSON
// @Description Returns the persisted desired/reported JSON documents used for fleet remote configuration. Requires Bearer JWT and `RequireMachineURLAccess(machineId)` (same org or platform admin). Invalid UUID in path → **400** (`invalid_machine_id`). Missing shadow row → **404** `machine_shadow_not_found`. This is not live MQTT; it is the last reconciled projection in Postgres.
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
// @Description Idempotency header is required (same key space as other commerce writes). Treat **200** with `replay:true` as the provider having retried the same logical event; responses remain JSON-safe for your payment connector. Conflicting provider state may yield **409** with a commerce-specific `error.code`.
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
