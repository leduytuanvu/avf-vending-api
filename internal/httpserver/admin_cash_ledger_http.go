package httpserver

import (
	"net/http"
	"strings"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func mountAdminCashLedgerRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.Commerce == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermCashRead))
		r.Get("/cash/ledger", listAdminCashLedger(app))
		r.Get("/cash/ledger/{eventId}", getAdminCashLedgerEvent(app))
		r.Get("/machines/{machineId}/cash-position", getAdminMachineCashPosition(app))
		r.Get("/orders/{orderId}/cash-forensics", getAdminOrderCashForensics(app))
	})
	r.Group(func(r chi.Router) {
		r.Use(auth.RequireAnyPermission(auth.PermCashWrite))
		r.With(writeRL).Post("/machines/{machineId}/cash-adjustments", postAdminCashAdjustment(app))
	})
}

func parseCashLedgerTimeRange(r *http.Request) (from, to time.Time, err error) {
	fromRaw := strings.TrimSpace(r.URL.Query().Get("from"))
	toRaw := strings.TrimSpace(r.URL.Query().Get("to"))
	if fromRaw == "" || toRaw == "" {
		return time.Time{}, time.Time{}, errInvalidTimeRange
	}
	from, err = time.Parse(time.RFC3339Nano, fromRaw)
	if err != nil {
		from, err = time.Parse(time.RFC3339, fromRaw)
	}
	if err != nil {
		return time.Time{}, time.Time{}, errInvalidTimeRange
	}
	to, err = time.Parse(time.RFC3339Nano, toRaw)
	if err != nil {
		to, err = time.Parse(time.RFC3339, toRaw)
	}
	if err != nil {
		return time.Time{}, time.Time{}, errInvalidTimeRange
	}
	return from.UTC(), to.UTC(), nil
}

var errInvalidTimeRange = &timeParseError{}

type timeParseError struct{}

func (e *timeParseError) Error() string { return "invalid time range" }

func listAdminCashLedger(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		from, to, err := parseCashLedgerTimeRange(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_time_range", "from and to must be RFC3339")
			return
		}
		machineRaw := strings.TrimSpace(r.URL.Query().Get("machineId"))
		if machineRaw == "" {
			machineRaw = strings.TrimSpace(r.URL.Query().Get("machine_id"))
		}
		machineID, err := uuid.Parse(machineRaw)
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "machineId required")
			return
		}
		limit, _, perr := parseAdminLimitOffset(r)
		if perr != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_pagination", perr.Error())
			return
		}
		var orderID *uuid.UUID
		if raw := strings.TrimSpace(r.URL.Query().Get("orderId")); raw != "" {
			oid, oerr := uuid.Parse(raw)
			if oerr == nil {
				orderID = &oid
			}
		}
		var afterTime *time.Time
		var afterID *uuid.UUID
		if cursor := strings.TrimSpace(r.URL.Query().Get("afterCursor")); cursor != "" {
			parts := strings.SplitN(cursor, "|", 2)
			if len(parts) == 2 {
				if t, terr := time.Parse(time.RFC3339Nano, parts[0]); terr == nil {
					afterTime = &t
				}
				if id, ierr := uuid.Parse(parts[1]); ierr == nil {
					afterID = &id
				}
			}
		}
		res, err := app.Commerce.ListCashLedger(r.Context(), appcommerce.ListCashLedgerInput{
			MachineID:       &machineID,
			OrderID:         orderID,
			From:            from,
			To:              to,
			Limit:           limit,
			AfterOccurredAt: afterTime,
			AfterID:         afterID,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "ledger_query_failed", err.Error())
			return
		}
		items := make([]V1CashLedgerItem, 0, len(res.Items))
		for _, row := range res.Items {
			var orderIDStr *string
			if row.OrderID != nil {
				s := row.OrderID.String()
				orderIDStr = &s
			}
			items = append(items, V1CashLedgerItem{
				EventID:           row.ID.String(),
				MachineID:         row.MachineID.String(),
				OrderID:           orderIDStr,
				MovementClass:     row.MovementClass,
				EventType:         row.EventType,
				Destination:       row.Destination,
				DenominationMinor: row.DenominationMinor,
				AmountMinor:       row.AmountMinor,
				Currency:          row.Currency,
				OccurredAt:        formatAPITimeRFC3339Nano(row.OccurredAtDevice),
				RecordedAt:        formatAPITimeRFC3339Nano(row.RecordedAt),
				EvidenceStatus:    row.EvidenceStatus,
				DeviceEventID:     row.DeviceEventID,
			})
		}
		var nextCursor *string
		if res.NextCursor != nil {
			s := formatAPITimeRFC3339Nano(res.NextCursor.OccurredAt) + "|" + res.NextCursor.ID.String()
			nextCursor = &s
		}
		writeJSON(w, http.StatusOK, V1CashLedgerListResponse{
			Items:      items,
			NextCursor: nextCursor,
			Meta:       V1CollectionListMeta{Limit: limit, Returned: len(items)},
		})
	}
}

func getAdminCashLedgerEvent(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		eventID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "eventId")))
		if err != nil || eventID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_event_id", "invalid eventId")
			return
		}
		movementClass := strings.TrimSpace(r.URL.Query().Get("movementClass"))
		if movementClass == "" {
			movementClass = "acceptance"
		}
		detail, err := app.Commerce.GetCashLedgerEventDetail(r.Context(), eventID, movementClass)
		if err != nil {
			if isCommerceNotFound(err) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "ledger_event_not_found", "event not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "ledger_detail_failed", err.Error())
			return
		}
		var orderIDStr *string
		if detail.Row.OrderID != nil {
			s := detail.Row.OrderID.String()
			orderIDStr = &s
		}
		writeJSON(w, http.StatusOK, V1CashLedgerEventDetailResponse{
			Event: V1CashLedgerItem{
				EventID:           detail.Row.ID.String(),
				MachineID:         detail.Row.MachineID.String(),
				OrderID:           orderIDStr,
				MovementClass:     detail.Row.MovementClass,
				EventType:         detail.Row.EventType,
				Destination:       detail.Row.Destination,
				DenominationMinor: detail.Row.DenominationMinor,
				AmountMinor:       detail.Row.AmountMinor,
				Currency:          detail.Row.Currency,
				OccurredAt:        formatAPITimeRFC3339Nano(detail.Row.OccurredAtDevice),
				RecordedAt:        formatAPITimeRFC3339Nano(detail.Row.RecordedAt),
				EvidenceStatus:    detail.Row.EvidenceStatus,
				DeviceEventID:     detail.Row.DeviceEventID,
			},
			Extra: detail.Extra,
		})
	}
}

func getAdminMachineCashPosition(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		cur := strings.TrimSpace(r.URL.Query().Get("currency"))
		var asOf *time.Time
		if raw := strings.TrimSpace(r.URL.Query().Get("asOf")); raw != "" {
			t, perr := time.Parse(time.RFC3339Nano, raw)
			if perr != nil {
				t, perr = time.Parse(time.RFC3339, raw)
			}
			if perr != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_as_of", "asOf must be RFC3339")
				return
			}
			utc := t.UTC()
			asOf = &utc
		}
		pos, err := app.Commerce.GetMachinePhysicalCashPosition(r.Context(), machineID, cur, asOf)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "cash_position_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, V1AdminMachineCashPositionResponse{
			MachineID:                     pos.MachineID.String(),
			Currency:                      pos.Currency,
			AsOf:                          formatAPITimeRFC3339Nano(pos.AsOf),
			PhysicalExpectedCashboxMinor:  pos.PhysicalExpectedCashboxMinor,
			PhysicalExpectedRecyclerMinor: pos.PhysicalExpectedRecyclerMinor,
			PhysicalExpectedMachineMinor:  pos.PhysicalExpectedMachineMinor,
			SalesNetExpectedMinor:         pos.SalesNetExpectedMinor,
			ObservedRecyclerMinor:         pos.ObservedRecyclerMinor,
			ObservedRecyclerCount:         pos.ObservedRecyclerCount,
			ObservedRecyclerDenomMinor:    pos.ObservedRecyclerDenomMinor,
			UnresolvedLiabilityMinor:      pos.UnresolvedLiabilityMinor,
			AmbiguousEventCount:           pos.AmbiguousEventCount,
			EvidenceStatus:                pos.EvidenceStatus,
			Disclosure:                    pos.Disclosure,
		})
	}
}

func getAdminOrderCashForensics(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orderID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "orderId")))
		if err != nil || orderID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_order_id", "invalid orderId")
			return
		}
		view, err := app.Commerce.GetOrderCashForensics(r.Context(), orderID)
		if err != nil {
			if isCommerceNotFound(err) {
				writeAPIError(w, r.Context(), http.StatusNotFound, "order_not_found", "order not found")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "cash_forensics_failed", err.Error())
			return
		}
		out := V1OrderCashForensicsResponse{
			OrderID:                  view.OrderID.String(),
			OutstandingLiabilityMinor: view.OutstandingLiability,
			RemainderMinor:           view.RemainderMinor,
			AcceptanceEvents:     make([]V1OrderMoneyAcceptanceEvent, 0, len(view.AcceptanceEvents)),
		}
		if view.WinningPaymentID != nil {
			s := view.WinningPaymentID.String()
			out.WinningPaymentID = &s
		}
		for _, ev := range view.AcceptanceEvents {
			out.AcceptanceEvents = append(out.AcceptanceEvents, V1OrderMoneyAcceptanceEvent{
				DeviceEventID:     ev.DeviceEventID,
				DenominationMinor: ev.DenominationMinor,
				CreditSource:      ev.CreditSource,
				AcceptedAt:        formatAPITimeRFC3339Nano(ev.AcceptedAt.UTC()),
			})
		}
		if view.CashAllocation != nil {
			out.CashAllocation = &V1OrderMoneyCashAllocation{
				AmountMinor:            view.CashAllocation.AmountMinor,
				PreOrderCreditMinor:    view.CashAllocation.PreOrderCreditMinor,
				PostOrderInsertedMinor: view.CashAllocation.PostOrderInsertedMinor,
				ConsentSource:          view.CashAllocation.ConsentSource,
			}
		}
		if view.CashChange != nil {
			out.CashChange = &V1OrderMoneyCashChange{
				ChangeDueMinor:       view.CashChange.ChangeDueMinor,
				ChangeDispensedMinor: view.CashChange.ChangeDispensedMinor,
				Outcome:              view.CashChange.Outcome,
				LiabilityMinor:       view.CashChange.LiabilityMinor,
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

type cashAdjustmentBody struct {
	AmountMinor int64  `json:"amountMinor"`
	Bucket      string `json:"bucket"`
	Reason      string `json:"reason"`
	Currency    string `json:"currency"`
}

func postAdminCashAdjustment(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		idem, err := requireWriteIdempotencyKey(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		var body cashAdjustmentBody
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		cur := strings.TrimSpace(body.Currency)
		if cur == "" {
			cur = "VND"
		}
		var operatorID *uuid.UUID
		if p, ok := auth.PrincipalFromContext(r.Context()); ok {
			if aid, aerr := uuid.Parse(strings.TrimSpace(p.Subject)); aerr == nil {
				operatorID = &aid
			}
		}
		adj, err := app.Commerce.InsertCashAdjustment(r.Context(), appcommerce.InsertCashAdjustmentInput{
			MachineID:         machineID,
			AmountMinor:       body.AmountMinor,
			Bucket:            strings.TrimSpace(body.Bucket),
			Reason:            strings.TrimSpace(body.Reason),
			OperatorAccountID: operatorID,
			IdempotencyKey:    idem,
			Currency:          cur,
		})
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "cash_adjustment_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, V1CashAdjustmentResponse{
			ID:          adj.ID.String(),
			MachineID:   adj.MachineID.String(),
			AmountMinor: adj.AmountMinor,
			Bucket:      adj.Bucket,
			Reason:      adj.Reason,
			Currency:    adj.Currency,
			CreatedAt:   formatAPITimeRFC3339Nano(adj.CreatedAt),
		})
	}
}

func isCommerceNotFound(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "not found")
}
