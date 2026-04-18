package httpserver

// OpenAPI / Swagger documentation types (runtime responses use compatible JSON shapes).
// Handlers may return additional fields; these structs capture stable fields for spec generation.

// V1StandardError is the usual JSON error envelope for /v1 handlers (see writeAPIError).
type V1StandardError struct {
	Error V1StandardErrorBody `json:"error"`
}

// V1StandardErrorBody is the inner object for V1StandardError.
type V1StandardErrorBody struct {
	Code    string `json:"code" example:"invalid_json"`
	Message string `json:"message"`
}

// V1NotImplementedError is returned for some list endpoints that are not yet backed (HTTP 501).
type V1NotImplementedError struct {
	Error V1NotImplementedBody `json:"error"`
}

// V1NotImplementedBody extends the standard envelope with capability metadata.
type V1NotImplementedBody struct {
	Code        string `json:"code" example:"not_implemented"`
	Message     string `json:"message"`
	Capability  string `json:"capability" example:"v1.admin.commands.list"`
	Implemented bool   `json:"implemented" example:"false"`
}

// V1CapabilityNotConfiguredError is returned when optional wiring is missing (HTTP 503).
type V1CapabilityNotConfiguredError struct {
	Error V1CapabilityNotConfiguredBody `json:"error"`
}

// V1CapabilityNotConfiguredBody extends the standard envelope for optional infrastructure gaps.
type V1CapabilityNotConfiguredBody struct {
	Code        string `json:"code" example:"capability_not_configured"`
	Message     string `json:"message"`
	Capability  string `json:"capability" example:"mqtt_command_dispatch"`
	Implemented bool   `json:"implemented" example:"false"`
}

// V1BearerAuthError is returned by Bearer middleware for auth failures (HTTP 401/403/503 from auth layer).
// Note: misconfiguration responses may use plain text from http.Error on readiness; Bearer paths use JSON.
type V1BearerAuthError struct {
	Error V1BearerAuthErrorBody `json:"error"`
}

// V1BearerAuthErrorBody is a minimal shape (no `code` field — branch on HTTP status and message text).
type V1BearerAuthErrorBody struct {
	Message string `json:"message" example:"unauthenticated"`
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
