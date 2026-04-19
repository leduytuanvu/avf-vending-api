package httpserver

import (
	"net/http"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
)

func parseTenantCommerceListScope(r *http.Request) (listscope.TenantCommerce, error) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return listscope.TenantCommerce{}, listscope.ErrInvalidListQuery
	}
	limit, offset, err := parseAdminLimitOffset(r)
	if err != nil {
		return listscope.TenantCommerce{}, listscope.ErrInvalidListQuery
	}
	q := r.URL.Query()
	var orgID uuid.UUID
	if p.HasRole(auth.RolePlatformAdmin) {
		raw := strings.TrimSpace(q.Get("organization_id"))
		id, perr := uuid.Parse(raw)
		if perr != nil || id == uuid.Nil {
			return listscope.TenantCommerce{}, api.ErrCommerceOrganizationQueryRequired
		}
		orgID = id
	} else {
		if !p.HasOrganization() {
			return listscope.TenantCommerce{}, api.ErrCommerceOrganizationQueryRequired
		}
		orgID = p.OrganizationID
		if raw := strings.TrimSpace(q.Get("organization_id")); raw != "" {
			qid, perr := uuid.Parse(raw)
			if perr != nil || qid != orgID {
				return listscope.TenantCommerce{}, listscope.ErrInvalidListQuery
			}
		}
	}
	var machineID *uuid.UUID
	if raw := strings.TrimSpace(q.Get("machine_id")); raw != "" {
		mid, perr := uuid.Parse(raw)
		if perr != nil || mid == uuid.Nil {
			return listscope.TenantCommerce{}, listscope.ErrInvalidListQuery
		}
		machineID = &mid
	}
	var from *time.Time
	if raw := strings.TrimSpace(q.Get("from")); raw != "" {
		t, perr := time.Parse(time.RFC3339Nano, raw)
		if perr != nil {
			t, perr = time.Parse(time.RFC3339, raw)
		}
		if perr != nil {
			return listscope.TenantCommerce{}, listscope.ErrInvalidListQuery
		}
		utc := t.UTC()
		from = &utc
	}
	var to *time.Time
	if raw := strings.TrimSpace(q.Get("to")); raw != "" {
		t, perr := time.Parse(time.RFC3339Nano, raw)
		if perr != nil {
			t, perr = time.Parse(time.RFC3339, raw)
		}
		if perr != nil {
			return listscope.TenantCommerce{}, listscope.ErrInvalidListQuery
		}
		utc := t.UTC()
		to = &utc
	}
	return listscope.TenantCommerce{
		IsPlatformAdmin: p.HasRole(auth.RolePlatformAdmin),
		OrganizationID:  orgID,
		Limit:           limit,
		Offset:          offset,
		Status:          strings.TrimSpace(q.Get("status")),
		MachineID:       machineID,
		PaymentMethod:   strings.TrimSpace(q.Get("payment_method")),
		Search:          strings.TrimSpace(q.Get("search")),
		From:            from,
		To:              to,
	}, nil
}
