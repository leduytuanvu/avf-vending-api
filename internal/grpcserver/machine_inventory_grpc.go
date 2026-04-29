package grpcserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/inventoryapp"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type machineInventoryServer struct {
	machinev1.UnimplementedMachineInventoryServiceServer
	deps MachineGRPCServicesDeps
}

func (s *machineInventoryServer) GetInventorySnapshot(ctx context.Context, req *machinev1.GetInventorySnapshotRequest) (*machinev1.GetInventorySnapshotResponse, error) {
	if req == nil {
		req = &machinev1.GetInventorySnapshotRequest{}
	}
	claims, slots, err := s.inventorySlots(ctx)
	if err != nil {
		return nil, err
	}
	rid := ""
	if req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	return &machinev1.GetInventorySnapshotResponse{
		MachineId:      claims.MachineID.String(),
		OrganizationId: claims.OrganizationID.String(),
		ServerTime:     timestamppb.New(time.Now().UTC()),
		Slots:          slots,
		Meta:           responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
	}, nil
}

func (s *machineInventoryServer) GetPlanogram(ctx context.Context, req *machinev1.GetPlanogramRequest) (*machinev1.GetPlanogramResponse, error) {
	claims, slots, err := s.inventorySlots(ctx)
	if err != nil {
		return nil, err
	}
	if req != nil && strings.TrimSpace(req.GetMachineId()) != "" {
		if _, err := resolveMachineScope(claims.MachineID, req.GetMachineId()); err != nil {
			return nil, err
		}
	}
	return &machinev1.GetPlanogramResponse{
		MachineId:      claims.MachineID.String(),
		OrganizationId: claims.OrganizationID.String(),
		ServerTime:     timestamppb.New(time.Now().UTC()),
		Slots:          slots,
	}, nil
}

func (s *machineInventoryServer) inventorySlots(ctx context.Context) (plauth.MachineAccessClaims, []*machinev1.InventorySlotRow, error) {
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return plauth.MachineAccessClaims{}, nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	q := db.New(s.deps.Pool)
	if err := machineRuntimeInventoryGate(ctx, q, claims); err != nil {
		return plauth.MachineAccessClaims{}, nil, err
	}
	rows, err := q.InventoryAdminListMachineSlots(ctx, claims.MachineID)
	if err != nil {
		return plauth.MachineAccessClaims{}, nil, status.Errorf(codes.Internal, "inventory snapshot failed")
	}
	slots := make([]*machinev1.InventorySlotRow, 0, len(rows))
	for _, row := range rows {
		pr := &machinev1.InventorySlotRow{
			PlanogramId:              row.PlanogramID.String(),
			PlanogramName:            row.PlanogramName,
			SlotIndex:                row.SlotIndex,
			CurrentQuantity:          row.CurrentQuantity,
			MaxQuantity:              row.MaxQuantity,
			PriceMinor:               row.PriceMinor,
			PlanogramRevisionApplied: row.PlanogramRevisionApplied,
			CabinetCode:              row.CabinetCode,
			CabinetIndex:             row.CabinetIndex,
			IsEmpty:                  row.IsEmpty,
		}
		if row.ProductID.Valid {
			pr.ProductId = uuid.UUID(row.ProductID.Bytes).String()
		}
		if row.ProductSku.Valid {
			pr.ProductSku = row.ProductSku.String
		}
		if row.ProductName.Valid {
			pr.ProductName = row.ProductName.String
		}
		slots = append(slots, pr)
	}
	return claims, slots, nil
}

func (s *machineInventoryServer) SubmitRestock(ctx context.Context, req *machinev1.SubmitRestockRequest) (*machinev1.SubmitRestockResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	res, err := s.submitInventoryMutation(ctx, wctx, "restock", compliance.ActionMachineInventoryRestock, req.GetLines(), nil)
	if err != nil {
		return nil, err
	}
	return &machinev1.SubmitRestockResponse{Replay: res.Replay, EventIds: res.EventIDs}, nil
}

func (s *machineInventoryServer) SubmitFillResult(ctx context.Context, req *machinev1.SubmitFillResultRequest) (*machinev1.SubmitFillResultResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	res, err := s.submitInventoryMutation(ctx, wctx, "restock", "inventory.fill_submitted", req.GetLines(), nil)
	if err != nil {
		return nil, err
	}
	return &machinev1.SubmitFillResultResponse{Replay: res.Replay, EventIds: res.EventIDs}, nil
}

func (s *machineInventoryServer) SubmitFillReport(ctx context.Context, req *machinev1.SubmitFillResultRequest) (*machinev1.SubmitFillResultResponse, error) {
	return s.SubmitFillResult(ctx, req)
}

func (s *machineInventoryServer) SubmitInventoryAdjustment(ctx context.Context, req *machinev1.SubmitInventoryAdjustmentRequest) (*machinev1.SubmitInventoryAdjustmentResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(req.GetReason())
	if reason == "" {
		return nil, status.Error(codes.InvalidArgument, "reason required")
	}
	if strings.EqualFold(reason, "restock") {
		return nil, status.Error(codes.InvalidArgument, "use SubmitRestock for restock")
	}
	res, err := s.submitInventoryMutation(ctx, wctx, reason, compliance.ActionInventoryAdjusted, nil, req.GetLines())
	if err != nil {
		return nil, err
	}
	return &machinev1.SubmitInventoryAdjustmentResponse{Replay: res.Replay, EventIds: res.EventIDs}, nil
}

func (s *machineInventoryServer) SubmitStockAdjustment(ctx context.Context, req *machinev1.SubmitInventoryAdjustmentRequest) (*machinev1.SubmitInventoryAdjustmentResponse, error) {
	return s.SubmitInventoryAdjustment(ctx, req)
}

func (s *machineInventoryServer) SubmitStockSnapshot(ctx context.Context, req *machinev1.SubmitStockSnapshotRequest) (*machinev1.SubmitStockSnapshotResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	wctx, err := parseMachineMutationContext(ctx, req.GetContext())
	if err != nil {
		return nil, err
	}
	res, err := s.submitInventoryMutation(ctx, wctx, "machine_reconcile", "inventory.snapshot_submitted", nil, req.GetLines())
	if err != nil {
		return nil, err
	}
	return &machinev1.SubmitStockSnapshotResponse{Replay: res.Replay, EventIds: res.EventIDs}, nil
}

func (s *machineInventoryServer) submitInventoryMutation(ctx context.Context, wctx machineMutationContext, reason, auditAction string, restock []*machinev1.RestockLine, adj []*machinev1.AdjustmentLine) (inventoryapp.AdjustmentBatchResult, error) {
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return inventoryapp.AdjustmentBatchResult{}, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	q := db.New(s.deps.Pool)
	if err := machineRuntimeInventoryGate(ctx, q, claims); err != nil {
		return inventoryapp.AdjustmentBatchResult{}, err
	}
	if err := validateOperatorSessionForMachine(ctx, q, claims, wctx.OperatorSessionID); err != nil {
		return inventoryapp.AdjustmentBatchResult{}, err
	}
	if s.deps.InventoryLedger == nil {
		return inventoryapp.AdjustmentBatchResult{}, status.Error(codes.Unavailable, "inventory ledger not configured")
	}

	var items []inventoryapp.AdjustmentItem
	switch {
	case len(restock) > 0 && len(adj) == 0:
		var err error
		items, err = mapRestockLinesToItems(ctx, q, claims.MachineID, restock)
		if err != nil {
			return inventoryapp.AdjustmentBatchResult{}, err
		}
	case len(adj) > 0 && len(restock) == 0:
		var err error
		items, err = mapAdjustmentLinesToItems(ctx, q, claims.MachineID, adj)
		if err != nil {
			return inventoryapp.AdjustmentBatchResult{}, err
		}
	default:
		return inventoryapp.AdjustmentBatchResult{}, status.Error(codes.InvalidArgument, "lines required")
	}
	if len(items) == 0 {
		return inventoryapp.AdjustmentBatchResult{}, status.Error(codes.InvalidArgument, "lines must contain at least one entry")
	}

	occurredAt := wctx.ClientCreatedAt
	res, err := s.deps.InventoryLedger.CreateInventoryAdjustmentBatch(ctx, inventoryapp.AdjustmentBatchInput{
		OrganizationID:    claims.OrganizationID,
		MachineID:         claims.MachineID,
		OperatorSessionID: wctx.OperatorSessionID,
		Reason:            reason,
		IdempotencyKey:    wctx.IdempotencyKey,
		ClientEventID:     wctx.ClientEventID,
		Items:             items,
		OccurredAt:        &occurredAt,
	})
	if err != nil {
		return inventoryapp.AdjustmentBatchResult{}, mapMachineInventoryLedgerError(err)
	}

	if !res.Replay {
		src := strings.TrimSpace(strings.ToLower(reason))
		if src == "" {
			src = "unknown"
		}
		productionmetrics.RecordInventoryAdjustment(src)
	}

	if strings.TrimSpace(auditAction) == "" {
		auditAction = compliance.ActionInventoryAdjusted
	}
	if !res.Replay && s.deps.EnterpriseAudit != nil {
		meta, _ := json.Marshal(map[string]any{
			"idempotency_key": wctx.IdempotencyKey,
			"client_event_id": wctx.ClientEventID,
			"reason":          reason,
			"event_ids":       res.EventIDs,
			"line_count":      len(items),
		})
		machineIDStr := claims.MachineID.String()
		_ = s.deps.EnterpriseAudit.Record(ctx, compliance.EnterpriseAuditRecord{
			OrganizationID: claims.OrganizationID,
			ActorType:      compliance.ActorMachine,
			ActorID:        &machineIDStr,
			Action:         auditAction,
			ResourceType:   "machine",
			ResourceID:     &machineIDStr,
			Metadata:       meta,
		})
	}

	return res, nil
}

func mapMachineInventoryLedgerError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, inventoryapp.ErrQuantityBeforeMismatch):
		return status.Error(codes.FailedPrecondition, "quantity_before mismatch")
	case errors.Is(err, inventoryapp.ErrAdjustmentSlotNotFound):
		return status.Error(codes.NotFound, "slot not found")
	case errors.Is(err, inventoryapp.ErrInvalidStockAdjustmentReason):
		return status.Error(codes.InvalidArgument, "invalid reason")
	case errors.Is(err, inventoryapp.ErrIdempotencyKeyConflict):
		return status.Error(codes.Aborted, "idempotency key conflict")
	case errors.Is(err, postgres.ErrMachineOrganizationMismatch):
		return status.Error(codes.PermissionDenied, "organization mismatch")
	default:
		return status.Error(codes.Internal, "inventory write failed")
	}
}

func mapRestockLinesToItems(ctx context.Context, q *db.Queries, machineID uuid.UUID, lines []*machinev1.RestockLine) ([]inventoryapp.AdjustmentItem, error) {
	out := make([]inventoryapp.AdjustmentItem, 0, len(lines))
	for _, ln := range lines {
		if ln == nil {
			return nil, status.Error(codes.InvalidArgument, "line must not be null")
		}
		it, err := mapOneInventoryLine(ctx, q, machineID, ln.GetPlanogramId(), ln.GetSlotIndex(), ln.GetQuantityBefore(), ln.GetQuantityAfter(), strings.TrimSpace(ln.GetCabinetCode()), strings.TrimSpace(ln.GetSlotCode()), ln.GetProductId())
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

func mapAdjustmentLinesToItems(ctx context.Context, q *db.Queries, machineID uuid.UUID, lines []*machinev1.AdjustmentLine) ([]inventoryapp.AdjustmentItem, error) {
	out := make([]inventoryapp.AdjustmentItem, 0, len(lines))
	for _, ln := range lines {
		if ln == nil {
			return nil, status.Error(codes.InvalidArgument, "line must not be null")
		}
		it, err := mapOneInventoryLine(ctx, q, machineID, ln.GetPlanogramId(), ln.GetSlotIndex(), ln.GetQuantityBefore(), ln.GetQuantityAfter(), strings.TrimSpace(ln.GetCabinetCode()), strings.TrimSpace(ln.GetSlotCode()), ln.GetProductId())
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

func mapOneInventoryLine(ctx context.Context, q *db.Queries, machineID uuid.UUID, planogramIDStr string, slotIndex, qtyBefore, qtyAfter int32, cabinetCode, slotCode, productIDStr string) (inventoryapp.AdjustmentItem, error) {
	pgID, err := uuid.Parse(strings.TrimSpace(planogramIDStr))
	if err != nil || pgID == uuid.Nil {
		return inventoryapp.AdjustmentItem{}, status.Error(codes.InvalidArgument, "invalid planogram_id")
	}
	var pid *uuid.UUID
	if strings.TrimSpace(productIDStr) != "" {
		u, perr := uuid.Parse(strings.TrimSpace(productIDStr))
		if perr != nil || u == uuid.Nil {
			return inventoryapp.AdjustmentItem{}, status.Error(codes.InvalidArgument, "invalid product_id")
		}
		pid = &u
	}
	var cabID *uuid.UUID
	if cabinetCode != "" {
		cabRow, cerr := q.FleetAdminGetMachineCabinetByMachineAndCode(ctx, db.FleetAdminGetMachineCabinetByMachineAndCodeParams{
			MachineID:   machineID,
			CabinetCode: cabinetCode,
		})
		if cerr == nil {
			cabID = &cabRow.ID
		}
	}
	return inventoryapp.AdjustmentItem{
		PlanogramID:      pgID,
		SlotIndex:        slotIndex,
		QuantityBefore:   qtyBefore,
		QuantityAfter:    qtyAfter,
		CabinetCode:      cabinetCode,
		SlotCode:         slotCode,
		MachineCabinetID: cabID,
		ProductID:        pid,
	}, nil
}

func (s *machineInventoryServer) AckInventorySync(ctx context.Context, req *machinev1.AckInventorySyncRequest) (*machinev1.AckInventorySyncResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "empty request")
	}
	claims, ok := plauth.MachineAccessClaimsFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing machine credentials")
	}
	if s.deps.Pool == nil {
		return nil, status.Error(codes.Unavailable, "database_not_configured")
	}
	q := db.New(s.deps.Pool)
	if err := machineRuntimeInventoryGate(ctx, q, claims); err != nil {
		return nil, err
	}
	rid := ""
	if req.GetMeta() != nil {
		rid = req.GetMeta().GetRequestId()
	}
	if s.deps.EnterpriseAudit != nil {
		cur := strings.TrimSpace(req.GetSyncCursor())
		actorID := claims.MachineID.String()
		meta, _ := json.Marshal(map[string]any{"sync_cursor": cur})
		if len(meta) == 0 {
			meta = []byte("{}")
		}
		_ = s.deps.EnterpriseAudit.Record(ctx, compliance.EnterpriseAuditRecord{
			OrganizationID: claims.OrganizationID,
			ActorType:      compliance.ActorMachine,
			ActorID:        &actorID,
			Action:         "machine.inventory.sync_acknowledged",
			ResourceType:   "machine",
			ResourceID:     &actorID,
			Metadata:       meta,
		})
	}
	return &machinev1.AckInventorySyncResponse{
		Meta: responseMetaCtx(ctx, rid, machinev1.MachineResponseStatus_MACHINE_RESPONSE_STATUS_ACCEPTED),
	}, nil
}
