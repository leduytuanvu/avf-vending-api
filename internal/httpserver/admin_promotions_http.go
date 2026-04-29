package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// rbac:inherited-mount: routes are registered from mountAdminCatalogRoutes (catalog read/write permission layers).

func registerAdminPromotionReadRoutes(r chi.Router, svc *appcatalogadmin.Service) {
	r.Get("/promotions", listAdminPromotions(svc))
	r.Post("/promotions/preview", postAdminPromotionPreview(svc))
	r.Get("/promotions/{promotionId}", getAdminPromotionDetail(svc))
}

func registerAdminPromotionWriteRoutes(r chi.Router, svc *appcatalogadmin.Service, writeRL func(http.Handler) http.Handler) {
	r.With(writeRL).Post("/promotions", postAdminPromotionCreate(svc))
	r.With(writeRL).Patch("/promotions/{promotionId}", patchAdminPromotion(svc))
	r.With(writeRL).Post("/promotions/{promotionId}/activate", postAdminPromotionActivate(svc))
	r.With(writeRL).Post("/promotions/{promotionId}/pause", postAdminPromotionPause(svc))
	r.With(writeRL).Post("/promotions/{promotionId}/deactivate", postAdminPromotionDeactivate(svc))
	r.With(writeRL).Post("/promotions/{promotionId}/archive", postAdminPromotionArchive(svc))
	r.With(writeRL).Post("/promotions/{promotionId}/assign-target", postAdminPromotionAssignTarget(svc))
	r.With(writeRL).Delete("/promotions/{promotionId}/targets/{targetId}", deleteAdminPromotionTarget(svc))
}

func listAdminPromotions(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		limit, offset, err := parseAdminLimitOffset(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", err.Error())
			return
		}
		incDeact := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("include_deactivated")), "true") ||
			strings.TrimSpace(r.URL.Query().Get("include_deactivated")) == "1"
		rows, total, err := svc.ListPromotions(r.Context(), appcatalogadmin.ListPromotionsParams{
			OrganizationID:     orgID,
			Limit:              limit,
			Offset:             offset,
			IncludeDeactivated: incDeact,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]V1AdminPromotion, 0, len(rows))
		for _, row := range rows {
			items = append(items, mapAdminPromotion(row))
		}
		writeJSON(w, http.StatusOK, V1AdminPromotionListEnvelope{
			Items: items,
			Meta: V1AdminPageMeta{
				Limit:      limit,
				Offset:     offset,
				Returned:   len(items),
				TotalCount: total,
			},
		})
	}
}

func getAdminPromotionDetail(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		pid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "promotionId")))
		if err != nil || pid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_promotion_id", "invalid promotionId")
			return
		}
		row, err := svc.GetPromotion(r.Context(), orgID, pid)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		rules, err := svc.ListPromotionRules(r.Context(), pid)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		targets, err := svc.ListPromotionTargets(r.Context(), orgID, pid)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		ruleItems := make([]V1AdminPromotionRule, 0, len(rules))
		for _, ru := range rules {
			ruleItems = append(ruleItems, mapAdminPromotionRule(ru))
		}
		tgtItems := make([]V1AdminPromotionTarget, 0, len(targets))
		for _, t := range targets {
			tgtItems = append(tgtItems, mapAdminPromotionTarget(t))
		}
		writeJSON(w, http.StatusOK, V1AdminPromotionDetail{
			Promotion: mapAdminPromotion(row),
			Rules:     ruleItems,
			Targets:   tgtItems,
		})
	}
}

func postAdminPromotionCreate(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		var body V1AdminPromotionCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		startsAt, err := parseRFC3339Flexible(body.StartsAt)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_argument", "startsAt must be RFC3339/RFC3339Nano")
			return
		}
		endsAt, err := parseRFC3339Flexible(body.EndsAt)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_argument", "endsAt must be RFC3339/RFC3339Nano")
			return
		}
		rules := make([]appcatalogadmin.PromotionRuleInput, 0, len(body.Rules))
		for _, rr := range body.Rules {
			payload := rr.Payload
			if len(payload) == 0 {
				payload = json.RawMessage([]byte(`{}`))
			}
			rules = append(rules, appcatalogadmin.PromotionRuleInput{
				RuleType: rr.RuleType,
				Payload:  payload,
				Priority: rr.Priority,
			})
		}
		row, err := svc.CreatePromotion(r.Context(), appcatalogadmin.CreatePromotionInput{
			OrganizationID:   orgID,
			Name:             body.Name,
			StartsAt:         startsAt,
			EndsAt:           endsAt,
			Priority:         body.Priority,
			Stackable:        body.Stackable,
			BudgetLimitMinor: body.BudgetLimitMinor,
			RedemptionLimit:  body.RedemptionLimit,
			ChannelScope:     body.ChannelScope,
			Rules:            rules,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminPromotion(row))
	}
}

func patchAdminPromotion(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		pid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "promotionId")))
		if err != nil || pid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_promotion_id", "invalid promotionId")
			return
		}
		var body V1AdminPromotionPatchRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		var patch appcatalogadmin.PatchPromotionInput
		patch.Name = body.Name
		if body.StartsAt != nil {
			t, err := parseRFC3339Flexible(*body.StartsAt)
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_argument", "startsAt must be RFC3339/RFC3339Nano")
				return
			}
			patch.StartsAt = &t
		}
		if body.EndsAt != nil {
			t, err := parseRFC3339Flexible(*body.EndsAt)
			if err != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_argument", "endsAt must be RFC3339/RFC3339Nano")
				return
			}
			patch.EndsAt = &t
		}
		patch.Priority = body.Priority
		patch.Stackable = body.Stackable
		patch.BudgetLimitMinor = body.BudgetLimitMinor
		patch.RedemptionLimit = body.RedemptionLimit
		patch.ChannelScope = body.ChannelScope
		patch.ApprovalStatus = body.ApprovalStatus
		if body.Rules != nil {
			rules := make([]appcatalogadmin.PromotionRuleInput, 0, len(*body.Rules))
			for _, rr := range *body.Rules {
				payload := rr.Payload
				if len(payload) == 0 {
					payload = json.RawMessage([]byte(`{}`))
				}
				rules = append(rules, appcatalogadmin.PromotionRuleInput{
					RuleType: rr.RuleType,
					Payload:  payload,
					Priority: rr.Priority,
				})
			}
			patch.Rules = &rules
		}
		row, err := svc.PatchPromotion(r.Context(), orgID, pid, patch)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminPromotion(row))
	}
}

func postAdminPromotionActivate(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		pid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "promotionId")))
		if err != nil || pid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_promotion_id", "invalid promotionId")
			return
		}
		row, err := svc.ActivatePromotion(r.Context(), orgID, pid)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminPromotion(row))
	}
}

func postAdminPromotionPause(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		pid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "promotionId")))
		if err != nil || pid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_promotion_id", "invalid promotionId")
			return
		}
		row, err := svc.PausePromotion(r.Context(), orgID, pid)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminPromotion(row))
	}
}

func postAdminPromotionDeactivate(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		pid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "promotionId")))
		if err != nil || pid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_promotion_id", "invalid promotionId")
			return
		}
		row, err := svc.DeactivatePromotion(r.Context(), orgID, pid)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminPromotion(row))
	}
}

func postAdminPromotionArchive(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		pid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "promotionId")))
		if err != nil || pid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_promotion_id", "invalid promotionId")
			return
		}
		row, err := svc.DeactivatePromotion(r.Context(), orgID, pid)
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminPromotion(row))
	}
}

func postAdminPromotionAssignTarget(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		pid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "promotionId")))
		if err != nil || pid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_promotion_id", "invalid promotionId")
			return
		}
		var body V1AdminPromotionAssignTargetRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid JSON body")
			return
		}
		productID, err := uuidFromOptionalString(body.ProductID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_product_id", "invalid productId")
			return
		}
		categoryID, err := uuidFromOptionalString(body.CategoryID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_category_id", "invalid categoryId")
			return
		}
		machineID, err := uuidFromOptionalString(body.MachineID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		siteID, err := uuidFromOptionalString(body.SiteID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_site_id", "invalid siteId")
			return
		}
		orgTargetID, err := uuidFromOptionalString(body.OrganizationTargetID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_organization_target_id", "invalid organizationTargetId")
			return
		}
		tagID, err := uuidFromOptionalString(body.TagID)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_tag_id", "invalid tagId")
			return
		}
		row, err := svc.AssignPromotionTarget(r.Context(), appcatalogadmin.AssignPromotionTargetInput{
			OrganizationID: orgID,
			PromotionID:    pid,
			TargetType:     body.TargetType,
			ProductID:      productID,
			CategoryID:     categoryID,
			MachineID:      machineID,
			SiteID:         siteID,
			OrgTargetID:    orgTargetID,
			TagID:          tagID,
		})
		if err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, mapAdminPromotionTarget(row))
	}
}

func deleteAdminPromotionTarget(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		pid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "promotionId")))
		if err != nil || pid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_promotion_id", "invalid promotionId")
			return
		}
		tid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "targetId")))
		if err != nil || tid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_target_id", "invalid targetId")
			return
		}
		if err := svc.DeletePromotionTarget(r.Context(), orgID, pid, tid); err != nil {
			writeAdminCatalogError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postAdminPromotionPreview(svc *appcatalogadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orgID, err := adminCatalogOrganizationID(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_scope", err.Error())
			return
		}
		var body V1AdminPromotionPreviewRequest
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
		res, err := svc.PreviewPromotions(r.Context(), appcatalogadmin.PromotionPreviewParams{
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
		lines := make([]V1AdminPromotionPreviewLine, 0, len(res.Lines))
		for _, ln := range res.Lines {
			appliedPromos := make([]string, 0, len(ln.AppliedPromotionIDs))
			for _, id := range ln.AppliedPromotionIDs {
				appliedPromos = append(appliedPromos, id.String())
			}
			skipped := make([]V1AdminPromotionSkippedRule, 0, len(ln.SkippedRules))
			for _, sk := range ln.SkippedRules {
				item := V1AdminPromotionSkippedRule{
					PromotionID: sk.PromotionID.String(),
					RuleType:    sk.RuleType,
					Reason:      sk.Reason,
				}
				if sk.RuleID != uuid.Nil {
					item.RuleID = sk.RuleID.String()
				}
				skipped = append(skipped, item)
			}
			lines = append(lines, V1AdminPromotionPreviewLine{
				ProductID:           ln.ProductID.String(),
				BasePriceMinor:      ln.BasePriceMinor,
				DiscountMinor:       ln.DiscountMinor,
				FinalPriceMinor:     ln.FinalPriceMinor,
				Currency:            ln.Currency,
				AppliedPromotionIDs: appliedPromos,
				AppliedRuleIDs:      ln.AppliedRuleIDs,
				SkippedRules:        skipped,
			})
		}
		writeJSON(w, http.StatusOK, V1AdminPromotionPreviewResponse{
			At:    formatAPITimeRFC3339Nano(res.At),
			Lines: lines,
		})
	}
}

func mapAdminPromotion(p db.Promotion) V1AdminPromotion {
	out := V1AdminPromotion{
		ID:              p.ID.String(),
		OrganizationID:  p.OrganizationID.String(),
		Name:            p.Name,
		ApprovalStatus:  p.ApprovalStatus,
		LifecycleStatus: p.LifecycleStatus,
		Priority:        p.Priority,
		Stackable:       p.Stackable,
		StartsAt:        formatAPITimeRFC3339Nano(p.StartsAt),
		EndsAt:          formatAPITimeRFC3339Nano(p.EndsAt),
		CreatedAt:       formatAPITimeRFC3339Nano(p.CreatedAt),
		UpdatedAt:       formatAPITimeRFC3339Nano(p.UpdatedAt),
	}
	if p.BudgetLimitMinor.Valid {
		v := p.BudgetLimitMinor.Int64
		out.BudgetLimitMinor = &v
	}
	if p.RedemptionLimit.Valid {
		v := p.RedemptionLimit.Int32
		out.RedemptionLimit = &v
	}
	if p.ChannelScope.Valid {
		s := p.ChannelScope.String
		out.ChannelScope = &s
	}
	return out
}

func mapAdminPromotionRule(r db.PromotionRule) V1AdminPromotionRule {
	payload := r.Payload
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}
	return V1AdminPromotionRule{
		ID:          r.ID.String(),
		PromotionID: r.PromotionID.String(),
		RuleType:    r.RuleType,
		Priority:    r.Priority,
		Payload:     json.RawMessage(bytes.Clone(payload)),
	}
}

func mapAdminPromotionTarget(t db.PromotionTarget) V1AdminPromotionTarget {
	out := V1AdminPromotionTarget{
		ID:             t.ID.String(),
		PromotionID:    t.PromotionID.String(),
		OrganizationID: t.OrganizationID.String(),
		TargetType:     t.TargetType,
		CreatedAt:      formatAPITimeRFC3339Nano(t.CreatedAt),
	}
	if t.ProductID.Valid {
		s := uuid.UUID(t.ProductID.Bytes).String()
		out.ProductID = &s
	}
	if t.CategoryID.Valid {
		s := uuid.UUID(t.CategoryID.Bytes).String()
		out.CategoryID = &s
	}
	if t.MachineID.Valid {
		s := uuid.UUID(t.MachineID.Bytes).String()
		out.MachineID = &s
	}
	if t.SiteID.Valid {
		s := uuid.UUID(t.SiteID.Bytes).String()
		out.SiteID = &s
	}
	if t.OrganizationTargetID.Valid {
		s := uuid.UUID(t.OrganizationTargetID.Bytes).String()
		out.OrganizationTargetID = &s
	}
	if t.TagID.Valid {
		s := uuid.UUID(t.TagID.Bytes).String()
		out.TagID = &s
	}
	return out
}
