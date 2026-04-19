package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/avf/avf-vending-api/internal/apierr"
	"github.com/avf/avf-vending-api/internal/app/api"
	appmw "github.com/avf/avf-vending-api/internal/middleware"
)

func writeAPIError(w http.ResponseWriter, ctx context.Context, status int, code, message string) {
	writeAPIErrorDetails(w, ctx, status, code, message, nil)
}

func writeAPIErrorDetails(w http.ResponseWriter, ctx context.Context, status int, code, message string, details map[string]any) {
	rid := appmw.RequestIDFromContext(ctx)
	writeJSON(w, status, apierr.V1(rid, code, message, details))
}

func writeV1ListError(w http.ResponseWriter, ctx context.Context, err error) {
	if errors.Is(err, api.ErrInvalidListQuery) {
		writeAPIError(w, ctx, http.StatusBadRequest, "invalid_query", "invalid list query parameters")
		return
	}
	if errors.Is(err, api.ErrCommerceOrganizationQueryRequired) {
		writeAPIError(w, ctx, http.StatusBadRequest, "organization_id_required", "organization_id query parameter is required for platform administrators")
		return
	}
	var capErr *api.CapabilityError
	if errors.As(err, &capErr) {
		writeJSON(w, http.StatusNotImplemented, apierr.V1(
			appmw.RequestIDFromContext(ctx),
			"not_implemented",
			capErr.Message,
			map[string]any{
				"capability":  capErr.Capability,
				"implemented": false,
			},
		))
		return
	}
	if errors.Is(err, api.ErrAdminTenantScopeRequired) {
		writeAPIError(w, ctx, http.StatusBadRequest, "tenant_scope_required", "organization scope is required for this list")
		return
	}
	writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
}

func writeV1ListView(w http.ResponseWriter, ctx context.Context, out *api.ListView, err error) {
	if err != nil {
		writeV1ListError(w, ctx, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// writeV1Collection writes a typed JSON list envelope (items + meta) or maps list errors.
func writeV1Collection(w http.ResponseWriter, ctx context.Context, v any, err error) {
	if err != nil {
		writeV1ListError(w, ctx, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func writeMachineShadowLoadError(w http.ResponseWriter, ctx context.Context, err error) {
	if errors.Is(err, api.ErrMachineShadowNotFound) {
		writeAPIError(w, ctx, http.StatusNotFound, "machine_shadow_not_found", "machine shadow row does not exist")
		return
	}
	writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
}

func writeCapabilityNotConfigured(w http.ResponseWriter, ctx context.Context, capability, message string) {
	writeJSON(w, http.StatusServiceUnavailable, apierr.V1(
		appmw.RequestIDFromContext(ctx),
		"capability_not_configured",
		message,
		map[string]any{
			"capability":  capability,
			"implemented": false,
		},
	))
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
