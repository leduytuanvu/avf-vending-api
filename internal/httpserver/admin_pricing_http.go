package httpserver

// rbac:inherited-mount: catalog price-book and pricing preview routes are wired from mountAdminCatalogRoutes (catalog RBAC groups).

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func parseRFC3339Flexible(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("empty time")
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func pgUUIDPtrFromOptionalString(s *string) (pgtype.UUID, error) {
	var out pgtype.UUID
	if s == nil {
		return out, nil
	}
	raw := strings.TrimSpace(*s)
	if raw == "" {
		return out, nil
	}
	u, err := uuid.Parse(raw)
	if err != nil {
		return out, err
	}
	return pgtype.UUID{Bytes: u, Valid: true}, nil
}

func mergePriceBookPatch(cur db.PriceBook, body V1AdminPriceBookPatchRequest) (db.CatalogWriteUpdatePriceBookParams, error) {
	p := db.CatalogWriteUpdatePriceBookParams{
		OrganizationID: cur.OrganizationID,
		ID:             cur.ID,
		Name:           cur.Name,
		Currency:       cur.Currency,
		EffectiveFrom:  cur.EffectiveFrom,
		EffectiveTo:    cur.EffectiveTo,
		IsDefault:      cur.IsDefault,
		Active:         cur.Active,
		ScopeType:      cur.ScopeType,
		SiteID:         cur.SiteID,
		MachineID:      cur.MachineID,
		Priority:       cur.Priority,
	}
	if body.Name != nil {
		p.Name = strings.TrimSpace(*body.Name)
	}
	if body.Currency != nil {
		p.Currency = strings.TrimSpace(*body.Currency)
	}
	if body.EffectiveFrom != nil {
		t, err := parseRFC3339Flexible(*body.EffectiveFrom)
		if err != nil {
			return db.CatalogWriteUpdatePriceBookParams{}, err
		}
		p.EffectiveFrom = t
	}
	if body.EffectiveTo != nil {
		s := strings.TrimSpace(*body.EffectiveTo)
		if s == "" {
			p.EffectiveTo = pgtype.Timestamptz{}
		} else {
			t, err := parseRFC3339Flexible(s)
			if err != nil {
				return db.CatalogWriteUpdatePriceBookParams{}, err
			}
			p.EffectiveTo = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}
	if body.IsDefault != nil {
		p.IsDefault = *body.IsDefault
	}
	if body.Active != nil {
		p.Active = *body.Active
	}
	if body.ScopeType != nil {
		sc := strings.TrimSpace(strings.ToLower(*body.ScopeType))
		p.ScopeType = sc
		switch sc {
		case "organization":
			p.SiteID = pgtype.UUID{}
			p.MachineID = pgtype.UUID{}
		case "site":
			p.MachineID = pgtype.UUID{}
		case "machine":
			p.SiteID = pgtype.UUID{}
		default:
			break
		}
	}
	if body.SiteID != nil || body.MachineID != nil {
		sid, err := pgUUIDPtrFromOptionalString(body.SiteID)
		if err != nil {
			return db.CatalogWriteUpdatePriceBookParams{}, err
		}
		mid, err := pgUUIDPtrFromOptionalString(body.MachineID)
		if err != nil {
			return db.CatalogWriteUpdatePriceBookParams{}, err
		}
		if body.SiteID != nil {
			p.SiteID = sid
		}
		if body.MachineID != nil {
			p.MachineID = mid
		}
	}
	if body.Priority != nil {
		p.Priority = *body.Priority
	}
	return p, nil
}

func postAdminPriceBookCreate(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		var body V1AdminPriceBookWriteRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		effFrom, err := parseRFC3339Flexible(body.EffectiveFrom)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_argument", "effectiveFrom must be RFC3339/RFC3339Nano")
			return
		}
		var effTo pgtype.Timestamptz
		if body.EffectiveTo != nil && strings.TrimSpace(*body.EffectiveTo) != "" {
			t, err := parseRFC3339Flexible(*body.EffectiveTo)
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_argument", "effectiveTo must be RFC3339/RFC3339Nano")
				return
			}
			effTo = pgtype.Timestamptz{Time: t, Valid: true}
		}
		siteID, err := pgUUIDPtrFromOptionalString(body.SiteID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_site_id", "invalid siteId")
			return
		}
		machineID, err := pgUUIDPtrFromOptionalString(body.MachineID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		row, err := svc.CreatePriceBook(r.Context(), appcatalogadmin.CreatePriceBookInput{
			OrganizationID: orgID,
			Name:           body.Name,
			Currency:       body.Currency,
			EffectiveFrom:  effFrom,
			EffectiveTo:    effTo,
			IsDefault:      body.IsDefault,
			ScopeType:      body.ScopeType,
			SiteID:         siteID,
			MachineID:      machineID,
			Priority:       body.Priority,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapPriceBook(row))
	}
}

func getAdminPriceBookDetail(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		bid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "priceBookId")))
		if err != nil || bid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_price_book_id", "invalid priceBookId")
			return
		}
		row, err := svc.GetPriceBook(r.Context(), orgID, bid)
		if err != nil {
			if errors.Is(err, appcatalogadmin.ErrNotFound) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "price book not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, mapPriceBook(row))
	}
}

func patchAdminPriceBook(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		bid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "priceBookId")))
		if err != nil || bid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_price_book_id", "invalid priceBookId")
			return
		}
		cur, err := svc.GetPriceBook(r.Context(), orgID, bid)
		if err != nil {
			if errors.Is(err, appcatalogadmin.ErrNotFound) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "price book not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		var body V1AdminPriceBookPatchRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		next, err := mergePriceBookPatch(cur, body)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_argument", err.Error())
			return
		}
		row, err := svc.UpdatePriceBook(r.Context(), next)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapPriceBook(row))
	}
}

func postAdminPriceBookDeactivate(svc *appcatalogadmin.Service, app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		bid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "priceBookId")))
		if err != nil || bid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_price_book_id", "invalid priceBookId")
			return
		}
		row, err := svc.DeactivatePriceBook(r.Context(), orgID, bid)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		if app != nil && app.EnterpriseAudit != nil {
			bs := bid.String()
			md, _ := json.Marshal(map[string]any{"price_book_id": bs})
			at, aid := compliance.ActorUser, ""
			if p, ok := auth.PrincipalFromContext(r.Context()); ok {
				at, aid = p.Actor()
			}
			_ = app.EnterpriseAudit.Record(r.Context(), compliance.EnterpriseAuditRecord{
				OrganizationID: orgID,
				ActorType:      at,
				ActorID:        stringPtrOrNil(aid),
				Action:         compliance.ActionPriceBookDeactivated,
				ResourceType:   "catalog.price_book",
				ResourceID:     &bs,
				Metadata:       md,
				Outcome:        compliance.OutcomeSuccess,
			})
		}
		writeJSON(w, http.StatusOK, mapPriceBook(row))
	}
}

func postAdminPriceBookActivate(svc *appcatalogadmin.Service, app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		bid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "priceBookId")))
		if err != nil || bid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_price_book_id", "invalid priceBookId")
			return
		}
		row, err := svc.ActivatePriceBook(r.Context(), orgID, bid)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		if app != nil && app.EnterpriseAudit != nil {
			bs := bid.String()
			md, _ := json.Marshal(map[string]any{"price_book_id": bs})
			at, aid := compliance.ActorUser, ""
			if p, ok := auth.PrincipalFromContext(r.Context()); ok {
				at, aid = p.Actor()
			}
			_ = app.EnterpriseAudit.Record(r.Context(), compliance.EnterpriseAuditRecord{
				OrganizationID: orgID,
				ActorType:      at,
				ActorID:        stringPtrOrNil(aid),
				Action:         compliance.ActionPriceBookActivated,
				ResourceType:   "catalog.price_book",
				ResourceID:     &bs,
				Metadata:       md,
				Outcome:        compliance.OutcomeSuccess,
			})
		}
		writeJSON(w, http.StatusOK, mapPriceBook(row))
	}
}

func postAdminPriceBookArchive(svc *appcatalogadmin.Service, app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		bid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "priceBookId")))
		if err != nil || bid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_price_book_id", "invalid priceBookId")
			return
		}
		row, err := svc.DeactivatePriceBook(r.Context(), orgID, bid)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		if app != nil && app.EnterpriseAudit != nil {
			bs := bid.String()
			md, _ := json.Marshal(map[string]any{"price_book_id": bs})
			at, aid := compliance.ActorUser, ""
			if p, ok := auth.PrincipalFromContext(r.Context()); ok {
				at, aid = p.Actor()
			}
			_ = app.EnterpriseAudit.Record(r.Context(), compliance.EnterpriseAuditRecord{
				OrganizationID: orgID,
				ActorType:      at,
				ActorID:        stringPtrOrNil(aid),
				Action:         compliance.ActionPriceBookArchived,
				ResourceType:   "catalog.price_book",
				ResourceID:     &bs,
				Metadata:       md,
				Outcome:        compliance.OutcomeSuccess,
			})
		}
		writeJSON(w, http.StatusOK, mapPriceBook(row))
	}
}

func getAdminPriceBookItems(svc *appcatalogadmin.Service) http.HandlerFunc {
	type row struct {
		ProductID      string `json:"productId"`
		UnitPriceMinor int64  `json:"unitPriceMinor"`
		PriceBookID    string `json:"priceBookId"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		bid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "priceBookId")))
		if err != nil || bid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_price_book_id", "invalid priceBookId")
			return
		}
		items, err := svc.ListPriceBookItems(r.Context(), orgID, bid)
		if err != nil {
			if errors.Is(err, appcatalogadmin.ErrNotFound) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "not_found", "price book not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		out := make([]row, 0, len(items))
		for _, it := range items {
			out = append(out, row{
				ProductID:      it.ProductID.String(),
				UnitPriceMinor: it.UnitPriceMinor,
				PriceBookID:    it.PriceBookID.String(),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": out})
	}
}

func putAdminPriceBookItems(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		bid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "priceBookId")))
		if err != nil || bid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_price_book_id", "invalid priceBookId")
			return
		}
		var body V1AdminPriceBookItemsPutRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		items := make([]appcatalogadmin.PriceBookItemRow, 0, len(body.Items))
		for _, it := range body.Items {
			pid, err := uuid.Parse(strings.TrimSpace(it.ProductID))
			if err != nil || pid == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_product_id", "invalid productId in items")
				return
			}
			items = append(items, appcatalogadmin.PriceBookItemRow{
				ProductID:      pid,
				UnitPriceMinor: it.UnitPriceMinor,
			})
		}
		if err := svc.ReplacePriceBookItems(r.Context(), orgID, bid, items); err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func patchAdminPriceBookItem(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		bid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "priceBookId")))
		if err != nil || bid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_price_book_id", "invalid priceBookId")
			return
		}
		pid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "productId")))
		if err != nil || pid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_product_id", "invalid productId")
			return
		}
		var body struct {
			UnitPriceMinor int64 `json:"unitPriceMinor"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		row, err := svc.UpsertPriceBookItem(r.Context(), orgID, bid, pid, body.UnitPriceMinor)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"productId":      row.ProductID.String(),
			"priceBookId":    row.PriceBookID.String(),
			"unitPriceMinor": row.UnitPriceMinor,
		})
	}
}

func deleteAdminPriceBookItem(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		bid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "priceBookId")))
		if err != nil || bid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_price_book_id", "invalid priceBookId")
			return
		}
		pid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "productId")))
		if err != nil || pid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_product_id", "invalid productId")
			return
		}
		if err := svc.DeletePriceBookItem(r.Context(), orgID, bid, pid); err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postAdminPriceBookAssignTarget(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		bid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "priceBookId")))
		if err != nil || bid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_price_book_id", "invalid priceBookId")
			return
		}
		var body V1AdminPriceBookAssignTargetRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		var siteID *uuid.UUID
		var machineID *uuid.UUID
		if body.SiteID != nil {
			u, err := uuid.Parse(strings.TrimSpace(*body.SiteID))
			if err != nil || u == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_site_id", "invalid siteId")
				return
			}
			siteID = &u
		}
		if body.MachineID != nil {
			u, err := uuid.Parse(strings.TrimSpace(*body.MachineID))
			if err != nil || u == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
				return
			}
			machineID = &u
		}
		row, err := svc.AssignPriceBookTarget(r.Context(), appcatalogadmin.AssignPriceBookTargetInput{
			OrganizationID: orgID,
			PriceBookID:    bid,
			SiteID:         siteID,
			MachineID:      machineID,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"id":          row.ID.String(),
			"priceBookId": row.PriceBookID.String(),
			"siteId":      uuidPtrFromPgUUID(row.SiteID),
			"machineId":   uuidPtrFromPgUUID(row.MachineID),
			"createdAt":   formatAPITimeRFC3339Nano(row.CreatedAt),
		})
	}
}

func deleteAdminPriceBookTarget(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := requireWriteIdempotencyKey(r); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		bid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "priceBookId")))
		if err != nil || bid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_price_book_id", "invalid priceBookId")
			return
		}
		tid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "targetId")))
		if err != nil || tid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_target_id", "invalid targetId")
			return
		}
		if err := svc.DeletePriceBookTarget(r.Context(), orgID, bid, tid); err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postAdminPricingPreview(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		var body V1AdminPricingPreviewRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		if len(body.ProductIDs) == 0 {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_argument", "productIds required")
			return
		}
		pids := make([]uuid.UUID, 0, len(body.ProductIDs))
		for _, s := range body.ProductIDs {
			u, err := uuid.Parse(strings.TrimSpace(s))
			if err != nil || u == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_product_id", "invalid productIds entry")
				return
			}
			pids = append(pids, u)
		}
		var mid *uuid.UUID
		if body.MachineID != nil && strings.TrimSpace(*body.MachineID) != "" {
			u, err := uuid.Parse(strings.TrimSpace(*body.MachineID))
			if err != nil || u == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
				return
			}
			mid = &u
		}
		var sid *uuid.UUID
		if body.SiteID != nil && strings.TrimSpace(*body.SiteID) != "" {
			u, err := uuid.Parse(strings.TrimSpace(*body.SiteID))
			if err != nil || u == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_site_id", "invalid siteId")
				return
			}
			sid = &u
		}
		var at time.Time
		if body.At != nil && strings.TrimSpace(*body.At) != "" {
			at, err = parseRFC3339Flexible(*body.At)
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_argument", "at must be RFC3339/RFC3339Nano")
				return
			}
		}
		res, err := svc.PreviewPricing(r.Context(), appcatalogadmin.PricingPreviewParams{
			OrganizationID: orgID,
			MachineID:      mid,
			SiteID:         sid,
			ProductIDs:     pids,
			At:             at,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		lines := make([]V1AdminPricingPreviewLine, 0, len(res.Lines))
		for _, ln := range res.Lines {
			pbid := ""
			if ln.PriceBookID != uuid.Nil {
				pbid = ln.PriceBookID.String()
			}
			lines = append(lines, V1AdminPricingPreviewLine{
				ProductID:      ln.ProductID.String(),
				BasePrice:      ln.BasePriceMinor,
				EffectivePrice: ln.EffectiveMinor,
				Currency:       ln.Currency,
				PriceBookID:    pbid,
				AppliedRuleIDs: ln.AppliedRuleIDs,
				Reasons:        ln.Reasons,
			})
		}
		writeJSON(w, http.StatusOK, V1AdminPricingPreviewResponse{
			At:       formatAPITimeRFC3339Nano(res.At),
			Currency: res.Currency,
			Lines:    lines,
		})
	}
}
