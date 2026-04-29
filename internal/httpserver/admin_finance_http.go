package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/api"
	appfinance "github.com/avf/avf-vending-api/internal/app/finance"
	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// rbac:inherited-mount: mountAdminFinanceDailyCloseRoutes is wired in server.go inside auth.RequireAnyPermission(PermCashWrite).

func mountAdminFinanceDailyCloseRoutes(r chi.Router, app *api.HTTPApplication) {
	if app == nil || app.Finance == nil {
		return
	}
	fsvc := app.Finance
	r.Post("/finance/daily-close", postAdminFinanceDailyClose(fsvc))
	r.Get("/finance/daily-close", getAdminFinanceDailyCloseList(fsvc))
	r.Get("/finance/daily-close/{closeId}", getAdminFinanceDailyCloseByID(fsvc))
}

func parseAdminTenantOrganization(r *http.Request) (uuid.UUID, error) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return uuid.Nil, listscope.ErrInvalidListQuery
	}
	qv := r.URL.Query()
	var orgID uuid.UUID
	if p.HasRole(auth.RolePlatformAdmin) {
		raw := strings.TrimSpace(qv.Get("organization_id"))
		id, perr := uuid.Parse(raw)
		if perr != nil || id == uuid.Nil {
			return uuid.Nil, api.ErrCommerceOrganizationQueryRequired
		}
		orgID = id
	} else {
		if !p.HasOrganization() {
			return uuid.Nil, api.ErrCommerceOrganizationQueryRequired
		}
		orgID = p.OrganizationID
		if raw := strings.TrimSpace(qv.Get("organization_id")); raw != "" {
			qid, perr := uuid.Parse(raw)
			if perr != nil || qid != orgID {
				return uuid.Nil, listscope.ErrInvalidListQuery
			}
		}
	}
	return orgID, nil
}

func postAdminFinanceDailyClose(svc api.FinanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idem, err := requireWriteIdempotencyKey(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "idempotency_required", err.Error())
			return
		}
		orgID, err := parseAdminTenantOrganization(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		var body struct {
			CloseDate string  `json:"closeDate"`
			Timezone  string  `json:"timezone"`
			SiteID    *string `json:"siteId"`
			MachineID *string `json:"machineId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body must be JSON")
			return
		}
		in := appfinance.CreateDailyCloseInput{
			OrganizationID: orgID,
			CloseDate:      body.CloseDate,
			Timezone:       body.Timezone,
			IdempotencyKey: idem,
		}
		if body.SiteID != nil && strings.TrimSpace(*body.SiteID) != "" {
			sid, perr := uuid.Parse(strings.TrimSpace(*body.SiteID))
			if perr != nil || sid == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_site_id", "invalid siteId")
				return
			}
			in.SiteID = sid
		}
		if body.MachineID != nil && strings.TrimSpace(*body.MachineID) != "" {
			mid, perr := uuid.Parse(strings.TrimSpace(*body.MachineID))
			if perr != nil || mid == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
				return
			}
			in.MachineID = mid
		}
		if p, ok := auth.PrincipalFromContext(r.Context()); ok {
			at, aid := p.Actor()
			in.ActorType = at
			if aid != "" {
				in.ActorID = &aid
			}
		}
		out, err := svc.CreateDailyClose(r.Context(), in)
		if err != nil {
			writeFinanceDailyCloseError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusCreated, out)
	}
}

func getAdminFinanceDailyCloseList(svc api.FinanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminTenantOrganization(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeV1ListError(w, r.Context(), listscope.ErrInvalidListQuery)
			return
		}
		out, err := svc.ListDailyClose(r.Context(), appfinance.ListDailyCloseParams{
			OrganizationID: orgID,
			Limit:          limit,
			Offset:         offset,
		})
		writeV1Collection(w, r.Context(), out, err)
	}
}

func getAdminFinanceDailyCloseByID(svc api.FinanceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := parseAdminTenantOrganization(r)
		if err != nil {
			writeV1ListError(w, r.Context(), err)
			return
		}
		raw := chi.URLParam(r, "closeId")
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil || id == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_close_id", "invalid closeId")
			return
		}
		out, err := svc.GetDailyClose(r.Context(), orgID, id)
		if err != nil {
			writeFinanceDailyCloseError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func writeFinanceDailyCloseError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case errors.Is(err, appfinance.ErrDuplicateDailyClose):
		writeAPIError(w, ctx, http.StatusConflict, "daily_close_exists", err.Error())
	case errors.Is(err, appfinance.ErrDailyCloseNotFound):
		writeAPIError(w, ctx, http.StatusNotFound, "daily_close_not_found", err.Error())
	case errors.Is(err, appfinance.ErrInvalidDailyCloseInput):
		writeAPIError(w, ctx, http.StatusBadRequest, "invalid_daily_close", err.Error())
	default:
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
	}
}
