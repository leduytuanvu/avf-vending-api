package api

import (
	"context"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/google/uuid"
)

// InternalMachineQueryService exposes machine-centric read models for internal transports.
type InternalMachineQueryService interface {
	GetMachineBootstrap(ctx context.Context, machineID uuid.UUID) (setupapp.MachineBootstrap, error)
	GetMachineSlotView(ctx context.Context, machineID uuid.UUID) (setupapp.MachineSlotView, error)
	GetShadow(ctx context.Context, machineID uuid.UUID) (*ShadowView, error)
}

// TelemetrySnapshotView is the app-layer DTO for current telemetry state.
type TelemetrySnapshotView struct {
	MachineID         uuid.UUID
	OrganizationID    uuid.UUID
	SiteID            uuid.UUID
	ReportedState     []byte
	MetricsState      []byte
	LastHeartbeatAt   *time.Time
	AppVersion        *string
	FirmwareVersion   *string
	UpdatedAt         time.Time
	AndroidID         *string
	SimSerial         *string
	SimIccid          *string
	DeviceModel       *string
	OSVersion         *string
	LastIdentityAt    *time.Time
	EffectiveTimezone string
}

// MachineIncidentView is the app-layer DTO for recent telemetry incidents.
type MachineIncidentView struct {
	ID        uuid.UUID
	MachineID uuid.UUID
	Severity  string
	Code      string
	Title     *string
	Detail    []byte
	DedupeKey *string
	OpenedAt  time.Time
	UpdatedAt time.Time
}

// InternalTelemetryQueryService exposes telemetry read models for internal transports.
type InternalTelemetryQueryService interface {
	GetTelemetrySnapshot(ctx context.Context, machineID uuid.UUID) (TelemetrySnapshotView, error)
	ListMachineIncidentsRecent(ctx context.Context, machineID uuid.UUID, limit int32) ([]MachineIncidentView, error)
}

// InternalCommerceQueryService exposes authoritative commerce state for internal transports.
type InternalCommerceQueryService interface {
	GetCheckoutStatus(ctx context.Context, organizationID, orderID uuid.UUID, slotIndex int32) (appcommerce.CheckoutStatusView, error)
}

// InternalPaymentQueryService exposes payment-only read models for internal transports.
type InternalPaymentQueryService interface {
	GetPaymentByID(ctx context.Context, organizationID, paymentID uuid.UUID) (domaincommerce.Payment, error)
	GetLatestPaymentForOrder(ctx context.Context, organizationID, orderID uuid.UUID) (domaincommerce.Payment, error)
}

type internalMachineQueryService struct {
	setup  *postgres.SetupRepository
	shadow MachineShadowService
}

// NewInternalMachineQueryService returns an internal machine read service backed by existing adapters.
func NewInternalMachineQueryService(store *postgres.Store, shadow MachineShadowService) InternalMachineQueryService {
	if store == nil || store.Pool() == nil {
		panic("api.NewInternalMachineQueryService: nil store")
	}
	if shadow == nil {
		panic("api.NewInternalMachineQueryService: nil shadow service")
	}
	return &internalMachineQueryService{
		setup:  postgres.NewSetupRepository(store.Pool()),
		shadow: shadow,
	}
}

func (s *internalMachineQueryService) GetMachineBootstrap(ctx context.Context, machineID uuid.UUID) (setupapp.MachineBootstrap, error) {
	return s.setup.GetMachineBootstrap(ctx, machineID)
}

func (s *internalMachineQueryService) GetMachineSlotView(ctx context.Context, machineID uuid.UUID) (setupapp.MachineSlotView, error) {
	return s.setup.GetMachineSlotView(ctx, machineID)
}

func (s *internalMachineQueryService) GetShadow(ctx context.Context, machineID uuid.UUID) (*ShadowView, error) {
	return s.shadow.GetShadow(ctx, machineID)
}

type internalTelemetryQueryService struct {
	store *postgres.Store
}

// NewInternalTelemetryQueryService returns internal telemetry reads backed by the existing store.
func NewInternalTelemetryQueryService(store *postgres.Store) InternalTelemetryQueryService {
	if store == nil {
		panic("api.NewInternalTelemetryQueryService: nil store")
	}
	return &internalTelemetryQueryService{store: store}
}

func (s *internalTelemetryQueryService) GetTelemetrySnapshot(ctx context.Context, machineID uuid.UUID) (TelemetrySnapshotView, error) {
	row, err := s.store.GetTelemetrySnapshot(ctx, machineID)
	if err != nil {
		return TelemetrySnapshotView{}, err
	}
	return TelemetrySnapshotView{
		MachineID:         row.MachineID,
		OrganizationID:    row.OrganizationID,
		SiteID:            row.SiteID,
		ReportedState:     row.ReportedState,
		MetricsState:      row.MetricsState,
		LastHeartbeatAt:   row.LastHeartbeatAt,
		AppVersion:        row.AppVersion,
		FirmwareVersion:   row.FirmwareVersion,
		UpdatedAt:         row.UpdatedAt,
		AndroidID:         row.AndroidID,
		SimSerial:         row.SimSerial,
		SimIccid:          row.SimIccid,
		DeviceModel:       row.DeviceModel,
		OSVersion:         row.OSVersion,
		LastIdentityAt:    row.LastIdentityAt,
		EffectiveTimezone: row.EffectiveTimezone,
	}, nil
}

func (s *internalTelemetryQueryService) ListMachineIncidentsRecent(ctx context.Context, machineID uuid.UUID, limit int32) ([]MachineIncidentView, error) {
	rows, err := s.store.ListMachineIncidentsRecent(ctx, machineID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]MachineIncidentView, 0, len(rows))
	for _, row := range rows {
		out = append(out, MachineIncidentView{
			ID:        row.ID,
			MachineID: row.MachineID,
			Severity:  row.Severity,
			Code:      row.Code,
			Title:     row.Title,
			Detail:    row.Detail,
			DedupeKey: row.DedupeKey,
			OpenedAt:  row.OpenedAt,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return out, nil
}

type internalCommerceQueryService struct {
	commerce *appcommerce.Service
}

// NewInternalCommerceQueryService returns an internal commerce read service.
func NewInternalCommerceQueryService(commerce *appcommerce.Service) InternalCommerceQueryService {
	if commerce == nil {
		panic("api.NewInternalCommerceQueryService: nil commerce service")
	}
	return &internalCommerceQueryService{commerce: commerce}
}

func (s *internalCommerceQueryService) GetCheckoutStatus(ctx context.Context, organizationID, orderID uuid.UUID, slotIndex int32) (appcommerce.CheckoutStatusView, error) {
	return s.commerce.GetCheckoutStatus(ctx, organizationID, orderID, slotIndex)
}

type internalPaymentQueryService struct {
	store *postgres.Store
}

// NewInternalPaymentQueryService returns internal payment reads backed by the existing store.
func NewInternalPaymentQueryService(store *postgres.Store) InternalPaymentQueryService {
	if store == nil {
		panic("api.NewInternalPaymentQueryService: nil store")
	}
	return &internalPaymentQueryService{store: store}
}

func (s *internalPaymentQueryService) GetPaymentByID(ctx context.Context, organizationID, paymentID uuid.UUID) (domaincommerce.Payment, error) {
	pay, err := s.store.GetPaymentByID(ctx, paymentID)
	if err != nil {
		return domaincommerce.Payment{}, err
	}
	order, err := s.store.GetOrderByID(ctx, pay.OrderID)
	if err != nil {
		return domaincommerce.Payment{}, err
	}
	if order.OrganizationID != organizationID {
		return domaincommerce.Payment{}, appcommerce.ErrOrgMismatch
	}
	return pay, nil
}

func (s *internalPaymentQueryService) GetLatestPaymentForOrder(ctx context.Context, organizationID, orderID uuid.UUID) (domaincommerce.Payment, error) {
	order, err := s.store.GetOrderByID(ctx, orderID)
	if err != nil {
		return domaincommerce.Payment{}, err
	}
	if order.OrganizationID != organizationID {
		return domaincommerce.Payment{}, appcommerce.ErrOrgMismatch
	}
	return s.store.GetLatestPaymentForOrder(ctx, orderID)
}
