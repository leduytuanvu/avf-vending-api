package httpserver

import "encoding/json"

// OpenAPI / Swagger documentation types (runtime responses use compatible JSON shapes).
// Handlers may return additional fields; these structs capture stable fields for spec generation.

// V1ErrorBody is the inner object for all JSON API errors under /v1 (handlers and auth middleware).
type V1ErrorBody struct {
	Code      string         `json:"code" example:"invalid_json"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details"`
	RequestID string         `json:"requestId" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// V1StandardError is the usual handler JSON error (see writeAPIError).
type V1StandardError struct {
	Error V1ErrorBody `json:"error"`
}

type V1ReportMeta struct {
	Limit    int   `json:"limit" example:"50"`
	Offset   int   `json:"offset" example:"0"`
	Returned int   `json:"returned" example:"1"`
	Total    int64 `json:"total" example:"1"`
}

type V1AdminReportSalesResponse struct {
	OrganizationID string         `json:"organizationId"`
	From           string         `json:"from"`
	To             string         `json:"to"`
	GroupBy        string         `json:"groupBy"`
	Summary        map[string]any `json:"summary"`
	Breakdown      []any          `json:"breakdown"`
}

type V1AdminReportPaymentsResponse struct {
	OrganizationID string       `json:"organizationId"`
	From           string       `json:"from"`
	To             string       `json:"to"`
	Timezone       string       `json:"timezone"`
	Items          []any        `json:"items"`
	Meta           V1ReportMeta `json:"meta"`
}

type V1AdminReportListResponse struct {
	OrganizationID string       `json:"organizationId"`
	From           string       `json:"from"`
	To             string       `json:"to"`
	Meta           V1ReportMeta `json:"meta"`
	Items          []any        `json:"items"`
}

// V1NotImplementedError is HTTP **501**; error.details carries capability + implemented.
type V1NotImplementedError struct {
	Error V1ErrorBody `json:"error"`
}

// V1CapabilityNotConfiguredError is HTTP **503** when optional wiring is missing; details carry capability + implemented.
type V1CapabilityNotConfiguredError struct {
	Error V1ErrorBody `json:"error"`
}

// V1BearerAuthError is returned by Bearer and RBAC middleware (HTTP 401/403/400/503 from auth layer).
type V1BearerAuthError struct {
	Error V1ErrorBody `json:"error"`
}

// V1OperatorListEnvelope matches writeOperatorListEnvelope (items + meta.limit + meta.returned).
type V1OperatorListEnvelope struct {
	Items []any      `json:"items"`
	Meta  V1ListMeta `json:"meta"`
}

// V1OperatorSessionEnvelope wraps a session object for login/logout/heartbeat success bodies.
type V1OperatorSessionEnvelope struct {
	Session map[string]any `json:"session"`
}

// V1OperatorCurrentEnvelope is the /operator-sessions/current success shape.
type V1OperatorCurrentEnvelope struct {
	ActiveSession         any     `json:"active_session"`
	TechnicianDisplayName *string `json:"technician_display_name,omitempty"`
}

// V1CommerceCreateOrderResponse matches commerceCreateOrderResponse JSON.
type V1CommerceCreateOrderResponse struct {
	OrderID       string `json:"order_id"`
	VendSessionID string `json:"vend_session_id"`
	Replay        bool   `json:"replay"`
	OrderStatus   string `json:"order_status" enums:"created,quoted,paid,vending,completed,failed,cancelled"`
	VendState     string `json:"vend_state" enums:"pending,in_progress,success,failed"`
	SlotID        string `json:"slot_id"`
	CabinetCode   string `json:"cabinet_code"`
	SlotCode      string `json:"slot_code"`
	SlotIndex     int32  `json:"slot_index"`
	SubtotalMinor int64  `json:"subtotal_minor"`
	TaxMinor      int64  `json:"tax_minor"`
	TotalMinor    int64  `json:"total_minor"`
	PriceMinor    int64  `json:"price_minor"`
}

// V1CommerceCashCheckoutResponse matches commerceCashCheckoutResponse JSON (POST /v1/commerce/cash-checkout).
// See docs/api/setup-machine.md (commerce) and OpenAPI example on that path.
type V1CommerceCashCheckoutResponse struct {
	OrderID       string `json:"order_id"`
	VendSessionID string `json:"vend_session_id"`
	PaymentID     string `json:"payment_id"`
	OrderStatus   string `json:"order_status" enums:"created,quoted,paid,vending,completed,failed,cancelled"`
	PaymentState  string `json:"payment_state" enums:"created,authorized,captured,failed,refunded"`
	Replay        bool   `json:"replay"`
}

// V1ListViewEnvelope is the success shape for admin and tenant list endpoints.
type V1ListViewEnvelope struct {
	Items []any `json:"items"`
	Meta  any   `json:"meta,omitempty"`
}

// V1ListMeta is common pagination metadata for list responses.
type V1ListMeta struct {
	Limit    int32 `json:"limit" example:"50"`
	Returned int   `json:"returned" example:"12"`
}

// --- Auth session (POST /v1/auth/login, /v1/auth/refresh; GET /v1/auth/me; POST /v1/auth/logout) ---

// V1AuthLoginRequest is documented in tools/build_openapi.py (example organizationId + email + password).
type V1AuthLoginRequest struct {
	OrganizationID string `json:"organizationId" example:"11111111-1111-1111-1111-111111111111"`
	Email          string `json:"email" example:"admin@example.com"`
	Password       string `json:"password" example:"••••••••"`
}

// V1AuthTokenPair is nested under login/refresh responses.
type V1AuthTokenPair struct {
	AccessToken      string `json:"accessToken"`
	AccessExpiresAt  string `json:"accessExpiresAt"`
	RefreshToken     string `json:"refreshToken"`
	RefreshExpiresAt string `json:"refreshExpiresAt"`
	TokenType        string `json:"tokenType" example:"Bearer"`
}

// V1AuthLoginResponse documents POST /v1/auth/login success.
type V1AuthLoginResponse struct {
	AccountID             string          `json:"accountId"`
	OrganizationID        string          `json:"organizationId"`
	Email                 string          `json:"email"`
	Roles                 []string        `json:"roles"`
	Tokens                V1AuthTokenPair `json:"tokens"`
	MFARequired           bool            `json:"mfaRequired,omitempty"`
	MFAEnrollmentRequired bool            `json:"mfaEnrollmentRequired,omitempty"`
	MFAChallengeToken     string          `json:"mfaChallengeToken,omitempty"`
	MFAExpiresAt          *string         `json:"mfaExpiresAt,omitempty"`
}

// V1AuthMeResponse documents GET /v1/auth/me success.
type V1AuthMeResponse struct {
	AccountID      string   `json:"accountId"`
	OrganizationID string   `json:"organizationId"`
	Email          string   `json:"email"`
	Roles          []string `json:"roles"`
}

// V1AuthRefreshRequest is the refresh body.
type V1AuthRefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// V1AuthRefreshResponse is POST /v1/auth/refresh success.
type V1AuthRefreshResponse struct {
	Tokens V1AuthTokenPair `json:"tokens"`
}

// V1AuthLogoutRequest optionally revokes a single refresh token or all sessions.
type V1AuthLogoutRequest struct {
	RefreshToken string `json:"refreshToken,omitempty"`
	RevokeAll    bool   `json:"revokeAll,omitempty"`
}

// V1AuthChangePasswordRequest is POST /v1/auth/change-password (Bearer session).
type V1AuthChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type V1AuthPasswordResetRequest struct {
	OrganizationID string `json:"organizationId"`
	Email          string `json:"email"`
}

type V1AuthPasswordResetAccepted struct {
	Accepted bool `json:"accepted"`
}

type V1AuthPasswordResetConfirmRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

// V1AuthMFAEnrollResponse documents POST /v1/auth/mfa/totp/enroll.
type V1AuthMFAEnrollResponse struct {
	OTPAuthURI string `json:"otpauthUri"`
	Secret     string `json:"secret"`
}

// V1AuthMFAVerifyRequest documents POST /v1/auth/mfa/totp/verify.
type V1AuthMFAVerifyRequest struct {
	Code string `json:"code"`
}

// V1AuthMFADisableRequest documents POST /v1/auth/mfa/totp/disable.
type V1AuthMFADisableRequest struct {
	CurrentPassword string `json:"currentPassword"`
	TOTPCode        string `json:"totpCode"`
}

// V1AuthSessionsEnvelope documents GET /v1/auth/sessions.
type V1AuthSessionsEnvelope struct {
	Sessions []V1AuthSessionItem `json:"sessions"`
}

// V1AuthSessionItem is one admin console session row (no refresh material).
type V1AuthSessionItem struct {
	SessionID      string  `json:"sessionId"`
	OrganizationID string  `json:"organizationId"`
	IPAddress      *string `json:"ipAddress,omitempty"`
	UserAgent      *string `json:"userAgent,omitempty"`
	CreatedAt      string  `json:"createdAt"`
	LastUsedAt     *string `json:"lastUsedAt,omitempty"`
	ExpiresAt      string  `json:"expiresAt"`
	Status         string  `json:"status"`
}

// V1AuthRevokeOtherSessionsRequest documents DELETE /v1/auth/sessions.
type V1AuthRevokeOtherSessionsRequest struct {
	ExceptRefreshToken string `json:"exceptRefreshToken"`
}

// V1AdminAuthSessionsEnvelope documents GET .../users/{id}/sessions.
type V1AdminAuthSessionsEnvelope struct {
	Sessions []V1AuthSessionItem `json:"sessions"`
}

// V1AdminAuthAccount is one row under GET/PATCH /v1/admin/auth/users (no password fields).
type V1AdminAuthAccount struct {
	AccountID      string   `json:"accountId"`
	OrganizationID string   `json:"organizationId"`
	Email          string   `json:"email"`
	Roles          []string `json:"roles"`
	Status         string   `json:"status"`
	CreatedAt      string   `json:"createdAt"`
	UpdatedAt      string   `json:"updatedAt"`
}

// V1AdminAuthUsersListEnvelope is GET /v1/admin/auth/users success.
type V1AdminAuthUsersListEnvelope struct {
	Items []V1AdminAuthAccount `json:"items"`
	Meta  V1CollectionListMeta `json:"meta"`
	// RbacReference is documentation-only in OpenAPI; production responses omit this field.
	RbacReference *V1RBACPermissionMatrixDoc `json:"rbacReference,omitempty"`
}

// V1AdminAuthUsersCreateRequest is POST /v1/admin/auth/users.
type V1AdminAuthUsersCreateRequest struct {
	Email    string   `json:"email"`
	Password string   `json:"password"`
	Roles    []string `json:"roles"`
	Status   string   `json:"status,omitempty"`
}

// V1AdminAuthUsersPatchRequest is PATCH /v1/admin/auth/users/{accountId}.
type V1AdminAuthUsersPatchRequest struct {
	Email  *string   `json:"email,omitempty"`
	Roles  *[]string `json:"roles,omitempty"`
	Status *string   `json:"status,omitempty"`
}

// V1AdminAuthUsersStatusPatchRequest is PATCH .../users/{id}/status (status transitions without generic PATCH).
type V1AdminAuthUsersStatusPatchRequest struct {
	Status string `json:"status"`
}

// V1AdminAuthResetPasswordRequest is POST .../reset-password.
type V1AdminAuthResetPasswordRequest struct {
	Password string `json:"password"`
}

// V1RBACPermissionMatrixDoc is documentation-only metadata for OpenAPI (no HTTP route). Authoritative mapping lives in internal/platform/auth/permissions.go and internal/platform/auth/admin_rbac.go (tenant scoping).
type V1RBACPermissionMatrixDoc struct {
	PermissionExamples []string `json:"permissionExamples" example:"user:read,user:write,user:roles,user:sessions:revoke,catalog:read,fleet:read,payment:read,payment:refund,report:read,audit:read,machine:command,setup:machine,technician:operate"`
	RoleSummary        string   `json:"roleSummary" example:"platform_admin→admin.all (any org via explicit organizationId in URL or org query); org_admin→org matrix only for JWT org_id; org_member/viewer→read-only baseline"`
	AuditActionsNote   string   `json:"auditActionsNote" example:"User admin mutations emit auth.user.* and role.changed; auth.login.success/failed; MFA TOTP emits auth.mfa.*; session revoke emits auth.session.* and user.sessions.revoked"`
}

// --- Enterprise audit (audit_events) ---

// V1EnterpriseAuditEvent is one append-only row from GET /v1/admin/audit/events.
type V1EnterpriseAuditEvent struct {
	ID             string          `json:"id"`
	OrganizationID string          `json:"organizationId"`
	ActorType      string          `json:"actorType"`
	ActorID        *string         `json:"actorId,omitempty"`
	Action         string          `json:"action"`
	ResourceType   string          `json:"resourceType"`
	ResourceID     *string         `json:"resourceId,omitempty"`
	MachineID      *string         `json:"machineId,omitempty"`
	SiteID         *string         `json:"siteId,omitempty"`
	RequestID      *string         `json:"requestId,omitempty"`
	TraceID        *string         `json:"traceId,omitempty"`
	IPAddress      *string         `json:"ipAddress,omitempty"`
	UserAgent      *string         `json:"userAgent,omitempty"`
	BeforeJSON     json.RawMessage `json:"beforeJson,omitempty"`
	AfterJSON      json.RawMessage `json:"afterJson,omitempty"`
	Metadata       json.RawMessage `json:"metadata"`
	Outcome        string          `json:"outcome"`
	OccurredAt     string          `json:"occurredAt"`
	CreatedAt      string          `json:"createdAt"`
}

// V1EnterpriseAuditEventsListEnvelope is GET /v1/admin/audit/events success.
type V1EnterpriseAuditEventsListEnvelope struct {
	Items []V1EnterpriseAuditEvent `json:"items"`
	Meta  V1CollectionListMeta     `json:"meta"`
}

// V1AdminOutboxPipelineStats is a snapshot from outbox_events for platform operators.
type V1AdminOutboxPipelineStats struct {
	PendingTotal           int64   `json:"pendingTotal"`
	PendingDueNow          int64   `json:"pendingDueNow"`
	DeadLetteredTotal      int64   `json:"deadLetteredTotal"`
	PublishingLeasedTotal  int64   `json:"publishingLeasedTotal"`
	MaxPendingAttempts     int64   `json:"maxPendingAttempts"`
	OldestPendingCreatedAt *string `json:"oldestPendingCreatedAt,omitempty"`
}

// V1AdminOutboxRow is one row from GET /v1/admin/ops/outbox or GET /v1/admin/system/outbox.
type V1AdminOutboxRow struct {
	ID                   int64           `json:"id"`
	OrganizationID       *string         `json:"organizationId,omitempty"`
	Topic                string          `json:"topic"`
	EventType            string          `json:"eventType"`
	Payload              json.RawMessage `json:"payload"`
	AggregateType        string          `json:"aggregateType"`
	AggregateID          string          `json:"aggregateId"`
	IdempotencyKey       *string         `json:"idempotencyKey,omitempty"`
	CreatedAt            string          `json:"createdAt"`
	PublishedAt          *string         `json:"publishedAt,omitempty"`
	PublishAttemptCount  int32           `json:"publishAttemptCount"`
	Attempts             int32           `json:"attempts"`
	MaxAttempts          int32           `json:"maxAttempts"`
	LastPublishError     *string         `json:"lastPublishError,omitempty"`
	LastPublishAttemptAt *string         `json:"lastPublishAttemptAt,omitempty"`
	NextPublishAfter     *string         `json:"nextPublishAfter,omitempty"`
	NextAttemptAt        *string         `json:"nextAttemptAt,omitempty"`
	DeadLetteredAt       *string         `json:"deadLetteredAt,omitempty"`
	Status               string          `json:"status"`
	LockedBy             *string         `json:"lockedBy,omitempty"`
	LockedUntil          *string         `json:"lockedUntil,omitempty"`
	UpdatedAt            string          `json:"updatedAt"`
}

// V1AdminOutboxStatsEnvelope is GET /v1/admin/system/outbox/stats.
type V1AdminOutboxStatsEnvelope struct {
	Stats V1AdminOutboxPipelineStats `json:"stats"`
}

// V1AdminOutboxMarkDLQEnvelope is POST /v1/admin/system/outbox/{eventId}/mark-dlq.
type V1AdminOutboxMarkDLQEnvelope struct {
	Marked bool `json:"marked"`
}

// V1AdminOutboxOpsEnvelope is GET /v1/admin/ops/outbox success.
type V1AdminOutboxOpsEnvelope struct {
	Stats V1AdminOutboxPipelineStats `json:"stats"`
	Rows  []V1AdminOutboxRow         `json:"rows"`
	Meta  V1CollectionListMeta       `json:"meta"`
}

// V1AdminOutboxRetryEnvelope is POST /v1/admin/ops/outbox/{outboxId}/retry success.
type V1AdminOutboxRetryEnvelope struct {
	Retried bool `json:"retried"`
}

// V1AdminRetentionTableStatus is one table summary from GET /v1/admin/ops/retention.
type V1AdminRetentionTableStatus struct {
	TableName           string  `json:"tableName"`
	TotalRows           int64   `json:"totalRows"`
	OldestRecordAt      *string `json:"oldestRecordAt,omitempty"`
	OldestRecordAgeDays *int64  `json:"oldestRecordAgeDays,omitempty"`
}

// V1AdminRetentionOpsEnvelope is GET /v1/admin/ops/retention success.
type V1AdminRetentionOpsEnvelope struct {
	Tables []V1AdminRetentionTableStatus `json:"tables"`
}

// V1AdminSystemRetentionStatsEnvelope is GET /v1/admin/system/retention/stats success.
type V1AdminSystemRetentionStatsEnvelope struct {
	Tables []V1AdminSystemRetentionTableRow `json:"tables"`
	Policy V1AdminRetentionPolicySnapshot   `json:"policy"`
	Flags  V1AdminRetentionRuntimeFlags     `json:"runtime"`
}

// V1AdminSystemRetentionTableRow is one footprint row for system retention stats.
type V1AdminSystemRetentionTableRow struct {
	TableName      string  `json:"tableName"`
	TotalRows      int64   `json:"totalRows"`
	OldestRecordAt *string `json:"oldestRecordAt,omitempty"`
}

// V1AdminRetentionPolicySnapshot matches configured horizons (days).
type V1AdminRetentionPolicySnapshot struct {
	TelemetryRetentionDays           int `json:"telemetryRetentionDays"`
	TelemetryCriticalRetentionDays   int `json:"telemetryCriticalRetentionDays"`
	AuditRetentionDays               int `json:"auditRetentionDays"`
	CommandRetentionDays             int `json:"commandRetentionDays"`
	CommandReceiptRetentionDays      int `json:"commandReceiptRetentionDays"`
	PaymentWebhookEventRetentionDays int `json:"paymentWebhookEventRetentionDays"`
	OutboxPublishedRetentionDays     int `json:"outboxPublishedRetentionDays"`
	ProcessedMessageRetentionDays    int `json:"processedMessageRetentionDays"`
	OfflineEventRetentionDays        int `json:"offlineEventRetentionDays"`
	InventoryEventRetentionDays      int `json:"inventoryEventRetentionDays"`
}

// V1AdminRetentionRuntimeFlags matches worker gates and APP_ENV destructive guard.
type V1AdminRetentionRuntimeFlags struct {
	EnableRetentionWorker       bool `json:"enableRetentionWorker"`
	TelemetryCleanupEnabled     bool `json:"telemetryCleanupEnabled"`
	EnterpriseCleanupEnabled    bool `json:"enterpriseCleanupEnabled"`
	GlobalDryRun                bool `json:"globalDryRun"`
	DestructiveRetentionAllowed bool `json:"destructiveRetentionAllowed"`
}

// V1AdminSystemRetentionRunEnvelope is POST /v1/admin/system/retention/dry-run or /run success.
type V1AdminSystemRetentionRunEnvelope struct {
	Telemetry           V1AdminSystemRetentionTelemetryOutcome  `json:"telemetry"`
	Enterprise          V1AdminSystemRetentionEnterpriseOutcome `json:"enterprise"`
	OverallDryRun       bool                                    `json:"overallDryRun"`
	WouldModifyDatabase bool                                    `json:"wouldModifyDatabase"`
}

// V1AdminSystemRetentionTelemetryOutcome is telemetry subsystem retention output.
type V1AdminSystemRetentionTelemetryOutcome struct {
	Enabled bool             `json:"enabled"`
	DryRun  bool             `json:"dryRun"`
	Stages  map[string]int64 `json:"stages,omitempty"`
}

// V1AdminSystemRetentionEnterpriseOutcome is enterprise subsystem retention output.
type V1AdminSystemRetentionEnterpriseOutcome struct {
	Enabled    bool             `json:"enabled"`
	DryRun     bool             `json:"dryRun"`
	Candidates map[string]int64 `json:"candidates,omitempty"`
	Deleted    map[string]int64 `json:"deleted,omitempty"`
}

// --- Admin catalog (read-only) ---

// V1AdminPageMeta is pagination metadata for admin catalog lists.
type V1AdminPageMeta struct {
	Limit      int32 `json:"limit"`
	Offset     int32 `json:"offset"`
	Returned   int   `json:"returned"`
	TotalCount int64 `json:"totalCount"`
}

// V1AdminProductListItem is a row in GET /v1/admin/products.
type V1AdminProductListItem struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organizationId"`
	Sku            string  `json:"sku"`
	Barcode        *string `json:"barcode,omitempty"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	Active         bool    `json:"active"`
	CategoryID     *string `json:"categoryId,omitempty"`
	BrandID        *string `json:"brandId,omitempty"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

// V1AdminProductListEnvelope matches listAdminProducts success JSON.
type V1AdminProductListEnvelope struct {
	Items []V1AdminProductListItem `json:"items"`
	Meta  V1AdminPageMeta          `json:"meta"`
}

// V1AdminProduct is GET /v1/admin/products/{productId} success.
type V1AdminProduct struct {
	ID             string          `json:"id"`
	OrganizationID string          `json:"organizationId"`
	Sku            string          `json:"sku"`
	Barcode        *string         `json:"barcode,omitempty"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Attrs          json.RawMessage `json:"attrs,omitempty"`
	Active         bool            `json:"active"`
	CategoryID     *string         `json:"categoryId,omitempty"`
	BrandID        *string         `json:"brandId,omitempty"`
	PrimaryImageID *string         `json:"primaryImageId,omitempty"`
	// ImageURL matches DisplayURL when a primary image exists (compat alias).
	ImageURL        *string  `json:"imageUrl,omitempty"`
	DisplayURL      *string  `json:"displayUrl,omitempty"`
	ThumbURL        *string  `json:"thumbUrl,omitempty"`
	CountryOfOrigin *string  `json:"countryOfOrigin,omitempty"`
	AgeRestricted   bool     `json:"ageRestricted"`
	AllergenCodes   []string `json:"allergenCodes"`
	NutritionalNote *string  `json:"nutritionalNote,omitempty"`
	CreatedAt       string   `json:"createdAt"`
	UpdatedAt       string   `json:"updatedAt"`
}

// V1AdminProductMutationRequest is the body for POST/PUT/PATCH /v1/admin/products.
type V1AdminProductMutationRequest struct {
	Sku             string          `json:"sku"`
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	Attrs           json.RawMessage `json:"attrs,omitempty"`
	Active          bool            `json:"active"`
	CategoryID      *string         `json:"categoryId,omitempty"`
	BrandID         *string         `json:"brandId,omitempty"`
	Barcode         *string         `json:"barcode,omitempty"`
	CountryOfOrigin *string         `json:"countryOfOrigin,omitempty"`
	AgeRestricted   bool            `json:"ageRestricted"`
	AllergenCodes   []string        `json:"allergenCodes,omitempty"`
	NutritionalNote *string         `json:"nutritionalNote,omitempty"`
}

// V1AdminBrand is a brand row.
type V1AdminBrand struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	Active         bool   `json:"active"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// V1AdminBrandListEnvelope is GET /v1/admin/brands.
type V1AdminBrandListEnvelope struct {
	Items []V1AdminBrand  `json:"items"`
	Meta  V1AdminPageMeta `json:"meta"`
}

// V1AdminBrandMutationRequest is POST/PUT/PATCH /v1/admin/brands.
type V1AdminBrandMutationRequest struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// V1AdminCategory is a category row.
type V1AdminCategory struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organizationId"`
	Slug           string  `json:"slug"`
	Name           string  `json:"name"`
	ParentID       *string `json:"parentId,omitempty"`
	Active         bool    `json:"active"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

// V1AdminCategoryListEnvelope is GET /v1/admin/categories.
type V1AdminCategoryListEnvelope struct {
	Items []V1AdminCategory `json:"items"`
	Meta  V1AdminPageMeta   `json:"meta"`
}

// V1AdminCategoryMutationRequest is POST/PUT/PATCH /v1/admin/categories.
type V1AdminCategoryMutationRequest struct {
	Slug     string  `json:"slug"`
	Name     string  `json:"name"`
	ParentID *string `json:"parentId,omitempty"`
	Active   bool    `json:"active"`
}

// V1AdminTag is a tag row.
type V1AdminTag struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	Active         bool   `json:"active"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// V1AdminTagListEnvelope is GET /v1/admin/tags.
type V1AdminTagListEnvelope struct {
	Items []V1AdminTag    `json:"items"`
	Meta  V1AdminPageMeta `json:"meta"`
}

// V1AdminTagMutationRequest is POST/PUT/PATCH /v1/admin/tags.
type V1AdminTagMutationRequest struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// V1AdminProductImageBindRequest binds CDN image URLs to a product (primary image).
type V1AdminProductImageBindRequest struct {
	ArtifactID  string `json:"artifactId"`
	ThumbURL    string `json:"thumbUrl"`
	DisplayURL  string `json:"displayUrl"`
	ContentHash string `json:"contentHash,omitempty"`
	Width       int32  `json:"width,omitempty"`
	Height      int32  `json:"height,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// V1AdminMediaUploadInitRequest is POST /v1/admin/media/uploads.
type V1AdminMediaUploadInitRequest struct {
	ContentType string `json:"content_type"`
}

// V1AdminMediaUploadInitResponse returns presigned PUT targets for the original object.
type V1AdminMediaUploadInitResponse struct {
	MediaID       string              `json:"media_id"`
	UploadURL     string              `json:"upload_url"`
	UploadMethod  string              `json:"upload_method"`
	UploadHeaders map[string][]string `json:"upload_headers"`
	ExpiresAt     string              `json:"expires_at"`
	CompletePath  string              `json:"complete_path"`
}

// V1AdminMediaUploadCompleteRequest finalizes an organization-scoped upload.
type V1AdminMediaUploadCompleteRequest struct {
	MediaID string `json:"media_id"`
}

// V1AdminMediaAsset is a row in GET /v1/admin/media (no raw object keys).
type V1AdminMediaAsset struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Kind           string `json:"kind"`
	Status         string `json:"status"`
	MimeType       string `json:"mime_type,omitempty"`
	SizeBytes      int64  `json:"size_bytes,omitempty"`
	Sha256         string `json:"sha256,omitempty"`
	Width          int32  `json:"width,omitempty"`
	Height         int32  `json:"height,omitempty"`
	ObjectVersion  int32  `json:"object_version"`
	Etag           string `json:"etag,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// V1AdminMediaListEnvelope is GET /v1/admin/media.
type V1AdminMediaListEnvelope struct {
	Items []V1AdminMediaAsset `json:"items"`
	Meta  V1AdminPageMeta     `json:"meta"`
}

// V1AdminProductMediaBindRequest binds a ready media_assets row as primary product image.
type V1AdminProductMediaBindRequest struct {
	MediaID string `json:"media_id"`
}

// V1AdminProductImagePatchRequest updates product-image presentation metadata.
type V1AdminProductImagePatchRequest struct {
	SortOrder *int32  `json:"sort_order,omitempty"`
	IsPrimary *bool   `json:"is_primary,omitempty"`
	AltText   *string `json:"alt_text,omitempty"`
}

// V1AdminPriceBook is a row in GET /v1/admin/price-books.
type V1AdminPriceBook struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organizationId"`
	Name           string  `json:"name"`
	Currency       string  `json:"currency"`
	EffectiveFrom  string  `json:"effectiveFrom"`
	EffectiveTo    *string `json:"effectiveTo,omitempty"`
	IsDefault      bool    `json:"isDefault"`
	Active         bool    `json:"active"`
	ScopeType      string  `json:"scopeType"`
	SiteID         *string `json:"siteId,omitempty"`
	MachineID      *string `json:"machineId,omitempty"`
	Priority       int32   `json:"priority"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

// V1AdminPriceBookWriteRequest is POST /v1/admin/price-books.
type V1AdminPriceBookWriteRequest struct {
	Name          string  `json:"name"`
	Currency      string  `json:"currency"`
	EffectiveFrom string  `json:"effectiveFrom"`
	EffectiveTo   *string `json:"effectiveTo,omitempty"`
	IsDefault     bool    `json:"isDefault"`
	ScopeType     string  `json:"scopeType"`
	SiteID        *string `json:"siteId,omitempty"`
	MachineID     *string `json:"machineId,omitempty"`
	Priority      int32   `json:"priority"`
}

// V1AdminPriceBookPatchRequest is PATCH /v1/admin/price-books/{priceBookId}.
// Send effectiveTo as empty string to clear the upper bound.
type V1AdminPriceBookPatchRequest struct {
	Name          *string `json:"name,omitempty"`
	Currency      *string `json:"currency,omitempty"`
	EffectiveFrom *string `json:"effectiveFrom,omitempty"`
	EffectiveTo   *string `json:"effectiveTo,omitempty"`
	IsDefault     *bool   `json:"isDefault,omitempty"`
	Active        *bool   `json:"active,omitempty"`
	ScopeType     *string `json:"scopeType,omitempty"`
	SiteID        *string `json:"siteId,omitempty"`
	MachineID     *string `json:"machineId,omitempty"`
	Priority      *int32  `json:"priority,omitempty"`
}

// V1AdminPriceBookItemRow is one catalog price line.
type V1AdminPriceBookItemRow struct {
	ProductID      string `json:"productId"`
	UnitPriceMinor int64  `json:"unitPriceMinor"`
}

// V1AdminPriceBookItemsPutRequest is PUT .../items (replaces all lines).
type V1AdminPriceBookItemsPutRequest struct {
	Items []V1AdminPriceBookItemRow `json:"items"`
}

// V1AdminPriceBookAssignTargetRequest is POST .../assign-target.
type V1AdminPriceBookAssignTargetRequest struct {
	SiteID    *string `json:"siteId,omitempty"`
	MachineID *string `json:"machineId,omitempty"`
}

// V1AdminPricingPreviewRequest is POST /v1/admin/pricing/preview.
type V1AdminPricingPreviewRequest struct {
	MachineID  *string  `json:"machineId,omitempty"`
	SiteID     *string  `json:"siteId,omitempty"`
	ProductIDs []string `json:"productIds"`
	At         *string  `json:"at,omitempty"`
}

// V1AdminPricingPreviewLine is one resolved row for preview.
type V1AdminPricingPreviewLine struct {
	ProductID      string   `json:"productId"`
	BasePrice      int64    `json:"basePrice"`
	EffectivePrice int64    `json:"effectivePrice"`
	Currency       string   `json:"currency"`
	PriceBookID    string   `json:"priceBookId"`
	AppliedRuleIDs []string `json:"appliedRuleIds"`
	Reasons        []string `json:"reasons"`
}

// V1AdminPricingPreviewResponse is POST /v1/admin/pricing/preview success body.
type V1AdminPricingPreviewResponse struct {
	At       string                      `json:"at"`
	Currency string                      `json:"currency"`
	Lines    []V1AdminPricingPreviewLine `json:"lines"`
}

// --- Admin promotions ---

// V1AdminPromotion is a promotion header row.
type V1AdminPromotion struct {
	ID               string  `json:"id"`
	OrganizationID   string  `json:"organizationId"`
	Name             string  `json:"name"`
	ApprovalStatus   string  `json:"approvalStatus"`
	LifecycleStatus  string  `json:"lifecycleStatus"`
	Priority         int32   `json:"priority"`
	Stackable        bool    `json:"stackable"`
	StartsAt         string  `json:"startsAt"`
	EndsAt           string  `json:"endsAt"`
	BudgetLimitMinor *int64  `json:"budgetLimitMinor,omitempty"`
	RedemptionLimit  *int32  `json:"redemptionLimit,omitempty"`
	ChannelScope     *string `json:"channelScope,omitempty"`
	CreatedAt        string  `json:"createdAt"`
	UpdatedAt        string  `json:"updatedAt"`
}

// V1AdminPromotionRule is one discount rule on a promotion.
type V1AdminPromotionRule struct {
	ID          string          `json:"id"`
	PromotionID string          `json:"promotionId"`
	RuleType    string          `json:"ruleType"`
	Priority    int32           `json:"priority"`
	Payload     json.RawMessage `json:"payload"`
}

// V1AdminPromotionTarget scopes a promotion (exactly one FK set per row matches target_type).
type V1AdminPromotionTarget struct {
	ID                   string  `json:"id"`
	PromotionID          string  `json:"promotionId"`
	OrganizationID       string  `json:"organizationId"`
	TargetType           string  `json:"targetType"`
	ProductID            *string `json:"productId,omitempty"`
	CategoryID           *string `json:"categoryId,omitempty"`
	MachineID            *string `json:"machineId,omitempty"`
	SiteID               *string `json:"siteId,omitempty"`
	OrganizationTargetID *string `json:"organizationTargetId,omitempty"`
	TagID                *string `json:"tagId,omitempty"`
	CreatedAt            string  `json:"createdAt"`
}

// V1AdminPromotionDetail is GET /v1/admin/promotions/{promotionId}.
type V1AdminPromotionDetail struct {
	Promotion V1AdminPromotion         `json:"promotion"`
	Rules     []V1AdminPromotionRule   `json:"rules"`
	Targets   []V1AdminPromotionTarget `json:"targets"`
}

// V1AdminPromotionListEnvelope is GET /v1/admin/promotions.
type V1AdminPromotionListEnvelope struct {
	Items []V1AdminPromotion `json:"items"`
	Meta  V1AdminPageMeta    `json:"meta"`
}

// V1AdminPromotionRuleInput binds a rule on create/patch.
type V1AdminPromotionRuleInput struct {
	RuleType string          `json:"ruleType"`
	Priority int32           `json:"priority"`
	Payload  json.RawMessage `json:"payload"`
}

// V1AdminPromotionCreateRequest is POST /v1/admin/promotions.
type V1AdminPromotionCreateRequest struct {
	Name             string                      `json:"name"`
	StartsAt         string                      `json:"startsAt"`
	EndsAt           string                      `json:"endsAt"`
	Priority         int32                       `json:"priority"`
	Stackable        bool                        `json:"stackable"`
	BudgetLimitMinor *int64                      `json:"budgetLimitMinor,omitempty"`
	RedemptionLimit  *int32                      `json:"redemptionLimit,omitempty"`
	ChannelScope     *string                     `json:"channelScope,omitempty"`
	Rules            []V1AdminPromotionRuleInput `json:"rules,omitempty"`
}

// V1AdminPromotionPatchRequest is PATCH /v1/admin/promotions/{promotionId}.
type V1AdminPromotionPatchRequest struct {
	Name             *string                      `json:"name,omitempty"`
	StartsAt         *string                      `json:"startsAt,omitempty"`
	EndsAt           *string                      `json:"endsAt,omitempty"`
	Priority         *int32                       `json:"priority,omitempty"`
	Stackable        *bool                        `json:"stackable,omitempty"`
	BudgetLimitMinor *int64                       `json:"budgetLimitMinor,omitempty"`
	RedemptionLimit  *int32                       `json:"redemptionLimit,omitempty"`
	ChannelScope     *string                      `json:"channelScope,omitempty"`
	ApprovalStatus   *string                      `json:"approvalStatus,omitempty"`
	Rules            *[]V1AdminPromotionRuleInput `json:"rules,omitempty"`
}

// V1AdminPromotionAssignTargetRequest is POST .../assign-target.
type V1AdminPromotionAssignTargetRequest struct {
	TargetType           string  `json:"targetType"`
	ProductID            *string `json:"productId,omitempty"`
	CategoryID           *string `json:"categoryId,omitempty"`
	MachineID            *string `json:"machineId,omitempty"`
	SiteID               *string `json:"siteId,omitempty"`
	OrganizationTargetID *string `json:"organizationTargetId,omitempty"`
	TagID                *string `json:"tagId,omitempty"`
}

// V1AdminPromotionPreviewRequest is POST /v1/admin/promotions/preview.
type V1AdminPromotionPreviewRequest struct {
	MachineID  *string  `json:"machineId,omitempty"`
	SiteID     *string  `json:"siteId,omitempty"`
	ProductIDs []string `json:"productIds"`
	At         *string  `json:"at,omitempty"`
}

// V1AdminPromotionSkippedRule explains a skipped rule in preview.
type V1AdminPromotionSkippedRule struct {
	PromotionID string `json:"promotionId"`
	RuleID      string `json:"ruleId,omitempty"`
	RuleType    string `json:"ruleType"`
	Reason      string `json:"reason"`
}

// V1AdminPromotionPreviewLine is one product row after promotions.
type V1AdminPromotionPreviewLine struct {
	ProductID           string                        `json:"productId"`
	BasePriceMinor      int64                         `json:"basePriceMinor"`
	DiscountMinor       int64                         `json:"discountMinor"`
	FinalPriceMinor     int64                         `json:"finalPriceMinor"`
	Currency            string                        `json:"currency"`
	AppliedPromotionIDs []string                      `json:"appliedPromotionIds"`
	AppliedRuleIDs      []string                      `json:"appliedRuleIds"`
	SkippedRules        []V1AdminPromotionSkippedRule `json:"skippedRules"`
}

// V1AdminPromotionPreviewResponse is POST /v1/admin/promotions/preview success body.
type V1AdminPromotionPreviewResponse struct {
	At    string                        `json:"at"`
	Lines []V1AdminPromotionPreviewLine `json:"lines"`
}

// V1AdminPriceBookListEnvelope matches listAdminPriceBooks success JSON.
type V1AdminPriceBookListEnvelope struct {
	Items []V1AdminPriceBook `json:"items"`
	Meta  V1AdminPageMeta    `json:"meta"`
}

// V1AdminPlanogram is a planogram summary row.
type V1AdminPlanogram struct {
	ID             string          `json:"id"`
	OrganizationID string          `json:"organizationId"`
	Name           string          `json:"name"`
	Revision       int32           `json:"revision"`
	Status         string          `json:"status"`
	Meta           json.RawMessage `json:"meta,omitempty"`
	CreatedAt      string          `json:"createdAt"`
}

// V1AdminPlanogramListEnvelope matches GET /v1/admin/planograms.
type V1AdminPlanogramListEnvelope struct {
	Items []V1AdminPlanogram `json:"items"`
	Meta  V1AdminPageMeta    `json:"meta"`
}

// V1AdminPlanogramSlot is a slot assignment on a planogram.
type V1AdminPlanogramSlot struct {
	ID          string  `json:"id"`
	PlanogramID string  `json:"planogramId"`
	SlotIndex   int32   `json:"slotIndex"`
	ProductID   *string `json:"productId,omitempty"`
	MaxQuantity int32   `json:"maxQuantity"`
	ProductSku  *string `json:"productSku,omitempty"`
	ProductName *string `json:"productName,omitempty"`
	CreatedAt   string  `json:"createdAt"`
}

// V1AdminPlanogramDetail is GET /v1/admin/planograms/{planogramId} (includes slot layout).
type V1AdminPlanogramDetail struct {
	Planogram V1AdminPlanogram       `json:"planogram"`
	Slots     []V1AdminPlanogramSlot `json:"slots"`
}

// --- Admin inventory (read-only) ---

// V1AdminMachineSlot is a machine slot projection with catalog joins.
type V1AdminMachineSlot struct {
	MachineID                string  `json:"machineId"`
	MachineName              string  `json:"machineName"`
	MachineStatus            string  `json:"machineStatus"`
	PlanogramID              string  `json:"planogramId"`
	PlanogramName            string  `json:"planogramName"`
	SlotIndex                int32   `json:"slotIndex"`
	CabinetCode              string  `json:"cabinetCode"`
	CabinetIndex             int32   `json:"cabinetIndex"`
	SlotCode                 string  `json:"slotCode"`
	CurrentQuantity          int32   `json:"currentQuantity"`
	CurrentStock             int32   `json:"currentStock"`
	MaxQuantity              int32   `json:"maxQuantity"`
	Capacity                 int32   `json:"capacity"`
	ParLevel                 int32   `json:"parLevel"`
	LowStockThreshold        int32   `json:"lowStockThreshold"`
	PriceMinor               int64   `json:"priceMinor"`
	Currency                 string  `json:"currency"`
	Status                   string  `json:"status"`
	PlanogramRevisionApplied int32   `json:"planogramRevisionApplied"`
	UpdatedAt                string  `json:"updatedAt"`
	ProductID                *string `json:"productId,omitempty"`
	ProductSku               *string `json:"productSku,omitempty"`
	ProductName              *string `json:"productName,omitempty"`
	IsEmpty                  bool    `json:"isEmpty"`
	LowStock                 bool    `json:"lowStock"`
}

// V1AdminStockAdjustmentItem is one slot adjustment in POST /v1/admin/machines/{machineId}/stock-adjustments.
type V1AdminStockAdjustmentItem struct {
	PlanogramID    string  `json:"planogramId"`
	SlotIndex      int32   `json:"slotIndex"`
	QuantityBefore int32   `json:"quantityBefore"`
	QuantityAfter  int32   `json:"quantityAfter"`
	CabinetCode    string  `json:"cabinetCode,omitempty"`
	SlotCode       string  `json:"slotCode,omitempty"`
	ProductID      *string `json:"productId,omitempty"`
}

// V1AdminStockAdjustmentsRequest is POST /v1/admin/machines/{machineId}/stock-adjustments.
// See docs/api/inventory-adjustments.md and OpenAPI examples on that path.
type V1AdminStockAdjustmentsRequest struct {
	OperatorSessionID string                       `json:"operator_session_id"`
	Reason            string                       `json:"reason"`
	OccurredAt        *string                      `json:"occurredAt,omitempty"`
	Items             []V1AdminStockAdjustmentItem `json:"items"`
}

// V1AdminStockAdjustmentsResponse is the success body for stock adjustments.
type V1AdminStockAdjustmentsResponse struct {
	Replay   bool    `json:"replay"`
	EventIds []int64 `json:"eventIds,omitempty"`
}

// V1AdminInventoryEvent is one append-only inventory_events row (audit / refill / future vend).
type V1AdminInventoryEvent struct {
	ID                      int64   `json:"id"`
	OrganizationID          string  `json:"organizationId"`
	MachineID               string  `json:"machineId"`
	CabinetCode             *string `json:"cabinetCode,omitempty"`
	SlotCode                *string `json:"slotCode,omitempty"`
	ProductID               *string `json:"productId,omitempty"`
	EventType               string  `json:"eventType"`
	ReasonCode              *string `json:"reasonCode,omitempty"`
	QuantityBefore          *int32  `json:"quantityBefore,omitempty"`
	QuantityDelta           int32   `json:"quantityDelta"`
	QuantityAfter           *int32  `json:"quantityAfter,omitempty"`
	UnitPriceMinor          int64   `json:"unitPriceMinor"`
	Currency                string  `json:"currency"`
	CorrelationID           *string `json:"correlationId,omitempty"`
	OperatorSessionID       *string `json:"operatorSessionId,omitempty"`
	TechnicianID            *string `json:"technicianId,omitempty"`
	TechnicianDisplayName   *string `json:"technicianDisplayName,omitempty"`
	RefillSessionID         *string `json:"refillSessionId,omitempty"`
	InventoryCountSessionID *string `json:"inventoryCountSessionId,omitempty"`
	OccurredAt              string  `json:"occurredAt"`
	RecordedAt              string  `json:"recordedAt"`
}

// V1AdminInventoryEventListEnvelope is GET /v1/admin/machines/{machineId}/inventory-events.
type V1AdminInventoryEventListEnvelope struct {
	Items []V1AdminInventoryEvent `json:"items"`
}

// V1AdminMachineSlotListEnvelope is GET /v1/admin/machines/{machineId}/slots.
type V1AdminMachineSlotListEnvelope struct {
	Items []V1AdminMachineSlot `json:"items"`
}

// V1AdminMachineInventoryLine is a rolled-up inventory row per product.
type V1AdminMachineInventoryLine struct {
	MachineID          string  `json:"machineId"`
	MachineName        string  `json:"machineName"`
	MachineStatus      string  `json:"machineStatus"`
	ProductID          string  `json:"productId"`
	ProductName        string  `json:"productName"`
	ProductSku         string  `json:"productSku"`
	TotalQuantity      int64   `json:"totalQuantity"`
	SlotCount          int64   `json:"slotCount"`
	MaxCapacityAnySlot int32   `json:"maxCapacityAnySlot"`
	LowStock           bool    `json:"lowStock"`
	CabinetCode        *string `json:"cabinetCode,omitempty"`
	CabinetIndex       *int32  `json:"cabinetIndex,omitempty"`
}

// V1AdminMachineInventoryEnvelope is GET /v1/admin/machines/{machineId}/inventory.
type V1AdminMachineInventoryEnvelope struct {
	Items []V1AdminMachineInventoryLine `json:"items"`
}

// V1AdminInventoryRefillForecastMeta is pagination metadata for refill forecast lists.
type V1AdminInventoryRefillForecastMeta struct {
	Limit    int32 `json:"limit"`
	Offset   int32 `json:"offset"`
	Returned int   `json:"returned"`
	Total    int64 `json:"total"`
}

// V1AdminInventoryRefillForecastItem is one slot-level refill / velocity estimate.
type V1AdminInventoryRefillForecastItem struct {
	MachineID               string   `json:"machineId"`
	MachineName             string   `json:"machineName"`
	SiteID                  string   `json:"siteId"`
	SiteName                string   `json:"siteName"`
	PlanogramID             string   `json:"planogramId"`
	PlanogramName           string   `json:"planogramName"`
	SlotIndex               int32    `json:"slotIndex"`
	ProductID               string   `json:"productId"`
	ProductSku              string   `json:"productSku,omitempty"`
	ProductName             string   `json:"productName,omitempty"`
	CurrentQuantity         int32    `json:"currentQuantity"`
	MaxQuantity             int32    `json:"maxQuantity"`
	UnitsSoldInWindow       int64    `json:"unitsSoldInWindow"`
	DailyVelocity           float64  `json:"dailyVelocity"`
	DaysToEmpty             *float64 `json:"daysToEmpty,omitempty"`
	FillRatio               float64  `json:"fillRatio"`
	SuggestedRefillQuantity int32    `json:"suggestedRefillQuantity"`
	Urgency                 string   `json:"urgency"`
}

// V1AdminInventoryRefillForecastResponse is GET /v1/admin/inventory/low-stock,
// GET /v1/admin/inventory/refill-suggestions, and GET /v1/admin/machines/{machineId}/refill-suggestions.
type V1AdminInventoryRefillForecastResponse struct {
	OrganizationID     string                               `json:"organizationId"`
	VelocityWindowDays int                                  `json:"velocityWindowDays"`
	WindowStart        string                               `json:"windowStart"`
	WindowEnd          string                               `json:"windowEnd"`
	Items              []V1AdminInventoryRefillForecastItem `json:"items"`
	Meta               V1AdminInventoryRefillForecastMeta   `json:"meta"`
}

// --- Machine setup (technician bootstrap + admin topology / planogram) ---

// V1SetupMachineBootstrapResponse is GET /v1/setup/machines/{machineId}/bootstrap.
// Integration notes: docs/api/setup-machine.md; copy-paste examples in docs/swagger/swagger.json.
type V1SetupMachineBootstrapResponse struct {
	Machine                     V1SetupMachineSummary       `json:"machine"`
	Topology                    V1SetupTopology             `json:"topology"`
	Catalog                     V1SetupCatalog              `json:"catalog"`
	CatalogFingerprint          string                      `json:"catalogFingerprint,omitempty"`
	PricingFingerprint          string                      `json:"pricingFingerprint,omitempty"`
	PlanogramFingerprint        string                      `json:"planogramFingerprint,omitempty"`
	MediaFingerprint            string                      `json:"mediaFingerprint,omitempty"`
	PublishedPlanogramVersionID string                      `json:"publishedPlanogramVersionId,omitempty"`
	PublishedPlanogramVersionNo int32                       `json:"publishedPlanogramVersionNo,omitempty"`
	RuntimeHints                *V1SetupMachineRuntimeHints `json:"runtimeHints,omitempty"`
}

// V1SetupMachineRuntimeHints is optional machine-local rollout context (forward-compatible).
type V1SetupMachineRuntimeHints struct {
	FeatureFlags                 map[string]bool                     `json:"featureFlags,omitempty"`
	AppliedMachineConfigRevision int32                               `json:"appliedMachineConfigRevision,omitempty"`
	PendingMachineConfigRollouts []V1PendingMachineConfigRolloutHint `json:"pendingMachineConfigRollouts,omitempty"`
}

// V1PendingMachineConfigRolloutHint summarizes an active staged rollout affecting this machine.
type V1PendingMachineConfigRolloutHint struct {
	RolloutID          string `json:"rolloutId"`
	TargetVersionID    string `json:"targetVersionId"`
	TargetVersionLabel string `json:"targetVersionLabel,omitempty"`
	Status             string `json:"status"`
}

// V1SetupMachineSummary is machine identity for setup clients.
type V1SetupMachineSummary struct {
	MachineID         string  `json:"machineId"`
	OrganizationID    string  `json:"organizationId"`
	SiteID            string  `json:"siteId"`
	HardwareProfileID *string `json:"hardwareProfileId,omitempty"`
	SerialNumber      string  `json:"serialNumber"`
	Name              string  `json:"name"`
	Status            string  `json:"status"`
	CommandSequence   int64   `json:"commandSequence"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
}

// V1SetupTopology is nested cabinets with current slot assignments.
type V1SetupTopology struct {
	Cabinets []V1SetupTopologyCabinet `json:"cabinets"`
}

// V1SetupTopologyCabinet is one cabinet and its current slots.
type V1SetupTopologyCabinet struct {
	ID        string                `json:"id"`
	Code      string                `json:"code"`
	Title     string                `json:"title"`
	SortOrder int32                 `json:"sortOrder"`
	Metadata  json.RawMessage       `json:"metadata,omitempty"`
	Slots     []V1SetupTopologySlot `json:"slots"`
}

// V1SetupTopologySlot is a current cabinet slot config row.
type V1SetupTopologySlot struct {
	ConfigID          string          `json:"configId"`
	SlotCode          string          `json:"slotCode"`
	SlotIndex         *int32          `json:"slotIndex,omitempty"`
	ProductID         *string         `json:"productId,omitempty"`
	ProductSku        string          `json:"productSku"`
	ProductName       string          `json:"productName"`
	MaxQuantity       int32           `json:"maxQuantity"`
	PriceMinor        int64           `json:"priceMinor"`
	EffectiveFrom     string          `json:"effectiveFrom"`
	IsCurrent         bool            `json:"isCurrent"`
	MachineSlotLayout string          `json:"machineSlotLayoutId"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
}

// V1SetupCatalog lists assortment products available for slot assignment on this machine.
type V1SetupCatalog struct {
	Products []V1SetupCatalogProduct `json:"products"`
}

// V1SetupCatalogProduct is one assortment line for the machine's primary binding.
type V1SetupCatalogProduct struct {
	ProductID      string `json:"productId"`
	Sku            string `json:"sku"`
	Name           string `json:"name"`
	SortOrder      int32  `json:"sortOrder"`
	AssortmentID   string `json:"assortmentId"`
	AssortmentName string `json:"assortmentName"`
}

// V1AdminPlanogramPublishResponse is POST /v1/admin/machines/{machineId}/planograms/publish.
type V1AdminPlanogramPublishResponse struct {
	DesiredConfigVersion int32                       `json:"desiredConfigVersion"`
	PlanogramID          string                      `json:"planogramId"`
	PlanogramRevision    int32                       `json:"planogramRevision"`
	Command              V1AdminPlanogramCommandInfo `json:"command"`
}

// V1AdminPlanogramCommandInfo summarizes the MQTT command ledger row after publish/sync dispatch.
type V1AdminPlanogramCommandInfo struct {
	CommandID     string `json:"commandId"`
	Sequence      int64  `json:"sequence"`
	DispatchState string `json:"dispatchState"`
	Replay        bool   `json:"replay"`
}

// V1AdminMachineSyncResponse is POST /v1/admin/machines/{machineId}/sync.
type V1AdminMachineSyncResponse struct {
	Command V1AdminPlanogramCommandInfo `json:"command"`
}

// --- Operational collection lists (GET /v1/orders, /v1/payments, /v1/admin/* lists) ---

// V1CollectionListMeta is shared pagination metadata (limit, offset, returned, total).
type V1CollectionListMeta struct {
	Limit    int32 `json:"limit"`
	Offset   int32 `json:"offset"`
	Returned int   `json:"returned"`
	Total    int64 `json:"total"`
}

// V1OrderListItem is one row in GET /v1/orders.
type V1OrderListItem struct {
	OrderID        string  `json:"orderId"`
	OrganizationID string  `json:"organizationId"`
	MachineID      string  `json:"machineId"`
	Status         string  `json:"status"`
	Currency       string  `json:"currency"`
	SubtotalMinor  int64   `json:"subtotalMinor"`
	TaxMinor       int64   `json:"taxMinor"`
	TotalMinor     int64   `json:"totalMinor"`
	IdempotencyKey *string `json:"idempotencyKey,omitempty"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

// V1OrdersListResponse is GET /v1/orders success body.
type V1OrdersListResponse struct {
	Items []V1OrderListItem    `json:"items"`
	Meta  V1CollectionListMeta `json:"meta"`
}

// V1PaymentListItem is one row in GET /v1/payments.
type V1PaymentListItem struct {
	PaymentID            string `json:"paymentId"`
	OrderID              string `json:"orderId"`
	OrganizationID       string `json:"organizationId"`
	MachineID            string `json:"machineId"`
	Provider             string `json:"provider"`
	PaymentState         string `json:"paymentState"`
	OrderStatus          string `json:"orderStatus"`
	AmountMinor          int64  `json:"amountMinor"`
	Currency             string `json:"currency"`
	ReconciliationStatus string `json:"reconciliationStatus"`
	SettlementStatus     string `json:"settlementStatus"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
}

// V1PaymentsListResponse is GET /v1/payments success body.
type V1PaymentsListResponse struct {
	Items []V1PaymentListItem  `json:"items"`
	Meta  V1CollectionListMeta `json:"meta"`
}

// V1CommerceReconciliationCase is an operator-visible payment/vend/refund review row.
type V1CommerceReconciliationCase struct {
	ID              string         `json:"id"`
	OrganizationID  string         `json:"organizationId"`
	CaseType        string         `json:"caseType"`
	Status          string         `json:"status"`
	Severity        string         `json:"severity"`
	OrderID         *string        `json:"orderId,omitempty"`
	PaymentID       *string        `json:"paymentId,omitempty"`
	VendSessionID   *string        `json:"vendSessionId,omitempty"`
	MachineID       *string        `json:"machineId,omitempty"`
	RefundID        *string        `json:"refundId,omitempty"`
	Provider        *string        `json:"provider,omitempty"`
	ProviderEventID *int64         `json:"providerEventId,omitempty"`
	Reason          string         `json:"reason"`
	Metadata        map[string]any `json:"metadata"`
	FirstDetectedAt string         `json:"firstDetectedAt"`
	LastDetectedAt  string         `json:"lastDetectedAt"`
	ResolvedAt      *string        `json:"resolvedAt,omitempty"`
	ResolvedBy      *string        `json:"resolvedBy,omitempty"`
	ResolutionNote  *string        `json:"resolutionNote,omitempty"`
}

// V1CommerceReconciliationListResponse is GET admin commerce reconciliation success body.
type V1CommerceReconciliationListResponse struct {
	Items []V1CommerceReconciliationCase `json:"items"`
	Meta  V1CollectionListMeta           `json:"meta"`
}

type V1CommerceReconciliationResolveRequest struct {
	Status string `json:"status"`
	Note   string `json:"note,omitempty"`
}

type V1CommerceReconciliationIgnoreRequest struct {
	Note string `json:"note,omitempty"`
}

type V1OrderTimelineEvent struct {
	ID         string         `json:"id"`
	EventType  string         `json:"eventType"`
	ActorType  string         `json:"actorType"`
	ActorID    *string        `json:"actorId,omitempty"`
	Payload    map[string]any `json:"payload"`
	OccurredAt string         `json:"occurredAt"`
	CreatedAt  string         `json:"createdAt"`
}

type V1OrderTimelineListResponse struct {
	Items []V1OrderTimelineEvent `json:"items"`
	Meta  V1CollectionListMeta   `json:"meta"`
}

type V1RefundRequestRow struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organizationId"`
	OrderID        string  `json:"orderId"`
	PaymentID      *string `json:"paymentId,omitempty"`
	RefundID       *string `json:"refundId,omitempty"`
	AmountMinor    int64   `json:"amountMinor"`
	Currency       string  `json:"currency"`
	Status         string  `json:"status"`
	Reason         *string `json:"reason,omitempty"`
	RequestedBy    *string `json:"requestedBy,omitempty"`
	IdempotencyKey *string `json:"idempotencyKey,omitempty"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
	CompletedAt    *string `json:"completedAt,omitempty"`
}

type V1RefundRequestsListResponse struct {
	Items []V1RefundRequestRow `json:"items"`
	Meta  V1CollectionListMeta `json:"meta"`
}

type V1AdminOrderRefundPostRequest struct {
	AmountMinor *int64 `json:"amountMinor,omitempty"`
	Currency    string `json:"currency,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

type V1AdminOrderRefundPostResponse struct {
	RefundRequest     V1RefundRequestRow `json:"refundRequest"`
	LedgerRefundID    string             `json:"ledgerRefundId"`
	LedgerState       string             `json:"ledgerState"`
	LedgerAmountMinor int64              `json:"ledgerAmountMinor"`
	LedgerCurrency    string             `json:"ledgerCurrency"`
}

// V1AdminMachineInventorySummary is slot-derived counts for admin machine payloads.
type V1AdminMachineInventorySummary struct {
	TotalSlots      int64 `json:"totalSlots"`
	OccupiedSlots   int64 `json:"occupiedSlots"`
	LowStockSlots   int64 `json:"lowStockSlots"`
	OutOfStockSlots int64 `json:"outOfStockSlots"`
}

// V1AdminAssignedTechnician is an active technician–machine assignment.
type V1AdminAssignedTechnician struct {
	TechnicianID string  `json:"technicianId"`
	DisplayName  string  `json:"displayName"`
	Role         string  `json:"role"`
	ValidFrom    string  `json:"validFrom"`
	ValidTo      *string `json:"validTo,omitempty"`
}

// V1AdminCurrentOperator is the active operator session on a machine (if any).
type V1AdminCurrentOperator struct {
	SessionID             string  `json:"sessionId"`
	ActorType             string  `json:"actorType"`
	TechnicianID          *string `json:"technicianId,omitempty"`
	TechnicianDisplayName *string `json:"technicianDisplayName,omitempty"`
	UserPrincipal         *string `json:"userPrincipal,omitempty"`
	SessionStartedAt      string  `json:"sessionStartedAt"`
	SessionStatus         string  `json:"sessionStatus"`
	SessionExpiresAt      *string `json:"sessionExpiresAt,omitempty"`
}

// V1AdminMachineListItem is one machine in GET /v1/admin/machines and GET /v1/admin/machines/{machineId}.
type V1AdminMachineListItem struct {
	MachineID           string                         `json:"machineId"`
	MachineName         string                         `json:"machineName"`
	OrganizationID      string                         `json:"organizationId"`
	SiteID              string                         `json:"siteId"`
	SiteName            string                         `json:"siteName"`
	HardwareProfileID   *string                        `json:"hardwareProfileId,omitempty"`
	SerialNumber        string                         `json:"serialNumber"`
	Name                string                         `json:"name"`
	Status              string                         `json:"status"`
	CommandSequence     int64                          `json:"commandSequence"`
	CreatedAt           string                         `json:"createdAt"`
	UpdatedAt           string                         `json:"updatedAt"`
	AndroidID           *string                        `json:"androidId,omitempty"`
	SimSerial           *string                        `json:"simSerial,omitempty"`
	SimIccid            *string                        `json:"simIccid,omitempty"`
	AppVersion          *string                        `json:"appVersion,omitempty"`
	FirmwareVersion     *string                        `json:"firmwareVersion,omitempty"`
	LastHeartbeatAt     *string                        `json:"lastHeartbeatAt,omitempty"`
	EffectiveTimezone   string                         `json:"effectiveTimezone"`
	AssignedTechnicians []V1AdminAssignedTechnician    `json:"assignedTechnicians"`
	CurrentOperator     *V1AdminCurrentOperator        `json:"currentOperator"`
	InventorySummary    V1AdminMachineInventorySummary `json:"inventorySummary"`
}

// V1MachineTelemetrySnapshotResponse is GET /v1/machines/{machineId}/telemetry/snapshot.
// All timestamps are RFC3339Nano strings with explicit timezone offset (responses use UTC, "Z").
type V1MachineTelemetrySnapshotResponse struct {
	MachineID         string          `json:"machineId"`
	OrganizationID    string          `json:"organizationId"`
	SiteID            string          `json:"siteId"`
	ReportedState     json.RawMessage `json:"reportedState"`
	MetricsState      json.RawMessage `json:"metricsState"`
	LastHeartbeatAt   *string         `json:"lastHeartbeatAt,omitempty"`
	AppVersion        *string         `json:"appVersion,omitempty"`
	FirmwareVersion   *string         `json:"firmwareVersion,omitempty"`
	UpdatedAt         string          `json:"updatedAt"`
	AndroidID         *string         `json:"androidId,omitempty"`
	SimSerial         *string         `json:"simSerial,omitempty"`
	SimIccid          *string         `json:"simIccid,omitempty"`
	DeviceModel       *string         `json:"deviceModel,omitempty"`
	OSVersion         *string         `json:"osVersion,omitempty"`
	LastIdentityAt    *string         `json:"lastIdentityAt,omitempty"`
	EffectiveTimezone string          `json:"effectiveTimezone"`
}

// V1MachineTelemetryIncidentItem is one element of GET /v1/machines/{machineId}/telemetry/incidents items.
type V1MachineTelemetryIncidentItem struct {
	ID        string          `json:"id"`
	Severity  string          `json:"severity"`
	Code      string          `json:"code"`
	Title     *string         `json:"title,omitempty"`
	Detail    json.RawMessage `json:"detail"`
	DedupeKey *string         `json:"dedupeKey,omitempty"`
	OpenedAt  string          `json:"openedAt"`
	UpdatedAt string          `json:"updatedAt"`
}

// V1MachineTelemetryIncidentsMeta is the meta object for telemetry incidents.
type V1MachineTelemetryIncidentsMeta struct {
	Limit    int32 `json:"limit"`
	Returned int   `json:"returned"`
}

// V1MachineTelemetryIncidentsResponse is GET /v1/machines/{machineId}/telemetry/incidents.
type V1MachineTelemetryIncidentsResponse struct {
	Items []V1MachineTelemetryIncidentItem `json:"items"`
	Meta  V1MachineTelemetryIncidentsMeta  `json:"meta"`
}

// V1MachineTelemetryRollupItem is one telemetry rollup bucket row.
type V1MachineTelemetryRollupItem struct {
	BucketStart string          `json:"bucketStart"`
	Granularity string          `json:"granularity"`
	MetricKey   string          `json:"metricKey"`
	SampleCount int64           `json:"sampleCount"`
	Sum         *float64        `json:"sum,omitempty"`
	Min         *float64        `json:"min,omitempty"`
	Max         *float64        `json:"max,omitempty"`
	Last        *float64        `json:"last,omitempty"`
	Extra       json.RawMessage `json:"extra"`
}

// V1MachineTelemetryRollupsMeta documents the window and query echo for rollup listing.
type V1MachineTelemetryRollupsMeta struct {
	Granularity string `json:"granularity"`
	From        string `json:"from"`
	To          string `json:"to"`
	Returned    int    `json:"returned"`
	Note        string `json:"note"`
}

// V1MachineTelemetryRollupsResponse is GET /v1/machines/{machineId}/telemetry/rollups.
type V1MachineTelemetryRollupsResponse struct {
	Items []V1MachineTelemetryRollupItem `json:"items"`
	Meta  V1MachineTelemetryRollupsMeta  `json:"meta"`
}

// V1AdminMachinesListResponse is GET /v1/admin/machines success body.
type V1AdminMachinesListResponse struct {
	Items []V1AdminMachineListItem `json:"items"`
	Meta  V1CollectionListMeta     `json:"meta"`
}

// V1AdminTechnicianListItem is one technician in GET /v1/admin/technicians.
type V1AdminTechnicianListItem struct {
	TechnicianID    string  `json:"technicianId"`
	OrganizationID  string  `json:"organizationId"`
	DisplayName     string  `json:"displayName"`
	Email           *string `json:"email,omitempty"`
	Phone           *string `json:"phone,omitempty"`
	ExternalSubject *string `json:"externalSubject,omitempty"`
	CreatedAt       string  `json:"createdAt"`
}

// V1AdminTechniciansListResponse is GET /v1/admin/technicians success body.
type V1AdminTechniciansListResponse struct {
	Items []V1AdminTechnicianListItem `json:"items"`
	Meta  V1CollectionListMeta        `json:"meta"`
}

// V1AdminAssignmentListItem is one assignment in GET /v1/admin/assignments.
type V1AdminAssignmentListItem struct {
	AssignmentID          string  `json:"assignmentId"`
	TechnicianID          string  `json:"technicianId"`
	TechnicianDisplayName string  `json:"technicianDisplayName"`
	MachineID             string  `json:"machineId"`
	MachineName           string  `json:"machineName"`
	MachineSerialNumber   string  `json:"machineSerialNumber"`
	Role                  string  `json:"role"`
	ValidFrom             string  `json:"validFrom"`
	ValidTo               *string `json:"validTo,omitempty"`
	CreatedAt             string  `json:"createdAt"`
}

// V1AdminAssignmentsListResponse is GET /v1/admin/assignments success body.
type V1AdminAssignmentsListResponse struct {
	Items []V1AdminAssignmentListItem `json:"items"`
	Meta  V1CollectionListMeta        `json:"meta"`
}

// V1AdminCommandListItem is one command in GET /v1/admin/commands.
type V1AdminCommandListItem struct {
	CommandID           string  `json:"commandId"`
	MachineID           string  `json:"machineId"`
	OrganizationID      string  `json:"organizationId"`
	MachineName         string  `json:"machineName"`
	MachineSerialNumber string  `json:"machineSerialNumber"`
	Sequence            int64   `json:"sequence"`
	CommandType         string  `json:"commandType"`
	CreatedAt           string  `json:"createdAt"`
	AttemptCount        int32   `json:"attemptCount"`
	LatestAttemptStatus string  `json:"latestAttemptStatus"`
	CorrelationID       *string `json:"correlationId,omitempty"`
}

// V1AdminCommandsListResponse is GET /v1/admin/commands success body.
type V1AdminCommandsListResponse struct {
	Items []V1AdminCommandListItem `json:"items"`
	Meta  V1CollectionListMeta     `json:"meta"`
}

// V1AdminOTAListItem is one OTA campaign in GET /v1/admin/ota.
type V1AdminOTAListItem struct {
	CampaignID         string  `json:"campaignId"`
	OrganizationID     string  `json:"organizationId"`
	CampaignName       string  `json:"campaignName"`
	Strategy           string  `json:"strategy"`
	CampaignStatus     string  `json:"campaignStatus"`
	CreatedAt          string  `json:"createdAt"`
	ArtifactID         string  `json:"artifactId"`
	ArtifactSemver     *string `json:"artifactSemver,omitempty"`
	ArtifactStorageKey string  `json:"artifactStorageKey"`
}

// V1AdminOTAListResponse is GET /v1/admin/ota success body.
type V1AdminOTAListResponse struct {
	Items []V1AdminOTAListItem `json:"items"`
	Meta  V1CollectionListMeta `json:"meta"`
}

// V1AdminOTACampaignListItem is one row in GET /v1/admin/ota/campaigns.
type V1AdminOTACampaignListItem struct {
	CampaignID         string  `json:"campaignId"`
	OrganizationID     string  `json:"organizationId"`
	Name               string  `json:"name"`
	RolloutStrategy    string  `json:"rolloutStrategy"`
	Status             string  `json:"status"`
	CampaignType       string  `json:"campaignType"`
	CanaryPercent      int32   `json:"canaryPercent"`
	RolloutNextOffset  int32   `json:"rolloutNextOffset"`
	ArtifactID         string  `json:"artifactId"`
	ArtifactSemver     *string `json:"artifactSemver,omitempty"`
	ArtifactStorageKey string  `json:"artifactStorageKey"`
	ArtifactVersion    *string `json:"artifactVersion,omitempty"`
	RollbackArtifactID *string `json:"rollbackArtifactId,omitempty"`
	CreatedAt          string  `json:"createdAt"`
	UpdatedAt          string  `json:"updatedAt"`
	ApprovedAt         *string `json:"approvedAt,omitempty"`
}

// V1AdminOTACampaignListResponse is GET /v1/admin/ota/campaigns.
type V1AdminOTACampaignListResponse struct {
	Items []V1AdminOTACampaignListItem `json:"items"`
	Meta  V1CollectionListMeta         `json:"meta"`
}

// V1AdminOTACampaignDetail is GET/PATCH/lifecycle responses for /v1/admin/ota/campaigns/*.
type V1AdminOTACampaignDetail struct {
	V1AdminOTACampaignListItem
	CreatedBy  *string `json:"createdBy,omitempty"`
	ApprovedBy *string `json:"approvedBy,omitempty"`
	PausedAt   *string `json:"pausedAt,omitempty"`
}

// V1AdminOTACampaignTargetsResponse is GET .../targets.
type V1AdminOTACampaignTargetsResponse struct {
	Items []V1AdminOTACampaignTargetItem `json:"items"`
}

// V1AdminOTACampaignTargetItem is one machine target row.
type V1AdminOTACampaignTargetItem struct {
	MachineID string  `json:"machineId"`
	State     string  `json:"state"`
	LastError *string `json:"lastError,omitempty"`
	UpdatedAt string  `json:"updatedAt"`
}

// V1AdminOTACampaignResultsResponse is GET .../results.
type V1AdminOTACampaignResultsResponse struct {
	Items []V1AdminOTACampaignMachineResultItem `json:"items"`
}

// V1AdminOTACampaignMachineResultItem is one dispatched wave row per machine.
type V1AdminOTACampaignMachineResultItem struct {
	MachineID string  `json:"machineId"`
	Wave      string  `json:"wave"`
	CommandID *string `json:"commandId,omitempty"`
	Status    string  `json:"status"`
	LastError *string `json:"lastError,omitempty"`
	UpdatedAt string  `json:"updatedAt"`
	CreatedAt string  `json:"createdAt"`
}

// V1CashDenominationExpectation is an optional breakdown hint (not hardware-sourced today).
type V1CashDenominationExpectation struct {
	DenominationMinor int64  `json:"denominationMinor"`
	ExpectedCount     int64  `json:"expectedCount"`
	Source            string `json:"source"` // e.g. bill_recycler, vault_model
}

// V1AdminMachineCashboxResponse is GET /v1/admin/machines/{machineId}/cashbox.
type V1AdminMachineCashboxResponse struct {
	MachineID                    string                          `json:"machineId"`
	Currency                     string                          `json:"currency"`
	ExpectedCashboxMinor         int64                           `json:"expectedCashboxMinor"` // legacy alias; same as ExpectedCloudCashMinor
	ExpectedCloudCashMinor       int64                           `json:"expectedCloudCashMinor"`
	ExpectedRecyclerMinor        int64                           `json:"expectedRecyclerMinor"`
	LastCollectionAt             *string                         `json:"lastCollectionAt,omitempty"`
	Denominations                []V1CashDenominationExpectation `json:"denominations"`
	OpenCollectionID             *string                         `json:"openCollectionId,omitempty"`
	VarianceReviewThresholdMinor int64                           `json:"varianceReviewThresholdMinor"`
	Disclosure                   string                          `json:"disclosure"`
}

// V1AdminCashCollection is one cash collection session row (open or closed).
type V1AdminCashCollection struct {
	ID                       string  `json:"id"`
	MachineID                string  `json:"machine_id"`
	OrganizationID           string  `json:"organization_id"`
	CollectedAt              string  `json:"collected_at"`
	OpenedAt                 string  `json:"opened_at"`
	ClosedAt                 *string `json:"closed_at,omitempty"`
	LifecycleStatus          string  `json:"lifecycle_status"`
	CountedAmountMinor       int64   `json:"counted_amount_minor"`
	ExpectedAmountMinor      int64   `json:"expected_amount_minor"`
	VarianceAmountMinor      int64   `json:"variance_amount_minor"`
	CountedPhysicalCashMinor int64   `json:"countedPhysicalCashMinor"`
	ExpectedCloudCashMinor   int64   `json:"expectedCloudCashMinor"`
	VarianceMinor            int64   `json:"varianceMinor"`
	ReviewState              string  `json:"reviewState"`
	RequiresReview           bool    `json:"requires_review"`
	CloseRequestHashHex      *string `json:"close_request_hash_hex,omitempty"`
	Currency                 string  `json:"currency"`
	ReconciliationStatus     string  `json:"reconciliation_status"`
	Disclosure               string  `json:"disclosure"`
}

// V1AdminCashCollectionListResponse is GET /v1/admin/machines/{machineId}/cash-collections.
type V1AdminCashCollectionListResponse struct {
	Items []V1AdminCashCollection `json:"items"`
	Meta  V1CollectionListMeta    `json:"meta"`
}

// --- Admin operations (P1.2): tenant-scoped machine health, commands, inventory anomalies ---

// V1AdminOperationsMachineHealthItem is a row from GET .../operations/machines/health or GET .../machines/{machineId}/health.
type V1AdminOperationsMachineHealthItem struct {
	MachineID                 string  `json:"machineId"`
	Status                    string  `json:"status"`
	PendingCommandCount       int32   `json:"pendingCommandCount"`
	FailedCommandCount        int32   `json:"failedCommandCount"`
	InventoryAnomalyCount     int32   `json:"inventoryAnomalyCount"`
	LastSeenAt                *string `json:"lastSeenAt,omitempty"`
	LastCheckInAt             *string `json:"lastCheckInAt,omitempty"`
	AppVersion                *string `json:"appVersion,omitempty"`
	ConfigVersion             *string `json:"configVersion,omitempty"`
	CatalogVersion            *string `json:"catalogVersion,omitempty"`
	MediaVersion              *string `json:"mediaVersion,omitempty"`
	MqttConnected             *bool   `json:"mqttConnected,omitempty"`
	LastErrorCode             *string `json:"lastErrorCode,omitempty"`
	TelemetryFreshnessSeconds *int64  `json:"telemetryFreshnessSeconds,omitempty"`
}

// V1AdminOperationsMachineHealthListResponse is GET .../operations/machines/health.
type V1AdminOperationsMachineHealthListResponse struct {
	Items []V1AdminOperationsMachineHealthItem `json:"items"`
}

// V1AdminOperationsTimelineEvent is one merged timeline row.
type V1AdminOperationsTimelineEvent struct {
	OccurredAt string          `json:"occurredAt"`
	EventKind  string          `json:"eventKind"`
	Title      string          `json:"title"`
	Payload    json.RawMessage `json:"payload"`
	RefID      *string         `json:"refId,omitempty"`
}

// V1AdminOperationsTimelineListResponse is GET .../machines/{machineId}/timeline.
type V1AdminOperationsTimelineListResponse struct {
	Items []V1AdminOperationsTimelineEvent `json:"items"`
}

// V1AdminOperationsCommandAttemptItem is one machine_command_attempts row in command detail.
type V1AdminOperationsCommandAttemptItem struct {
	ID               string  `json:"id"`
	AttemptNo        int32   `json:"attemptNo"`
	Status           string  `json:"status"`
	SentAt           string  `json:"sentAt"`
	DispatchState    string  `json:"dispatchState"`
	AckDeadlineAt    *string `json:"ackDeadlineAt,omitempty"`
	ResultReceivedAt *string `json:"resultReceivedAt,omitempty"`
	TimeoutReason    *string `json:"timeoutReason,omitempty"`
}

// V1AdminOperationsCommandDetailResponse is GET .../commands/{commandId}.
type V1AdminOperationsCommandDetailResponse struct {
	CommandID      string                                `json:"commandId"`
	MachineID      string                                `json:"machineId"`
	OrganizationID string                                `json:"organizationId"`
	Sequence       int64                                 `json:"sequence"`
	CommandType    string                                `json:"commandType"`
	Payload        json.RawMessage                       `json:"payload"`
	CreatedAt      string                                `json:"createdAt"`
	CorrelationID  *string                               `json:"correlationId,omitempty"`
	IdempotencyKey *string                               `json:"idempotencyKey,omitempty"`
	Attempts       []V1AdminOperationsCommandAttemptItem `json:"attempts"`
}

// V1AdminOperationsCommandRetryResponse is POST .../commands/{commandId}/retry.
type V1AdminOperationsCommandRetryResponse struct {
	CommandID        string `json:"commandId"`
	Sequence         int64  `json:"sequence"`
	AttemptID        string `json:"attemptId"`
	DispatchState    string `json:"dispatchState"`
	Replay           bool   `json:"replay"`
	SkippedRepublish bool   `json:"skippedRepublish"`
}

// V1AdminOperationsCommandCancelResponse is POST .../commands/{commandId}/cancel.
type V1AdminOperationsCommandCancelResponse struct {
	AttemptsCancelled int32 `json:"attemptsCancelled"`
}

// V1AdminOperationsMachineCommandDispatchRequest is POST .../machines/{machineId}/commands.
type V1AdminOperationsMachineCommandDispatchRequest struct {
	CommandType string          `json:"commandType"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

// V1AdminOperationsMachineCommandDispatchResponse is POST .../machines/{machineId}/commands (202).
type V1AdminOperationsMachineCommandDispatchResponse struct {
	CommandID     string `json:"commandId"`
	Sequence      int64  `json:"sequence"`
	AttemptID     string `json:"attemptId"`
	DispatchState string `json:"dispatchState"`
	Replay        bool   `json:"replay"`
}

// V1AdminOperationsInventoryAnomalyItem is one inventory anomaly row.
type V1AdminOperationsInventoryAnomalyItem struct {
	ID                  string          `json:"id"`
	OrganizationID      string          `json:"organizationId"`
	MachineID           string          `json:"machineId"`
	MachineName         string          `json:"machineName"`
	MachineSerialNumber string          `json:"machineSerialNumber"`
	AnomalyType         string          `json:"anomalyType"`
	Status              string          `json:"status"`
	Fingerprint         string          `json:"fingerprint"`
	DetectedAt          string          `json:"detectedAt"`
	CreatedAt           string          `json:"createdAt"`
	UpdatedAt           string          `json:"updatedAt"`
	SlotCode            *string         `json:"slotCode,omitempty"`
	ProductID           *string         `json:"productId,omitempty"`
	Payload             json.RawMessage `json:"payload,omitempty"`
	ResolvedAt          *string         `json:"resolvedAt,omitempty"`
	ResolvedBy          *string         `json:"resolvedBy,omitempty"`
	ResolutionNote      *string         `json:"resolutionNote,omitempty"`
}

// V1AdminOperationsInventoryAnomalyListResponse is GET .../inventory/anomalies.
type V1AdminOperationsInventoryAnomalyListResponse struct {
	Items []V1AdminOperationsInventoryAnomalyItem `json:"items"`
}

// V1AdminOperationsInventoryAnomalyResolveRequest is POST .../inventory/anomalies/{anomalyId}/resolve.
type V1AdminOperationsInventoryAnomalyResolveRequest struct {
	Note string `json:"note,omitempty"`
}

// V1AdminOperationsInventoryAnomalyResolveResponse is POST .../inventory/anomalies/{anomalyId}/resolve.
type V1AdminOperationsInventoryAnomalyResolveResponse struct {
	AnomalyID string `json:"anomalyId"`
	Status    string `json:"status"`
}

// V1AdminOperationsInventoryReconcileRequest is POST .../machines/{machineId}/inventory/reconcile.
type V1AdminOperationsInventoryReconcileRequest struct {
	Reason string `json:"reason,omitempty"`
}

// V1AdminOperationsInventoryReconcileResponse is POST .../machines/{machineId}/inventory/reconcile (202).
type V1AdminOperationsInventoryReconcileResponse struct {
	InventoryEventID int64 `json:"inventoryEventId"`
}

// --- P2.1 provisioning & fleet rollout ---

// V1AdminProvisioningBulkMachineRow is one row inside bulk provisioning POST.
type V1AdminProvisioningBulkMachineRow struct {
	SerialNumber string `json:"serialNumber"`
	Name         string `json:"name,omitempty"`
	Model        string `json:"model,omitempty"`
}

// V1AdminProvisioningBulkCreateRequest is POST .../provisioning/machines/bulk.
type V1AdminProvisioningBulkCreateRequest struct {
	SiteID                  string                              `json:"siteId"`
	HardwareProfileID       *string                             `json:"hardwareProfileId,omitempty"`
	CabinetType             string                              `json:"cabinetType"`
	Machines                []V1AdminProvisioningBulkMachineRow `json:"machines"`
	GenerateActivationCodes bool                                `json:"generateActivationCodes"`
	ExpiresInMinutes        int32                               `json:"expiresInMinutes,omitempty"`
	MaxUses                 int32                               `json:"maxUses,omitempty"`
}

// V1AdminProvisioningBulkMachineOut is one machine returned from bulk provisioning (plaintext activation codes appear once).
type V1AdminProvisioningBulkMachineOut struct {
	MachineID        string `json:"machineId"`
	SerialNumber     string `json:"serialNumber"`
	ActivationCode   string `json:"activationCode,omitempty"`
	ActivationCodeID string `json:"activationCodeId,omitempty"`
}

// V1AdminProvisioningBulkCreateResponse is POST .../provisioning/machines/bulk (201).
type V1AdminProvisioningBulkCreateResponse struct {
	BatchID      string                              `json:"batchId"`
	Status       string                              `json:"status"`
	Machines     []V1AdminProvisioningBulkMachineOut `json:"machines"`
	MachineCount int                                 `json:"machineCount"`
}

// V1AdminProvisioningBatchDetailResponse is GET .../provisioning/batches/{batchId}.
type V1AdminProvisioningBatchDetailResponse struct {
	Batch    map[string]any   `json:"batch"`
	Machines []map[string]any `json:"machines"`
}

// V1AdminRolloutCreateRequest is POST .../rollouts.
type V1AdminRolloutCreateRequest struct {
	RolloutType   string          `json:"rolloutType"`
	TargetVersion string          `json:"targetVersion"`
	Strategy      json.RawMessage `json:"strategy,omitempty"`
}

// V1AdminRolloutCampaign describes rollout_campaigns rows for OpenAPI.
type V1AdminRolloutCampaign struct {
	ID             string          `json:"id"`
	OrganizationID string          `json:"organizationId"`
	RolloutType    string          `json:"rolloutType"`
	TargetVersion  string          `json:"targetVersion"`
	Status         string          `json:"status"`
	Strategy       json.RawMessage `json:"strategy,omitempty"`
	CreatedBy      *string         `json:"createdBy,omitempty"`
	CreatedAt      string          `json:"createdAt"`
	UpdatedAt      string          `json:"updatedAt"`
	StartedAt      *string         `json:"startedAt,omitempty"`
	CompletedAt    *string         `json:"completedAt,omitempty"`
	CancelledAt    *string         `json:"cancelledAt,omitempty"`
}

// V1AdminRolloutTarget is one rollout_targets row.
type V1AdminRolloutTarget struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organizationId"`
	CampaignID     string  `json:"campaignId"`
	MachineID      string  `json:"machineId"`
	Status         string  `json:"status"`
	Error          *string `json:"error,omitempty"`
	CommandID      *string `json:"commandId,omitempty"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

// V1AdminRolloutDetailResponse is GET .../rollouts/{rolloutId} (and lifecycle POST bodies echo campaign + targets).
type V1AdminRolloutDetailResponse struct {
	Campaign V1AdminRolloutCampaign `json:"campaign"`
	Targets  []V1AdminRolloutTarget `json:"targets"`
}

// V1AdminRolloutListMeta is pagination metadata for GET .../rollouts.
type V1AdminRolloutListMeta struct {
	Limit    int32 `json:"limit"`
	Offset   int32 `json:"offset"`
	Returned int   `json:"returned"`
}

// V1AdminRolloutListResponse is GET .../rollouts.
type V1AdminRolloutListResponse struct {
	Items []V1AdminRolloutCampaign `json:"items"`
	Meta  V1AdminRolloutListMeta   `json:"meta"`
}
