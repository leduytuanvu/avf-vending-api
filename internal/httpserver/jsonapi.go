package httpserver

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
)

// apiErrJSON returns the standard v1 error envelope: {"error":{"code":"...","message":"..."}}.
// All JSON APIs under /v1 should use this shape so clients can branch on `error.code` reliably.
func apiErrJSON(code, message string) map[string]any {
	return map[string]any{"error": map[string]any{"code": code, "message": message}}
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, apiErrJSON(code, message))
}

func writeV1ListError(w http.ResponseWriter, err error) {
	var capErr *api.CapabilityError
	if errors.As(err, &capErr) {
		writeJSON(w, http.StatusNotImplemented, map[string]any{
			"error": map[string]any{
				"code":        "not_implemented",
				"message":     capErr.Message,
				"capability":  capErr.Capability,
				"implemented": false,
			},
		})
		return
	}
	if errors.Is(err, api.ErrAdminTenantScopeRequired) {
		writeAPIError(w, http.StatusBadRequest, "tenant_scope_required", "organization scope is required for this list")
		return
	}
	writeAPIError(w, http.StatusInternalServerError, "internal", err.Error())
}

func writeV1ListView(w http.ResponseWriter, out *api.ListView, err error) {
	if err != nil {
		writeV1ListError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func writeMachineShadowLoadError(w http.ResponseWriter, err error) {
	if errors.Is(err, api.ErrMachineShadowNotFound) {
		writeAPIError(w, http.StatusNotFound, "machine_shadow_not_found", "machine shadow row does not exist")
		return
	}
	writeAPIError(w, http.StatusInternalServerError, "internal", err.Error())
}

func writeCapabilityNotConfigured(w http.ResponseWriter, capability, message string) {
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{
		"error": map[string]any{
			"code":        "capability_not_configured",
			"message":     message,
			"capability":  capability,
			"implemented": false,
		},
	})
}

// Operator list endpoints share the same limit semantics as app/operator clampListLimit (defaults + cap).
const (
	operatorListLimitDefault int32 = 50
	operatorListLimitMax     int32 = 500
)

// parseOperatorListLimit parses optional query "limit" for operator history, timeline, and insight lists.
// Omitted or blank → default 50. Out-of-range positive values are clamped to 500. Non-positive or
// non-numeric values return an error (400 invalid_limit).
func parseOperatorListLimit(r *http.Request) (int32, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return operatorListLimitDefault, nil
	}
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("limit must be a positive integer")
	}
	if n > int64(operatorListLimitMax) {
		return operatorListLimitMax, nil
	}
	return int32(n), nil
}

// writeOperatorListEnvelope writes a successful list payload with stable paging metadata.
// Existing clients may ignore `meta`; new clients should prefer meta.returned over inferring from items.
func writeOperatorListEnvelope(w http.ResponseWriter, items any, limit int32, returned int) {
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items,
		"meta": map[string]any{
			"limit":    limit,
			"returned": returned,
		},
	})
}
