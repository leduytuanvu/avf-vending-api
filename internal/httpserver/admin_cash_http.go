package httpserver

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appinventoryadmin "github.com/avf/avf-vending-api/internal/app/inventoryadmin"
	cashdomain "github.com/avf/avf-vending-api/internal/domain/cash"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func mountAdminCashSettlementRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.TelemetryStore == nil || app.TelemetryStore.Pool() == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	r.Get("/machines/{machineId}/cashbox", getAdminMachineCashbox(app))
	r.With(writeRL).Post("/machines/{machineId}/cash-collections", postAdminMachineCashCollectionStart(app))
	r.Get("/machines/{machineId}/cash-collections", listAdminMachineCashCollections(app))
	r.With(writeRL).Post("/machines/{machineId}/cash-collections/{collectionId}/close", postAdminMachineCashCollectionClose(app))
	r.Get("/machines/{machineId}/cash-collections/{collectionId}", getAdminMachineCashCollection(app))
}

func cashVarianceThreshold(app *api.HTTPApplication) int64 {
	if app == nil {
		return 500
	}
	if app.CashSettlementVarianceReviewThresholdMinor > 0 {
		return app.CashSettlementVarianceReviewThresholdMinor
	}
	return 500
}

func getAdminMachineCashbox(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.TelemetryStore == nil {
			writeCapabilityNotConfigured(w, r.Context(), "database", "database pool is not configured for this API process")
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		head, err := resolveInventoryMachine(r, app.InventoryAdmin, machineID)
		if err != nil {
			writeInventoryAccessOrResolveError(w, r, err)
			return
		}
		cur := strings.TrimSpace(r.URL.Query().Get("currency"))
		summary, err := app.TelemetryStore.GetMachineCashboxSummary(r.Context(), head.OrganizationID, machineID, cur, cashVarianceThreshold(app))
		if err != nil {
			writeCashSettlementError(w, r, err)
			return
		}
		var openID *string
		if summary.OpenCollectionID != nil {
			s := summary.OpenCollectionID.String()
			openID = &s
		}
		var last *string
		if summary.LastClosedAt != nil {
			s := formatAPITimeRFC3339Nano(*summary.LastClosedAt)
			last = &s
		}
		denoms := make([]V1CashDenominationExpectation, 0)
		writeJSON(w, http.StatusOK, V1AdminMachineCashboxResponse{
			MachineID:                    machineID.String(),
			Currency:                     summary.Currency,
			ExpectedCashboxMinor:         summary.ExpectedCashboxMinor,
			ExpectedCloudCashMinor:       summary.ExpectedCashboxMinor,
			ExpectedRecyclerMinor:        summary.ExpectedRecyclerMinor,
			LastCollectionAt:             last,
			Denominations:                denoms,
			OpenCollectionID:             openID,
			VarianceReviewThresholdMinor: summary.VarianceReviewThresholdMinor,
			Disclosure:                   "Accounting-only: cloud ledger expectation only; does not sense or command physical cash hardware.",
		})
	}
}

type cashCollectionStartBody struct {
	OperatorSessionID string `json:"operator_session_id"`
	StartedAt         string `json:"startedAt"`
	Currency          string `json:"currency"`
	Notes             string `json:"notes,omitempty"`
}

func postAdminMachineCashCollectionStart(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.TelemetryStore == nil {
			writeCapabilityNotConfigured(w, r.Context(), "database", "database pool is not configured for this API process")
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		head, err := resolveInventoryMachine(r, app.InventoryAdmin, machineID)
		if err != nil {
			writeInventoryAccessOrResolveError(w, r, err)
			return
		}
		idem, err := requireWriteIdempotencyKey(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		var body cashCollectionStartBody
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		sid, ok := parseOperatorSessionIDField(w, r, body.OperatorSessionID)
		if !ok {
			return
		}
		if !requireActiveOperatorSession(w, r, app, machineID, sid) {
			return
		}
		openedAt := time.Now().UTC()
		if ts := strings.TrimSpace(body.StartedAt); ts != "" {
			t, perr := time.Parse(time.RFC3339Nano, ts)
			if perr != nil {
				t, perr = time.Parse(time.RFC3339, ts)
			}
			if perr != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_started_at", "startedAt must be RFC3339 or RFC3339Nano")
				return
			}
			openedAt = t.UTC()
		}
		cur := strings.TrimSpace(body.Currency)
		if cur == "" {
			cur = "USD"
		}
		row, err := app.TelemetryStore.StartMachineCashCollection(r.Context(), postgres.StartMachineCashCollectionInput{
			OrganizationID:      head.OrganizationID,
			MachineID:           machineID,
			OperatorSessionID:   &sid,
			Currency:            cur,
			Notes:               body.Notes,
			StartIdempotencyKey: idem,
			CorrelationID:       correlationUUIDFromRequest(r.Context()),
			OpenedAt:            openedAt,
		})
		if err != nil {
			writeCashSettlementError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, v1CashCollectionFromDB(row))
	}
}

func listAdminMachineCashCollections(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.TelemetryStore == nil {
			writeCapabilityNotConfigured(w, r.Context(), "database", "database pool is not configured for this API process")
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		head, err := resolveInventoryMachine(r, app.InventoryAdmin, machineID)
		if err != nil {
			writeInventoryAccessOrResolveError(w, r, err)
			return
		}
		limit, offset, perr := parseAdminLimitOffset(r)
		if perr != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", perr.Error())
			return
		}
		rows, err := app.TelemetryStore.ListMachineCashCollections(r.Context(), head.OrganizationID, machineID, limit, offset)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]V1AdminCashCollection, 0, len(rows))
		for _, row := range rows {
			items = append(items, v1CashCollectionFromDB(row))
		}
		writeJSON(w, http.StatusOK, V1AdminCashCollectionListResponse{
			Items: items,
			Meta: V1CollectionListMeta{
				Limit:    limit,
				Offset:   offset,
				Returned: len(items),
			},
		})
	}
}

func getAdminMachineCashCollection(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.TelemetryStore == nil {
			writeCapabilityNotConfigured(w, r.Context(), "database", "database pool is not configured for this API process")
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		collectionID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "collectionId")))
		if err != nil || collectionID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_collection_id", "invalid collectionId")
			return
		}
		head, err := resolveInventoryMachine(r, app.InventoryAdmin, machineID)
		if err != nil {
			writeInventoryAccessOrResolveError(w, r, err)
			return
		}
		row, err := app.TelemetryStore.GetMachineCashCollection(r.Context(), head.OrganizationID, machineID, collectionID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "collection_not_found", "collection not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, v1CashCollectionFromDB(row))
	}
}

type cashCollectionCloseBody struct {
	OperatorSessionID    string `json:"operator_session_id"`
	Currency             string `json:"currency"`
	Notes                string `json:"notes,omitempty"`
	CountedAmountMinor   *int64 `json:"counted_amount_minor,omitempty"`
	CountedCashboxMinor  *int64 `json:"countedCashboxMinor,omitempty"`
	CountedRecyclerMinor *int64 `json:"countedRecyclerMinor,omitempty"`
	Denominations        []struct {
		DenominationMinor int64 `json:"denominationMinor"`
		Count             int64 `json:"count"`
	} `json:"denominations,omitempty"`
	ClosedAt            *string `json:"closedAt,omitempty"`
	EvidenceArtifactURL *string `json:"evidence_artifact_url,omitempty"`
	Evidence            *struct {
		PhotoArtifactID string `json:"photoArtifactId"`
	} `json:"evidence,omitempty"`
}

func postAdminMachineCashCollectionClose(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.TelemetryStore == nil {
			writeCapabilityNotConfigured(w, r.Context(), "database", "database pool is not configured for this API process")
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		collectionID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "collectionId")))
		if err != nil || collectionID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_collection_id", "invalid collectionId")
			return
		}
		head, err := resolveInventoryMachine(r, app.InventoryAdmin, machineID)
		if err != nil {
			writeInventoryAccessOrResolveError(w, r, err)
			return
		}
		var body cashCollectionCloseBody
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		curClose := strings.TrimSpace(body.Currency)
		if curClose == "" {
			curClose = "USD"
		}
		sid, ok := parseOperatorSessionIDField(w, r, body.OperatorSessionID)
		if !ok {
			return
		}
		if !requireActiveOperatorSession(w, r, app, machineID, sid) {
			return
		}
		ev := ""
		if body.EvidenceArtifactURL != nil {
			ev = strings.TrimSpace(*body.EvidenceArtifactURL)
		}
		photoID := ""
		if body.Evidence != nil {
			photoID = strings.TrimSpace(body.Evidence.PhotoArtifactID)
		}
		usesExtended := len(body.Denominations) > 0 ||
			(body.ClosedAt != nil && strings.TrimSpace(*body.ClosedAt) != "") ||
			photoID != "" ||
			body.CountedCashboxMinor != nil ||
			body.CountedRecyclerMinor != nil

		var total, cashbox, recycler int64
		if usesExtended {
			if body.CountedCashboxMinor == nil || body.CountedRecyclerMinor == nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_counted_split", "countedCashboxMinor and countedRecyclerMinor are required for this close payload shape")
				return
			}
			cashbox = *body.CountedCashboxMinor
			recycler = *body.CountedRecyclerMinor
			total = cashbox + recycler
		} else {
			if body.CountedAmountMinor == nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_counted_amount", "counted_amount_minor is required unless countedCashboxMinor/countedRecyclerMinor are provided")
				return
			}
			total = *body.CountedAmountMinor
		}
		denoms := make([]postgres.CloseDenominationCount, 0, len(body.Denominations))
		for _, d := range body.Denominations {
			denoms = append(denoms, postgres.CloseDenominationCount{
				DenominationMinor: d.DenominationMinor,
				Count:             d.Count,
			})
		}
		closedAtStr := ""
		if body.ClosedAt != nil {
			closedAtStr = strings.TrimSpace(*body.ClosedAt)
		}
		row, err := app.TelemetryStore.CloseMachineCashCollection(r.Context(), postgres.CloseMachineCashCollectionInput{
			OrganizationID:          head.OrganizationID,
			MachineID:               machineID,
			CollectionID:            collectionID,
			OperatorSessionID:       &sid,
			CountedAmountMinor:      total,
			CountedCashboxMinor:     cashbox,
			CountedRecyclerMinor:    recycler,
			Currency:                curClose,
			Notes:                   body.Notes,
			EvidenceArtifactURL:     ev,
			EvidencePhotoArtifactID: photoID,
			Denominations:           denoms,
			ClosedAtRFC3339:         closedAtStr,
			CorrelationID:           correlationUUIDFromRequest(r.Context()),
			VarianceReviewThreshold: cashVarianceThreshold(app),
			UsesExtendedCloseHash:   usesExtended,
		})
		if err != nil {
			writeCashSettlementError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, v1CashCollectionFromDB(row))
	}
}

func writeCashSettlementError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		writeAPIError(w, r.Context(), http.StatusNotFound, "machine_not_found", "machine not found")
	case errors.Is(err, postgres.ErrMachineOrganizationMismatch), errors.Is(err, appinventoryadmin.ErrMachineNotFound):
		writeAPIError(w, r.Context(), http.StatusNotFound, "machine_not_found", "machine not found")
	case errors.Is(err, appinventoryadmin.ErrForbidden):
		writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, cashdomain.ErrCollectionNotFound):
		writeAPIError(w, r.Context(), http.StatusNotFound, "collection_not_found", "collection not found")
	case errors.Is(err, cashdomain.ErrOpenCollectionExists):
		writeAPIError(w, r.Context(), http.StatusConflict, "open_collection_exists", "machine already has an open cash collection")
	case errors.Is(err, cashdomain.ErrClosePayloadConflict):
		writeAPIError(w, r.Context(), http.StatusConflict, "close_payload_conflict", "close payload does not match stored close")
	case errors.Is(err, cashdomain.ErrInvalidCountedAmount):
		writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_counted_amount", "counted_amount_minor must be non-negative")
	case errors.Is(err, cashdomain.ErrCurrencyMismatch):
		writeAPIError(w, r.Context(), http.StatusBadRequest, "currency_mismatch", "currency does not match open collection")
	default:
		if strings.Contains(strings.ToLower(err.Error()), "idempotency") {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
	}
}

func cashCollectionReviewState(row db.CashCollection) string {
	switch strings.ToLower(strings.TrimSpace(row.LifecycleStatus)) {
	case "open":
		return "open"
	case "closed":
		if row.RequiresReview {
			return "pending_review"
		}
		if row.VarianceAmountMinor == 0 {
			return "matched"
		}
		return "variance_recorded"
	default:
		if row.RequiresReview {
			return "pending_review"
		}
		return strings.ToLower(strings.TrimSpace(row.LifecycleStatus))
	}
}

func v1CashCollectionFromDB(row db.CashCollection) V1AdminCashCollection {
	var closedAt *string
	if row.ClosedAt.Valid {
		s := formatAPITimeRFC3339Nano(row.ClosedAt.Time)
		closedAt = &s
	}
	hash := postgres.FormatCloseRequestHashHex(row.CloseRequestHash)
	var hashPtr *string
	if hash != "" {
		hashPtr = &hash
	}
	phys := int64(0)
	cloud := int64(0)
	variance := int64(0)
	if strings.EqualFold(strings.TrimSpace(row.LifecycleStatus), "closed") {
		phys = row.AmountMinor
		cloud = row.ExpectedAmountMinor
		variance = row.VarianceAmountMinor
	}
	return V1AdminCashCollection{
		ID:                       row.ID.String(),
		MachineID:                row.MachineID.String(),
		OrganizationID:           row.OrganizationID.String(),
		CollectedAt:              formatAPITimeRFC3339Nano(row.CollectedAt),
		OpenedAt:                 formatAPITimeRFC3339Nano(row.OpenedAt),
		ClosedAt:                 closedAt,
		LifecycleStatus:          row.LifecycleStatus,
		CountedAmountMinor:       row.AmountMinor,
		ExpectedAmountMinor:      row.ExpectedAmountMinor,
		VarianceAmountMinor:      row.VarianceAmountMinor,
		CountedPhysicalCashMinor: phys,
		ExpectedCloudCashMinor:   cloud,
		VarianceMinor:            variance,
		ReviewState:              cashCollectionReviewState(row),
		RequiresReview:           row.RequiresReview,
		CloseRequestHashHex:      hashPtr,
		Currency:                 row.Currency,
		ReconciliationStatus:     row.ReconciliationStatus,
		Disclosure:               "Accounting-only: cloud ledger vs operator physical count; does not sense or command hardware.",
	}
}
