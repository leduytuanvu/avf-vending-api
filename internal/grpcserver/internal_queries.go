package grpcserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appapi "github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/app/listscope"
	appsalecatalog "github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	domainfleet "github.com/avf/avf-vending-api/internal/domain/fleet"
	internalv1 "github.com/avf/avf-vending-api/internal/gen/avfinternalv1"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// InternalQueryServices wires read-only internal gRPC handlers (avf.internal.v1) to application query ports.
type InternalQueryServices struct {
	Machine   appapi.InternalMachineQueryService
	Telemetry appapi.InternalTelemetryQueryService
	Commerce  appapi.InternalCommerceQueryService
	Payment   appapi.InternalPaymentQueryService
	Catalog   appsalecatalog.SnapshotBuilder
	Inventory appapi.InternalInventoryQueryService
	Reporting appapi.ReportingService
}

// RegisterInternalQueryServices registers split-ready internal query RPCs (separate listener; see NewInternalGRPCServer).
func RegisterInternalQueryServices(services InternalQueryServices) ServiceRegistrar {
	return func(s *grpc.Server) error {
		if services.Machine == nil || services.Telemetry == nil || services.Commerce == nil || services.Payment == nil ||
			services.Catalog == nil || services.Inventory == nil || services.Reporting == nil {
			return fmt.Errorf("grpcserver: internal query services require machine, telemetry, commerce, payment, catalog, inventory, and reporting deps")
		}
		internalv1.RegisterInternalMachineQueryServiceServer(s, &machineQueryServer{
			machine:   services.Machine,
			telemetry: services.Telemetry,
		})
		internalv1.RegisterInternalTelemetryQueryServiceServer(s, &telemetryQueryServer{
			telemetry: services.Telemetry,
		})
		internalv1.RegisterInternalCommerceQueryServiceServer(s, &commerceQueryServer{
			commerce: services.Commerce,
		})
		internalv1.RegisterInternalPaymentQueryServiceServer(s, &paymentQueryServer{
			payment: services.Payment,
		})
		internalv1.RegisterInternalCatalogQueryServiceServer(s, &catalogQueryServer{
			catalog: services.Catalog,
		})
		internalv1.RegisterInternalInventoryQueryServiceServer(s, &inventoryQueryServer{
			inv:     services.Inventory,
			machine: services.Machine,
		})
		internalv1.RegisterInternalReportingQueryServiceServer(s, &reportingQueryServer{
			reporting: services.Reporting,
		})
		return nil
	}
}

type machineQueryServer struct {
	internalv1.UnimplementedInternalMachineQueryServiceServer
	machine   appapi.InternalMachineQueryService
	telemetry appapi.InternalTelemetryQueryService
}

func (s *machineQueryServer) GetMachineSummary(ctx context.Context, req *internalv1.GetMachineSummaryRequest) (*internalv1.GetMachineSummaryResponse, error) {
	machineID, err := requireUUID(req.GetMachineId(), "machine_id")
	if err != nil {
		return nil, err
	}
	bootstrap, err := s.machine.GetMachineBootstrap(ctx, machineID)
	if err != nil {
		return nil, mapError(err)
	}
	if err := authorizeInternalQueryRead(ctx, bootstrap.Machine.OrganizationID); err != nil {
		return nil, err
	}
	return &internalv1.GetMachineSummaryResponse{Machine: mapMachineSummary(bootstrap.Machine)}, nil
}

func (s *machineQueryServer) GetMachineState(ctx context.Context, req *internalv1.GetMachineStateRequest) (*internalv1.GetMachineStateResponse, error) {
	machineID, err := requireUUID(req.GetMachineId(), "machine_id")
	if err != nil {
		return nil, err
	}
	snapshot, err := s.telemetry.GetTelemetrySnapshot(ctx, machineID)
	if err != nil {
		return nil, mapError(err)
	}
	if err := authorizeInternalQueryRead(ctx, snapshot.OrganizationID); err != nil {
		return nil, err
	}
	shadow, err := s.machine.GetShadow(ctx, machineID)
	if err != nil {
		return nil, mapError(err)
	}
	return &internalv1.GetMachineStateResponse{
		Shadow:   mapShadowState(shadow),
		Snapshot: mapTelemetrySnapshot(snapshot),
	}, nil
}

func (s *machineQueryServer) GetMachineCabinetSlotSummary(ctx context.Context, req *internalv1.GetMachineCabinetSlotSummaryRequest) (*internalv1.GetMachineCabinetSlotSummaryResponse, error) {
	machineID, err := requireUUID(req.GetMachineId(), "machine_id")
	if err != nil {
		return nil, err
	}
	bootstrap, err := s.machine.GetMachineBootstrap(ctx, machineID)
	if err != nil {
		return nil, mapError(err)
	}
	if err := authorizeInternalQueryRead(ctx, bootstrap.Machine.OrganizationID); err != nil {
		return nil, err
	}
	slotView, err := s.machine.GetMachineSlotView(ctx, machineID)
	if err != nil {
		return nil, mapError(err)
	}
	resp := &internalv1.GetMachineCabinetSlotSummaryResponse{
		Machine:            mapMachineSummary(bootstrap.Machine),
		Cabinets:           make([]*internalv1.MachineCabinetSummary, 0, len(bootstrap.Cabinets)),
		ConfiguredSlots:    make([]*internalv1.MachineConfiguredSlotSummary, 0, len(bootstrap.CurrentCabinetSlots)),
		LegacySlots:        make([]*internalv1.MachineLegacySlotSummary, 0, len(slotView.LegacySlots)),
		AssortmentProducts: make([]*internalv1.MachineAssortmentProductSummary, 0, len(bootstrap.AssortmentProducts)),
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
	internalv1.UnimplementedInternalTelemetryQueryServiceServer
	telemetry appapi.InternalTelemetryQueryService
}

func (s *telemetryQueryServer) GetLatestMachineTelemetry(ctx context.Context, req *internalv1.GetLatestMachineTelemetryRequest) (*internalv1.GetLatestMachineTelemetryResponse, error) {
	machineID, err := requireUUID(req.GetMachineId(), "machine_id")
	if err != nil {
		return nil, err
	}
	snapshot, err := s.telemetry.GetTelemetrySnapshot(ctx, machineID)
	if err != nil {
		return nil, mapError(err)
	}
	if err := authorizeInternalQueryRead(ctx, snapshot.OrganizationID); err != nil {
		return nil, err
	}
	return &internalv1.GetLatestMachineTelemetryResponse{Snapshot: mapTelemetrySnapshot(snapshot)}, nil
}

func (s *telemetryQueryServer) GetMachineIncidentSummary(ctx context.Context, req *internalv1.GetMachineIncidentSummaryRequest) (*internalv1.GetMachineIncidentSummaryResponse, error) {
	machineID, err := requireUUID(req.GetMachineId(), "machine_id")
	if err != nil {
		return nil, err
	}
	snapshot, err := s.telemetry.GetTelemetrySnapshot(ctx, machineID)
	if err != nil {
		return nil, mapError(err)
	}
	if err := authorizeInternalQueryRead(ctx, snapshot.OrganizationID); err != nil {
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
	resp := &internalv1.GetMachineIncidentSummaryResponse{
		MachineId: machineID.String(),
		Limit:     limit,
		Returned:  int32(len(items)),
		Items:     make([]*internalv1.MachineIncidentSummary, 0, len(items)),
	}
	for _, item := range items {
		resp.Items = append(resp.Items, mapIncidentSummary(item))
	}
	return resp, nil
}

type commerceQueryServer struct {
	internalv1.UnimplementedInternalCommerceQueryServiceServer
	commerce appapi.InternalCommerceQueryService
}

func (s *commerceQueryServer) GetOrderPaymentVendState(ctx context.Context, req *internalv1.GetOrderPaymentVendStateRequest) (*internalv1.GetOrderPaymentVendStateResponse, error) {
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
	if err := authorizeInternalQueryRead(ctx, organizationID); err != nil {
		return nil, err
	}
	out, err := s.commerce.GetCheckoutStatus(ctx, organizationID, orderID, req.GetSlotIndex())
	if err != nil {
		return nil, mapError(err)
	}
	resp := &internalv1.GetOrderPaymentVendStateResponse{
		Order:          mapCommerceOrder(out.Order),
		Vend:           mapCommerceVend(out.Vend),
		PaymentPresent: out.PaymentPresent,
	}
	if out.PaymentPresent {
		resp.Payment = mapCommercePayment(out.Payment)
	}
	return resp, nil
}

type paymentQueryServer struct {
	internalv1.UnimplementedInternalPaymentQueryServiceServer
	payment appapi.InternalPaymentQueryService
}

func (s *paymentQueryServer) GetPaymentById(ctx context.Context, req *internalv1.GetPaymentByIdRequest) (*internalv1.GetPaymentByIdResponse, error) {
	organizationID, err := requireUUID(req.GetOrganizationId(), "organization_id")
	if err != nil {
		return nil, err
	}
	paymentID, err := requireUUID(req.GetPaymentId(), "payment_id")
	if err != nil {
		return nil, err
	}
	if err := authorizeInternalQueryRead(ctx, organizationID); err != nil {
		return nil, err
	}
	pay, err := s.payment.GetPaymentByID(ctx, organizationID, paymentID)
	if err != nil {
		return nil, mapError(err)
	}
	return &internalv1.GetPaymentByIdResponse{Payment: mapCommercePayment(pay)}, nil
}

func (s *paymentQueryServer) GetLatestPaymentForOrder(ctx context.Context, req *internalv1.GetLatestPaymentForOrderRequest) (*internalv1.GetLatestPaymentForOrderResponse, error) {
	organizationID, err := requireUUID(req.GetOrganizationId(), "organization_id")
	if err != nil {
		return nil, err
	}
	orderID, err := requireUUID(req.GetOrderId(), "order_id")
	if err != nil {
		return nil, err
	}
	if err := authorizeInternalQueryRead(ctx, organizationID); err != nil {
		return nil, err
	}
	pay, err := s.payment.GetLatestPaymentForOrder(ctx, organizationID, orderID)
	if err != nil {
		return nil, mapError(err)
	}
	return &internalv1.GetLatestPaymentForOrderResponse{Payment: mapCommercePayment(pay)}, nil
}

type catalogQueryServer struct {
	internalv1.UnimplementedInternalCatalogQueryServiceServer
	catalog appsalecatalog.SnapshotBuilder
}

func (s *catalogQueryServer) GetSaleCatalogSnapshot(ctx context.Context, req *internalv1.GetSaleCatalogSnapshotRequest) (*internalv1.GetSaleCatalogSnapshotResponse, error) {
	machineID, err := requireUUID(req.GetMachineId(), "machine_id")
	if err != nil {
		return nil, err
	}
	opts := appsalecatalog.Options{
		IncludeUnavailable: req.GetIncludeUnavailable(),
		IncludeImages:      req.GetIncludeImages(),
	}
	if req.IfNoneMatchConfigVersion != nil {
		v := *req.IfNoneMatchConfigVersion
		opts.IfNoneMatchConfigVersion = &v
	}
	snap, err := s.catalog.BuildSnapshot(ctx, machineID, opts)
	if err != nil {
		return nil, mapError(err)
	}
	if err := authorizeInternalQueryRead(ctx, snap.OrganizationID); err != nil {
		return nil, err
	}
	if snap.NotModified {
		return &internalv1.GetSaleCatalogSnapshotResponse{NotModified: true, CatalogJson: "{}"}, nil
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		return nil, status.Error(codes.Internal, "catalog encode failed")
	}
	return &internalv1.GetSaleCatalogSnapshotResponse{CatalogJson: string(raw)}, nil
}

type inventoryQueryServer struct {
	internalv1.UnimplementedInternalInventoryQueryServiceServer
	inv     appapi.InternalInventoryQueryService
	machine appapi.InternalMachineQueryService
}

func (s *inventoryQueryServer) GetMachineSlotInventory(ctx context.Context, req *internalv1.GetMachineSlotInventoryRequest) (*internalv1.GetMachineSlotInventoryResponse, error) {
	machineID, err := requireUUID(req.GetMachineId(), "machine_id")
	if err != nil {
		return nil, err
	}
	bootstrap, err := s.machine.GetMachineBootstrap(ctx, machineID)
	if err != nil {
		return nil, mapError(err)
	}
	if err := authorizeInternalQueryRead(ctx, bootstrap.Machine.OrganizationID); err != nil {
		return nil, err
	}
	slotView, err := s.inv.GetMachineSlotView(ctx, machineID)
	if err != nil {
		return nil, mapError(err)
	}
	out := &internalv1.GetMachineSlotInventoryResponse{
		MachineId:   machineID.String(),
		LegacySlots: make([]*internalv1.InventoryLegacySlotRow, 0, len(slotView.LegacySlots)),
	}
	for _, legacy := range slotView.LegacySlots {
		out.LegacySlots = append(out.LegacySlots, mapInventoryLegacySlot(legacy))
	}
	return out, nil
}

type reportingQueryServer struct {
	internalv1.UnimplementedInternalReportingQueryServiceServer
	reporting appapi.ReportingService
}

func (s *reportingQueryServer) GetSalesSummary(ctx context.Context, req *internalv1.GetSalesSummaryRequest) (*internalv1.GetSalesSummaryResponse, error) {
	organizationID, err := requireUUID(req.GetOrganizationId(), "organization_id")
	if err != nil {
		return nil, err
	}
	if err := authorizeInternalQueryRead(ctx, organizationID); err != nil {
		return nil, err
	}
	from, err := time.Parse(time.RFC3339, strings.TrimSpace(req.GetFromRfc3339()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "from_rfc3339 must be RFC3339")
	}
	to, err := time.Parse(time.RFC3339, strings.TrimSpace(req.GetToRfc3339()))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "to_rfc3339 must be RFC3339")
	}
	q := listscope.ReportingQuery{
		OrganizationID:  organizationID,
		From:            from,
		To:              to,
		GroupBy:         strings.TrimSpace(req.GetGroupBy()),
		IsPlatformAdmin: false,
	}
	out, err := s.reporting.SalesSummary(ctx, q)
	if err != nil {
		return nil, mapError(err)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, status.Error(codes.Internal, "reporting encode failed")
	}
	return &internalv1.GetSalesSummaryResponse{SummaryJson: string(raw)}, nil
}

func requireUUID(raw string, field string) (uuid.UUID, error) {
	id, err := uuid.Parse(raw)
	if err != nil || id == uuid.Nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "%s must be a valid UUID", field)
	}
	return id, nil
}

func authorizeInternalQueryRead(ctx context.Context, organizationID uuid.UUID) error {
	principal, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing principal")
	}
	if !strings.EqualFold(strings.TrimSpace(principal.JWTAudience), auth.AudienceInternalGRPC) ||
		!strings.EqualFold(strings.TrimSpace(principal.JWTType), auth.JWTClaimTypeInternalService) {
		return status.Error(codes.PermissionDenied, "requires avf-internal-grpc service token")
	}
	if !principal.HasRole(auth.RoleService) && !principal.HasRole(auth.RolePlatformAdmin) {
		return status.Error(codes.PermissionDenied, "principal is not allowed to use internal gRPC queries")
	}
	if principal.HasRole(auth.RoleService) {
		if principal.HasOrganization() && principal.OrganizationID != uuid.Nil && organizationID != uuid.Nil &&
			principal.OrganizationID != organizationID {
			return status.Error(codes.PermissionDenied, "organization scope mismatch")
		}
		return nil
	}
	if principal.HasRole(auth.RolePlatformAdmin) {
		return nil
	}
	if !principal.HasOrganization() || principal.OrganizationID != organizationID {
		return status.Error(codes.PermissionDenied, "organization scope mismatch")
	}
	return nil
}

func mapMachineSummary(m domainfleet.Machine) *internalv1.MachineSummary {
	var hardwareProfileID *wrapperspb.StringValue
	if m.HardwareProfileID != nil && *m.HardwareProfileID != uuid.Nil {
		hardwareProfileID = wrapperspb.String(m.HardwareProfileID.String())
	}
	return &internalv1.MachineSummary{
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

func mapShadowState(v *appapi.ShadowView) *internalv1.MachineShadowState {
	if v == nil {
		return nil
	}
	return &internalv1.MachineShadowState{
		MachineId:     v.MachineID.String(),
		DesiredState:  structFromMap(v.DesiredState),
		ReportedState: structFromMap(v.ReportedState),
		Version:       v.Version,
	}
}

func mapTelemetrySnapshot(v appapi.TelemetrySnapshotView) *internalv1.MachineTelemetrySnapshot {
	return &internalv1.MachineTelemetrySnapshot{
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

func mapCabinetSummary(v setupapp.CabinetView) *internalv1.MachineCabinetSummary {
	return &internalv1.MachineCabinetSummary{
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

func mapConfiguredSlotSummary(v setupapp.CabinetSlotConfigView) *internalv1.MachineConfiguredSlotSummary {
	var productID *wrapperspb.StringValue
	if v.ProductID != nil && *v.ProductID != uuid.Nil {
		productID = wrapperspb.String(v.ProductID.String())
	}
	var slotIndex *wrapperspb.Int32Value
	if v.SlotIndex != nil {
		slotIndex = wrapperspb.Int32(*v.SlotIndex)
	}
	return &internalv1.MachineConfiguredSlotSummary{
		ConfigId:            v.ConfigID.String(),
		CabinetCode:         v.CabinetCode,
		SlotCode:            v.SlotCode,
		SlotIndex:           slotIndex,
		ProductId:           productID,
		ProductSku:          v.ProductSKU,
		ProductName:         v.ProductName,
		MaxQuantity:         v.MaxQuantity,
		PriceMinor:          v.PriceMinor,
		EffectiveFrom:       timestamppb.New(v.EffectiveFrom.UTC()),
		IsCurrent:           v.IsCurrent,
		MachineSlotLayoutId: v.MachineSlotLayout.String(),
	}
}

func mapLegacySlotSummary(v setupapp.LegacySlotRow) *internalv1.MachineLegacySlotSummary {
	var productID *wrapperspb.StringValue
	if v.ProductID != nil && *v.ProductID != uuid.Nil {
		productID = wrapperspb.String(v.ProductID.String())
	}
	return &internalv1.MachineLegacySlotSummary{
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

func mapInventoryLegacySlot(v setupapp.LegacySlotRow) *internalv1.InventoryLegacySlotRow {
	var productID *wrapperspb.StringValue
	if v.ProductID != nil && *v.ProductID != uuid.Nil {
		productID = wrapperspb.String(v.ProductID.String())
	}
	return &internalv1.InventoryLegacySlotRow{
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

func mapAssortmentProductSummary(v setupapp.AssortmentProductView) *internalv1.MachineAssortmentProductSummary {
	return &internalv1.MachineAssortmentProductSummary{
		ProductId:      v.ProductID.String(),
		Sku:            v.SKU,
		Name:           v.Name,
		SortOrder:      v.SortOrder,
		AssortmentId:   v.AssortmentID.String(),
		AssortmentName: v.AssortmentName,
	}
}

func mapIncidentSummary(v appapi.MachineIncidentView) *internalv1.MachineIncidentSummary {
	return &internalv1.MachineIncidentSummary{
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

func mapCommerceOrder(v domaincommerce.Order) *internalv1.CommerceOrderSummary {
	return &internalv1.CommerceOrderSummary{
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

func mapCommerceVend(v domaincommerce.VendSession) *internalv1.CommerceVendSummary {
	var finalCommandAttemptID *wrapperspb.StringValue
	if v.FinalCommandAttemptID != nil && *v.FinalCommandAttemptID != uuid.Nil {
		finalCommandAttemptID = wrapperspb.String(v.FinalCommandAttemptID.String())
	}
	return &internalv1.CommerceVendSummary{
		Id:                    v.ID.String(),
		OrderId:               v.OrderID.String(),
		MachineId:             v.MachineID.String(),
		SlotIndex:             v.SlotIndex,
		ProductId:             v.ProductID.String(),
		State:                 v.State,
		FinalCommandAttemptId: finalCommandAttemptID,
		CreatedAt:             timestamppb.New(v.CreatedAt.UTC()),
	}
}

func mapCommercePayment(v domaincommerce.Payment) *internalv1.CommercePaymentSummary {
	var settlementBatchID *wrapperspb.StringValue
	if v.SettlementBatchID != nil && *v.SettlementBatchID != uuid.Nil {
		settlementBatchID = wrapperspb.String(v.SettlementBatchID.String())
	}
	return &internalv1.CommercePaymentSummary{
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
