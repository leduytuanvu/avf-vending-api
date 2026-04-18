package api

import (
	"context"

	"github.com/google/uuid"
)

// ListView is the JSON shape returned by versioned admin collection endpoints.
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

// AdminListScope carries principal-derived scope for platform admin routes.
type AdminListScope struct {
	IsPlatformAdmin bool
	OrganizationID  uuid.UUID
	SiteID          *uuid.UUID
}

// TenantCommerceScope carries organization context for commerce list routes.
type TenantCommerceScope struct {
	IsPlatformAdmin bool
	OrganizationID  uuid.UUID
}

// MachinesAdminService is the application port for admin machine operations.
type MachinesAdminService interface {
	ListMachines(ctx context.Context, scope AdminListScope) (*ListView, error)
}

// TechniciansAdminService is the application port for admin technician operations.
type TechniciansAdminService interface {
	ListTechnicians(ctx context.Context, scope AdminListScope) (*ListView, error)
}

// AssignmentsAdminService is the application port for technician–machine assignments.
type AssignmentsAdminService interface {
	ListAssignments(ctx context.Context, scope AdminListScope) (*ListView, error)
}

// CommandsAdminService is the application port for fleet command orchestration (admin).
type CommandsAdminService interface {
	ListCommands(ctx context.Context, scope AdminListScope) (*ListView, error)
}

// OTAAdminService is the application port for OTA campaign and artifact administration.
type OTAAdminService interface {
	ListOTA(ctx context.Context, scope AdminListScope) (*ListView, error)
}

// PaymentsService is the application port for payment operations exposed on the public versioned API.
type PaymentsService interface {
	ListPayments(ctx context.Context, scope TenantCommerceScope) (*ListView, error)
}

// OrdersService is the application port for order operations exposed on the public versioned API.
type OrdersService interface {
	ListOrders(ctx context.Context, scope TenantCommerceScope) (*ListView, error)
}

// MachineShadowService reads machine shadow state for a device.
type MachineShadowService interface {
	GetShadow(ctx context.Context, machineID uuid.UUID) (*ShadowView, error)
}
