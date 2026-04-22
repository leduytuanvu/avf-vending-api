package grpcserver

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appapi "github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	domainfleet "github.com/avf/avf-vending-api/internal/domain/fleet"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	avfv1 "github.com/avf/avf-vending-api/proto/avf/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type InternalQueryServices struct {
	Machine   appapi.InternalMachineQueryService
	Telemetry appapi.InternalTelemetryQueryService
	Commerce  appapi.InternalCommerceQueryService
}

func RegisterInternalQueryServices(services InternalQueryServices) ServiceRegistrar {
	return func(s *grpc.Server) error {
		if services.Machine == nil || services.Telemetry == nil || services.Commerce == nil {
			return fmt.Errorf("grpcserver: internal query services require machine, telemetry, and commerce deps")
		}
		avfv1.RegisterInternalMachineQueryServiceServer(s, &machineQueryServer{
			machine:   services.Machine,
			telemetry: services.Telemetry,
		})
		avfv1.RegisterInternalTelemetryQueryServiceServer(s, &telemetryQueryServer{
			telemetry: services.Telemetry,
		})
		avfv1.RegisterInternalCommerceQueryServiceServer(s, &commerceQueryServer{
			commerce: services.Commerce,
		})
		return nil
	}
}

type machineQueryServer struct {
	avfv1.UnimplementedInternalMachineQueryServiceServer
	machine   appapi.InternalMachineQueryService
	telemetry appapi.InternalTelemetryQueryService
}

func (s *machineQueryServer) GetMachineSummary(ctx context.Context, req *avfv1.GetMachineRequest) (*avfv1.GetMachineSummaryResponse, error) {
	machineID, err := requireUUID(req.GetMachineId(), "machine_id")
	if err != nil {
		return nil, err
	}
	bootstrap, err := s.machine.GetMachineBootstrap(ctx, machineID)
	if err != nil {
		return nil, mapError(err)
	}
	if err := authorizeOrganizationRead(ctx, bootstrap.Machine.OrganizationID); err != nil {
		return nil, err
	}
	return &avfv1.GetMachineSummaryResponse{Machine: mapMachineSummary(bootstrap.Machine)}, nil
}

func (s *machineQueryServer) GetMachineState(ctx context.Context, req *avfv1.GetMachineRequest) (*avfv1.GetMachineStateResponse, error) {
	machineID, err := requireUUID(req.GetMachineId(), "machine_id")
	if err != nil {
		return nil, err
	}
	snapshot, err := s.telemetry.GetTelemetrySnapshot(ctx, machineID)
	if err != nil {
		return nil, mapError(err)
	}
	if err := authorizeOrganizationRead(ctx, snapshot.OrganizationID); err != nil {
		return nil, err
	}
	shadow, err := s.machine.GetShadow(ctx, machineID)
	if err != nil {
		return nil, mapError(err)
	}
	return &avfv1.GetMachineStateResponse{
		Shadow:   mapShadowState(shadow),
		Snapshot: mapTelemetrySnapshot(snapshot),
	}, nil
}

func (s *machineQueryServer) GetMachineCabinetSlotSummary(ctx context.Context, req *avfv1.GetMachineRequest) (*avfv1.GetMachineCabinetSlotSummaryResponse, error) {
	machineID, err := requireUUID(req.GetMachineId(), "machine_id")
	if err != nil {
		return nil, err
	}
	bootstrap, err := s.machine.GetMachineBootstrap(ctx, machineID)
	if err != nil {
		return nil, mapError(err)
	}
	if err := authorizeOrganizationRead(ctx, bootstrap.Machine.OrganizationID); err != nil {
		return nil, err
	}
	slotView, err := s.machine.GetMachineSlotView(ctx, machineID)
	if err != nil {
		return nil, mapError(err)
	}
	resp := &avfv1.GetMachineCabinetSlotSummaryResponse{
		Machine:            mapMachineSummary(bootstrap.Machine),
		Cabinets:           make([]*avfv1.MachineCabinetSummary, 0, len(bootstrap.Cabinets)),
		ConfiguredSlots:    make([]*avfv1.MachineConfiguredSlotSummary, 0, len(bootstrap.CurrentCabinetSlots)),
		LegacySlots:        make([]*avfv1.MachineLegacySlotSummary, 0, len(slotView.LegacySlots)),
		AssortmentProducts: make([]*avfv1.MachineAssortmentProductSummary, 0, len(bootstrap.AssortmentProducts)),
	}
	for _, cabinet := range bootstrap.Cabinets {
		resp.Cabinets = append(resp.Cabinets, mapCabinetSummary(cabinet))
	}
	for _, slot := range bootstrap.CurrentCabinetSlots {
		resp.ConfiguredSlots = append(resp.ConfiguredSlots, mapConfiguredSlotSummary(slot))
	}
	for _, legacy := range slotView.LegacySlots {
		resp.LegacySlots = append(resp.LegacySlots, mapLegacySlotSummary(legacy))
	}
	for _, product := range bootstrap.AssortmentProducts {
		resp.AssortmentProducts = append(resp.AssortmentProducts, mapAssortmentProductSummary(product))
	}
	return resp, nil
}

type telemetryQueryServer struct {
	avfv1.UnimplementedInternalTelemetryQueryServiceServer
	telemetry appapi.InternalTelemetryQueryService
}

func (s *telemetryQueryServer) GetLatestMachineTelemetry(ctx context.Context, req *avfv1.GetMachineRequest) (*avfv1.GetLatestMachineTelemetryResponse, error) {
	machineID, err := requireUUID(req.GetMachineId(), "machine_id")
	if err != nil {
		return nil, err
	}
	snapshot, err := s.telemetry.GetTelemetrySnapshot(ctx, machineID)
	if err != nil {
		return nil, mapError(err)
	}
	if err := authorizeOrganizationRead(ctx, snapshot.OrganizationID); err != nil {
		return nil, err
	}
	return &avfv1.GetLatestMachineTelemetryResponse{Snapshot: mapTelemetrySnapshot(snapshot)}, nil
}

func (s *telemetryQueryServer) GetMachineIncidentSummary(ctx context.Context, req *avfv1.GetMachineIncidentSummaryRequest) (*avfv1.GetMachineIncidentSummaryResponse, error) {
	machineID, err := requireUUID(req.GetMachineId(), "machine_id")
	if err != nil {
		return nil, err
	}
	snapshot, err := s.telemetry.GetTelemetrySnapshot(ctx, machineID)
	if err != nil {
		return nil, mapError(err)
	}
	if err := authorizeOrganizationRead(ctx, snapshot.OrganizationID); err != nil {
		return nil, err
	}
	limit := req.GetLimit()
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	items, err := s.telemetry.ListMachineIncidentsRecent(ctx, machineID, limit)
	if err != nil {
		return nil, mapError(err)
	}
	resp := &avfv1.GetMachineIncidentSummaryResponse{
		MachineId: machineID.String(),
		Limit:     limit,
		Returned:  int32(len(items)),
		Items:     make([]*avfv1.MachineIncidentSummary, 0, len(items)),
	}
	for _, item := range items {
		resp.Items = append(resp.Items, mapIncidentSummary(item))
	}
	return resp, nil
}

type commerceQueryServer struct {
	avfv1.UnimplementedInternalCommerceQueryServiceServer
	commerce appapi.InternalCommerceQueryService
}

func (s *commerceQueryServer) GetOrderPaymentVendState(ctx context.Context, req *avfv1.GetOrderPaymentVendStateRequest) (*avfv1.GetOrderPaymentVendStateResponse, error) {
	organizationID, err := requireUUID(req.GetOrganizationId(), "organization_id")
	if err != nil {
		return nil, err
	}
	orderID, err := requireUUID(req.GetOrderId(), "order_id")
	if err != nil {
		return nil, err
	}
	if req.GetSlotIndex() < 0 {
		return nil, status.Error(codes.InvalidArgument, "slot_index must be non-negative")
	}
	if err := authorizeOrganizationRead(ctx, organizationID); err != nil {
		return nil, err
	}
	out, err := s.commerce.GetCheckoutStatus(ctx, organizationID, orderID, req.GetSlotIndex())
	if err != nil {
		return nil, mapError(err)
	}
	resp := &avfv1.GetOrderPaymentVendStateResponse{
		Order:          mapCommerceOrder(out.Order),
		Vend:           mapCommerceVend(out.Vend),
		PaymentPresent: out.PaymentPresent,
	}
	if out.PaymentPresent {
		resp.Payment = mapCommercePayment(out.Payment)
	}
	return resp, nil
}

func requireUUID(raw string, field string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "%s must be a valid UUID", field)
	}
	return id, nil
}

func authorizeOrganizationRead(ctx context.Context, organizationID uuid.UUID) error {
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing principal")
	}
	if !principal.HasAnyRole(auth.RoleService, auth.RolePlatformAdmin, auth.RoleOrgAdmin, auth.RoleOrgMember) {
		return status.Error(codes.PermissionDenied, "principal is not allowed to use internal gRPC queries")
	}
	if principal.HasRole(auth.RoleService) || principal.HasRole(auth.RolePlatformAdmin) {
		return nil
	}
	if !principal.HasOrganization() || principal.OrganizationID != organizationID {
		return status.Error(codes.PermissionDenied, "organization scope mismatch")
	}
	return nil
}

func mapMachineSummary(m domainfleet.Machine) *avfv1.MachineSummary {
	var hardwareProfileID *wrapperspb.StringValue
	if m.HardwareProfileID != nil && *m.HardwareProfileID != uuid.Nil {
		hardwareProfileID = wrapperspb.String(m.HardwareProfileID.String())
	}
	return &avfv1.MachineSummary{
		MachineId:         m.ID.String(),
		OrganizationId:    m.OrganizationID.String(),
		SiteId:            m.SiteID.String(),
		HardwareProfileId: hardwareProfileID,
		SerialNumber:      m.SerialNumber,
		Name:              m.Name,
		Status:            m.Status,
		CommandSequence:   m.CommandSequence,
		CreatedAt:         timestamppb.New(m.CreatedAt.UTC()),
		UpdatedAt:         timestamppb.New(m.UpdatedAt.UTC()),
	}
}

func mapShadowState(v *appapi.ShadowView) *avfv1.MachineShadowState {
	if v == nil {
		return nil
	}
	return &avfv1.MachineShadowState{
		MachineId:     v.MachineID.String(),
		DesiredState:  structFromMap(v.DesiredState),
		ReportedState: structFromMap(v.ReportedState),
		Version:       v.Version,
	}
}

func mapTelemetrySnapshot(v appapi.TelemetrySnapshotView) *avfv1.MachineTelemetrySnapshot {
	return &avfv1.MachineTelemetrySnapshot{
		MachineId:         v.MachineID.String(),
		OrganizationId:    v.OrganizationID.String(),
		SiteId:            v.SiteID.String(),
		ReportedState:     structFromJSON(v.ReportedState),
		MetricsState:      structFromJSON(v.MetricsState),
		LastHeartbeatAt:   timestampPtr(v.LastHeartbeatAt),
		AppVersion:        stringPtr(v.AppVersion),
		FirmwareVersion:   stringPtr(v.FirmwareVersion),
		UpdatedAt:         timestamppb.New(v.UpdatedAt.UTC()),
		AndroidId:         stringPtr(v.AndroidID),
		SimSerial:         stringPtr(v.SimSerial),
		SimIccid:          stringPtr(v.SimIccid),
		DeviceModel:       stringPtr(v.DeviceModel),
		OsVersion:         stringPtr(v.OSVersion),
		LastIdentityAt:    timestampPtr(v.LastIdentityAt),
		EffectiveTimezone: v.EffectiveTimezone,
	}
}

func mapCabinetSummary(v setupapp.CabinetView) *avfv1.MachineCabinetSummary {
	return &avfv1.MachineCabinetSummary{
		Id:        v.ID.String(),
		MachineId: v.MachineID.String(),
		Code:      v.Code,
		Title:     v.Title,
		SortOrder: v.SortOrder,
		Metadata:  structFromJSON(v.Metadata),
		CreatedAt: timestamppb.New(v.CreatedAt.UTC()),
		UpdatedAt: timestamppb.New(v.UpdatedAt.UTC()),
	}
}

func mapConfiguredSlotSummary(v setupapp.CabinetSlotConfigView) *avfv1.MachineConfiguredSlotSummary {
	var productID *wrapperspb.StringValue
	if v.ProductID != nil && *v.ProductID != uuid.Nil {
		productID = wrapperspb.String(v.ProductID.String())
	}
	var slotIndex *wrapperspb.Int32Value
	if v.SlotIndex != nil {
		slotIndex = wrapperspb.Int32(*v.SlotIndex)
	}
	return &avfv1.MachineConfiguredSlotSummary{
		ConfigId:           v.ConfigID.String(),
		CabinetCode:        v.CabinetCode,
		SlotCode:           v.SlotCode,
		SlotIndex:          slotIndex,
		ProductId:          productID,
		ProductSku:         v.ProductSKU,
		ProductName:        v.ProductName,
		MaxQuantity:        v.MaxQuantity,
		PriceMinor:         v.PriceMinor,
		EffectiveFrom:      timestamppb.New(v.EffectiveFrom.UTC()),
		IsCurrent:          v.IsCurrent,
		MachineSlotLayoutId: v.MachineSlotLayout.String(),
	}
}

func mapLegacySlotSummary(v setupapp.LegacySlotRow) *avfv1.MachineLegacySlotSummary {
	var productID *wrapperspb.StringValue
	if v.ProductID != nil && *v.ProductID != uuid.Nil {
		productID = wrapperspb.String(v.ProductID.String())
	}
	return &avfv1.MachineLegacySlotSummary{
		PlanogramId:       v.PlanogramID.String(),
		PlanogramName:     v.PlanogramName,
		SlotIndex:         v.SlotIndex,
		CurrentQuantity:   v.CurrentQuantity,
		MaxQuantity:       v.MaxQuantity,
		PriceMinor:        v.PriceMinor,
		ProductId:         productID,
		ProductSku:        v.ProductSKU,
		ProductName:       v.ProductName,
		PlanogramRevision: v.PlanogramRevision,
	}
}

func mapAssortmentProductSummary(v setupapp.AssortmentProductView) *avfv1.MachineAssortmentProductSummary {
	return &avfv1.MachineAssortmentProductSummary{
		ProductId:      v.ProductID.String(),
		Sku:            v.SKU,
		Name:           v.Name,
		SortOrder:      v.SortOrder,
		AssortmentId:   v.AssortmentID.String(),
		AssortmentName: v.AssortmentName,
	}
}

func mapIncidentSummary(v appapi.MachineIncidentView) *avfv1.MachineIncidentSummary {
	return &avfv1.MachineIncidentSummary{
		Id:        v.ID.String(),
		MachineId: v.MachineID.String(),
		Severity:  v.Severity,
		Code:      v.Code,
		Title:     stringPtr(v.Title),
		Detail:    structFromJSON(v.Detail),
		DedupeKey: stringPtr(v.DedupeKey),
		OpenedAt:  timestamppb.New(v.OpenedAt.UTC()),
		UpdatedAt: timestamppb.New(v.UpdatedAt.UTC()),
	}
}

func mapCommerceOrder(v domaincommerce.Order) *avfv1.CommerceOrderSummary {
	return &avfv1.CommerceOrderSummary{
		Id:             v.ID.String(),
		OrganizationId: v.OrganizationID.String(),
		MachineId:      v.MachineID.String(),
		Status:         v.Status,
		Currency:       v.Currency,
		SubtotalMinor:  v.SubtotalMinor,
		TaxMinor:       v.TaxMinor,
		TotalMinor:     v.TotalMinor,
		IdempotencyKey: stringPtr(v.IdempotencyKey),
		CreatedAt:      timestamppb.New(v.CreatedAt.UTC()),
		UpdatedAt:      timestamppb.New(v.UpdatedAt.UTC()),
	}
}

func mapCommerceVend(v domaincommerce.VendSession) *avfv1.CommerceVendSummary {
	var finalCommandAttemptID *wrapperspb.StringValue
	if v.FinalCommandAttemptID != nil && *v.FinalCommandAttemptID != uuid.Nil {
		finalCommandAttemptID = wrapperspb.String(v.FinalCommandAttemptID.String())
	}
	return &avfv1.CommerceVendSummary{
		Id:                   v.ID.String(),
		OrderId:              v.OrderID.String(),
		MachineId:            v.MachineID.String(),
		SlotIndex:            v.SlotIndex,
		ProductId:            v.ProductID.String(),
		State:                v.State,
		FinalCommandAttemptId: finalCommandAttemptID,
		CreatedAt:            timestamppb.New(v.CreatedAt.UTC()),
	}
}

func mapCommercePayment(v domaincommerce.Payment) *avfv1.CommercePaymentSummary {
	var settlementBatchID *wrapperspb.StringValue
	if v.SettlementBatchID != nil && *v.SettlementBatchID != uuid.Nil {
		settlementBatchID = wrapperspb.String(v.SettlementBatchID.String())
	}
	return &avfv1.CommercePaymentSummary{
		Id:                   v.ID.String(),
		OrderId:              v.OrderID.String(),
		Provider:             v.Provider,
		State:                v.State,
		AmountMinor:          v.AmountMinor,
		Currency:             v.Currency,
		IdempotencyKey:       stringPtr(v.IdempotencyKey),
		ReconciliationStatus: v.ReconciliationStatus,
		SettlementStatus:     v.SettlementStatus,
		SettlementBatchId:    settlementBatchID,
		CreatedAt:            timestamppb.New(v.CreatedAt.UTC()),
	}
}

func structFromMap(v map[string]any) *structpb.Struct {
	if v == nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	s, err := structpb.NewStruct(v)
	if err != nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	return s
}

func structFromJSON(raw []byte) *structpb.Struct {
	if len(raw) == 0 {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return &structpb.Struct{Fields: map[string]*structpb.Value{}}
	}
	return structFromMap(out)
}

func stringPtr(v *string) *wrapperspb.StringValue {
	if v == nil {
		return nil
	}
	return wrapperspb.String(*v)
}

func timestampPtr(v *time.Time) *timestamppb.Timestamp {
	if v == nil {
		return nil
	}
	return timestamppb.New(v.UTC())
}
