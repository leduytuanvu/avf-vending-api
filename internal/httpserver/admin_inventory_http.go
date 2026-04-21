package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appdevice "github.com/avf/avf-vending-api/internal/app/device"
	appinventoryadmin "github.com/avf/avf-vending-api/internal/app/inventoryadmin"
	"github.com/avf/avf-vending-api/internal/app/inventoryapp"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	domaindevice "github.com/avf/avf-vending-api/internal/domain/device"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	adminMachinePlanogramPublishCommandType = "machine_planogram_publish"
	adminMachineSetupSyncCommandType        = "machine_setup_sync"
)

func mountAdminInventoryRoutes(r chi.Router, app *api.HTTPApplication, writeRL func(http.Handler) http.Handler) {
	if app == nil || app.InventoryAdmin == nil {
		return
	}
	if writeRL == nil {
		writeRL = func(h http.Handler) http.Handler { return h }
	}
	svc := app.InventoryAdmin
	r.Get("/machines/{machineId}/slots", listAdminMachineSlots(svc))
	r.Get("/machines/{machineId}/inventory", getAdminMachineInventory(svc))
	r.With(writeRL).Post("/machines/{machineId}/stock-adjustments", postAdminMachineStockAdjustments(app))
	r.With(writeRL).Put("/machines/{machineId}/topology", putAdminMachineTopology(app))
	r.With(writeRL).Put("/machines/{machineId}/planograms/draft", putAdminMachinePlanogramDraft(app))
	r.With(writeRL).Post("/machines/{machineId}/planograms/publish", postAdminMachinePlanogramPublish(app))
	r.With(writeRL).Post("/machines/{machineId}/sync", postAdminMachineSetupSync(app))
}

func listAdminMachineSlots(svc *appinventoryadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || mid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		head, err := resolveInventoryMachine(r, svc, mid)
		if err != nil {
			writeInventoryAccessOrResolveError(w, r, err)
			return
		}
		rows, err := svc.ListSlotInventoryView(r.Context(), mid)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]V1AdminMachineSlot, 0, len(rows))
		for _, row := range rows {
			var pid *string
			if row.ProductID != nil {
				s := row.ProductID.String()
				pid = &s
			}
			items = append(items, V1AdminMachineSlot{
				MachineID:                row.MachineID.String(),
				MachineName:              head.Name,
				MachineStatus:            head.Status,
				PlanogramID:              row.PlanogramID.String(),
				PlanogramName:            row.PlanogramName,
				SlotIndex:                row.SlotIndex,
				CabinetCode:              row.CabinetCode,
				SlotCode:                 row.SlotCode,
				CurrentQuantity:          row.CurrentStock,
				CurrentStock:             row.CurrentStock,
				MaxQuantity:              row.Capacity,
				Capacity:                 row.Capacity,
				ParLevel:                 row.ParLevel,
				LowStockThreshold:        row.LowStockThreshold,
				PriceMinor:               row.PriceMinor,
				Currency:                 row.Currency,
				Status:                   row.Status,
				PlanogramRevisionApplied: row.PlanogramRevision,
				UpdatedAt:                row.UpdatedAt.UTC().Format(timeRFC3339Nano),
				ProductID:                pid,
				ProductSku:               strPtrOrNil(row.ProductSku),
				ProductName:              strPtrOrNil(row.ProductName),
				IsEmpty:                  row.IsEmpty,
				LowStock:                 row.LowStock,
			})
		}
		writeJSON(w, http.StatusOK, V1AdminMachineSlotListEnvelope{Items: items})
	}
}

func strPtrOrNil(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	out := s
	return &out
}

type stockAdjustmentsRequest struct {
	OperatorSessionID string                     `json:"operator_session_id"`
	Reason            string                     `json:"reason"`
	Items             []stockAdjustmentsItemJSON `json:"items"`
}

type stockAdjustmentsItemJSON struct {
	PlanogramID    string  `json:"planogramId"`
	SlotIndex      int32   `json:"slotIndex"`
	QuantityBefore int32   `json:"quantityBefore"`
	QuantityAfter  int32   `json:"quantityAfter"`
	CabinetCode    string  `json:"cabinetCode,omitempty"`
	SlotCode       string  `json:"slotCode,omitempty"`
	ProductID      *string `json:"productId,omitempty"`
}

func postAdminMachineStockAdjustments(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app == nil || app.InventoryAdmin == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "inventory admin not configured")
			return
		}
		if app.TelemetryStore == nil || app.TelemetryStore.Pool() == nil {
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
		var body stockAdjustmentsRequest
		if !decodeStrictJSON(w, r, &body) {
			return
		}
		if len(body.Items) == 0 {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "items_required", "items must contain at least one entry")
			return
		}
		sid, ok := parseOperatorSessionIDField(w, r, body.OperatorSessionID)
		if !ok {
			return
		}
		if !requireActiveOperatorSession(w, r, app, machineID, sid) {
			return
		}
		if _, err := inventoryapp.StockAdjustmentReasonToEventType(body.Reason); err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_reason", "reason must be one of: restock, cycle_count, manual_adjustment, machine_reconcile")
			return
		}

		q := db.New(app.TelemetryStore.Pool())
		items := make([]inventoryapp.AdjustmentItem, 0, len(body.Items))
		for _, it := range body.Items {
			pgID, err := uuid.Parse(strings.TrimSpace(it.PlanogramID))
			if err != nil || pgID == uuid.Nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_planogram_id", "each item.planogramId must be a UUID")
				return
			}
			var pid *uuid.UUID
			if it.ProductID != nil && strings.TrimSpace(*it.ProductID) != "" {
				u, perr := uuid.Parse(strings.TrimSpace(*it.ProductID))
				if perr != nil {
					writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_product_id", "productId must be a UUID when set")
					return
				}
				pid = &u
			}
			var cabID *uuid.UUID
			if cc := strings.TrimSpace(it.CabinetCode); cc != "" {
				cabRow, cerr := q.FleetAdminGetMachineCabinetByMachineAndCode(r.Context(), db.FleetAdminGetMachineCabinetByMachineAndCodeParams{
					MachineID:   machineID,
					CabinetCode: cc,
				})
				if cerr == nil {
					cabID = &cabRow.ID
				}
			}
			items = append(items, inventoryapp.AdjustmentItem{
				PlanogramID:      pgID,
				SlotIndex:        it.SlotIndex,
				QuantityBefore:   it.QuantityBefore,
				QuantityAfter:    it.QuantityAfter,
				SlotCode:         strings.TrimSpace(it.SlotCode),
				MachineCabinetID: cabID,
				ProductID:        pid,
			})
		}

		repo := postgres.NewInventoryRepository(app.TelemetryStore.Pool())
		res, err := repo.CreateInventoryAdjustmentBatch(r.Context(), inventoryapp.AdjustmentBatchInput{
			OrganizationID:    head.OrganizationID,
			MachineID:         machineID,
			OperatorSessionID: &sid,
			CorrelationID:     correlationUUIDFromRequest(r.Context()),
			Reason:            strings.TrimSpace(body.Reason),
			IdempotencyKey:    idem,
			Items:             items,
		})
		if err != nil {
			writeStockAdjustmentError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, V1AdminStockAdjustmentsResponse{Replay: res.Replay, EventIds: res.EventIDs})
	}
}

func writeStockAdjustmentError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case errors.Is(err, inventoryapp.ErrQuantityBeforeMismatch):
		writeAPIError(w, ctx, http.StatusConflict, "quantity_before_mismatch", err.Error())
	case errors.Is(err, inventoryapp.ErrInvalidStockAdjustmentReason):
		writeAPIError(w, ctx, http.StatusBadRequest, "invalid_reason", err.Error())
	case errors.Is(err, inventoryapp.ErrAdjustmentSlotNotFound):
		writeAPIError(w, ctx, http.StatusNotFound, "slot_not_found", err.Error())
	default:
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
	}
}

func getAdminMachineInventory(svc *appinventoryadmin.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mid, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || mid == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		head, err := resolveInventoryMachine(r, svc, mid)
		if err != nil {
			writeInventoryAccessOrResolveError(w, r, err)
			return
		}
		rows, err := svc.AggregateInventory(r.Context(), mid)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
			return
		}
		items := make([]V1AdminMachineInventoryLine, 0, len(rows))
		for _, row := range rows {
			items = append(items, V1AdminMachineInventoryLine{
				MachineID:          head.ID.String(),
				MachineName:        head.Name,
				MachineStatus:      head.Status,
				ProductID:          row.ProductID.String(),
				ProductName:        row.ProductName,
				ProductSku:         row.ProductSku,
				TotalQuantity:      row.TotalQuantity,
				SlotCount:          row.SlotCount,
				MaxCapacityAnySlot: row.MaxCapacityAnySlot,
				LowStock:           row.LowStock,
			})
		}
		writeJSON(w, http.StatusOK, V1AdminMachineInventoryEnvelope{Items: items})
	}
}

func resolveInventoryMachine(r *http.Request, svc *appinventoryadmin.Service, machineID uuid.UUID) (appinventoryadmin.MachineHead, error) {
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		return appinventoryadmin.MachineHead{}, errInventoryUnauthenticated
	}
	head, err := svc.ResolveMachine(r.Context(), machineID)
	if err != nil {
		return appinventoryadmin.MachineHead{}, err
	}
	if p.HasRole(auth.RolePlatformAdmin) {
		return head, nil
	}
	if p.HasRole(auth.RoleOrgAdmin) && p.HasOrganization() && head.OrganizationID == p.OrganizationID {
		return head, nil
	}
	return appinventoryadmin.MachineHead{}, appinventoryadmin.ErrForbidden
}

func writeInventoryAccessOrResolveError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errInventoryUnauthenticated):
		writeAPIError(w, r.Context(), http.StatusUnauthorized, "unauthenticated", auth.ErrUnauthenticated.Error())
	case errors.Is(err, appinventoryadmin.ErrForbidden):
		writeAPIError(w, r.Context(), http.StatusForbidden, "forbidden", "forbidden")
	case errors.Is(err, appinventoryadmin.ErrMachineNotFound):
		writeAPIError(w, r.Context(), http.StatusNotFound, "machine_not_found", "machine not found")
	default:
		writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", err.Error())
	}
}

func setupPool(app *api.HTTPApplication) (*postgres.SetupRepository, bool) {
	if app == nil || app.TelemetryStore == nil || app.TelemetryStore.Pool() == nil {
		return nil, false
	}
	return postgres.NewSetupRepository(app.TelemetryStore.Pool()), true
}

func decodeStrictJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "request body is required")
			return false
		}
		writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_json", "invalid request body")
		return false
	}
	return true
}

func requireActiveOperatorSession(w http.ResponseWriter, r *http.Request, app *api.HTTPApplication, machineID, sessionID uuid.UUID) bool {
	if app == nil || app.MachineOperator == nil {
		writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "operator service not configured")
		return false
	}
	sess, err := app.MachineOperator.GetSessionIfMatchesMachine(r.Context(), sessionID, machineID)
	if err != nil {
		writeOperatorError(w, r.Context(), err)
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(sess.Status), domainoperator.SessionStatusActive) {
		writeOperatorError(w, r.Context(), domainoperator.ErrSessionNotActive)
		return false
	}
	return true
}

func parseOperatorSessionIDField(w http.ResponseWriter, r *http.Request, raw string) (uuid.UUID, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		writeAPIError(w, r.Context(), http.StatusBadRequest, "operator_session_id_required", "operator_session_id is required")
		return uuid.Nil, false
	}
	id, err := uuid.Parse(s)
	if err != nil || id == uuid.Nil {
		writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_operator_session_id", "operator_session_id must be a UUID")
		return uuid.Nil, false
	}
	return id, true
}

type topologyPutBody struct {
	OperatorSessionID string               `json:"operator_session_id"`
	Cabinets          []cabinetUpsertJSON  `json:"cabinets"`
	Layouts           []topologyLayoutJSON `json:"layouts"`
}

type cabinetUpsertJSON struct {
	Code      string          `json:"code"`
	Title     string          `json:"title"`
	SortOrder int32           `json:"sortOrder"`
	Metadata  json.RawMessage `json:"metadata,omitempty"`
}

type topologyLayoutJSON struct {
	CabinetCode string          `json:"cabinetCode"`
	LayoutKey   string          `json:"layoutKey"`
	Revision    int32           `json:"revision"`
	LayoutSpec  json.RawMessage `json:"layoutSpec,omitempty"`
	Status      string          `json:"status"`
}

type planogramSlotBody struct {
	OperatorSessionID   string                 `json:"operator_session_id"`
	PlanogramID         string                 `json:"planogramId"`
	PlanogramRevision   int32                  `json:"planogramRevision"`
	SyncLegacyReadModel bool                   `json:"syncLegacyReadModel"`
	Items               []planogramSlotItemDTO `json:"items"`
}

type planogramSlotItemDTO struct {
	CabinetCode     string          `json:"cabinetCode"`
	LayoutKey       string          `json:"layoutKey"`
	LayoutRevision  int32           `json:"layoutRevision"`
	SlotCode        string          `json:"slotCode"`
	LegacySlotIndex *int32          `json:"legacySlotIndex,omitempty"`
	ProductID       *string         `json:"productId,omitempty"`
	MaxQuantity     int32           `json:"maxQuantity"`
	PriceMinor      int64           `json:"priceMinor"`
	EffectiveFrom   *time.Time      `json:"effectiveFrom,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
}

type machineSyncBody struct {
	OperatorSessionID string `json:"operator_session_id"`
	Reason            string `json:"reason,omitempty"`
}

func putAdminMachineTopology(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, ok := setupPool(app)
		if !ok {
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
		var body topologyPutBody
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
		cabs := make([]setupapp.CabinetUpsert, 0, len(body.Cabinets))
		for _, c := range body.Cabinets {
			meta := []byte(c.Metadata)
			if len(meta) == 0 {
				meta = []byte("{}")
			}
			if !json.Valid(meta) {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_metadata", "cabinet metadata must be JSON")
				return
			}
			cabs = append(cabs, setupapp.CabinetUpsert{
				Code:      strings.TrimSpace(c.Code),
				Title:     strings.TrimSpace(c.Title),
				SortOrder: c.SortOrder,
				Metadata:  meta,
			})
		}
		layouts := make([]setupapp.TopologyLayoutUpsert, 0, len(body.Layouts))
		for _, lay := range body.Layouts {
			spec := []byte(lay.LayoutSpec)
			if len(spec) == 0 {
				spec = []byte("{}")
			}
			if !json.Valid(spec) {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_layout_spec", "layout layoutSpec must be JSON")
				return
			}
			layouts = append(layouts, setupapp.TopologyLayoutUpsert{
				CabinetCode: strings.TrimSpace(lay.CabinetCode),
				LayoutKey:   strings.TrimSpace(lay.LayoutKey),
				Revision:    lay.Revision,
				LayoutSpec:  spec,
				Status:      strings.TrimSpace(lay.Status),
			})
		}
		if err := repo.UpsertMachineTopology(r.Context(), machineID, cabs, layouts); err != nil {
			writeSetupMutationError(w, r.Context(), err)
			return
		}
		_ = head // resolved for tenant access; topology is machine-scoped
		w.WriteHeader(http.StatusNoContent)
	}
}

func putAdminMachinePlanogramDraft(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, ok := setupPool(app)
		if !ok {
			writeCapabilityNotConfigured(w, r.Context(), "database", "database pool is not configured for this API process")
			return
		}
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		if _, err := resolveInventoryMachine(r, app.InventoryAdmin, machineID); err != nil {
			writeInventoryAccessOrResolveError(w, r, err)
			return
		}
		in, ok := decodePlanogramSlotBody(w, r)
		if !ok {
			return
		}
		sid, ok := parseOperatorSessionIDField(w, r, in.OperatorSessionID)
		if !ok {
			return
		}
		if !requireActiveOperatorSession(w, r, app, machineID, sid) {
			return
		}
		saveIn, ok := planogramBodyToSaveInput(w, r, in, false)
		if !ok {
			return
		}
		if err := repo.SaveDraftOrCurrentSlotConfigs(r.Context(), machineID, saveIn); err != nil {
			writeSetupMutationError(w, r.Context(), err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func postAdminMachinePlanogramPublish(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repo, ok := setupPool(app)
		if !ok {
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
		if app.TelemetryStore == nil || app.TelemetryStore.Pool() == nil {
			writeCapabilityNotConfigured(w, r.Context(), "database", "database pool is not configured for this API process")
			return
		}
		idem, err := requireWriteIdempotencyKey(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		in, ok := decodePlanogramSlotBody(w, r)
		if !ok {
			return
		}
		sid, ok := parseOperatorSessionIDField(w, r, in.OperatorSessionID)
		if !ok {
			return
		}
		if !requireActiveOperatorSession(w, r, app, machineID, sid) {
			return
		}
		saveIn, ok := planogramBodyToSaveInput(w, r, in, true)
		if !ok {
			return
		}

		pool := app.TelemetryStore.Pool()
		q := db.New(pool)
		existing, ledgerErr := q.GetCommandLedgerByMachineIdempotency(r.Context(), db.GetCommandLedgerByMachineIdempotencyParams{
			MachineID:      machineID,
			IdempotencyKey: pgtype.Text{String: idem, Valid: true},
		})
		if ledgerErr == nil && strings.TrimSpace(existing.CommandType) != adminMachinePlanogramPublishCommandType {
			writeAPIError(w, r.Context(), http.StatusConflict, "idempotency_key_conflict", "Idempotency-Key already used for a different command on this machine")
			return
		}
		if ledgerErr == nil && strings.TrimSpace(existing.CommandType) == adminMachinePlanogramPublishCommandType {
			out, dispatchErr := finishPublishFromReplay(r.Context(), app, machineID, idem, sid, existing.Payload)
			if dispatchErr != nil {
				if errors.Is(dispatchErr, appdevice.ErrMQTTCommandPublisherMissing) {
					writeCapabilityNotConfigured(w, r.Context(), "mqtt_command_dispatch", "MQTT broker client is not configured for this API process (set MQTT_BROKER_URL and MQTT_CLIENT_ID)")
					return
				}
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "dispatch_failed", dispatchErr.Error())
				return
			}
			writeJSON(w, http.StatusOK, out)
			return
		}
		if ledgerErr != nil && !errors.Is(ledgerErr, pgx.ErrNoRows) {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", ledgerErr.Error())
			return
		}

		if err := repo.SaveDraftOrCurrentSlotConfigs(r.Context(), machineID, saveIn); err != nil {
			writeSetupMutationError(w, r.Context(), err)
			return
		}
		_, cfgRev, cerr := insertMachineConfigSnapshot(r.Context(), pool, head.OrganizationID, machineID, sid, in.PlanogramID, in.PlanogramRevision)
		if cerr != nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", cerr.Error())
			return
		}

		payload := planogramPublishPayload{
			PlanogramID:          in.PlanogramID,
			PlanogramRevision:    in.PlanogramRevision,
			DesiredConfigVersion: cfgRev,
		}
		payloadBytes, _ := json.Marshal(payload)
		desired, _ := json.Marshal(map[string]any{
			"desiredConfigVersion": cfgRev,
			"planogramId":          in.PlanogramID,
			"planogramRevision":    in.PlanogramRevision,
		})
		if app.RemoteCommands == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "remote command dispatcher is not configured")
			return
		}
		disp, derr := app.RemoteCommands.DispatchRemoteMQTTCommand(r.Context(), appdevice.RemoteCommandDispatchInput{
			Append: domaindevice.AppendCommandInput{
				MachineID:         machineID,
				CommandType:       adminMachinePlanogramPublishCommandType,
				Payload:           payloadBytes,
				IdempotencyKey:    idem,
				DesiredState:      desired,
				OperatorSessionID: &sid,
			},
		})
		if derr != nil {
			if errors.Is(derr, appdevice.ErrMQTTCommandPublisherMissing) {
				writeCapabilityNotConfigured(w, r.Context(), "mqtt_command_dispatch", "MQTT broker client is not configured for this API process (set MQTT_BROKER_URL and MQTT_CLIENT_ID)")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "dispatch_failed", derr.Error())
			return
		}
		out := V1AdminPlanogramPublishResponse{
			DesiredConfigVersion: cfgRev,
			PlanogramID:          in.PlanogramID,
			PlanogramRevision:    in.PlanogramRevision,
			Command:              mapDispatchToCommandInfo(disp),
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func postAdminMachineSetupSync(app *api.HTTPApplication) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		machineID, err := uuid.Parse(strings.TrimSpace(chi.URLParam(r, "machineId")))
		if err != nil || machineID == uuid.Nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		if _, err := resolveInventoryMachine(r, app.InventoryAdmin, machineID); err != nil {
			writeInventoryAccessOrResolveError(w, r, err)
			return
		}
		if app.TelemetryStore == nil || app.TelemetryStore.Pool() == nil {
			writeCapabilityNotConfigured(w, r.Context(), "database", "database pool is not configured for this API process")
			return
		}
		idem, err := requireWriteIdempotencyKey(r)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "missing_idempotency_key", err.Error())
			return
		}
		var body machineSyncBody
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
		pool := app.TelemetryStore.Pool()
		q := db.New(pool)
		existing, ledgerErr := q.GetCommandLedgerByMachineIdempotency(r.Context(), db.GetCommandLedgerByMachineIdempotencyParams{
			MachineID:      machineID,
			IdempotencyKey: pgtype.Text{String: idem, Valid: true},
		})
		if ledgerErr == nil && strings.TrimSpace(existing.CommandType) != adminMachineSetupSyncCommandType {
			writeAPIError(w, r.Context(), http.StatusConflict, "idempotency_key_conflict", "Idempotency-Key already used for a different command on this machine")
			return
		}
		if ledgerErr == nil && strings.TrimSpace(existing.CommandType) == adminMachineSetupSyncCommandType {
			if app.RemoteCommands == nil {
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "remote command dispatcher is not configured")
				return
			}
			reason := strings.TrimSpace(body.Reason)
			if reason == "" {
				reason = "sync"
			}
			payloadBytes, _ := json.Marshal(map[string]any{"reason": reason})
			disp, derr := app.RemoteCommands.DispatchRemoteMQTTCommand(r.Context(), appdevice.RemoteCommandDispatchInput{
				Append: domaindevice.AppendCommandInput{
					MachineID:         machineID,
					CommandType:       adminMachineSetupSyncCommandType,
					Payload:           payloadBytes,
					IdempotencyKey:    idem,
					DesiredState:      []byte("{}"),
					OperatorSessionID: &sid,
				},
			})
			if derr != nil {
				if errors.Is(derr, appdevice.ErrMQTTCommandPublisherMissing) {
					writeCapabilityNotConfigured(w, r.Context(), "mqtt_command_dispatch", "MQTT broker client is not configured for this API process (set MQTT_BROKER_URL and MQTT_CLIENT_ID)")
					return
				}
				writeAPIError(w, r.Context(), http.StatusInternalServerError, "dispatch_failed", derr.Error())
				return
			}
			writeJSON(w, http.StatusOK, V1AdminMachineSyncResponse{Command: mapDispatchToCommandInfo(disp)})
			return
		}
		if ledgerErr != nil && !errors.Is(ledgerErr, pgx.ErrNoRows) {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", ledgerErr.Error())
			return
		}
		if app.RemoteCommands == nil {
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "internal", "remote command dispatcher is not configured")
			return
		}
		reason := strings.TrimSpace(body.Reason)
		if reason == "" {
			reason = "sync"
		}
		payloadBytes, _ := json.Marshal(map[string]any{"reason": reason})
		disp, derr := app.RemoteCommands.DispatchRemoteMQTTCommand(r.Context(), appdevice.RemoteCommandDispatchInput{
			Append: domaindevice.AppendCommandInput{
				MachineID:         machineID,
				CommandType:       adminMachineSetupSyncCommandType,
				Payload:           payloadBytes,
				IdempotencyKey:    idem,
				DesiredState:      []byte("{}"),
				OperatorSessionID: &sid,
			},
		})
		if derr != nil {
			if errors.Is(derr, appdevice.ErrMQTTCommandPublisherMissing) {
				writeCapabilityNotConfigured(w, r.Context(), "mqtt_command_dispatch", "MQTT broker client is not configured for this API process (set MQTT_BROKER_URL and MQTT_CLIENT_ID)")
				return
			}
			writeAPIError(w, r.Context(), http.StatusInternalServerError, "dispatch_failed", derr.Error())
			return
		}
		writeJSON(w, http.StatusOK, V1AdminMachineSyncResponse{Command: mapDispatchToCommandInfo(disp)})
	}
}

type planogramPublishPayload struct {
	PlanogramID          string `json:"planogramId"`
	PlanogramRevision    int32  `json:"planogramRevision"`
	DesiredConfigVersion int32  `json:"desiredConfigVersion"`
}

func decodePlanogramSlotBody(w http.ResponseWriter, r *http.Request) (planogramSlotBody, bool) {
	var body planogramSlotBody
	if !decodeStrictJSON(w, r, &body) {
		return body, false
	}
	return body, true
}

func planogramBodyToSaveInput(w http.ResponseWriter, r *http.Request, body planogramSlotBody, publish bool) (setupapp.SlotConfigSaveInput, bool) {
	pgID, err := uuid.Parse(strings.TrimSpace(body.PlanogramID))
	if err != nil || pgID == uuid.Nil {
		writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_planogram_id", "planogramId must be a UUID")
		return setupapp.SlotConfigSaveInput{}, false
	}
	items := make([]setupapp.SlotConfigSaveItem, 0, len(body.Items))
	for _, it := range body.Items {
		meta := []byte(it.Metadata)
		if len(meta) == 0 {
			meta = []byte("{}")
		}
		if !json.Valid(meta) {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_metadata", "item metadata must be JSON")
			return setupapp.SlotConfigSaveInput{}, false
		}
		var pid *uuid.UUID
		if it.ProductID != nil && strings.TrimSpace(*it.ProductID) != "" {
			u, perr := uuid.Parse(strings.TrimSpace(*it.ProductID))
			if perr != nil {
				writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_product_id", "productId must be a UUID when set")
				return setupapp.SlotConfigSaveInput{}, false
			}
			pid = &u
		}
		eff := time.Time{}
		if it.EffectiveFrom != nil {
			eff = it.EffectiveFrom.UTC()
		}
		items = append(items, setupapp.SlotConfigSaveItem{
			CabinetCode:     strings.TrimSpace(it.CabinetCode),
			LayoutKey:       strings.TrimSpace(it.LayoutKey),
			LayoutRevision:  it.LayoutRevision,
			SlotCode:        strings.TrimSpace(it.SlotCode),
			LegacySlotIndex: it.LegacySlotIndex,
			ProductID:       pid,
			MaxQuantity:     it.MaxQuantity,
			PriceMinor:      it.PriceMinor,
			EffectiveFrom:   eff,
			Metadata:        meta,
		})
	}
	return setupapp.SlotConfigSaveInput{
		PlanogramID:         pgID,
		PlanogramRevision:   body.PlanogramRevision,
		PublishAsCurrent:    publish,
		SyncLegacyReadModel: body.SyncLegacyReadModel,
		Items:               items,
	}, true
}

func insertMachineConfigSnapshot(ctx context.Context, pool *pgxpool.Pool, orgID, machineID, sessionID uuid.UUID, planogramID string, planogramRevision int32) (db.MachineConfig, int32, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.MachineConfig{}, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var maxRev int32
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(config_revision), 0) FROM machine_configs WHERE machine_id = $1`, machineID).Scan(&maxRev); err != nil {
		return db.MachineConfig{}, 0, err
	}
	next := maxRev + 1
	meta, _ := json.Marshal(map[string]any{
		"planogramId":       planogramID,
		"planogramRevision": planogramRevision,
	})
	cfgPayload, _ := json.Marshal(map[string]any{
		"kind":                 "planogram_publish",
		"planogramId":          planogramID,
		"planogramRevision":    planogramRevision,
		"desiredConfigVersion": next,
	})
	q := db.New(tx)
	row, err := q.InsertMachineConfigApplication(ctx, db.InsertMachineConfigApplicationParams{
		OrganizationID:    orgID,
		MachineID:         machineID,
		AppliedAt:         time.Now().UTC(),
		ConfigRevision:    next,
		ConfigPayload:     cfgPayload,
		OperatorSessionID: pgtype.UUID{Bytes: sessionID, Valid: true},
		Metadata:          meta,
	})
	if err != nil {
		return db.MachineConfig{}, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.MachineConfig{}, 0, err
	}
	return row, next, nil
}

func finishPublishFromReplay(ctx context.Context, app *api.HTTPApplication, machineID uuid.UUID, idem string, sid uuid.UUID, ledgerPayload []byte) (V1AdminPlanogramPublishResponse, error) {
	var parsed planogramPublishPayload
	if err := json.Unmarshal(ledgerPayload, &parsed); err != nil {
		return V1AdminPlanogramPublishResponse{}, err
	}
	desired, _ := json.Marshal(map[string]any{
		"desiredConfigVersion": parsed.DesiredConfigVersion,
		"planogramId":          parsed.PlanogramID,
		"planogramRevision":    parsed.PlanogramRevision,
	})
	disp, err := app.RemoteCommands.DispatchRemoteMQTTCommand(ctx, appdevice.RemoteCommandDispatchInput{
		Append: domaindevice.AppendCommandInput{
			MachineID:         machineID,
			CommandType:       adminMachinePlanogramPublishCommandType,
			Payload:           ledgerPayload,
			IdempotencyKey:    idem,
			DesiredState:      desired,
			OperatorSessionID: &sid,
		},
	})
	if err != nil {
		return V1AdminPlanogramPublishResponse{}, err
	}
	return V1AdminPlanogramPublishResponse{
		DesiredConfigVersion: parsed.DesiredConfigVersion,
		PlanogramID:          parsed.PlanogramID,
		PlanogramRevision:    parsed.PlanogramRevision,
		Command:              mapDispatchToCommandInfo(disp),
	}, nil
}

func mapDispatchToCommandInfo(d appdevice.RemoteCommandDispatchResult) V1AdminPlanogramCommandInfo {
	return V1AdminPlanogramCommandInfo{
		CommandID:     d.CommandID.String(),
		Sequence:      d.Sequence,
		DispatchState: d.DispatchState,
		Replay:        d.Replay,
	}
}

func writeSetupMutationError(w http.ResponseWriter, ctx context.Context, err error) {
	switch {
	case errors.Is(err, setupapp.ErrNotFound):
		writeAPIError(w, ctx, http.StatusNotFound, "machine_not_found", "machine not found")
	case errors.Is(err, setupapp.ErrCabinetNotFound):
		writeAPIError(w, ctx, http.StatusBadRequest, "cabinet_not_found", err.Error())
	case errors.Is(err, setupapp.ErrSlotLayoutNotFound):
		writeAPIError(w, ctx, http.StatusBadRequest, "slot_layout_not_found", err.Error())
	default:
		writeAPIError(w, ctx, http.StatusInternalServerError, "internal", err.Error())
	}
}

var errInventoryUnauthenticated = errors.New("unauthenticated")

func boolFromPgBool(b pgtype.Bool) bool {
	return b.Valid && b.Bool
}
