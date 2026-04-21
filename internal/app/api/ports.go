package api

import (
	"context"

	appcommerceadmin "github.com/avf/avf-vending-api/internal/app/commerceadmin"
	appfleetadmin "github.com/avf/avf-vending-api/internal/app/fleetadmin"
	"github.com/avf/avf-vending-api/internal/app/listscope"
	appreporting "github.com/avf/avf-vending-api/internal/app/reporting"
	"github.com/google/uuid"
)

// ListView is the legacy JSON shape for untyped list endpoints.
type ListView struct {
	Items []map[string]any `json:"items"`
}

// ShadowView is the machine shadow payload for API contracts.
type ShadowView struct {
	MachineID     uuid.UUID      `json:"machine_id"`
	DesiredState  map[string]any `json:"desired_state"`
	ReportedState map[string]any `json:"reported_state"`
	Version       int64          `json:"version"`
}

// MachinesAdminService is the application port for admin machine operations.
type MachinesAdminService interface {
	ListMachines(ctx context.Context, scope listscope.AdminFleet) (*appfleetadmin.MachinesListResponse, error)
	GetMachine(ctx context.Context, organizationID, machineID uuid.UUID) (*appfleetadmin.AdminMachineListItem, error)
}

// TechniciansAdminService is the application port for admin technician operations.
type TechniciansAdminService interface {
	ListTechnicians(ctx context.Context, scope listscope.AdminFleet) (*appfleetadmin.TechniciansListResponse, error)
}

// AssignmentsAdminService is the application port for technician–machine assignments.
type AssignmentsAdminService interface {
	ListAssignments(ctx context.Context, scope listscope.AdminFleet) (*appfleetadmin.AssignmentsListResponse, error)
}

// CommandsAdminService is the application port for fleet command orchestration (admin).
type CommandsAdminService interface {
	ListCommands(ctx context.Context, scope listscope.AdminFleet) (*appfleetadmin.CommandsListResponse, error)
}

// OTAAdminService is the application port for OTA campaign and artifact administration.
type OTAAdminService interface {
	ListOTA(ctx context.Context, scope listscope.AdminFleet) (*appfleetadmin.OTAListResponse, error)
}

// PaymentsService is the application port for payment operations exposed on the public versioned API.
type PaymentsService interface {
	ListPayments(ctx context.Context, scope listscope.TenantCommerce) (*appcommerceadmin.PaymentsListResponse, error)
}

// OrdersService is the application port for order operations exposed on the public versioned API.
type OrdersService interface {
	ListOrders(ctx context.Context, scope listscope.TenantCommerce) (*appcommerceadmin.OrdersListResponse, error)
}

// MachineShadowService reads machine shadow state for a device.
type MachineShadowService interface {
	GetShadow(ctx context.Context, machineID uuid.UUID) (*ShadowView, error)
}

// ReportingService exposes read-only analytics for operations dashboards (GET /v1/reports/*).
type ReportingService interface {
	SalesSummary(ctx context.Context, q listscope.ReportingQuery) (*appreporting.SalesSummaryResponse, error)
	PaymentsSummary(ctx context.Context, q listscope.ReportingQuery) (*appreporting.PaymentsSummaryResponse, error)
	FleetHealth(ctx context.Context, q listscope.ReportingQuery) (*appreporting.FleetHealthResponse, error)
	InventoryExceptions(ctx context.Context, q listscope.ReportingQuery) (*appreporting.InventoryExceptionsResponse, error)
}
