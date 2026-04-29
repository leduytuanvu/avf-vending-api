package httpserver

// Swag-style operation documentation for the HTTP API.
//
// These declarations are not executed at runtime; they exist so tools/build_openapi.py can
// parse @Router and related annotations and emit docs/swagger/swagger.json (OpenAPI 3.0).
// Run: make swagger
//
// Auth contract: most `/v1/*` routes use BearerAccessTokenMiddlewareWithValidator plus route-level RBAC
// (RequireAnyRole, RequireOrganizationScope, RequireMachineTenantAccess / machine URL access). Provider payment webhooks under
// `/v1/commerce/orders/.../webhooks` are registered before Bearer middleware and use HMAC headers instead.
// All JSON errors—including
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
// @Summary Prometheus metrics scrape (public listener; optional)
// @Description When METRICS_ENABLED=true, **APP_ENV=production** defaults to **no** `/metrics` on the main listener — scrape **`HTTP_OPS_ADDR/metrics`** from private network instead. Non-production defaults to exposing `/metrics` on the main listener. If `METRICS_EXPOSE_ON_PUBLIC_HTTP=true` in production, `METRICS_SCRAPE_TOKEN` is required and callers must send `Authorization: Bearer <token>`. When the route is not registered, clients get **404**.
// @Tags System
// @Produce text/plain
// @Success 200 {string} string "Prometheus text exposition format"
// @Failure 401 {string} string "unauthorized when METRICS_SCRAPE_TOKEN is configured"
// @Failure 404 {string} string "not found when metrics are ops-only or METRICS_ENABLED=false"
// @Router /metrics [get]
func DocOpMetrics() {}

// DocOpVersion godoc
// @Summary Build and runtime version
// @Description Public JSON describing process name, semver, git SHA, build time, app environment, and optional runtime node metadata. No authentication. Values are non-secret operator diagnostics only.
// @Tags System
// @Produce json
// @Success 200 {object} V1VersionPayload
// @Router /version [get]
func DocOpVersion() {}

// --- OpenAPI / Swagger UI (no Bearer auth) ---

// DocOpSwaggerDocJSON godoc
// @Summary OpenAPI 3.0 document (embedded)
// @Description Served when `HTTP_OPENAPI_JSON_ENABLED=true` (default on). Unrelated to Swagger UI. No `Authorization` header required.
// @Tags System
// @Produce application/json
// @Success 200 {object} object "OpenAPI 3.0 document root"
// @Router /swagger/doc.json [get]
func DocOpSwaggerDocJSON() {}

// DocOpSwaggerIndex godoc
// @Summary Swagger UI (HTML)
// @Description Browser UI when `HTTP_SWAGGER_UI_ENABLED=true`; loads `/swagger/doc.json` (OpenAPI) when the JSON feature is on.
// @Tags System
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

// DocOpV1AuthChangePassword godoc
// @Summary Change password (self-service)
// @Description Authenticated API account rotates password using the current password; bcrypt replaces `password_hash` server-side and **revokes all refresh tokens** for this account. Requires Bearer JWT (`Authorization` header).
// @Tags Auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body V1AuthChangePasswordRequest true "Current and new password"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/auth/change-password [post]
func DocOpV1AuthChangePassword() {}

// DocOpV1AuthPasswordChange godoc
// @Summary Change password (self-service)
// @Description Alias for **POST /v1/auth/change-password**. Revokes all refresh tokens after setting a bcrypt hash.
// @Tags Auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body V1AuthChangePasswordRequest true "Current and new password"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Router /v1/auth/password/change [post]
func DocOpV1AuthPasswordChange() {}

// DocOpV1AuthPasswordResetRequest godoc
// @Summary Request password reset
// @Description Creates a short-lived one-time reset token when the email exists, but always returns the same accepted response so email existence is not leaked.
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body V1AuthPasswordResetRequest true "Organization and email"
// @Success 202 {object} V1AuthPasswordResetAccepted
// @Failure 400 {object} V1StandardError
// @Router /v1/auth/password/reset/request [post]
func DocOpV1AuthPasswordResetRequest() {}

// DocOpV1AuthPasswordResetConfirm godoc
// @Summary Confirm password reset
// @Description Consumes a hashed short-lived reset token once, sets the new bcrypt password hash, and revokes refresh sessions.
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body V1AuthPasswordResetConfirmRequest true "Reset token and new password"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Router /v1/auth/password/reset/confirm [post]
func DocOpV1AuthPasswordResetConfirm() {}

// DocOpV1AuthMFAEnroll godoc
// @Summary Start TOTP MFA enrollment
// @Description Creates a **pending** TOTP factor (encrypted at rest). Call **POST /v1/auth/mfa/totp/verify** with the same interactive access token while a pending factor exists, or complete production-required enrollment using the MFA challenge JWT from login. Requires `ADMIN_MFA_ENCRYPTION_KEY` (32-byte base64) when using MFA.
// @Tags Auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Success 200 {object} V1AuthMFAEnrollResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError "mfa_conflict when TOTP already active"
// @Failure 503 {object} V1StandardError "mfa_misconfigured"
// @Router /v1/auth/mfa/totp/enroll [post]
func DocOpV1AuthMFAEnroll() {}

// DocOpV1AuthMFAVerify godoc
// @Summary Verify TOTP (enrollment or login)
// @Description Supplies a TOTP code. With an **mfa_pending** JWT (from login when MFA is required), completes authentication and returns tokens. With a normal access token, activates a **pending** enrollment created by **/mfa/totp/enroll**.
// @Tags Auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body V1AuthMFAVerifyRequest true "6-digit TOTP code"
// @Success 200 {object} V1AuthLoginResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Router /v1/auth/mfa/totp/verify [post]
func DocOpV1AuthMFAVerify() {}

// DocOpV1AuthMFADisable godoc
// @Summary Disable TOTP for the current user
// @Description Requires interactive access (not mfa_pending). Validates password + active TOTP, disables factor, and revokes refresh tokens.
// @Tags Auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body V1AuthMFADisableRequest true "password and TOTP"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Router /v1/auth/mfa/totp/disable [post]
func DocOpV1AuthMFADisable() {}

// DocOpV1AuthSessionsList godoc
// @Summary List current admin sessions
// @Tags Auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} V1AuthSessionsEnvelope
// @Router /v1/auth/sessions [get]
func DocOpV1AuthSessionsList() {}

// DocOpV1AuthSessionDelete godoc
// @Summary Revoke one session
// @Tags Auth
// @Security BearerAuth
// @Produce json
// @Param sessionId path string true "Session UUID"
// @Success 204 {string} string "No Content"
// @Router /v1/auth/sessions/{sessionId} [delete]
func DocOpV1AuthSessionDelete() {}

// DocOpV1AuthSessionsRevokeOthers godoc
// @Summary Revoke other sessions
// @Description Revokes every active session except the refresh chain identified by **exceptRefreshToken** in the JSON body.
// @Tags Auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body V1AuthRevokeOtherSessionsRequest true "Keep current refresh token"
// @Success 204 {string} string "No Content"
// @Router /v1/auth/sessions [delete]
func DocOpV1AuthSessionsRevokeOthers() {}

// DocOpV1AdminAuthUsersList godoc
// @Summary List API accounts for an organization (admin)
// @Description Tenant-scoped directory of `platform_auth_accounts`. **platform_admin** must pass **organization_id** query; **org_admin** uses JWT organization scope. Pagination uses **limit**/**offset** (same semantics as other admin lists). **RBAC:** requires **user:read**; see **V1RBACPermissionMatrixDoc** in OpenAPI components for permission string examples and audit action notes.
// @Tags Auth Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminAuthUsersListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/auth/users [get]
func DocOpV1AdminAuthUsersList() {}

// DocOpV1AdminAuthUsersCreate godoc
// @Summary Create API account (admin)
// @Description Creates an auth account with bcrypt password hash and validated roles. Email is normalized to lowercase; passwords must be at least 10 characters; roles must be non-empty and from the platform whitelist. Subject to sensitive-write rate limiting when enabled. **RBAC:** **user:write**. Successful creates emit enterprise audit **auth.user.created**.
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body V1AdminAuthUsersCreateRequest true "email, password, roles"
// @Success 201 {object} V1AdminAuthAccount
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError "duplicate_email"
// @Failure 429 {object} V1StandardError "rate_limited when sensitive-write rate limit trips"
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/auth/users [post]
func DocOpV1AdminAuthUsersCreate() {}

// DocOpV1AdminAuthUsersGet godoc
// @Summary Get API account by id (admin)
// @Description Returns account metadata (no credential fields). Tenant scope enforced via organization resolution (`organization_id` query for platform admins).
// @Tags Auth Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param accountId path string true "Account UUID"
// @Success 200 {object} V1AdminAuthAccount
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/auth/users/{accountId} [get]
func DocOpV1AdminAuthUsersGet() {}

// DocOpV1AdminAuthUsersPatch godoc
// @Summary Patch API account (admin)
// @Description Partial update for email (normalized), roles (whitelist), and/or status (`active` | `disabled`). Cannot remove or deactivate the last **org_admin** for the organization.
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param accountId path string true "Account UUID"
// @Param body body V1AdminAuthUsersPatchRequest true "Optional fields"
// @Success 200 {object} V1AdminAuthAccount
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError "last_org_admin"
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError "duplicate_email"
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/auth/users/{accountId} [patch]
func DocOpV1AdminAuthUsersPatch() {}

// DocOpV1AdminAuthUsersActivate godoc
// @Summary Activate API account (admin)
// @Description Sets **status** to **active**.
// @Tags Auth Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param accountId path string true "Account UUID"
// @Success 200 {object} V1AdminAuthAccount
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/auth/users/{accountId}/activate [post]
func DocOpV1AdminAuthUsersActivate() {}

// DocOpV1AdminAuthUsersDeactivate godoc
// @Summary Deactivate API account (admin)
// @Description Sets **status** to **disabled** (login blocked). Cannot deactivate the last **org_admin**.
// @Tags Auth Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param accountId path string true "Account UUID"
// @Success 200 {object} V1AdminAuthAccount
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError "last_org_admin"
// @Failure 404 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/auth/users/{accountId}/deactivate [post]
func DocOpV1AdminAuthUsersDeactivate() {}

// DocOpV1AdminAuthUsersResetPassword godoc
// @Summary Reset password (admin)
// @Description Sets a new bcrypt hash and revokes **all refresh tokens** for the account. Minimum password length 10.
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param accountId path string true "Account UUID"
// @Param body body V1AdminAuthResetPasswordRequest true "New password"
// @Success 200 {object} V1AdminAuthAccount
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/auth/users/{accountId}/reset-password [post]
func DocOpV1AdminAuthUsersResetPassword() {}

// DocOpV1AdminAuthUsersPutRoles godoc
// @Summary Replace API account roles (admin)
// @Description Replaces the **roles** array only. Requires **user:roles**. Cannot remove or deactivate the last **org_admin**.
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param accountId path string true "Account UUID"
// @Param body body object true "{\"roles\":[\"viewer\"]}"
// @Success 200 {object} V1AdminAuthAccount
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/auth/users/{accountId}/roles [put]
func DocOpV1AdminAuthUsersPutRoles() {}

// DocOpV1AdminAuthUsersPostRoles godoc
// @Summary Replace API account roles (admin)
// @Description POST alias for replacing the roles array. Requires **user:roles**.
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param accountId path string true "Account UUID"
// @Param body body object true "{\"roles\":[\"viewer\"]}"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/auth/users/{accountId}/roles [post]
func DocOpV1AdminAuthUsersPostRoles() {}

// DocOpV1AdminAuthUsersPatchRoles godoc
// @Summary Replace API account roles — PATCH alias
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param accountId path string true "Account UUID"
// @Param body body object true "{\"roles\":[\"viewer\"]}"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/auth/users/{accountId}/roles [patch]
func DocOpV1AdminAuthUsersPatchRoles() {}

// DocOpV1AdminAuthUsersPatchStatus godoc
// @Summary Patch API account status only
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param accountId path string true "Account UUID"
// @Param body body V1AdminAuthUsersStatusPatchRequest true "status"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/auth/users/{accountId}/status [patch]
func DocOpV1AdminAuthUsersPatchStatus() {}

// DocOpV1AdminAuthUsersRevokeSessions godoc
// @Summary Revoke API account sessions (admin)
// @Description Revokes all refresh tokens for the target account and best-effort revokes access JWTs by subject. **RBAC:** **user:sessions:revoke**.
// @Tags Auth Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param accountId path string true "Account UUID"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/auth/users/{accountId}/revoke-sessions [post]
func DocOpV1AdminAuthUsersRevokeSessions() {}

// DocOpV1AdminAuthUsersSessions godoc
// @Summary List sessions for an API account (admin)
// @Tags Auth Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param accountId path string true "Account UUID"
// @Success 200 {object} V1AdminAuthSessionsEnvelope
// @Router /v1/admin/auth/users/{accountId}/sessions [get]
func DocOpV1AdminAuthUsersSessions() {}

// DocOpV1AdminUsersList godoc
// @Summary List API accounts (admin) — alternate path
// @Description Same as **GET /v1/admin/auth/users**. Requires **user:read**.
// @Tags Auth Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminAuthUsersListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/users [get]
func DocOpV1AdminUsersList() {}

// DocOpV1AdminUsersCreate godoc
// @Summary Create API account — alternate path
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body V1AdminAuthUsersCreateRequest true "email, password, roles"
// @Success 201 {object} V1AdminAuthAccount
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError
// @Router /v1/admin/users [post]
func DocOpV1AdminUsersCreate() {}

// DocOpV1AdminUsersGet godoc
// @Summary Get API account — alternate path
// @Tags Auth Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param userId path string true "Account UUID"
// @Success 200 {object} V1AdminAuthAccount
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/users/{userId} [get]
func DocOpV1AdminUsersGet() {}

// DocOpV1AdminUsersPatch godoc
// @Summary Patch API account — alternate path
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param userId path string true "Account UUID"
// @Param body body V1AdminAuthUsersPatchRequest true "Optional fields; **roles** requires **user:roles**"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/users/{userId} [patch]
func DocOpV1AdminUsersPatch() {}

// DocOpV1AdminUsersPutRoles godoc
// @Summary Replace roles — alternate path
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param userId path string true "Account UUID"
// @Param body body object true "{\"roles\":[\"viewer\"]}"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/users/{userId}/roles [put]
func DocOpV1AdminUsersPutRoles() {}

// DocOpV1AdminUsersPostRoles godoc
// @Summary Replace roles — alternate path
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param userId path string true "Account UUID"
// @Param body body object true "{\"roles\":[\"catalog_manager\"]}"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/users/{userId}/roles [post]
func DocOpV1AdminUsersPostRoles() {}

// DocOpV1AdminUsersPatchRoles godoc
// @Summary Replace roles — PATCH alias
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param userId path string true "Account UUID"
// @Param body body object true "{\"roles\":[\"catalog_manager\"]}"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/users/{userId}/roles [patch]
func DocOpV1AdminUsersPatchRoles() {}

// DocOpV1AdminUsersPatchStatus godoc
// @Summary Patch account status only — alternate path
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param userId path string true "Account UUID"
// @Param body body V1AdminAuthUsersStatusPatchRequest true "status"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/users/{userId}/status [patch]
func DocOpV1AdminUsersPatchStatus() {}

// DocOpV1AdminUsersRevokeSessions godoc
// @Summary Revoke user sessions — alternate path
// @Tags Auth Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param userId path string true "Account UUID"
// @Success 204 {string} string "No Content"
// @Router /v1/admin/users/{userId}/revoke-sessions [post]
func DocOpV1AdminUsersRevokeSessions() {}

// DocOpV1AdminUsersSessions godoc
// @Summary List user sessions — alternate path
// @Tags Auth Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param userId path string true "Account UUID"
// @Success 200 {object} V1AdminAuthSessionsEnvelope
// @Router /v1/admin/users/{userId}/sessions [get]
func DocOpV1AdminUsersSessions() {}

// DocOpV1AdminOrgUsersList godoc
// @Summary List organization users
// @Tags Auth Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param limit query int false "Page size"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminAuthUsersListEnvelope
// @Router /v1/admin/organizations/{organizationId}/users [get]
func DocOpV1AdminOrgUsersList() {}

// DocOpV1AdminOrgUsersCreate godoc
// @Summary Create organization user
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param body body V1AdminAuthUsersCreateRequest true "email, password, roles"
// @Success 201 {object} V1AdminAuthAccount
// @Router /v1/admin/organizations/{organizationId}/users [post]
func DocOpV1AdminOrgUsersCreate() {}

// DocOpV1AdminOrgUsersGet godoc
// @Summary Get organization user
// @Tags Auth Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param userId path string true "User UUID"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/organizations/{organizationId}/users/{userId} [get]
func DocOpV1AdminOrgUsersGet() {}

// DocOpV1AdminOrgUsersPatch godoc
// @Summary Patch organization user
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param userId path string true "User UUID"
// @Param body body V1AdminAuthUsersPatchRequest true "Optional fields"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/organizations/{organizationId}/users/{userId} [patch]
func DocOpV1AdminOrgUsersPatch() {}

// DocOpV1AdminOrgUsersDisable godoc
// @Summary Disable organization user
// @Tags Auth Admin
// @Security BearerAuth
// @Param organizationId path string true "Organization UUID"
// @Param userId path string true "User UUID"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/organizations/{organizationId}/users/{userId}/disable [post]
func DocOpV1AdminOrgUsersDisable() {}

// DocOpV1AdminOrgUsersEnable godoc
// @Summary Enable organization user
// @Tags Auth Admin
// @Security BearerAuth
// @Param organizationId path string true "Organization UUID"
// @Param userId path string true "User UUID"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/organizations/{organizationId}/users/{userId}/enable [post]
func DocOpV1AdminOrgUsersEnable() {}

// DocOpV1AdminOrgUsersRoles godoc
// @Summary Replace organization user roles
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param userId path string true "User UUID"
// @Param body body object true "{\"roles\":[\"support\"]}"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/organizations/{organizationId}/users/{userId}/roles [post]
func DocOpV1AdminOrgUsersRoles() {}

// DocOpV1AdminOrgUsersPatchRoles godoc
// @Summary Replace organization user roles — PATCH alias
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param userId path string true "User UUID"
// @Param body body object true "{\"roles\":[\"support\"]}"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/organizations/{organizationId}/users/{userId}/roles [patch]
func DocOpV1AdminOrgUsersPatchRoles() {}

// DocOpV1AdminOrgUsersDeleteRole godoc
// @Summary Remove one role from organization user
// @Description **user:roles** required. At least one role must remain; removing the last **org_admin** may be rejected.
// @Tags Auth Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param userId path string true "User UUID"
// @Param role path string true "Role name to remove (URL-encoded)"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/organizations/{organizationId}/users/{userId}/roles/{role} [delete]
func DocOpV1AdminOrgUsersDeleteRole() {}

// DocOpV1AdminOrgUsersPatchStatus godoc
// @Summary Patch organization user status only
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param userId path string true "User UUID"
// @Param body body V1AdminAuthUsersStatusPatchRequest true "status"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/organizations/{organizationId}/users/{userId}/status [patch]
func DocOpV1AdminOrgUsersPatchStatus() {}

// DocOpV1AdminOrgUsersRevokeSessions godoc
// @Summary Revoke organization user sessions
// @Tags Auth Admin
// @Security BearerAuth
// @Param organizationId path string true "Organization UUID"
// @Param userId path string true "User UUID"
// @Success 204 {string} string "No Content"
// @Router /v1/admin/organizations/{organizationId}/users/{userId}/revoke-sessions [post]
func DocOpV1AdminOrgUsersRevokeSessions() {}

// DocOpV1AdminOrgUsersSessions godoc
// @Summary List sessions for an organization user
// @Tags Auth Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param userId path string true "User UUID"
// @Success 200 {object} V1AdminAuthSessionsEnvelope
// @Router /v1/admin/organizations/{organizationId}/users/{userId}/sessions [get]
func DocOpV1AdminOrgUsersSessions() {}

// DocOpV1AdminOrgUsersResetPassword godoc
// @Summary Reset organization user password
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param userId path string true "User UUID"
// @Param body body V1AdminAuthResetPasswordRequest true "New password"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/organizations/{organizationId}/users/{userId}/reset-password [post]
func DocOpV1AdminOrgUsersResetPassword() {}

// DocOpV1AdminUsersDisable godoc
// @Summary Disable API account — alternate path
// @Tags Auth Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param userId path string true "Account UUID"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/users/{userId}/disable [post]
func DocOpV1AdminUsersDisable() {}

// DocOpV1AdminUsersEnable godoc
// @Summary Enable API account — alternate path
// @Tags Auth Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param userId path string true "Account UUID"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/users/{userId}/enable [post]
func DocOpV1AdminUsersEnable() {}

// DocOpV1AdminUsersResetPassword godoc
// @Summary Reset password — alternate path
// @Tags Auth Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param userId path string true "Account UUID"
// @Param body body V1AdminAuthResetPasswordRequest true "New password"
// @Success 200 {object} V1AdminAuthAccount
// @Router /v1/admin/users/{userId}/reset-password [post]
func DocOpV1AdminUsersResetPassword() {}

// DocOpV1AdminAuditEventsList godoc
// @Summary List enterprise audit events
// @Description Append-only **audit_events** trail for security-sensitive actions. **platform_admin** must pass **organization_id** query; **org_admin** uses JWT organization scope. Filter by optional **action**, **actorId**, **actorType** (user/machine/system/payment_provider/webhook/service), **outcome** (success/failure), **resourceType**, **resourceId**, **machineId** (UUID), **from**/**to** (RFC3339), and pagination (**limit**/**offset**). Requires **audit.read** permission.
// @Tags Audit Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param action query string false "Canonical action identifier filter"
// @Param actorId query string false "Actor id substring filter"
// @Param actorType query string false "Actor type filter (user, machine, system, payment_provider, webhook, service)"
// @Param outcome query string false "Outcome filter (success, failure)"
// @Param resourceType query string false "Resource type filter"
// @Param resourceId query string false "Resource id substring filter"
// @Param machineId query string false "Machine UUID filter"
// @Param from query string false "Include rows created at or after this time (RFC3339/RFC3339Nano)"
// @Param to query string false "Include rows created before this time (RFC3339/RFC3339Nano)"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1EnterpriseAuditEventsListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/audit/events [get]
func DocOpV1AdminAuditEventsList() {}

// DocOpV1AdminOrganizationAuditEventsList godoc
// @Summary List enterprise audit events (organization scope)
// @Description Same as `GET /v1/admin/audit/events`, with **organizationId** in the path (required for all interactive principals). Filter by **action**, **actorId**, **actorType** (user, machine, system, payment_provider, webhook, service), **outcome**, **resourceType**, **resourceId**, **machineId**, **from**/**to**, **limit**/**offset**. Requires **audit.read**.
// @Tags Audit Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param action query string false "Canonical action identifier filter"
// @Param actorId query string false "Actor id filter"
// @Param actorType query string false "Actor type filter"
// @Param outcome query string false "Outcome filter (success, failure)"
// @Param resourceType query string false "Resource type filter"
// @Param resourceId query string false "Resource id filter"
// @Param machineId query string false "Machine UUID filter"
// @Param from query string false "Include rows created at or after this time (RFC3339/RFC3339Nano)"
// @Param to query string false "Include rows created before this time (RFC3339/RFC3339Nano)"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1EnterpriseAuditEventsListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/audit-events [get]
func DocOpV1AdminOrganizationAuditEventsList() {}

// DocOpV1AdminOrganizationAuditEventGet godoc
// @Summary Get one enterprise audit event by id
// @Description Returns a single append-only **audit_events** row scoped to the path **organizationId**. Requires **audit.read**.
// @Tags Audit Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param auditEventId path string true "Audit event UUID"
// @Success 200 {object} V1EnterpriseAuditEvent
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/audit-events/{auditEventId} [get]
func DocOpV1AdminOrganizationAuditEventGet() {}

// DocOpV1AdminOutboxOpsGet godoc
// @Summary List transactional outbox rows and pipeline stats
// @Description **platform_admin** only. Returns aggregate counts (pending, due now, dead-letter, leased publishes) and a paginated slice of recent `outbox_events` for operations. Does not mutate broker state.
// @Tags Platform Ops
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminOutboxOpsEnvelope
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ops/outbox [get]
func DocOpV1AdminOutboxOpsGet() {}

// DocOpV1AdminRetentionOpsGet godoc
// @Summary Show retention table visibility
// @Description **platform_admin** only. Returns table counts and oldest record timestamps for enterprise retention targets: audit, command trace, payment webhook evidence, outbox, and message dedupe. This endpoint is read-only and does not run cleanup.
// @Tags Platform Ops
// @Security BearerAuth
// @Produce json
// @Success 200 {object} V1AdminRetentionOpsEnvelope
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ops/retention [get]
func DocOpV1AdminRetentionOpsGet() {}

// DocOpV1AdminOutboxRetry godoc
// @Summary Reset a dead-lettered outbox row for retry
// @Description **platform_admin** only. Clears quarantine fields on `outbox_events` when the row is dead-lettered so the worker can publish again.
// @Tags Platform Ops
// @Security BearerAuth
// @Produce json
// @Param outboxId path int true "Outbox row id"
// @Success 200 {object} V1AdminOutboxRetryEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ops/outbox/{outboxId}/retry [post]
func DocOpV1AdminOutboxRetry() {}

// DocOpV1AdminSystemOutboxStatsGet godoc
// @Summary Outbox pipeline statistics (system alias)
// @Description **platform_admin** only. Same aggregate counters as GET `/v1/admin/ops/outbox` **stats** component without listing rows.
// @Tags Platform Ops
// @Security BearerAuth
// @Produce json
// @Success 200 {object} V1AdminOutboxStatsEnvelope
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/system/outbox/stats [get]
func DocOpV1AdminSystemOutboxStatsGet() {}

// DocOpV1AdminSystemOutboxListGet godoc
// @Summary List transactional outbox rows (system alias)
// @Description **platform_admin** only. Paginated slice of recent `outbox_events` plus pipeline stats — equivalent to GET `/v1/admin/ops/outbox`.
// @Tags Platform Ops
// @Security BearerAuth
// @Produce json
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminOutboxOpsEnvelope
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/system/outbox [get]
func DocOpV1AdminSystemOutboxListGet() {}

// DocOpV1AdminSystemOutboxGet godoc
// @Summary Get one outbox row by id
// @Description **platform_admin** only. Returns a single `outbox_events` row including payload and lifecycle fields.
// @Tags Platform Ops
// @Security BearerAuth
// @Produce json
// @Param eventId path int true "Outbox row id"
// @Success 200 {object} V1AdminOutboxRow
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/system/outbox/{eventId} [get]
func DocOpV1AdminSystemOutboxGet() {}

// DocOpV1AdminSystemOutboxReplayPost godoc
// @Summary Replay a dead-lettered outbox row
// @Description **platform_admin** only. Clears quarantine fields when the row is dead-lettered so `cmd/worker` can publish again. Audited when `organization_id` is present on the row.
// @Tags Platform Ops
// @Security BearerAuth
// @Produce json
// @Param eventId path int true "Outbox row id"
// @Success 200 {object} V1AdminOutboxRetryEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/system/outbox/{eventId}/replay [post]
func DocOpV1AdminSystemOutboxReplayPost() {}

// DocOpV1AdminSystemOutboxMarkDLQPost godoc
// @Summary Manually move an outbox row to Postgres DLQ
// @Description **platform_admin** only. Sets terminal dead-letter state for unpublished rows when automation cannot safely drain them. Optional JSON body with a note field for diagnostics. Audited when organization_id is present on the row.
// @Tags Platform Ops
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param eventId path int true "Outbox row id"
// @Param body body object false "Optional operator note payload"
// @Success 200 {object} V1AdminOutboxMarkDLQEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/system/outbox/{eventId}/mark-dlq [post]
func DocOpV1AdminSystemOutboxMarkDLQPost() {}

// DocOpV1AdminSystemRetentionStatsGet godoc
// @Summary Data retention policy + table footprints (system)
// @Description **platform_admin** only. Returns enterprise retention target row counts, oldest timestamps, configured horizons (days), and runtime worker flags. Read-only.
// @Tags Platform Ops
// @Security BearerAuth
// @Produce json
// @Success 200 {object} V1AdminSystemRetentionStatsEnvelope
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/system/retention/stats [get]
func DocOpV1AdminSystemRetentionStatsGet() {}

// DocOpV1AdminSystemRetentionDryRunPost godoc
// @Summary Preview retention candidates (dry-run)
// @Description **platform_admin** only. Computes candidate row counts for telemetry and enterprise retention without issuing deletes. Audited as **retention.dry_run** when audit attribution resolves (`organization_id` query, JWT org scope, or oldest organizations row).
// @Tags Platform Ops
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Optional audit attribution organization id"
// @Success 200 {object} V1AdminSystemRetentionRunEnvelope
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/system/retention/dry-run [post]
func DocOpV1AdminSystemRetentionDryRunPost() {}

// DocOpV1AdminSystemRetentionRunPost godoc
// @Summary Run bounded Postgres retention
// @Description **platform_admin** only. Runs telemetry + enterprise retention when each subsystem cleanup is enabled. Blocked in development/test unless **RETENTION_ALLOW_DESTRUCTIVE_LOCAL=true** (HTTP 403). Respects **RETENTION_DRY_RUN** as a global dry-run override. Audited as **retention.run**.
// @Tags Platform Ops
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Optional audit attribution organization id"
// @Success 200 {object} V1AdminSystemRetentionRunEnvelope
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/system/retention/run [post]
func DocOpV1AdminSystemRetentionRunPost() {}

// --- Admin catalog (read-only) ---

// DocOpV1AdminProductsList godoc
// @Summary List products (admin catalog)
// @Description Paginated product directory for an organization. **platform_admin** must pass **organization_id** query; **org_admin** is scoped to JWT organization. Supports `q` substring search on sku/name, `active_only` boolean, and standard **limit**/**offset** pagination (default 50, max 500).
// @Tags Catalog Admin
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
// @Tags Catalog Admin
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

// DocOpV1AdminProductCreate godoc
// @Summary Create product (admin catalog)
// @Description Creates a product in the organization. **Idempotency-Key** required. SKU unique per org; barcode unique when set.
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "sku, name, description, active, optional categoryId, brandId, barcode"
// @Success 200 {object} V1AdminProduct
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/products [post]
func DocOpV1AdminProductCreate() {}

// DocOpV1AdminProductReplace godoc
// @Summary Update product (PUT/PATCH)
// @Description Full replacement of mutable fields. **Idempotency-Key** required.
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param productId path string true "Product UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "sku, name, description, active, optional categoryId, brandId, barcode"
// @Success 200 {object} V1AdminProduct
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/products/{productId} [put]
func DocOpV1AdminProductReplace() {}

// DocOpV1AdminProductPatch godoc
// @Summary Update product (PATCH)
// @Description Same payload as PUT. **Idempotency-Key** required.
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param productId path string true "Product UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "sku, name, description, active, optional categoryId, brandId, barcode"
// @Success 200 {object} V1AdminProduct
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/products/{productId} [patch]
func DocOpV1AdminProductPatch() {}

// DocOpV1AdminProductDelete godoc
// @Summary Deactivate product
// @Description Sets **active=false** (never hard-deletes). **Idempotency-Key** required.
// @Tags Catalog Admin
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
// @Router /v1/admin/products/{productId} [delete]
func DocOpV1AdminProductDelete() {}

// DocOpV1AdminProductImagePut godoc
// @Summary Bind primary product image
// @Description Binds the artifact-backed primary sale-catalog image. When API_ARTIFACTS_ENABLED wires object storage, bytes are copied to deterministic keys **org/{orgId}/products/{productId}/display.webp** and **thumb.webp** and URLs are short-lived presigned GET URLs (clients may omit displayUrl/thumbUrl). Legacy mode requires HTTPS display/thumb URLs. **Idempotency-Key** required.
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param productId path string true "Product UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "artifactId, thumbUrl, displayUrl, optional contentHash, width, height, mimeType"
// @Success 200 {object} V1AdminProduct
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/products/{productId}/image [put]
func DocOpV1AdminProductImagePut() {}

// DocOpV1AdminProductImagePost godoc
// @Summary Bind primary product image (alias)
// @Description Same semantics as PUT **/image**. Prefer POST for create/bind semantics; PUT retained for backward compatibility.
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param productId path string true "Product UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "artifactId, thumbUrl, displayUrl, optional contentHash, width, height, mimeType"
// @Success 200 {object} V1AdminProduct
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/products/{productId}/image [post]
func DocOpV1AdminProductImagePost() {}

// DocOpV1AdminProductImageDelete godoc
// @Summary Remove primary product image
// @Description Clears **primary_image_id** and deletes the primary image row. **Idempotency-Key** required.
// @Tags Catalog Admin
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
// @Router /v1/admin/products/{productId}/image [delete]
func DocOpV1AdminProductImageDelete() {}

// DocOpV1AdminMediaUploadInit godoc
// @Summary Start enterprise media upload (presigned PUT)
// @Description Creates a **pending** media_assets row and returns a presigned **PUT** URL for the **original** image. After upload, call **POST /v1/admin/media/{mediaId}/complete** to generate thumb/display variants and mark **ready**. Requires **API_ARTIFACTS_ENABLED** (object storage). **Idempotency-Key** required.
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body V1AdminMediaUploadInitRequest true "content_type must be image/*"
// @Success 200 {object} V1AdminMediaUploadInitResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 503 {object} V1StandardError "capability_not_configured when media service not wired"
// @Router /v1/admin/media/uploads [post]
func DocOpV1AdminMediaUploadInit() {}

// DocOpV1AdminMediaAssetsCreate godoc
// @Summary Start enterprise media asset upload
// @Description Alias for **POST /v1/admin/media/uploads**. Creates a pending **media_assets** row and returns a presigned PUT URL for the original image. No image bytes are stored in PostgreSQL.
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body V1AdminMediaUploadInitRequest true "content_type must be image/*"
// @Success 200 {object} V1AdminMediaUploadInitResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 503 {object} V1StandardError "capability_not_configured when media service not wired"
// @Router /v1/admin/media/assets [post]
func DocOpV1AdminMediaAssetsCreate() {}

// DocOpV1AdminMediaUploadComplete godoc
// @Summary Finalize media upload (variants + ready)
// @Description Heads the original object, generates **thumb.webp** / **display.webp** variants, records integrity metadata, marks asset **ready**. **Idempotency-Key** required.
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param mediaId path string true "Media asset UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminMediaAsset
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Router /v1/admin/media/{mediaId}/complete [post]
func DocOpV1AdminMediaUploadComplete() {}

// DocOpV1AdminOrganizationMediaUploadInit godoc
// @Summary Start organization-scoped product media upload
// @Description Creates a pending media asset and returns a presigned PUT URL for the original image. No image bytes are stored in PostgreSQL.
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param body body V1AdminMediaUploadInitRequest true "content_type must be image/jpeg, image/png, or image/webp"
// @Success 200 {object} V1AdminMediaUploadInitResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 503 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/media/uploads/init [post]
func DocOpV1AdminOrganizationMediaUploadInit() {}

// DocOpV1AdminOrganizationMediaUploadComplete godoc
// @Summary Complete organization-scoped product media upload
// @Description Validates original object size/MIME, generates deterministic display/thumb objects through the media pipeline, and marks the asset ready or failed.
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param body body V1AdminMediaUploadCompleteRequest true "media_id"
// @Success 200 {object} V1AdminMediaAsset
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/media/uploads/complete [post]
func DocOpV1AdminOrganizationMediaUploadComplete() {}

// DocOpV1AdminMediaList godoc
// @Summary List media assets for the tenant
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminMediaListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/media [get]
func DocOpV1AdminMediaList() {}

// DocOpV1AdminMediaAssetsList godoc
// @Summary List media assets for the tenant
// @Description Alias for **GET /v1/admin/media**.
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminMediaListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/media/assets [get]
func DocOpV1AdminMediaAssetsList() {}

// DocOpV1AdminMediaGet godoc
// @Summary Get one media asset
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param mediaId path string true "Media asset UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminMediaAsset
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/media/{mediaId} [get]
func DocOpV1AdminMediaGet() {}

// DocOpV1AdminMediaAssetsGet godoc
// @Summary Get one media asset
// @Description Alias for **GET /v1/admin/media/{mediaId}**.
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param mediaId path string true "Media asset UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminMediaAsset
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/media/assets/{mediaId} [get]
func DocOpV1AdminMediaAssetsGet() {}

// DocOpV1AdminMediaDelete godoc
// @Summary Soft-delete media and unbind from products
// @Description Sets asset status **deleted**, removes **product_images** bindings, best-effort deletes objects. **Idempotency-Key** required.
// @Tags Catalog Admin
// @Security BearerAuth
// @Param mediaId path string true "Media asset UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 204 {string} string "No Content"
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/media/{mediaId} [delete]
func DocOpV1AdminMediaDelete() {}

// DocOpV1AdminMediaAssetsDelete godoc
// @Summary Soft-delete media and unbind from products
// @Description Alias for **DELETE /v1/admin/media/{mediaId}**.
// @Tags Catalog Admin
// @Security BearerAuth
// @Param mediaId path string true "Media asset UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 204 {string} string "No Content"
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/media/assets/{mediaId} [delete]
func DocOpV1AdminMediaAssetsDelete() {}

// DocOpV1AdminProductMediaPost godoc
// @Summary Bind primary product image from media pipeline
// @Description Replaces primary image with a **ready** media_assets row (presigned thumb/display URLs). When object storage is disabled → **503** `capability_not_configured`. **Idempotency-Key** required.
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param productId path string true "Product UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body V1AdminProductMediaBindRequest true "media_id"
// @Success 200 {object} V1AdminProduct
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 503 {object} V1StandardError
// @Router /v1/admin/products/{productId}/media [post]
func DocOpV1AdminProductMediaPost() {}

// DocOpV1AdminProductMediaPut godoc
// @Summary Bind primary product image (alias)
// @Description Same as POST **/media**. **Idempotency-Key** required.
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param productId path string true "Product UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body V1AdminProductMediaBindRequest true "media_id"
// @Success 200 {object} V1AdminProduct
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 503 {object} V1StandardError
// @Router /v1/admin/products/{productId}/media [put]
func DocOpV1AdminProductMediaPut() {}

// DocOpV1AdminProductMediaDelete godoc
// @Summary Unbind product image for a media asset
// @Description Removes the **product_images** row for this **media_id** when bound to the product. **Idempotency-Key** required.
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param productId path string true "Product UUID"
// @Param mediaId path string true "Media asset UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminProduct
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 503 {object} V1StandardError
// @Router /v1/admin/products/{productId}/media/{mediaId} [delete]
func DocOpV1AdminProductMediaDelete() {}

// DocOpV1AdminOrganizationProductImagesPost godoc
// @Summary Attach product image from uploaded media
// @Description Binds a ready media asset as the product's active primary image and returns the updated product.
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param productId path string true "Product UUID"
// @Param body body V1AdminProductMediaBindRequest true "media_id"
// @Success 200 {object} V1AdminProduct
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 503 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/products/{productId}/images [post]
func DocOpV1AdminOrganizationProductImagesPost() {}

// DocOpV1AdminOrganizationMediaProductImagesPost godoc
// @Summary Init product image upload (org-scoped)
// @Description Same contract as **POST /v1/admin/media/uploads** with explicit **organizationId** path scope.
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param body body V1AdminMediaUploadInitRequest true "content_type must be image/*"
// @Success 200 {object} V1AdminMediaUploadInitResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/media/product-images [post]
func DocOpV1AdminOrganizationMediaProductImagesPost() {}

// DocOpV1AdminOrganizationMediaAssetsList godoc
// @Summary List media assets (org-scoped)
// @Description Same response as **GET /v1/admin/media/assets** with explicit **organizationId** in the path.
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Offset (default 0)"
// @Success 200 {object} V1AdminMediaListEnvelope
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/media/assets [get]
func DocOpV1AdminOrganizationMediaAssetsList() {}

// DocOpV1AdminOrganizationMediaAssetsGet godoc
// @Summary Get one media asset (org-scoped)
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param assetId path string true "Media asset UUID"
// @Success 200 {object} V1AdminMediaAsset
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/media/assets/{assetId} [get]
func DocOpV1AdminOrganizationMediaAssetsGet() {}

// DocOpV1AdminOrganizationMediaAssetsDelete godoc
// @Summary Soft-delete media asset (org-scoped)
// @Tags Catalog Admin
// @Security BearerAuth
// @Param organizationId path string true "Organization UUID"
// @Param assetId path string true "Media asset UUID"
// @Success 204 {string} string "No Content"
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/media/assets/{assetId} [delete]
func DocOpV1AdminOrganizationMediaAssetsDelete() {}

// DocOpV1AdminOrganizationProductMediaPost godoc
// @Summary Bind primary product image (org-scoped path)
// @Description Same as **POST /v1/admin/products/{productId}/media** with explicit organization scope.
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param productId path string true "Product UUID"
// @Param body body V1AdminProductMediaBindRequest true "media_id"
// @Success 200 {object} V1AdminProduct
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 503 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/products/{productId}/media [post]
func DocOpV1AdminOrganizationProductMediaPost() {}

// DocOpV1AdminOrganizationProductMediaDelete godoc
// @Summary Unbind product media (org-scoped path)
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param productId path string true "Product UUID"
// @Param mediaId path string true "Media asset UUID"
// @Success 200 {object} V1AdminProduct
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 503 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/products/{productId}/media/{mediaId} [delete]
func DocOpV1AdminOrganizationProductMediaDelete() {}

// DocOpV1AdminOrganizationProductImagesList godoc
// @Summary List product images
// @Description Lists active product images. Archived images are excluded unless include_archived=true.
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param productId path string true "Product UUID"
// @Param include_archived query bool false "Include archived images"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/products/{productId}/images [get]
func DocOpV1AdminOrganizationProductImagesList() {}

// DocOpV1AdminOrganizationProductImagesPatch godoc
// @Summary Update product image metadata
// @Description Updates sort order, primary flag, or alt text and increments media_version for app cache comparison.
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param productId path string true "Product UUID"
// @Param imageId path string true "Product image UUID"
// @Param body body V1AdminProductImagePatchRequest true "metadata patch"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/products/{productId}/images/{imageId} [patch]
func DocOpV1AdminOrganizationProductImagesPatch() {}

// DocOpV1AdminOrganizationProductImagesDelete godoc
// @Summary Archive product image
// @Description Archives the image so it no longer appears in active admin lists or runtime catalogs.
// @Tags Catalog Admin
// @Security BearerAuth
// @Param organizationId path string true "Organization UUID"
// @Param productId path string true "Product UUID"
// @Param imageId path string true "Product image UUID"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/products/{productId}/images/{imageId} [delete]
func DocOpV1AdminOrganizationProductImagesDelete() {}

// DocOpV1AdminBrandsList godoc
// @Summary List brands
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/brands [get]
func DocOpV1AdminBrandsList() {}

// DocOpV1AdminBrandCreate godoc
// @Summary Create brand
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "slug, name, active"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/brands [post]
func DocOpV1AdminBrandCreate() {}

// DocOpV1AdminBrandReplace godoc
// @Summary Update brand
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param brandId path string true "Brand UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "slug, name, active"
// @Success 200 {object} object
// @Router /v1/admin/brands/{brandId} [put]
func DocOpV1AdminBrandReplace() {}

// DocOpV1AdminBrandPatch godoc
// @Summary Update brand (PATCH)
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param brandId path string true "Brand UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "slug, name, active"
// @Success 200 {object} object
// @Router /v1/admin/brands/{brandId} [patch]
func DocOpV1AdminBrandPatch() {}

// DocOpV1AdminBrandDelete godoc
// @Summary Deactivate brand
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param brandId path string true "Brand UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} object
// @Router /v1/admin/brands/{brandId} [delete]
func DocOpV1AdminBrandDelete() {}

// DocOpV1AdminCategoriesList godoc
// @Summary List categories
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} object
// @Router /v1/admin/categories [get]
func DocOpV1AdminCategoriesList() {}

// DocOpV1AdminCategoryCreate godoc
// @Summary Create category
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "slug, name, optional parentId, active"
// @Success 200 {object} object
// @Router /v1/admin/categories [post]
func DocOpV1AdminCategoryCreate() {}

// DocOpV1AdminCategoryReplace godoc
// @Summary Update category
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param categoryId path string true "Category UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "slug, name, optional parentId, active"
// @Success 200 {object} object
// @Router /v1/admin/categories/{categoryId} [put]
func DocOpV1AdminCategoryReplace() {}

// DocOpV1AdminCategoryPatch godoc
// @Summary Update category (PATCH)
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param categoryId path string true "Category UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "slug, name, optional parentId, active"
// @Success 200 {object} object
// @Router /v1/admin/categories/{categoryId} [patch]
func DocOpV1AdminCategoryPatch() {}

// DocOpV1AdminCategoryDelete godoc
// @Summary Deactivate category
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param categoryId path string true "Category UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} object
// @Router /v1/admin/categories/{categoryId} [delete]
func DocOpV1AdminCategoryDelete() {}

// DocOpV1AdminTagsList godoc
// @Summary List tags
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} object
// @Router /v1/admin/tags [get]
func DocOpV1AdminTagsList() {}

// DocOpV1AdminTagCreate godoc
// @Summary Create tag
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "slug, name, active"
// @Success 200 {object} object
// @Router /v1/admin/tags [post]
func DocOpV1AdminTagCreate() {}

// DocOpV1AdminTagReplace godoc
// @Summary Update tag
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param tagId path string true "Tag UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "slug, name, active"
// @Success 200 {object} object
// @Router /v1/admin/tags/{tagId} [put]
func DocOpV1AdminTagReplace() {}

// DocOpV1AdminTagPatch godoc
// @Summary Update tag (PATCH)
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param tagId path string true "Tag UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "slug, name, active"
// @Success 200 {object} object
// @Router /v1/admin/tags/{tagId} [patch]
func DocOpV1AdminTagPatch() {}

// DocOpV1AdminTagDelete godoc
// @Summary Deactivate tag
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param tagId path string true "Tag UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} object
// @Router /v1/admin/tags/{tagId} [delete]
func DocOpV1AdminTagDelete() {}

// DocOpV1AdminPriceBooksList godoc
// @Summary List price books (admin catalog)
// @Description Operational pricing tables for the organization (effective windows, default flag, scope). **platform_admin** requires **organization_id** query.
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Param include_inactive query bool false "When true, include deactivated books"
// @Success 200 {object} V1AdminPriceBookListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/price-books [get]
func DocOpV1AdminPriceBooksList() {}

// DocOpV1AdminPriceBookGet godoc
// @Summary Get price book by ID
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param priceBookId path string true "Price book UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminPriceBook
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/price-books/{priceBookId} [get]
func DocOpV1AdminPriceBookGet() {}

// DocOpV1AdminPriceBookCreate godoc
// @Summary Create price book
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "name, currency, effectiveFrom, scopeType, …"
// @Success 200 {object} V1AdminPriceBook
// @Router /v1/admin/price-books [post]
func DocOpV1AdminPriceBookCreate() {}

// DocOpV1AdminPriceBookPatch godoc
// @Summary Patch price book
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param priceBookId path string true "Price book UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "Partial fields"
// @Success 200 {object} V1AdminPriceBook
// @Router /v1/admin/price-books/{priceBookId} [patch]
func DocOpV1AdminPriceBookPatch() {}

// DocOpV1AdminPriceBookDeactivate godoc
// @Summary Deactivate price book
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param priceBookId path string true "Price book UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminPriceBook
// @Router /v1/admin/price-books/{priceBookId}/deactivate [post]
func DocOpV1AdminPriceBookDeactivate() {}

// DocOpV1AdminPriceBookActivate godoc
// @Summary Activate price book
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param Idempotency-Key header string true "Write idempotency key"
// @Param priceBookId path string true "Price book UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminPriceBook
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/price-books/{priceBookId}/activate [post]
func DocOpV1AdminPriceBookActivate() {}

// DocOpV1AdminPriceBookArchive godoc
// @Summary Archive price book (deactivate)
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param Idempotency-Key header string true "Write idempotency key"
// @Param priceBookId path string true "Price book UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminPriceBook
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/price-books/{priceBookId}/archive [post]
func DocOpV1AdminPriceBookArchive() {}

// DocOpV1AdminPriceBookItemsGet godoc
// @Summary List price book items
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param priceBookId path string true "Price book UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} object
// @Router /v1/admin/price-books/{priceBookId}/items [get]
func DocOpV1AdminPriceBookItemsGet() {}

// DocOpV1AdminPriceBookItemsPut godoc
// @Summary Replace price book items
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param priceBookId path string true "Price book UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "{\"items\":[{\"productId\":\"…\",\"unitPriceMinor\":100}]}"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/price-books/{priceBookId}/items [put]
func DocOpV1AdminPriceBookItemsPut() {}

// DocOpV1AdminPriceBookItemPatch godoc
// @Summary Upsert one price book item
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param priceBookId path string true "Price book UUID"
// @Param productId path string true "Product UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "{\"unitPriceMinor\":100}"
// @Success 200 {object} object
// @Router /v1/admin/price-books/{priceBookId}/items/{productId} [patch]
func DocOpV1AdminPriceBookItemPatch() {}

// DocOpV1AdminPriceBookItemDelete godoc
// @Summary Delete price book item
// @Tags Catalog Admin
// @Security BearerAuth
// @Param priceBookId path string true "Price book UUID"
// @Param productId path string true "Product UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/price-books/{priceBookId}/items/{productId} [delete]
func DocOpV1AdminPriceBookItemDelete() {}

// DocOpV1AdminPriceBookAssignTarget godoc
// @Summary Assign organization price book to machine or site
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param priceBookId path string true "Price book UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "Exactly one of siteId or machineId"
// @Success 200 {object} object
// @Router /v1/admin/price-books/{priceBookId}/assign-target [post]
func DocOpV1AdminPriceBookAssignTarget() {}

// DocOpV1AdminPriceBookTargetDelete godoc
// @Summary Remove price book target assignment
// @Tags Catalog Admin
// @Security BearerAuth
// @Param priceBookId path string true "Price book UUID"
// @Param targetId path string true "Target UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 204 {string} string "No Content"
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/price-books/{priceBookId}/targets/{targetId} [delete]
func DocOpV1AdminPriceBookTargetDelete() {}

// DocOpV1AdminPromotionsList godoc
// @Summary List promotions (admin catalog)
// @Description Tenant-scoped promotions with pagination. **platform_admin** requires **organization_id** query. Use **include_deactivated** to list deactivated rows.
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Param include_deactivated query bool false "When true, include lifecycle deactivated"
// @Success 200 {object} V1AdminPromotionListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/promotions [get]
func DocOpV1AdminPromotionsList() {}

// DocOpV1AdminPromotionGet godoc
// @Summary Get promotion detail with rules and targets
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param promotionId path string true "Promotion UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminPromotionDetail
// @Failure 400 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/promotions/{promotionId} [get]
func DocOpV1AdminPromotionGet() {}

// DocOpV1AdminPromotionCreate godoc
// @Summary Create promotion (draft lifecycle)
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "name, startsAt, endsAt, priority, stackable, optional rules"
// @Success 200 {object} V1AdminPromotion
// @Failure 400 {object} V1StandardError
// @Router /v1/admin/promotions [post]
func DocOpV1AdminPromotionCreate() {}

// DocOpV1AdminPromotionPatch godoc
// @Summary Patch promotion fields or replace rules
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param promotionId path string true "Promotion UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "Partial fields"
// @Success 200 {object} V1AdminPromotion
// @Router /v1/admin/promotions/{promotionId} [patch]
func DocOpV1AdminPromotionPatch() {}

// DocOpV1AdminPromotionActivate godoc
// @Summary Activate promotion (lifecycle active)
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param promotionId path string true "Promotion UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminPromotion
// @Router /v1/admin/promotions/{promotionId}/activate [post]
func DocOpV1AdminPromotionActivate() {}

// DocOpV1AdminPromotionPause godoc
// @Summary Pause promotion
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param promotionId path string true "Promotion UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminPromotion
// @Router /v1/admin/promotions/{promotionId}/pause [post]
func DocOpV1AdminPromotionPause() {}

// DocOpV1AdminPromotionDeactivate godoc
// @Summary Deactivate promotion
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param promotionId path string true "Promotion UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminPromotion
// @Router /v1/admin/promotions/{promotionId}/deactivate [post]
func DocOpV1AdminPromotionDeactivate() {}

// DocOpV1AdminPromotionArchive godoc
// @Summary Archive promotion (deactivate with audit trail)
// @Tags Catalog Admin
// @Security BearerAuth
// @Produce json
// @Param promotionId path string true "Promotion UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminPromotion
// @Failure 400 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/promotions/{promotionId}/archive [post]
func DocOpV1AdminPromotionArchive() {}

// DocOpV1AdminPromotionAssignTarget godoc
// @Summary Assign a promotion target (organization, site, machine, product, category, tag)
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param promotionId path string true "Promotion UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "targetType plus matching id field"
// @Success 200 {object} V1AdminPromotionTarget
// @Router /v1/admin/promotions/{promotionId}/assign-target [post]
func DocOpV1AdminPromotionAssignTarget() {}

// DocOpV1AdminPromotionTargetDelete godoc
// @Summary Remove a promotion target assignment
// @Tags Catalog Admin
// @Security BearerAuth
// @Param promotionId path string true "Promotion UUID"
// @Param targetId path string true "Target row UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 204 {string} string "No Content"
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/promotions/{promotionId}/targets/{targetId} [delete]
func DocOpV1AdminPromotionTargetDelete() {}

// DocOpV1AdminPromotionsPreview godoc
// @Summary Preview promotion discounts on top of catalog pricing
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "productIds required; optional machineId, siteId, at"
// @Success 200 {object} V1AdminPromotionPreviewResponse
// @Router /v1/admin/promotions/preview [post]
func DocOpV1AdminPromotionsPreview() {}

// DocOpV1AdminPricingPreview godoc
// @Summary Preview effective prices for products
// @Tags Catalog Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "productIds required; optional machineId, siteId, at"
// @Success 200 {object} V1AdminPricingPreviewResponse
// @Router /v1/admin/pricing/preview [post]
func DocOpV1AdminPricingPreview() {}

// DocOpV1AdminPlanogramsList godoc
// @Summary List planograms (admin catalog)
// @Description Planogram revisions for slot layouts (draft/published/archived). **platform_admin** requires **organization_id** query.
// @Tags Catalog Admin
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
// @Tags Catalog Admin
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

// DocOpV1AdminMachineInventoryEvents godoc
// @Summary List append-only inventory ledger events for a machine
// @Description Returns **inventory_events** with **quantityBefore**, **quantityDelta**, **quantityAfter**, **reasonCode**, **cabinetCode**, **slotCode**, **operatorSessionId**, **technician** attribution, prices (**unitPriceMinor** + **currency**), and **occurredAt** / **recordedAt** as RFC3339Nano strings with explicit timezone offset (examples use UTC **Z**). Optional **from** / **to** filter `occurredAt` (inclusive bounds). Same auth scoping as other machine inventory routes.
// @Tags Inventory
// @Security BearerAuth
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param from query string false "Lower bound on occurredAt, inclusive (RFC3339Nano with explicit timezone offset)"
// @Param to query string false "Upper bound on occurredAt, inclusive (RFC3339Nano with explicit timezone offset)"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Offset"
// @Success 200 {object} V1AdminInventoryEventListEnvelope
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/inventory-events [get]
func DocOpV1AdminMachineInventoryEvents() {}

// DocOpV1AdminMachineSlots godoc
// @Summary List live slot inventory for a machine (restock / cycle-count UI)
// @Description Merges `machine_slot_state` (legacy planogram slots) with current `machine_slot_configs` for **cabinetCode** / **cabinetIndex** / **slotCode**. Machines without cabinet rows get **cabinetCode** **CAB-A** (see `inventory_admin` coalesce). Derives **capacity**, **parLevel**, **lowStockThreshold**, **status** (`ok` | `low_stock` | `out_of_stock`), and resolves **currency** from the organization's default price book (falls back to **USD**). **platform_admin** must pass **organization_id** for tenant pick.
// @Tags Inventory
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

// DocOpV1AdminMachineStockAdjustmentsPost godoc
// @Summary Apply stock adjustments (restock, cycle count, manual, reconcile)
// @Description Validates **quantityBefore** against `machine_slot_state`, appends **inventory_events** with **reasonCode**, **cabinetCode**, **slotCode**, **quantityBefore** / **quantityDelta** / **quantityAfter**, **unitPriceMinor**, **currency**, **operatorSessionId**, **technicianId** (from session), **occurredAt** / **recordedAt**, then updates `machine_slot_state` in the same transaction. Optional **occurredAt** on the body backdates the business time (defaults to now). Requires **operator_session_id** for an **ACTIVE** session and **Idempotency-Key** (replay returns **replay=true** without double-applying inventory). Reasons: **restock**, **cycle_count**, **manual_adjustment**, **machine_reconcile**.
// @Tags Inventory
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body V1AdminStockAdjustmentsRequest true "operator_session_id, reason, items[]"
// @Success 200 {object} V1AdminStockAdjustmentsResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError "quantity_before_mismatch"
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/stock-adjustments [post]
func DocOpV1AdminMachineStockAdjustmentsPost() {}

// DocOpV1AdminMachineInventory godoc
// @Summary Aggregate inventory by product for a machine
// @Description Rolls up slot quantities per product for refill planning (totals, slot coverage, low-stock flag). **cabinetCode** / **cabinetIndex** appear only when all slots for that SKU map to a single cabinet; omitted when stock spans multiple cabinets. Same scoping rules as slot list.
// @Tags Inventory
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

// DocOpV1AdminInventoryLowStock godoc
// @Summary List slots estimated to need refill soon (low stock)
// @Description Uses live slot inventory and successful **vend_sessions** velocity over **velocity_days** (default 14, clamped 7–90). Estimates **daysToEmpty**, **suggestedRefillQuantity** (up to slot capacity), and **urgency** (**critical** \| **high** \| **medium** \| **low**). Slots with no sales in the window omit **daysToEmpty** but still expose velocity **0** (no divide-by-zero). Restricts to empty slots or fill below 15%. **platform_admin** must pass **organization_id**.
// @Tags Inventory
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param site_id query string false "Filter by site UUID"
// @Param machine_id query string false "Filter by machine UUID"
// @Param product_id query string false "Filter by product UUID"
// @Param velocity_days query int false "Sales lookback window in days (default 14, min 7, max 90)"
// @Param urgency query string false "Filter by urgency: critical, high, medium, low"
// @Param days_threshold query number false "Include rows at or below this estimated days-to-empty (also keeps empty/low-fill)"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Offset"
// @Success 200 {object} V1AdminInventoryRefillForecastResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/inventory/low-stock [get]
func DocOpV1AdminInventoryLowStock() {}

// DocOpV1AdminInventoryRefillSuggestions godoc
// @Summary List refill suggestions across machines (all slots)
// @Description Same forecasting as low-stock, but includes every slot with an assigned product (not only classic low-stock rows). Filter by **site_id**, **machine_id**, **product_id**, **urgency**, **days_threshold**. Pagination applies after sorting by urgency then estimated runway.
// @Tags Inventory
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param site_id query string false "Filter by site UUID"
// @Param machine_id query string false "Filter by machine UUID"
// @Param product_id query string false "Filter by product UUID"
// @Param velocity_days query int false "Sales lookback window in days (default 14, min 7, max 90)"
// @Param urgency query string false "Filter by urgency: critical, high, medium, low"
// @Param days_threshold query number false "Include rows at or below this estimated days-to-empty"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Offset"
// @Success 200 {object} V1AdminInventoryRefillForecastResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/inventory/refill-suggestions [get]
func DocOpV1AdminInventoryRefillSuggestions() {}

// DocOpV1AdminMachineRefillSuggestions godoc
// @Summary Refill suggestions for one machine
// @Description Same projection as **GET /v1/admin/inventory/refill-suggestions** scoped to **machineId** (query **machine_id** must match the path when provided). **organization_id** must match the machine tenant for platform administrators.
// @Tags Inventory
// @Security BearerAuth
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param site_id query string false "Optional filter (must match machine site when set)"
// @Param machine_id query string false "Optional; must equal path machineId when set"
// @Param product_id query string false "Filter by product UUID"
// @Param velocity_days query int false "Sales lookback window in days (default 14, min 7, max 90)"
// @Param urgency query string false "Filter by urgency: critical, high, medium, low"
// @Param days_threshold query number false "Include rows at or below this estimated days-to-empty"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Offset"
// @Success 200 {object} V1AdminInventoryRefillForecastResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/refill-suggestions [get]
func DocOpV1AdminMachineRefillSuggestions() {}

// DocOpV1AdminMachineCashbox godoc
// @Summary Cashbox summary (expected vault from commerce)
// @Description Returns **expectedCashboxMinor** / **expectedRecyclerMinor** (recycler is **0** until recycler telemetry is integrated), optional **denominations** hints, **lastCollectionAt**, and **openCollectionId** when a session is open. **Accounting-only** — does not command bill recycler hardware. **platform_admin** passes **organization_id** query for tenant pick.
// @Tags Cash settlement
// @Security BearerAuth
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param currency query string false "ISO 4217 minor currency (default USD)"
// @Success 200 {object} V1AdminMachineCashboxResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/cashbox [get]
func DocOpV1AdminMachineCashbox() {}

// DocOpV1AdminMachineCashCollectionsPost godoc
// @Summary Start cash collection session
// @Description Opens a **cash_collections** row (**lifecycle_status=open**). Requires **ACTIVE** **operator_session_id**, optional **startedAt** (RFC3339), and **Idempotency-Key** (replay returns the same open row). At most one open collection per machine. **Accounting-only** — no hardware payout.
// @Tags Cash settlement
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "operator_session_id, currency, notes"
// @Success 200 {object} V1AdminCashCollection
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError "open_collection_exists"
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/cash-collections [post]
func DocOpV1AdminMachineCashCollectionsPost() {}

// DocOpV1AdminMachineCashCollectionsList godoc
// @Summary List cash collections for machine
// @Description Recent **cash_collections** rows (open and closed), newest first. Tenant-scoped like other admin machine routes.
// @Tags Cash settlement
// @Security BearerAuth
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Offset"
// @Success 200 {object} V1AdminCashCollectionListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/cash-collections [get]
func DocOpV1AdminMachineCashCollectionsList() {}

// DocOpV1AdminMachineCashCollectionGet godoc
// @Summary Get one cash collection
// @Description Returns a single **cash_collections** row when it belongs to the machine and organization.
// @Tags Cash settlement
// @Security BearerAuth
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Param collectionId path string true "Collection UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} V1AdminCashCollection
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/cash-collections/{collectionId} [get]
func DocOpV1AdminMachineCashCollectionGet() {}

// DocOpV1AdminMachineCashCollectionClosePost godoc
// @Summary Close cash collection with counted cash
// @Description Computes **expected** vault total from commerce (**cash** payments minus **refunds**) since the previous close, records **variance**, sets **requires_review** when abs(variance) exceeds configured threshold. **P1 payload**: **countedCashboxMinor** + **countedRecyclerMinor** (both required together), optional **denominations**, **closedAt**, **evidence.photoArtifactId**. Legacy **counted_amount_minor** remains supported. **Idempotent** on canonical close payload hash; conflicting re-close returns **409**. Requires **ACTIVE** **operator_session_id**. **Accounting-only** — no hardware payout.
// @Tags Cash settlement
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Param collectionId path string true "Collection UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "operator_session_id, currency, counted_amount_minor (legacy) OR countedCashboxMinor+countedRecyclerMinor, optional denominations, closedAt, evidence"
// @Success 200 {object} V1AdminCashCollection
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError "close_payload_conflict"
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/cash-collections/{collectionId}/close [post]
func DocOpV1AdminMachineCashCollectionClosePost() {}

// DocOpV1AdminFeatureFlagsList godoc
// @Summary List organization feature flags
// @Description Paginates tenant-scoped **feature_flags**. Requires **fleet.read**. **platform_admin** must pass **organization_id**.
// @Tags Fleet
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Offset"
// @Success 200 {object} object "items[], meta{limit,offset,returned,total}"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/feature-flags [get]
func DocOpV1AdminFeatureFlagsList() {}

// DocOpV1AdminFeatureFlagsPost godoc
// @Summary Create a feature flag
// @Description Inserts a **feature_flag** row keyed by **flagKey** (unique per organization). Requires **fleet.write**.
// @Tags Fleet
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "flagKey, displayName, description, enabled, metadata"
// @Success 201 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/feature-flags [post]
func DocOpV1AdminFeatureFlagsPost() {}

// DocOpV1AdminFeatureFlagGet godoc
// @Summary Get feature flag and scoped targets
// @Description Loads **feature_flags** plus **feature_flag_targets** ordered by priority. Requires **fleet.read**.
// @Tags Fleet
// @Security BearerAuth
// @Produce json
// @Param flagId path string true "Feature flag UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} object "flag, targets[]"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/feature-flags/{flagId} [get]
func DocOpV1AdminFeatureFlagGet() {}

// DocOpV1AdminFeatureFlagPatch godoc
// @Summary Patch feature flag metadata / master enabled bit
// @Description Updates display fields and JSON **metadata**. Requires **fleet.write**.
// @Tags Fleet
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param flagId path string true "Feature flag UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object false "optional displayName, description, enabled, metadata"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/feature-flags/{flagId} [patch]
func DocOpV1AdminFeatureFlagPatch() {}

// DocOpV1AdminFeatureFlagEnablePost godoc
// @Summary Enable feature flag (master switch)
// @Description Sets **enabled=true**. Requires **fleet.write**.
// @Tags Fleet
// @Security BearerAuth
// @Produce json
// @Param flagId path string true "Feature flag UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/feature-flags/{flagId}/enable [post]
func DocOpV1AdminFeatureFlagEnablePost() {}

// DocOpV1AdminFeatureFlagDisablePost godoc
// @Summary Disable feature flag (master switch)
// @Description Sets **enabled=false**. Requires **fleet.write**.
// @Tags Fleet
// @Security BearerAuth
// @Produce json
// @Param flagId path string true "Feature flag UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/feature-flags/{flagId}/disable [post]
func DocOpV1AdminFeatureFlagDisablePost() {}

// DocOpV1AdminFeatureFlagTargetsPut godoc
// @Summary Replace scoped targets for a feature flag
// @Description Deletes existing targets and inserts the provided list (**organization**, **site**, **machine**, **hardware_profile**, **canary**). Highest **priority** wins when evaluating on device bootstrap. Requires **fleet.write**.
// @Tags Fleet
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param flagId path string true "Feature flag UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "targets[]"
// @Success 200 {object} object "targets[]"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/feature-flags/{flagId}/targets [put]
func DocOpV1AdminFeatureFlagTargetsPut() {}

// DocOpV1AdminMachineConfigRolloutsPost godoc
// @Summary Create machine config rollout (or rollback)
// @Description Creates **machine_config_rollouts** targeting **targetVersionId** or creates a **machine_config_versions** row inline (**versionLabel** + **configPayload**). When **rollbackFromRolloutId** is set, rolls back that rollout. Requires **fleet.write**.
// @Tags Fleet
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "scopeType, optional targetVersionId or versionLabel+configPayload, optional rollbackFromRolloutId"
// @Success 201 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machine-config/rollouts [post]
func DocOpV1AdminMachineConfigRolloutsPost() {}

// DocOpV1AdminMachineConfigRolloutsList godoc
// @Summary List machine config rollouts
// @Description Paginates staged rollouts for the tenant. Requires **fleet.read**.
// @Tags Fleet
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Offset"
// @Success 200 {object} object "items[], meta"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machine-config/rollouts [get]
func DocOpV1AdminMachineConfigRolloutsList() {}

// DocOpV1AdminMachineConfigRolloutGet godoc
// @Summary Get one machine config rollout
// @Description Returns rollout scope and lineage (**previousVersionId**). Requires **fleet.read**.
// @Tags Fleet
// @Security BearerAuth
// @Produce json
// @Param rolloutId path string true "Rollout UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machine-config/rollouts/{rolloutId} [get]
func DocOpV1AdminMachineConfigRolloutGet() {}

// DocOpV1SetupMachineBootstrap godoc
// @Summary Machine setup bootstrap (topology + catalog)
// @Description Returns machine identity, nested topology (**cabinets** with **slots** from current cabinet slot configs), and **catalog.products** from the machine's primary assortment binding. Optional **runtimeHints** exposes evaluated **featureFlags**, latest applied **machine_configs** revision (`appliedMachineConfigRevision`), and pending staged rollouts when feature-flag services are enabled server-side. Requires **machine read** access (`RequireMachineTenantAccess` — org scope, explicit machine allow-list, or platform admin).
// @Tags Machine Setup
// @Security BearerAuth
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Success 200 {object} V1SetupMachineBootstrapResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 503 {object} V1CapabilityNotConfiguredError
// @Failure 500 {object} V1StandardError
// @Router /v1/setup/machines/{machineId}/bootstrap [get]
func DocOpV1SetupMachineBootstrap() {}

// DocOpV1SetupActivationClaimPost godoc
// @Summary Claim an activation code (public pre-auth)
// @Description Exchanges a valid **activationCode** and **deviceFingerprint** for a **machine-scoped JWT**, MQTT hints, and **bootstrapUrl**. Invalid, expired, exhausted, or revoked codes return **400** `activation_invalid` without revealing whether a machine exists. Same code + fingerprint replay returns the same token shape safely; distinct fingerprints consume additional **max_uses** slots when configured, otherwise exhausted codes reject further distinct fingerprints.
// @Tags Activation
// @Accept json
// @Produce json
// @Param body body object true "activationCode, deviceFingerprint (androidId, serialNumber, manufacturer, model, packageName, versionName, versionCode)"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/setup/activation-codes/claim [post]
func DocOpV1SetupActivationClaimPost() {}

// DocOpV1AdminMachineTopologyPut godoc
// @Summary Upsert machine cabinet topology and slot layouts
// @Description Upserts **cabinets** then **layouts** (cabinet-scoped `machine_slot_layouts`). Body requires **operator_session_id** for an **ACTIVE** session on this machine. **platform_admin** must pass **organization_id** query for tenant pick.
// @Tags Machine Setup
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "operator_session_id, cabinets[], layouts[]"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError "session_not_active"
// @Failure 429 {object} V1StandardError "rate_limited when sensitive-write rate limit trips"
// @Failure 503 {object} V1CapabilityNotConfiguredError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/topology [put]
func DocOpV1AdminMachineTopologyPut() {}

// DocOpV1AdminMachinePlanogramDraftPut godoc
// @Summary Save draft cabinet slot planogram assignments
// @Description Writes **draft** `machine_slot_configs` rows (not current) and optionally syncs legacy `machine_slot_state` when **syncLegacyReadModel** is true. Requires **operator_session_id** for an **ACTIVE** session on this machine.
// @Tags Machine Setup
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "operator_session_id, planogramId, planogramRevision, syncLegacyReadModel, items[]"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 503 {object} V1CapabilityNotConfiguredError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/planograms/draft [put]
func DocOpV1AdminMachinePlanogramDraftPut() {}

// DocOpV1AdminMachinePlanogramPublishPost godoc
// @Summary Publish draft planogram as current and dispatch device command
// @Description Applies current slot configs, records a **machine_configs** snapshot (monotonic **desiredConfigVersion** / `config_revision`), updates shadow **desired_state**, and enqueues **machine_planogram_publish** on the MQTT command path. **Idempotency-Key** header is required (same semantics as command dispatch). **operator_session_id** must reference an **ACTIVE** session on this machine.
// @Tags Machine Setup
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "operator_session_id, planogramId, planogramRevision, syncLegacyReadModel, items[]"
// @Success 200 {object} V1AdminPlanogramPublishResponse
// @Failure 400 {object} V1StandardError "missing_idempotency_key / invalid_json"
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 503 {object} V1CapabilityNotConfiguredError "mqtt_command_dispatch or database"
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/planograms/publish [post]
func DocOpV1AdminMachinePlanogramPublishPost() {}

// DocOpV1AdminMachineSetupSyncPost godoc
// @Summary Queue a machine setup / inventory sync command
// @Description Dispatches **machine_setup_sync** with optional **reason** in the payload. **Idempotency-Key** is required. **operator_session_id** must reference an **ACTIVE** session on this machine.
// @Tags Machine Setup
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "operator_session_id, optional reason"
// @Success 200 {object} V1AdminMachineSyncResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError "idempotency_key_conflict"
// @Failure 429 {object} V1StandardError
// @Failure 503 {object} V1CapabilityNotConfiguredError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/sync [post]
func DocOpV1AdminMachineSetupSyncPost() {}

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
// @Param from query string true "RFC3339Nano with explicit timezone offset (validated; reserved for future time filtering)"
// @Param to query string true "RFC3339Nano with explicit timezone offset (validated; reserved for future time filtering)"
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

// DocOpV1AdminOrgReportsSales godoc
// @Summary Organization sales report
// @Description Admin Web sales report for one organization in **[from,to)**. Supports timezone-aware day buckets via **timezone** and **group_by=day|site|machine|payment_method|product|none**. Optional **site_id**, **machine_id**, **product_id** narrow facts. `format=csv` returns stable CSV and audits the export action. Requires **reports.read**.
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param timezone query string false "IANA timezone (default UTC)"
// @Param site_id query string false "Site UUID filter"
// @Param machine_id query string false "Machine UUID filter"
// @Param product_id query string false "Product UUID filter"
// @Param group_by query string false "day | site | machine | payment_method | product | none"
// @Param format query string false "csv for text/csv export"
// @Success 200 {object} V1AdminReportSalesResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/reports/sales [get]
func DocOpV1AdminOrgReportsSales() {}

// DocOpV1AdminOrgReportsPayments godoc
// @Summary Payment settlement report
// @Description Provider/date/status settlement rollup for one organization. Groups by timezone-aware business day, provider, payment state, settlement status, and reconciliation status. `format=csv` returns stable CSV and audits the export action. Requires **reports.read**.
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param timezone query string false "IANA timezone (default UTC)"
// @Param site_id query string false "Site UUID filter"
// @Param machine_id query string false "Machine UUID filter"
// @Param product_id query string false "Product UUID filter"
// @Param format query string false "csv for text/csv export"
// @Success 200 {object} V1AdminReportPaymentsResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/reports/payments [get]
func DocOpV1AdminOrgReportsPayments() {}

// DocOpV1AdminOrgReportsRefunds godoc
// @Summary Refund report
// @Description Paged refund rows for one organization in **[from,to)**. `format=csv` returns stable CSV and audits the export action. Requires **reports.read**.
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Param format query string false "csv for text/csv export"
// @Success 200 {object} V1AdminReportListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/reports/refunds [get]
func DocOpV1AdminOrgReportsRefunds() {}

// DocOpV1AdminOrgReportsCash godoc
// @Summary Cash collection report
// @Description Paged cash collection rows for one organization in **[from,to)** on collected_at. Optional **site_id** and **machine_id** filters. `format=csv` exports all matching rows and audits the export action. Requires **reports.read**.
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param site_id query string false "Site UUID filter"
// @Param machine_id query string false "Machine UUID filter"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Param format query string false "csv for text/csv export"
// @Success 200 {object} V1AdminReportListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/reports/cash [get]
func DocOpV1AdminOrgReportsCash() {}

// DocOpV1AdminOrgReportsInventoryLowStock godoc
// @Summary Inventory low-stock report
// @Description Paged current low-stock slot report for one organization. Requires **reports.read**.
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param from query string true "Validated RFC3339 window start"
// @Param to query string true "Validated RFC3339 window end"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminReportListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/reports/inventory-low-stock [get]
func DocOpV1AdminOrgReportsInventoryLowStock() {}

// DocOpV1AdminOrgReportsMachineHealth godoc
// @Summary Machine health and offline report
// @Description Paged machine health report for one organization. Machines are marked offline by terminal/problem status, missing last_seen_at, or last_seen_at older than 15 minutes before **to**. Requires **reports.read**.
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param from query string true "Validated RFC3339 window start"
// @Param to query string true "Validated RFC3339 window end"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminReportListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/reports/machine-health [get]
func DocOpV1AdminOrgReportsMachineHealth() {}

// DocOpV1AdminOrgReportsFailedVends godoc
// @Summary Failed vend report
// @Description Paged failed vend session rows for one organization in **[from,to)**. Requires **reports.read**.
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminReportListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/reports/failed-vends [get]
func DocOpV1AdminOrgReportsFailedVends() {}

// DocOpV1AdminOrgReportsReconciliationQueue godoc
// @Summary Reconciliation queue report
// @Description Paged open/reviewing commerce reconciliation cases for one organization in **[from,to)**. Requires **reports.read**.
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminReportListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/reports/reconciliation-queue [get]
func DocOpV1AdminOrgReportsReconciliationQueue() {}

// DocOpV1AdminOrgReportsVends godoc
// @Summary Organization vend lifecycle summary
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param site_id query string false "Site UUID filter"
// @Param machine_id query string false "Machine UUID filter"
// @Param product_id query string false "Product UUID filter"
// @Param limit query int false "Page size for failed vend drill-down"
// @Param offset query int false "Offset"
// @Param format query string false "csv"
// @Success 200 {object} V1AdminReportListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/reports/vends [get]
func DocOpV1AdminOrgReportsVends() {}

// DocOpV1AdminOrgReportsInventoryUnified godoc
// @Summary Inventory BI (low stock or movement ledger)
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param kind query string false "low_stock | movement (default low_stock)"
// @Param exception_kind query string false "When kind=low_stock: all | low_stock | out_of_stock"
// @Param site_id query string false "Site UUID filter"
// @Param machine_id query string false "Machine UUID filter"
// @Param product_id query string false "Product UUID filter"
// @Param limit query int false "Page size"
// @Param offset query int false "Offset"
// @Param format query string false "csv (movement/low-stock exports)"
// @Success 200 {object} V1AdminReportListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/reports/inventory [get]
func DocOpV1AdminOrgReportsInventoryUnified() {}

// DocOpV1AdminOrgReportsMachines godoc
// @Summary Machine uptime / last-seen report (alias naming)
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param site_id query string false "Site UUID filter"
// @Param machine_id query string false "Machine UUID filter"
// @Param product_id query string false "Product UUID filter"
// @Param limit query int false "Page size"
// @Param offset query int false "Offset"
// @Param format query string false "csv"
// @Success 200 {object} V1AdminReportListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/reports/machines [get]
func DocOpV1AdminOrgReportsMachines() {}

// DocOpV1AdminOrgReportsProducts godoc
// @Summary Product performance report
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param site_id query string false "Site UUID filter"
// @Param machine_id query string false "Machine UUID filter"
// @Param product_id query string false "Product UUID filter"
// @Param format query string false "csv"
// @Success 200 {object} V1AdminReportListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/reports/products [get]
func DocOpV1AdminOrgReportsProducts() {}

// DocOpV1AdminOrgReportsReconciliationBI godoc
// @Summary Reconciliation BI (open/closed summaries + scoped cases)
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param reconciliation_scope query string false "open | closed | all (default all)"
// @Param site_id query string false "Site UUID filter"
// @Param machine_id query string false "Machine UUID filter"
// @Param product_id query string false "Product UUID filter"
// @Param limit query int false "Page size"
// @Param offset query int false "Offset"
// @Param format query string false "csv"
// @Success 200 {object} V1AdminReportListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/reports/reconciliation [get]
func DocOpV1AdminOrgReportsReconciliationBI() {}

// DocOpV1AdminOrgReportsCommands godoc
// @Summary Machine command failure report (terminal attempts only)
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param site_id query string false "Site UUID filter"
// @Param machine_id query string false "Machine UUID filter"
// @Param limit query int false "Page size"
// @Param offset query int false "Offset"
// @Param format query string false "csv"
// @Success 200 {object} V1AdminReportListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/reports/commands [get]
func DocOpV1AdminOrgReportsCommands() {}

// DocOpV1AdminOrgReportsTechnicianFills godoc
// @Summary Technician and fill / restock inventory operations
// @Tags Reporting
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param site_id query string false "Site UUID filter"
// @Param machine_id query string false "Machine UUID filter"
// @Param product_id query string false "Product UUID filter"
// @Param limit query int false "Page size"
// @Param offset query int false "Offset"
// @Param format query string false "csv"
// @Success 200 {object} V1AdminReportListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/reports/fills [get]
func DocOpV1AdminOrgReportsTechnicianFills() {}

// DocOpV1AdminOrgReportsExport godoc
// @Summary Unified CSV export dispatcher
// @Tags Reporting
// @Security BearerAuth
// @Produce text/csv
// @Param organizationId path string true "Organization UUID"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param report query string true "sales | payments | products | reconciliation | machines | vends | inventory | commands | fills"
// @Success 200 {string} string "CSV body"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Router /v1/admin/organizations/{organizationId}/reports/export [get]
func DocOpV1AdminOrgReportsExport() {}

// DocOpV1AdminReportsSalesSummaryExportCSV godoc
// @Summary Export sales summary as CSV (UTF-8)
// @Description Same filters as **GET /v1/reports/sales-summary** (**from**, **to**, **group_by**, **organization_id** for platform admins). Response is CSV with stable headers. Requires **reports.read**.
// @Tags Reporting
// @Security BearerAuth
// @Produce text/csv
// @Param organization_id query string false "Required for platform_admin"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param group_by query string false "day | site | machine | payment_method | none (default day)"
// @Success 200 {string} string "CSV body"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/reports/sales-summary/export.csv [get]
func DocOpV1AdminReportsSalesSummaryExportCSV() {}

// DocOpV1AdminReportsPaymentsSummaryExportCSV godoc
// @Summary Export payments summary as CSV (UTF-8)
// @Description Same filters as **GET /v1/reports/payments-summary**. Requires **reports.read**.
// @Tags Reporting
// @Security BearerAuth
// @Produce text/csv
// @Param organization_id query string false "Required for platform_admin"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param group_by query string false "day | payment_method | status | none (default day)"
// @Success 200 {string} string "CSV body"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/reports/payments-summary/export.csv [get]
func DocOpV1AdminReportsPaymentsSummaryExportCSV() {}

// DocOpV1AdminReportsCashCollectionsExportCSV godoc
// @Summary Export cash collection sessions as CSV (UTF-8)
// @Description Lists **cash_collections** joined to machines/sites in **[from,to)** on **collected_at**. Optional **site_id** and **machine_id** narrow the export (unset = organization-wide). Requires **reports.read**.
// @Tags Reporting
// @Security BearerAuth
// @Produce text/csv
// @Param organization_id query string false "Required for platform_admin"
// @Param from query string true "Inclusive window start (RFC3339)"
// @Param to query string true "Exclusive window end (RFC3339)"
// @Param site_id query string false "Optional site UUID filter"
// @Param machine_id query string false "Optional machine UUID filter"
// @Success 200 {string} string "CSV body"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/reports/cash-collections/export.csv [get]
func DocOpV1AdminReportsCashCollectionsExportCSV() {}

// DocOpV1AdminFinanceDailyClosePost godoc
// @Summary Create immutable finance daily close (requires Idempotency-Key)
// @Description Computes totals for **closeDate** interpreted in **timezone** (IANA). Optional **siteId** / **machineId** narrow machines included in aggregates. **finance_admin** (via **cash.write**) or roles with **cash.write**. Writes **finance_daily_closes** (immutable); duplicates same org/date/scope return **409**.
// @Tags Finance
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param Idempotency-Key header string true "Write idempotency key"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body V1FinanceDailyCloseCreateRequest true "Close payload"
// @Success 201 {object} V1FinanceDailyClose
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError "daily_close_exists"
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/finance/daily-close [post]
func DocOpV1AdminFinanceDailyClosePost() {}

// DocOpV1AdminFinanceDailyCloseList godoc
// @Summary List finance daily closes
// @Description Tenant-scoped list ordered by close date descending. Pagination **limit**/**offset**. Requires **cash.write** (includes **finance_admin**).
// @Tags Finance
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1FinanceDailyCloseListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/finance/daily-close [get]
func DocOpV1AdminFinanceDailyCloseList() {}

// DocOpV1AdminFinanceDailyCloseGet godoc
// @Summary Get one finance daily close by id
// @Description Immutable snapshot row for the tenant. Requires **cash.write**.
// @Tags Finance
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param closeId path string true "Daily close UUID"
// @Success 200 {object} V1FinanceDailyClose
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/finance/daily-close/{closeId} [get]
func DocOpV1AdminFinanceDailyCloseGet() {}

// --- Admin (platform_admin or org_admin) ---

// DocOpV1AdminMachinesList godoc
// @Summary List machines (admin)
// @Description Read-only operational list of machines for an organization. Each row includes site name, device snapshot identity, effective timezone, active technician assignments, current operator (when any), and `machine_slot_state` inventory summary—loaded in batch after the machine page query (**no N+1**). **platform_admin** must pass **organization_id** query (tenant pick). **org_admin** uses JWT organization scope. Optional filters: **site_id**, **machine_id**, **status** (machine.status), **from** / **to** on `updated_at` (RFC3339), **search** is ignored for this resource. Pagination: **limit** (default 50, max 500), **offset**.
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param site_id query string false "Filter by site UUID"
// @Param machine_id query string false "Filter to a single machine UUID"
// @Param status query string false "Filter by machine status (e.g. online, offline)"
// @Param from query string false "Inclusive lower bound for updated_at (RFC3339Nano, explicit timezone offset)"
// @Param to query string false "Inclusive upper bound for updated_at (RFC3339Nano, explicit timezone offset)"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset for pagination"
// @Success 200 {object} V1AdminMachinesListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines [get]
func DocOpV1AdminMachinesList() {}

// DocOpV1AdminMachineGet godoc
// @Summary Get machine (admin)
// @Description Single-machine fleet view: site, device identity from snapshot, effective timezone, active technician assignments, current operator session (from `v_machine_current_operator`), and slot inventory summary. **platform_admin** must pass **organization_id** query (tenant pick). **org_admin** uses JWT organization scope.
// @Tags Machine Admin
// @Security BearerAuth
// @Param organization_id query string false "Required for platform_admin"
// @Param machineId path string true "Machine UUID"
// @Produce json
// @Success 200 {object} V1AdminMachineListItem
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId} [get]
func DocOpV1AdminMachineGet() {}

// DocOpV1AdminSitesList godoc
// @Summary List sites (admin)
// @Description Requires **fleet.read**. **platform_admin** passes **organization_id**. Pagination **limit** / **offset**, optional **status** filter.
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param status query string false "Filter by site status"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/sites [get]
func DocOpV1AdminSitesList() {}

// DocOpV1AdminSiteGet godoc
// @Summary Get site by ID (admin)
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param siteId path string true "Site UUID"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/sites/{siteId} [get]
func DocOpV1AdminSiteGet() {}

// DocOpV1AdminSiteCreate godoc
// @Summary Create site (admin)
// @Description Requires **fleet.write**.
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "name, timezone, code, optional region_id, address"
// @Success 201 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/sites [post]
func DocOpV1AdminSiteCreate() {}

// DocOpV1AdminSitePatch godoc
// @Summary Patch site (admin)
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param siteId path string true "Site UUID"
// @Param body body object true "Partial fields"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/sites/{siteId} [patch]
func DocOpV1AdminSitePatch() {}

// DocOpV1AdminOrgSitesList godoc
// @Summary List organization sites
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/sites [get]
func DocOpV1AdminOrgSitesList() {}

// DocOpV1AdminOrgSitesCreate godoc
// @Summary Create organization site
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param body body object true "Site payload"
// @Success 201 {object} object
// @Router /v1/admin/organizations/{organizationId}/sites [post]
func DocOpV1AdminOrgSitesCreate() {}

// DocOpV1AdminOrgSiteGet godoc
// @Summary Get organization site
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param siteId path string true "Site UUID"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/sites/{siteId} [get]
func DocOpV1AdminOrgSiteGet() {}

// DocOpV1AdminOrgSitePatch godoc
// @Summary Patch organization site
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param siteId path string true "Site UUID"
// @Param body body object true "Partial site payload"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/sites/{siteId} [patch]
func DocOpV1AdminOrgSitePatch() {}

// DocOpV1AdminOrgSiteArchive godoc
// @Summary Archive organization site
// @Description Soft archive: sets site status inactive.
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param siteId path string true "Site UUID"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/sites/{siteId}/archive [post]
func DocOpV1AdminOrgSiteArchive() {}

// DocOpV1AdminOrgSiteDelete godoc
// @Summary Delete (deactivate) organization site
// @Description Archived when no active machines reference the site; otherwise **409**.
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param siteId path string true "Site UUID"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/sites/{siteId} [delete]
func DocOpV1AdminOrgSiteDelete() {}

// DocOpV1AdminOrgMachinesList godoc
// @Summary List organization machines
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Success 200 {object} V1AdminMachinesListResponse
// @Router /v1/admin/organizations/{organizationId}/machines [get]
func DocOpV1AdminOrgMachinesList() {}

// DocOpV1AdminOrgMachinesCreate godoc
// @Summary Create organization machine
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param body body object true "Machine payload"
// @Success 201 {object} object
// @Router /v1/admin/organizations/{organizationId}/machines [post]
func DocOpV1AdminOrgMachinesCreate() {}

// DocOpV1AdminOrgMachineGet godoc
// @Summary Get organization machine
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Success 200 {object} V1AdminMachineListItem
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId} [get]
func DocOpV1AdminOrgMachineGet() {}

// DocOpV1AdminOrgMachinePatch godoc
// @Summary Patch organization machine
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Param body body object true "Partial machine payload"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId} [patch]
func DocOpV1AdminOrgMachinePatch() {}

// DocOpV1AdminOrgMachineArchive godoc
// @Summary Archive organization machine
// @Description Soft archive: sets status **decommissioned** (terminal).
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId}/archive [post]
func DocOpV1AdminOrgMachineArchive() {}

// DocOpV1AdminOrgMachineSuspend godoc
// @Summary Suspend organization machine
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId}/suspend [post]
func DocOpV1AdminOrgMachineSuspend() {}

// DocOpV1AdminOrgMachineResume godoc
// @Summary Resume organization machine
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId}/resume [post]
func DocOpV1AdminOrgMachineResume() {}

// DocOpV1AdminOrgMachineMarkCompromised godoc
// @Summary Mark organization machine compromised
// @Description Sets status compromised and revokes machine credentials.
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId}/mark-compromised [post]
func DocOpV1AdminOrgMachineMarkCompromised() {}

// DocOpV1AdminOrgMachineRotateCredentials godoc
// @Summary Rotate organization machine credentials
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId}/rotate-credentials [post]
func DocOpV1AdminOrgMachineRotateCredentials() {}

// DocOpV1AdminOrgMachineRevokeCredentials godoc
// @Summary Revoke organization machine credentials
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId}/revoke-credentials [post]
func DocOpV1AdminOrgMachineRevokeCredentials() {}

// DocOpV1AdminOrgMachineRotateTokenVersion godoc
// @Summary Rotate machine credential version (alias)
// @Description Same as **rotate-credentials**: bumps **credential_version** and invalidates prior machine JWTs.
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId}/rotate-token-version [post]
func DocOpV1AdminOrgMachineRotateTokenVersion() {}

// DocOpV1AdminOrgMachineRevokeToken godoc
// @Summary Revoke machine JWT signing context (alias)
// @Description Same as **revoke-credentials**.
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId}/revoke-token [post]
func DocOpV1AdminOrgMachineRevokeToken() {}

// DocOpV1AdminOrgMachineTransferSite godoc
// @Summary Move machine to another site in the same organization
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Param body body object true "{\"site_id\":\"...\"}"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId}/transfer-site [post]
func DocOpV1AdminOrgMachineTransferSite() {}

// DocOpV1AdminOrgMachineTechniciansList godoc
// @Summary List machine technician assignments
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Success 200 {object} V1AdminAssignmentsListResponse
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId}/technicians [get]
func DocOpV1AdminOrgMachineTechniciansList() {}

// DocOpV1AdminOrgMachineTechniciansCreate godoc
// @Summary Assign technician to machine
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Param body body object true "userId, role, optional scope"
// @Success 201 {object} object
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId}/technicians [post]
func DocOpV1AdminOrgMachineTechniciansCreate() {}

// DocOpV1AdminOrgMachineTechnicianDelete godoc
// @Summary Remove technician assignment from machine
// @Tags Machine Admin
// @Security BearerAuth
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Param userId path string true "Technician/user UUID"
// @Success 204 {string} string ""
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId}/technicians/{userId} [delete]
func DocOpV1AdminOrgMachineTechnicianDelete() {}

// DocOpV1AdminOrgOperationsMachinesHealthList godoc
// @Summary List machine operational health rows for an organization
// @Description Requires **fleet:read** or **telemetry:read**.
// @Tags Operations
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param limit query int false "Page size"
// @Param offset query int false "Offset"
// @Success 200 {object} V1AdminOperationsMachineHealthListResponse
// @Router /v1/admin/organizations/{organizationId}/operations/machines/health [get]
func DocOpV1AdminOrgOperationsMachinesHealthList() {}

// DocOpV1AdminOrgMachineOperationalHealthGet godoc
// @Summary Get operational health for one machine
// @Description Requires **fleet:read** or **telemetry:read**.
// @Tags Operations
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Success 200 {object} V1AdminOperationsMachineHealthItem
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId}/health [get]
func DocOpV1AdminOrgMachineOperationalHealthGet() {}

// DocOpV1AdminOrgMachineOperationalTimeline godoc
// @Summary Machine timeline (commands, orders, check-ins)
// @Description Requires **fleet:read** or **telemetry:read**.
// @Tags Operations
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Param limit query int false "Page size"
// @Param offset query int false "Offset"
// @Success 200 {object} V1AdminOperationsTimelineListResponse
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId}/timeline [get]
func DocOpV1AdminOrgMachineOperationalTimeline() {}

// DocOpV1AdminOrgOperationsCommandsList godoc
// @Summary List remote commands for an organization
// @Description Requires **fleet:read**. Uses shared fleet command list filters (machine_id, time range, attempt status).
// @Tags Operations
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Success 200 {object} V1AdminCommandsListResponse
// @Router /v1/admin/organizations/{organizationId}/commands [get]
func DocOpV1AdminOrgOperationsCommandsList() {}

// DocOpV1AdminOrgOperationsCommandGet godoc
// @Summary Get command ledger detail with attempts (timeline)
// @Description Requires **fleet:read**.
// @Tags Operations
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param commandId path string true "Command ledger UUID"
// @Success 200 {object} V1AdminOperationsCommandDetailResponse
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/commands/{commandId} [get]
func DocOpV1AdminOrgOperationsCommandGet() {}

// DocOpV1AdminOrgOperationsCommandRetry godoc
// @Summary Retry a non-terminal retryable remote command
// @Description Requires **machine:command**. Requires ledger idempotency key; rejects terminal successes.
// @Tags Operations
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param commandId path string true "Command ledger UUID"
// @Success 200 {object} V1AdminOperationsCommandRetryResponse
// @Failure 409 {object} V1StandardError
// @Failure 503 {object} V1CapabilityNotConfiguredError
// @Router /v1/admin/organizations/{organizationId}/commands/{commandId}/retry [post]
func DocOpV1AdminOrgOperationsCommandRetry() {}

// DocOpV1AdminOrgOperationsCommandCancel godoc
// @Summary Cancel pending or sent command attempts
// @Description Requires **machine:command**.
// @Tags Operations
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param commandId path string true "Command ledger UUID"
// @Success 200 {object} V1AdminOperationsCommandCancelResponse
// @Failure 409 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/commands/{commandId}/cancel [post]
func DocOpV1AdminOrgOperationsCommandCancel() {}

// DocOpV1AdminOrgOperationsMachineCommandsDispatch godoc
// @Summary Dispatch a new remote command to a machine
// @Description Requires **machine:command**. Idempotency-Key header is mandatory.
// @Tags Operations
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Param body body V1AdminOperationsMachineCommandDispatchRequest true "Command type and payload"
// @Success 202 {object} V1AdminOperationsMachineCommandDispatchResponse
// @Failure 503 {object} V1CapabilityNotConfiguredError
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId}/commands [post]
func DocOpV1AdminOrgOperationsMachineCommandsDispatch() {}

// DocOpV1AdminOrgOperationsInventoryAnomaliesList godoc
// @Summary List inventory anomalies for an organization
// @Description Requires **inventory:read** or **fleet:read**. Query refresh=true runs detectors before listing.
// @Tags Operations
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param refresh query bool false "Run anomaly detectors before listing"
// @Param limit query int false "Page size"
// @Param offset query int false "Offset"
// @Success 200 {object} V1AdminOperationsInventoryAnomalyListResponse
// @Router /v1/admin/organizations/{organizationId}/inventory/anomalies [get]
func DocOpV1AdminOrgOperationsInventoryAnomaliesList() {}

// DocOpV1AdminOrgOperationsMachineInventoryAnomaliesList godoc
// @Summary List inventory anomalies for one machine
// @Tags Operations
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Param refresh query bool false "Run anomaly detectors before listing"
// @Param limit query int false "Page size"
// @Param offset query int false "Offset"
// @Success 200 {object} V1AdminOperationsInventoryAnomalyListResponse
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId}/inventory/anomalies [get]
func DocOpV1AdminOrgOperationsMachineInventoryAnomaliesList() {}

// DocOpV1AdminOrgOperationsInventoryAnomalyResolve godoc
// @Summary Resolve an open inventory anomaly
// @Description Requires **inventory:write**.
// @Tags Operations
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param anomalyId path string true "Anomaly UUID"
// @Param body body V1AdminOperationsInventoryAnomalyResolveRequest false "Optional resolution note"
// @Success 200 {object} V1AdminOperationsInventoryAnomalyResolveResponse
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/inventory/anomalies/{anomalyId}/resolve [post]
func DocOpV1AdminOrgOperationsInventoryAnomalyResolve() {}

// DocOpV1AdminOrgOperationalAnomaliesList godoc
// @Summary List unified operational anomalies (P2.4)
// @Description Requires **inventory:read**, **fleet:read**, or **telemetry:read**. Optional **refresh=true** runs inventory + operational detectors (deduped open fingerprints).
// @Tags Operations
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param refresh query bool false "Run anomaly detectors before listing"
// @Param limit query int false "Page size"
// @Param offset query int false "Offset"
// @Success 200 {object} V1AdminOperationsInventoryAnomalyListResponse
// @Router /v1/admin/organizations/{organizationId}/anomalies [get]
func DocOpV1AdminOrgOperationalAnomaliesList() {}

// DocOpV1AdminOrgOperationalAnomalyGet godoc
// @Summary Get one operational anomaly
// @Tags Operations
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param anomalyId path string true "Anomaly UUID"
// @Success 200 {object} V1AdminOperationsInventoryAnomalyItem
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/anomalies/{anomalyId} [get]
func DocOpV1AdminOrgOperationalAnomalyGet() {}

// DocOpV1AdminOrgOperationalAnomalyResolve godoc
// @Summary Resolve an open operational anomaly
// @Description Requires **inventory:adjust**. Audited as admin.operational_anomaly.resolve.
// @Tags Operations
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param anomalyId path string true "Anomaly UUID"
// @Param body body V1AdminOperationsInventoryAnomalyResolveRequest false "Optional note"
// @Success 200 {object} V1AdminOperationsInventoryAnomalyResolveResponse
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/anomalies/{anomalyId}/resolve [post]
func DocOpV1AdminOrgOperationalAnomalyResolve() {}

// DocOpV1AdminOrgOperationalAnomalyIgnore godoc
// @Summary Ignore an open operational anomaly
// @Description Requires **inventory:adjust**. Audited as admin.operational_anomaly.ignore.
// @Tags Operations
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param anomalyId path string true "Anomaly UUID"
// @Param body body V1AdminOperationsInventoryAnomalyResolveRequest false "Optional note"
// @Success 200 {object} V1AdminOperationsInventoryAnomalyResolveResponse
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/anomalies/{anomalyId}/ignore [post]
func DocOpV1AdminOrgOperationalAnomalyIgnore() {}

// DocOpV1AdminOrgRestockSuggestions godoc
// @Summary Organization-scoped restock suggestions (explainable forecast)
// @Description Same projection as GET /v1/admin/inventory/refill-suggestions. Requires **inventory:read**, **fleet:read**, or **telemetry:read**.
// @Tags Operations
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param velocity_days query int false "Lookback window for sales velocity (default 14)"
// @Param limit query int false "Page size"
// @Param offset query int false "Offset"
// @Success 200 {object} V1AdminInventoryRefillForecastResponse
// @Router /v1/admin/organizations/{organizationId}/restock/suggestions [get]
func DocOpV1AdminOrgRestockSuggestions() {}

// DocOpV1AdminOrgOperationsMachineInventoryReconcile godoc
// @Summary Append an inventory reconcile marker event for a machine
// @Description Requires **inventory:write**.
// @Tags Operations
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param machineId path string true "Machine UUID"
// @Param body body V1AdminOperationsInventoryReconcileRequest false "Optional human reason"
// @Success 202 {object} V1AdminOperationsInventoryReconcileResponse
// @Failure 404 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/machines/{machineId}/inventory/reconcile [post]
func DocOpV1AdminOrgOperationsMachineInventoryReconcile() {}

// DocOpV1AdminOrgProvisioningBulkCreate godoc
// @Summary Bulk-create machines with optional activation codes (batch export)
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param body body V1AdminProvisioningBulkCreateRequest true "Provisioning payload"
// @Success 201 {object} V1AdminProvisioningBulkCreateResponse
// @Router /v1/admin/organizations/{organizationId}/provisioning/machines/bulk [post]
func DocOpV1AdminOrgProvisioningBulkCreate() {}

// DocOpV1AdminOrgProvisioningBatchGet godoc
// @Summary Get provisioning batch detail (activation visibility / export metadata)
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param batchId path string true "Provisioning batch UUID"
// @Success 200 {object} V1AdminProvisioningBatchDetailResponse
// @Router /v1/admin/organizations/{organizationId}/provisioning/batches/{batchId} [get]
func DocOpV1AdminOrgProvisioningBatchGet() {}

// DocOpV1AdminOrgRolloutsCreate godoc
// @Summary Create a fleet rollout campaign (MQTT command ledger targets resolved at start)
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param body body V1AdminRolloutCreateRequest true "Rollout definition"
// @Success 201 {object} V1AdminRolloutCampaign
// @Router /v1/admin/organizations/{organizationId}/rollouts [post]
func DocOpV1AdminOrgRolloutsCreate() {}

// DocOpV1AdminOrgRolloutsList godoc
// @Summary List fleet rollout campaigns for an organization
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param limit query int false "Page size"
// @Param offset query int false "Offset"
// @Success 200 {object} V1AdminRolloutListResponse
// @Router /v1/admin/organizations/{organizationId}/rollouts [get]
func DocOpV1AdminOrgRolloutsList() {}

// DocOpV1AdminOrgRolloutsGet godoc
// @Summary Get rollout campaign detail including per-machine targets
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param rolloutId path string true "Rollout campaign UUID"
// @Success 200 {object} V1AdminRolloutDetailResponse
// @Router /v1/admin/organizations/{organizationId}/rollouts/{rolloutId} [get]
func DocOpV1AdminOrgRolloutsGet() {}

// DocOpV1AdminOrgRolloutsStart godoc
// @Summary Resolve targets and dispatch rollout commands via MQTT ledger
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param rolloutId path string true "Rollout campaign UUID"
// @Success 200 {object} V1AdminRolloutDetailResponse
// @Router /v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/start [post]
func DocOpV1AdminOrgRolloutsStart() {}

// DocOpV1AdminOrgRolloutsPause godoc
// @Summary Pause an in-flight rollout (stops dispatch loops)
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param rolloutId path string true "Rollout campaign UUID"
// @Success 200 {object} V1AdminRolloutDetailResponse
// @Router /v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/pause [post]
func DocOpV1AdminOrgRolloutsPause() {}

// DocOpV1AdminOrgRolloutsResume godoc
// @Summary Resume a paused rollout
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param rolloutId path string true "Rollout campaign UUID"
// @Success 200 {object} V1AdminRolloutDetailResponse
// @Router /v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/resume [post]
func DocOpV1AdminOrgRolloutsResume() {}

// DocOpV1AdminOrgRolloutsCancel godoc
// @Summary Cancel remaining pending rollout targets
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param rolloutId path string true "Rollout campaign UUID"
// @Success 200 {object} V1AdminRolloutDetailResponse
// @Router /v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/cancel [post]
func DocOpV1AdminOrgRolloutsCancel() {}

// DocOpV1AdminOrgRolloutsRollback godoc
// @Summary Roll back succeeded targets via new MQTT fleet_rollout_apply commands (rollback_version required)
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param rolloutId path string true "Rollout campaign UUID"
// @Success 200 {object} V1AdminRolloutDetailResponse
// @Router /v1/admin/organizations/{organizationId}/rollouts/{rolloutId}/rollback [post]
func DocOpV1AdminOrgRolloutsRollback() {}

// DocOpV1AdminSiteDisable godoc
// @Summary Disable site (admin)
// @Description Fails with **409** when non-retired machines still reference the site.
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param siteId path string true "Site UUID"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/sites/{siteId}/disable [post]
func DocOpV1AdminSiteDisable() {}

// DocOpV1AdminSiteDelete godoc
// @Summary Deactivate site (admin)
// @Description Fails with **409** when non-retired machines still reference the site.
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param siteId path string true "Site UUID"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/sites/{siteId} [delete]
func DocOpV1AdminSiteDelete() {}

// DocOpV1AdminMachineCreate godoc
// @Summary Create machine (admin)
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "site_id, serial_number, name, optional hardware_profile_id, status"
// @Success 201 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines [post]
func DocOpV1AdminMachineCreate() {}

// DocOpV1AdminMachinePatch godoc
// @Summary Patch machine metadata (admin)
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param machineId path string true "Machine UUID"
// @Param body body object true "name, status, optional hardware_profile_id"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId} [patch]
func DocOpV1AdminMachinePatch() {}

// DocOpV1AdminMachineDisable godoc
// @Summary Disable machine (admin)
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param machineId path string true "Machine UUID"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/disable [post]
func DocOpV1AdminMachineDisable() {}

// DocOpV1AdminMachineEnable godoc
// @Summary Enable machine (admin)
// @Description Returns a disabled machine to offline runtime state; retired machines cannot be enabled.
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param machineId path string true "Machine UUID"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/enable [post]
func DocOpV1AdminMachineEnable() {}

// DocOpV1AdminMachineRetire godoc
// @Summary Retire machine (admin)
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param machineId path string true "Machine UUID"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/retire [post]
func DocOpV1AdminMachineRetire() {}

// DocOpV1AdminMachineRotateCredential godoc
// @Summary Rotate machine credential (admin)
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param machineId path string true "Machine UUID"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/rotate-credential [post]
func DocOpV1AdminMachineRotateCredential() {}

// DocOpV1AdminTechnicianGet godoc
// @Summary Get technician by ID (admin)
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param technicianId path string true "Technician UUID"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/technicians/{technicianId} [get]
func DocOpV1AdminTechnicianGet() {}

// DocOpV1AdminTechnicianCreate godoc
// @Summary Create technician (admin)
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "display_name, optional email, phone, external_subject"
// @Success 201 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/technicians [post]
func DocOpV1AdminTechnicianCreate() {}

// DocOpV1AdminTechnicianPatch godoc
// @Summary Patch technician (admin)
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param technicianId path string true "Technician UUID"
// @Param body body object true "Partial fields"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/technicians/{technicianId} [patch]
func DocOpV1AdminTechnicianPatch() {}

// DocOpV1AdminTechnicianDisable godoc
// @Summary Disable technician (admin)
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param technicianId path string true "Technician UUID"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/technicians/{technicianId}/disable [post]
func DocOpV1AdminTechnicianDisable() {}

// DocOpV1AdminTechnicianEnable godoc
// @Summary Enable technician (admin)
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param technicianId path string true "Technician UUID"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/technicians/{technicianId}/enable [post]
func DocOpV1AdminTechnicianEnable() {}

// DocOpV1AdminTechnicianAssignmentsList godoc
// @Summary List technician assignments (alternate path)
// @Description Same response shape as **GET /v1/admin/assignments**. Requires **fleet.read**.
// @Tags Machine Admin
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
// @Router /v1/admin/technician-assignments [get]
func DocOpV1AdminTechnicianAssignmentsList() {}

// DocOpV1AdminTechnicianAssignmentCreate godoc
// @Summary Create technician–machine assignment (admin)
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "technician_id, machine_id, role"
// @Success 201 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/technician-assignments [post]
func DocOpV1AdminTechnicianAssignmentCreate() {}

// DocOpV1AdminTechnicianAssignmentGet godoc
// @Summary Get technician assignment by ID (admin)
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param assignmentId path string true "Assignment UUID"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/technician-assignments/{assignmentId} [get]
func DocOpV1AdminTechnicianAssignmentGet() {}

// DocOpV1AdminTechnicianAssignmentPatch godoc
// @Summary Patch technician assignment (admin)
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param assignmentId path string true "Assignment UUID"
// @Param body body object true "role, valid_to, status"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/technician-assignments/{assignmentId} [patch]
func DocOpV1AdminTechnicianAssignmentPatch() {}

// DocOpV1AdminTechnicianAssignmentCancel godoc
// @Summary Cancel technician assignment (admin)
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param assignmentId path string true "Assignment UUID"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/technician-assignments/{assignmentId}/cancel [post]
func DocOpV1AdminTechnicianAssignmentCancel() {}

// DocOpV1AdminTechnicianAssignmentDelete godoc
// @Summary Release technician assignment (admin)
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param assignmentId path string true "Assignment UUID"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/technician-assignments/{assignmentId} [delete]
func DocOpV1AdminTechnicianAssignmentDelete() {}

// DocOpV1AdminMachineActivationCodesPost godoc
// @Summary Create machine activation code
// @Description Returns the raw **activationCode** once (server stores a hash only). Requires org admin or platform admin with tenant scope. Subject to sensitive-write rate limiting when enabled.
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "expiresInMinutes, maxUses, optional notes"
// @Success 201 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/activation-codes [post]
func DocOpV1AdminMachineActivationCodesPost() {}

// DocOpV1AdminMachineActivationCodesList godoc
// @Summary List activation codes for a machine
// @Description Returns metadata only; **never** returns the raw activation code. **403** when the machine is outside caller scope.
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/activation-codes [get]
func DocOpV1AdminMachineActivationCodesList() {}

// DocOpV1AdminMachineActivationCodeDelete godoc
// @Summary Revoke an activation code
// @Tags Machine Admin
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Param activationCodeId path string true "Activation code row UUID"
// @Param organization_id query string false "Required for platform_admin"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/activation-codes/{activationCodeId} [delete]
func DocOpV1AdminMachineActivationCodeDelete() {}

// DocOpV1AdminOrgActivationCodesList godoc
// @Summary List activation codes for organization
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param limit query int false "Page size"
// @Param offset query int false "Offset"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/activation-codes [get]
func DocOpV1AdminOrgActivationCodesList() {}

// DocOpV1AdminOrgActivationCodesPost godoc
// @Summary Create activation code for a machine (organization path)
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param body body object true "{\"machineId\":\"...\",\"expiresInMinutes\":1440,\"maxUses\":1}"
// @Success 201 {object} object
// @Router /v1/admin/organizations/{organizationId}/activation-codes [post]
func DocOpV1AdminOrgActivationCodesPost() {}

// DocOpV1AdminOrgActivationCodeRevoke godoc
// @Summary Revoke activation code by id (organization path)
// @Tags Machine Admin
// @Security BearerAuth
// @Param organizationId path string true "Organization UUID"
// @Param codeId path string true "Activation code row UUID"
// @Success 204 {string} string ""
// @Router /v1/admin/organizations/{organizationId}/activation-codes/{codeId}/revoke [post]
func DocOpV1AdminOrgActivationCodeRevoke() {}

// DocOpV1AdminOrgTechniciansList godoc
// @Summary List technicians under organization path
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Success 200 {object} V1AdminTechniciansListResponse
// @Router /v1/admin/organizations/{organizationId}/technicians [get]
func DocOpV1AdminOrgTechniciansList() {}

// DocOpV1AdminOrgTechnicianGet godoc
// @Summary Get technician under organization path
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param technicianId path string true "Technician UUID"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/technicians/{technicianId} [get]
func DocOpV1AdminOrgTechnicianGet() {}

// DocOpV1AdminOrgTechniciansCreate godoc
// @Summary Create technician under organization path
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param body body object true "display_name, optional email, phone, external_subject"
// @Success 201 {object} object
// @Router /v1/admin/organizations/{organizationId}/technicians [post]
func DocOpV1AdminOrgTechniciansCreate() {}

// DocOpV1AdminOrgTechnicianPatch godoc
// @Summary Patch technician under organization path
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param technicianId path string true "Technician UUID"
// @Param body body object true "Partial fields"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/technicians/{technicianId} [patch]
func DocOpV1AdminOrgTechnicianPatch() {}

// DocOpV1AdminOrgTechnicianDisable godoc
// @Summary Disable technician under organization path
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param technicianId path string true "Technician UUID"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/technicians/{technicianId}/disable [post]
func DocOpV1AdminOrgTechnicianDisable() {}

// DocOpV1AdminOrgTechnicianEnable godoc
// @Summary Enable technician under organization path
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param technicianId path string true "Technician UUID"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/technicians/{technicianId}/enable [post]
func DocOpV1AdminOrgTechnicianEnable() {}

// DocOpV1AdminOrgAssignmentsList godoc
// @Summary List assignments under organization path
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Success 200 {object} V1AdminAssignmentsListResponse
// @Router /v1/admin/organizations/{organizationId}/assignments [get]
func DocOpV1AdminOrgAssignmentsList() {}

// DocOpV1AdminOrgAssignmentGet godoc
// @Summary Get assignment under organization path
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param assignmentId path string true "Assignment UUID"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/assignments/{assignmentId} [get]
func DocOpV1AdminOrgAssignmentGet() {}

// DocOpV1AdminOrgAssignmentsCreate godoc
// @Summary Create assignment under organization path
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param body body object true "technician_id, machine_id, role"
// @Success 201 {object} object
// @Router /v1/admin/organizations/{organizationId}/assignments [post]
func DocOpV1AdminOrgAssignmentsCreate() {}

// DocOpV1AdminOrgAssignmentDelete godoc
// @Summary Release assignment under organization path
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param assignmentId path string true "Assignment UUID"
// @Success 200 {object} object
// @Router /v1/admin/organizations/{organizationId}/assignments/{assignmentId} [delete]
func DocOpV1AdminOrgAssignmentDelete() {}

// DocOpV1AdminTechniciansList godoc
// @Summary List technicians (admin)
// @Description Directory of technicians for the organization. **platform_admin** requires **organization_id** query. Optional **technician_id**, **search** (matches display_name or email), **from** / **to** on `created_at`, pagination **limit** / **offset**.
// @Tags Machine Admin
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
// @Tags Machine Admin
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
// @Tags Machine Admin
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
// @Tags Machine Admin
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

// DocOpV1AdminOTACampaignsList godoc
// @Summary List OTA campaigns (lifecycle admin)
// @Description Paginated list of `ota_campaigns` with artifact metadata. **platform_admin** requires **organization_id**.
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param status query string false "Campaign status filter"
// @Param from query string false "created_at lower bound (RFC3339)"
// @Param to query string false "created_at upper bound (RFC3339)"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1AdminOTACampaignListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ota/campaigns [get]
func DocOpV1AdminOTACampaignsList() {}

// DocOpV1AdminOTACampaignCreate godoc
// @Summary Create OTA campaign (draft)
// @Description Requires **ota.write**. Idempotency header required.
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param Idempotency-Key header string true "Write idempotency key"
// @Param organization_id query string false "Required for platform_admin"
// @Param body body object true "Campaign create payload"
// @Success 201 {object} V1AdminOTACampaignDetail
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ota/campaigns [post]
func DocOpV1AdminOTACampaignCreate() {}

// DocOpV1AdminOTACampaignGet godoc
// @Summary Get OTA campaign detail
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param campaignId path string true "Campaign UUID"
// @Success 200 {object} V1AdminOTACampaignDetail
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ota/campaigns/{campaignId} [get]
func DocOpV1AdminOTACampaignGet() {}

// DocOpV1AdminOTACampaignPatch godoc
// @Summary Patch draft/approved OTA campaign
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param Idempotency-Key header string true "Write idempotency key"
// @Param organization_id query string false "Required for platform_admin"
// @Param campaignId path string true "Campaign UUID"
// @Param body body object true "Patch payload"
// @Success 200 {object} V1AdminOTACampaignDetail
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ota/campaigns/{campaignId} [patch]
func DocOpV1AdminOTACampaignPatch() {}

// DocOpV1AdminOTACampaignApprove godoc
// @Summary Approve OTA campaign
// @Description Requires **ota.write** and **org_admin** or **platform_admin**.
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param Idempotency-Key header string true "Write idempotency key"
// @Param organization_id query string false "Required for platform_admin"
// @Param campaignId path string true "Campaign UUID"
// @Success 200 {object} V1AdminOTACampaignDetail
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ota/campaigns/{campaignId}/approve [post]
func DocOpV1AdminOTACampaignApprove() {}

// DocOpV1AdminOTACampaignStart godoc
// @Summary Start OTA rollout (canary first wave)
// @Description Requires **ota.write** and **org_admin** or **platform_admin**.
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param Idempotency-Key header string true "Write idempotency key"
// @Param organization_id query string false "Required for platform_admin"
// @Param campaignId path string true "Campaign UUID"
// @Success 200 {object} V1AdminOTACampaignDetail
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ota/campaigns/{campaignId}/start [post]
func DocOpV1AdminOTACampaignStart() {}

// DocOpV1AdminOTACampaignPublish godoc
// @Summary Publish OTA campaign (approve + start when needed)
// @Description Requires **ota.write** and **org_admin** or **platform_admin**. Idempotent transitions.
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param Idempotency-Key header string true "Write idempotency key"
// @Param organization_id query string false "Required for platform_admin"
// @Param campaignId path string true "Campaign UUID"
// @Success 200 {object} V1AdminOTACampaignDetail
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ota/campaigns/{campaignId}/publish [post]
func DocOpV1AdminOTACampaignPublish() {}

// DocOpV1AdminOTACampaignPause godoc
// @Summary Pause active rollout
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param Idempotency-Key header string true "Write idempotency key"
// @Param organization_id query string false "Required for platform_admin"
// @Param campaignId path string true "Campaign UUID"
// @Success 200 {object} V1AdminOTACampaignDetail
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ota/campaigns/{campaignId}/pause [post]
func DocOpV1AdminOTACampaignPause() {}

// DocOpV1AdminOTACampaignResume godoc
// @Summary Resume paused rollout (remaining machines)
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param Idempotency-Key header string true "Write idempotency key"
// @Param organization_id query string false "Required for platform_admin"
// @Param campaignId path string true "Campaign UUID"
// @Success 200 {object} V1AdminOTACampaignDetail
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ota/campaigns/{campaignId}/resume [post]
func DocOpV1AdminOTACampaignResume() {}

// DocOpV1AdminOTACampaignCancel godoc
// @Summary Cancel OTA campaign
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param Idempotency-Key header string true "Write idempotency key"
// @Param organization_id query string false "Required for platform_admin"
// @Param campaignId path string true "Campaign UUID"
// @Success 200 {object} V1AdminOTACampaignDetail
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ota/campaigns/{campaignId}/cancel [post]
func DocOpV1AdminOTACampaignCancel() {}

// DocOpV1AdminOTACampaignRollback godoc
// @Summary Rollback OTA campaign (dispatch rollback commands)
// @Description Requires **ota.write** and **org_admin** or **platform_admin**. Body may override **rollbackArtifactId**.
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param Idempotency-Key header string true "Write idempotency key"
// @Param organization_id query string false "Required for platform_admin"
// @Param campaignId path string true "Campaign UUID"
// @Param body body object false "Optional rollback artifact override"
// @Success 200 {object} V1AdminOTACampaignDetail
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ota/campaigns/{campaignId}/rollback [post]
func DocOpV1AdminOTACampaignRollback() {}

// DocOpV1AdminOTACampaignTargetsGet godoc
// @Summary List campaign machine targets
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param campaignId path string true "Campaign UUID"
// @Success 200 {object} V1AdminOTACampaignTargetsResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ota/campaigns/{campaignId}/targets [get]
func DocOpV1AdminOTACampaignTargetsGet() {}

// DocOpV1AdminOTACampaignTargetsPut godoc
// @Summary Replace campaign machine targets (draft/approved only)
// @Tags Machine Admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param Idempotency-Key header string true "Write idempotency key"
// @Param organization_id query string false "Required for platform_admin"
// @Param campaignId path string true "Campaign UUID"
// @Param body body object true "{ machineIds: [] }"
// @Success 204 {string} string "No Content"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ota/campaigns/{campaignId}/targets [put]
func DocOpV1AdminOTACampaignTargetsPut() {}

// DocOpV1AdminOTACampaignResultsGet godoc
// @Summary List campaign machine rollout results
// @Tags Machine Admin
// @Security BearerAuth
// @Produce json
// @Param organization_id query string false "Required for platform_admin"
// @Param campaignId path string true "Campaign UUID"
// @Success 200 {object} V1AdminOTACampaignResultsResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/ota/campaigns/{campaignId}/results [get]
func DocOpV1AdminOTACampaignResultsGet() {}

// DocOpV1AdminMachineDiagnosticRequest godoc
// @Summary Request machine diagnostic bundle
// @Description Enqueues a safe command-ledger request for the machine to upload diagnostic/log metadata. The payload explicitly does not support arbitrary shell execution. Requires command.dispatch and an Idempotency-Key header.
// @Tags Fleet
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Param organization_id query string false "Required for platform_admin without org scope"
// @Param Idempotency-Key header string true "Write idempotency key"
// @Accept json
// @Produce json
// @Success 202 {object} object "{requestId,machineId,commandId,sequence,dispatchState,replay}"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 502 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/diagnostics/requests [post]
func DocOpV1AdminMachineDiagnosticRequest() {}

// DocOpV1AdminMachineDiagnosticBundlesList godoc
// @Summary List machine diagnostic bundles
// @Description Lists diagnostic bundle manifests reported by the machine. Blobs remain in object storage and are referenced by storage key.
// @Tags Fleet
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Param organization_id query string false "Required for platform_admin without org scope"
// @Param limit query int false "Default 50, max 500"
// @Param offset query int false "Default 0"
// @Produce json
// @Success 200 {object} object "{items,meta}"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/machines/{machineId}/diagnostics/bundles [get]
func DocOpV1AdminMachineDiagnosticBundlesList() {}

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
// @Tags Operator Sessions
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
// @Tags Operator Sessions
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

// DocOpV1AdminCommerceReconciliationList godoc
// @Summary List commerce reconciliation cases
// @Description Operator-visible payment/vend/refund reconciliation queue for one organization. Cases are created by background reconciliation and webhook replay/mismatch handling. Filters: **status** (open, reviewing, resolved, dismissed), **case_type** (payment_paid_vend_not_started, payment_paid_vend_failed, vend_started_no_terminal_ack, refund_pending_too_long, webhook_provider_mismatch, duplicate_provider_event, duplicate_payment), **limit**, **offset**.
// @Tags Commerce
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param status query string false "Case status"
// @Param case_type query string false "Case type"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1CommerceReconciliationListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/commerce/reconciliation [get]
func DocOpV1AdminCommerceReconciliationList() {}

// DocOpV1AdminCommerceReconciliationGet godoc
// @Summary Get commerce reconciliation case
// @Description Returns one operator-visible commerce reconciliation case scoped to the organization.
// @Tags Commerce
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param id path string true "Reconciliation case UUID"
// @Success 200 {object} V1CommerceReconciliationCase
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/commerce/reconciliation/{id} [get]
func DocOpV1AdminCommerceReconciliationGet() {}

// DocOpV1AdminCommerceReconciliationResolve godoc
// @Summary Resolve commerce reconciliation case
// @Description Marks an open/reviewing commerce reconciliation case as **resolved** or **dismissed** with an audit record. Does not refund or mutate money state by itself.
// @Tags Commerce
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param id path string true "Reconciliation case UUID"
// @Param body body V1CommerceReconciliationResolveRequest true "Resolution body: status=resolved|dismissed, note optional"
// @Success 200 {object} V1CommerceReconciliationCase
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/commerce/reconciliation/{id}/resolve [post]
func DocOpV1AdminCommerceReconciliationResolve() {}

// DocOpV1AdminCommerceReconciliationIgnore godoc
// @Summary Ignore commerce reconciliation case
// @Description Marks an open/reviewing/escalated reconciliation case as **ignored** with audit + optional order timeline append when an order is linked.
// @Tags Commerce
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param id path string true "Reconciliation case UUID"
// @Param body body V1CommerceReconciliationIgnoreRequest false "Optional note"
// @Success 200 {object} V1CommerceReconciliationCase
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/commerce/reconciliation/{id}/ignore [post]
func DocOpV1AdminCommerceReconciliationIgnore() {}

// DocOpV1AdminCommerceOrderTimelineGet godoc
// @Summary List commerce order timeline events
// @Description Append-only lifecycle timeline for one order (reconciliation resolutions, refund requests).
// @Tags Commerce
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param orderId path string true "Order UUID"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1OrderTimelineListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/orders/{orderId}/timeline [get]
func DocOpV1AdminCommerceOrderTimelineGet() {}

// DocOpV1AdminCommerceRefundRequestsList godoc
// @Summary List refund requests for organization
// @Description Operator-visible durable refund review rows (`refund_requests`) scoped to one organization.
// @Tags Commerce
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param status query string false "Refund request status filter"
// @Param limit query int false "Page size (default 50, max 500)"
// @Param offset query int false "Row offset"
// @Success 200 {object} V1RefundRequestsListResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/refunds [get]
func DocOpV1AdminCommerceRefundRequestsList() {}

// DocOpV1AdminCommerceRefundRequestGet godoc
// @Summary Get refund request by id
// @Tags Commerce
// @Security BearerAuth
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param refundId path string true "Refund request UUID"
// @Success 200 {object} V1RefundRequestRow
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/refunds/{refundId} [get]
func DocOpV1AdminCommerceRefundRequestGet() {}

// DocOpV1AdminCommerceOrderRefundPost godoc
// @Summary Create refund request + ledger refund (admin scoped)
// @Description Same semantics as POST `/v1/commerce/orders/{orderId}/refunds`, additionally persists `refund_requests` + order timeline. Requires **Idempotency-Key** header.
// @Tags Commerce
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param organizationId path string true "Organization UUID"
// @Param orderId path string true "Order UUID"
// @Param Idempotency-Key header string true "Write idempotency key"
// @Param body body V1AdminOrderRefundPostRequest true "Refund amount/reason"
// @Success 200 {object} V1AdminOrderRefundPostResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/admin/organizations/{organizationId}/orders/{orderId}/refunds [post]
func DocOpV1AdminCommerceOrderRefundPost() {}

// --- Fleet / device read ---

// DocOpV1MachineShadowGet godoc
// @Summary Get machine shadow JSON
// @Description Returns the persisted desired/reported JSON documents used for fleet remote configuration. Requires Bearer JWT and machine read access (`RequireMachineTenantAccess`). Invalid UUID in path → **400** (`invalid_machine_id`). Missing shadow row → **404** `machine_shadow_not_found`. This is not live MQTT; it is the last reconciled projection in Postgres.
// @Tags Device Runtime
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

// DocOpV1MachineSaleCatalogGet godoc
// @Summary Runtime sale catalog (planogram, price, stock, images)
// @Description Active **current** slots only. **isAvailable** reflects product active, positive price, and stock. By default unavailable rows are omitted; **include_unavailable=true** returns them with **unavailableReason**. **include_images=false** omits **image**. **if_none_match_config_version** returns **304** when unchanged. Responses include **configVersion** (device shadow revision) and **catalogVersion** (assortment fingerprint); cache on either as needed.
// @Tags Runtime Catalog
// @Security BearerAuth
// @Produce json
// @Param machineId path string true "Machine UUID"
// @Param if_none_match_config_version query int false "When equal to current configVersion, respond 304"
// @Param include_unavailable query bool false "Default false"
// @Param include_images query bool false "Default true"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/sale-catalog [get]
func DocOpV1MachineSaleCatalogGet() {}

// DocOpV1MachineTelemetrySnapshot godoc
// @Summary Current machine telemetry snapshot (projected)
// @Description Read-only `machine_current_snapshot` row (rollups + shadow projection). **404** when no snapshot exists yet. Not a raw MQTT history API. Timestamps are RFC3339Nano strings with explicit timezone offset (responses use UTC **Z**); **effectiveTimezone** is an IANA name for business-local context.
// @Tags Telemetry
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Produce json
// @Success 200 {object} V1MachineTelemetrySnapshotResponse
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
// @Tags Telemetry
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Param limit query int false "Default 50, max 500"
// @Produce json
// @Success 200 {object} V1MachineTelemetryIncidentsResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1BearerAuthError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/telemetry/incidents [get]
func DocOpV1MachineTelemetryIncidents() {}

// DocOpV1MachineTelemetryRollups godoc
// @Summary Telemetry rollup buckets (1m / 1h)
// @Description Aggregated `telemetry_rollups` only — not raw high-frequency streams. Query `from`/`to` as RFC3339Nano with explicit timezone offset (default last 24h), `granularity` (`1m` default, `1h`).
// @Tags Telemetry
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Param from query string false "Lower bound for buckets (RFC3339Nano, explicit offset)"
// @Param to query string false "Upper bound for buckets (RFC3339Nano, explicit offset)"
// @Param granularity query string false "1m or 1h"
// @Produce json
// @Success 200 {object} V1MachineTelemetryRollupsResponse
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
// @Tags Device Runtime
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
// @Tags Device Runtime
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
// @Tags Device Runtime
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

// DocOpV1DeviceMachineVendResults godoc
// @Summary Report vend outcome for an order (HTTP bridge)
// @Description Idempotency header required. **outcome** is **success** or **failed**. On **success**, order is finalized and inventory is decremented once (idempotent on the same key). On **failed**, inventory is not changed; **correlation_id** is preserved on the vend session when provided. Requires **commerce** and **telemetry store** wiring.
// @Tags Device Runtime
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Accept json
// @Produce json
// @Param body body object true "order_id, slot_index, outcome, optional failure_reason, optional correlation_id"
// @Success 200 {object} object "{order_id,order_status,vend_state,replay}"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/device/machines/{machineId}/vend-results [post]
func DocOpV1DeviceMachineVendResults() {}

// DocOpV1DeviceMachineCommandsPoll godoc
// @Summary Poll pending remote commands over HTTP (MQTT fallback)
// @Description Returns commands whose latest attempt is **pending** or **sent** (oldest sequence first). Optional JSON body **limit** (default 20, max 100). Nil RemoteCommands → **500**. OpenAPI example uses **`machine_planogram_publish`** payload shape (any `command_type` the ledger holds may appear).
// @Tags Device Runtime
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Accept json
// @Produce json
// @Param body body object false "limit"
// @Success 200 {object} object "{items:[{sequence,command_type,payload,correlation_id,idempotency_key}],meta}"
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/device/machines/{machineId}/commands/poll [post]
func DocOpV1DeviceMachineCommandsPoll() {}

// DocOpV1DeviceTelemetryReconcileBatch godoc
// @Summary Batch reconcile critical telemetry idempotency keys
// @Description Returns **processed**, **not_found**, **failed_retryable**, **failed_terminal**, etc., per key. **not_found** / **failed_retryable** imply device backoff retry; **failed_terminal** stops retry. Batch size **1–500**. Cross-machine rows are never visible.
// @Tags Telemetry Reconcile
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Accept json
// @Produce json
// @Param body body object true "{ idempotencyKeys: string[] }"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/device/machines/{machineId}/events/reconcile [post]
func DocOpV1DeviceTelemetryReconcileBatch() {}

// DocOpV1DeviceTelemetryReconcileStatusGet godoc
// @Summary Single critical telemetry idempotency status
// @Tags Telemetry Reconcile
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Param idempotencyKey path string true "Idempotency key (URL-encoded as needed)"
// @Produce json
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/device/machines/{machineId}/events/{idempotencyKey}/status [get]
func DocOpV1DeviceTelemetryReconcileStatusGet() {}

// --- Machine runtime (Android check-in & config ack) ---

// DocOpV1MachineCheckIn godoc
// @Summary Record Android check-in
// @Description Append-only check-in for device identity and runtime. **occurred_at** must be RFC3339 with timezone. Requires **RequireMachineURLAccess** (machine-scoped JWT). Subject to sensitive-write rate limit when enabled.
// @Tags Device Runtime
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Accept json
// @Produce json
// @Param body body object true "android_id, sim_serial, package_name, version_name, version_code, android_release, sdk_int, manufacturer, model, timezone, network_state, boot_id, occurred_at, metadata"
// @Success 201 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/check-ins [post]
func DocOpV1MachineCheckIn() {}

// DocOpV1MachineConfigApply godoc
// @Summary Acknowledge config applied on device
// @Description Persists a **machine_configs** row; **applied_at** RFC3339 with timezone; **config_version** maps to config_revision. Optional **operator_session_id**. Android context in **android_id** and **app_version** stored in metadata.
// @Tags Device Runtime
// @Security BearerAuth
// @Param machineId path string true "Machine UUID"
// @Accept json
// @Produce json
// @Param body body object true "config_version, applied_at, android_id, app_version, operator_session_id, config_payload"
// @Success 201 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/machines/{machineId}/config-applies [post]
func DocOpV1MachineConfigApply() {}

// --- Operator sessions (machine-scoped) ---

// DocOpV1OperatorSessionCurrent godoc
// @Summary Get current operator session
// @Description Routes not mounted when operator service is nil. Response is `{"active_session":null}` or `{"active_session":{...}}` (optional `technician_display_name`).
// @Tags Operator Sessions
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
// @Tags Operator Sessions
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
// @Tags Operator Sessions
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
// @Tags Operator Sessions
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
// @Tags Operator Sessions
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
// @Tags Operator Sessions
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
// @Tags Operator Sessions
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
// @Tags Operator Sessions
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

// DocOpV1CommerceCashCheckout godoc
// @Summary Create order, record captured cash payment, mark paid
// @Description Same body as **POST /v1/commerce/orders**; runs **StartPaymentWithOutbox** with **provider=cash** and **payment_state=captured**, then marks the order paid. Requires commerce outbox env (same guard as payment-session). **Idempotency-Key** required.
// @Tags Commerce
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body object true "machine_id, product_id, slot_id or (cabinet_code+slot_code) or slot_index (deprecated), currency; subtotal/tax/total must be omitted or zero (server pricing)"
// @Success 200 {object} V1CommerceCashCheckoutResponse
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 503 {object} V1CapabilityNotConfiguredError
// @Failure 500 {object} V1StandardError
// @Router /v1/commerce/cash-checkout [post]
func DocOpV1CommerceCashCheckout() {}

// DocOpV1CommerceCreateOrder godoc
// @Summary Create order and initial vend session
// @Description Commerce routes not mounted when commerce service nil. Requires org on JWT for non-platform users. **Idempotency-Key** or **X-Idempotency-Key** required. Not configured → **503** `capability_not_configured` (`v1.commerce.persistence`). Rate limit → **429**.
// @Tags Commerce
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body object true "machine_id, product_id, slot_id or (cabinet_code+slot_code) or slot_index (deprecated), currency; subtotal/tax/total must be omitted or zero (pricing from published assortment)"
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
// @Description **No Bearer JWT.** Verification mode **COMMERCE_PAYMENT_WEBHOOK_VERIFICATION=avf_hmac** (default): requires **COMMERCE_PAYMENT_WEBHOOK_SECRET** (aliases **COMMERCE_PAYMENT_WEBHOOK_HMAC_SECRET**, **PAYMENT_WEBHOOK_SECRET**) and headers **X-AVF-Webhook-Timestamp** (unix seconds) and **X-AVF-Webhook-Signature** (hex HMAC-SHA256 over `{timestamp}.{rawBody}`). Replay/stale protection uses **COMMERCE_PAYMENT_WEBHOOK_REPLAY_WINDOW** seconds (preferred; **COMMERCE_PAYMENT_WEBHOOK_TIMESTAMP_SKEW_SECONDS** is a legacy alias). **Staging and production** reject unsigned callbacks unless **COMMERCE_PAYMENT_WEBHOOK_UNSAFE_ALLOW_UNSIGNED_PRODUCTION=true** (documented unsafe). **Development or test** may set **COMMERCE_PAYMENT_WEBHOOK_ALLOW_UNSIGNED=true** with an empty secret for local testing only. JSON **webhook_event_id** is required and unique per **provider** for idempotency alongside **provider_reference**. **200** with `replay:true` means an idempotent retry. **403** `webhook_hmac_required`, **401** `webhook_auth_failed`, **409** `webhook_idempotency_conflict`. Do not log the webhook secret.
// @Tags Commerce
// @Param orderId path string true "Order UUID"
// @Param paymentId path string true "Payment UUID"
// @Accept json
// @Produce json
// @Param X-AVF-Webhook-Timestamp header string true "Unix seconds"
// @Param X-AVF-Webhook-Signature header string true "Hex digest (optional sha256= prefix)"
// @Param body body object true "provider, provider_reference, event_type, normalized_payment_state, payload_json, ..."
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1StandardError
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

// DocOpV1CommerceOrderCancel godoc
// @Summary Cancel order before payment capture
// @Description Allowed only while payment is not captured/settled; paid flows must use **refunds**. **Idempotency-Key** required.
// @Tags Commerce
// @Security BearerAuth
// @Param orderId path string true "Order UUID"
// @Accept json
// @Produce json
// @Param body body object true "reason, optional slot_index"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/commerce/orders/{orderId}/cancel [post]
func DocOpV1CommerceOrderCancel() {}

// DocOpV1CommerceRefundCreate godoc
// @Summary Create or replay a refund (idempotent)
// @Description Cannot exceed captured minus already refunded. **Idempotency-Key** required. Cash vs card semantics: see response **refund_state** and order flags.
// @Tags Commerce
// @Security BearerAuth
// @Param orderId path string true "Order UUID"
// @Accept json
// @Produce json
// @Param body body object true "reason, amount_minor, currency, optional metadata"
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 409 {object} V1StandardError
// @Failure 429 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/commerce/orders/{orderId}/refunds [post]
func DocOpV1CommerceRefundCreate() {}

// DocOpV1CommerceRefundsList godoc
// @Summary List refunds for an order
// @Tags Commerce
// @Security BearerAuth
// @Param orderId path string true "Order UUID"
// @Produce json
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/commerce/orders/{orderId}/refunds [get]
func DocOpV1CommerceRefundsList() {}

// DocOpV1CommerceRefundGet godoc
// @Summary Get one refund on an order
// @Tags Commerce
// @Security BearerAuth
// @Param orderId path string true "Order UUID"
// @Param refundId path string true "Refund UUID"
// @Produce json
// @Success 200 {object} object
// @Failure 400 {object} V1StandardError
// @Failure 401 {object} V1BearerAuthError
// @Failure 403 {object} V1StandardError
// @Failure 404 {object} V1StandardError
// @Failure 500 {object} V1StandardError
// @Router /v1/commerce/orders/{orderId}/refunds/{refundId} [get]
func DocOpV1CommerceRefundGet() {}
