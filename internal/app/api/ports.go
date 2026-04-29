package api

import (
	"context"

	appcommerceadmin "github.com/avf/avf-vending-api/internal/app/commerceadmin"
	appfinance "github.com/avf/avf-vending-api/internal/app/finance"
	appfleetadmin "github.com/avf/avf-vending-api/internal/app/fleetadmin"
	"github.com/avf/avf-vending-api/internal/app/listscope"
	appotaadmin "github.com/avf/avf-vending-api/internal/app/otaadmin"
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
	ListCampaigns(ctx context.Context, p appotaadmin.CampaignListParams) (*appotaadmin.CampaignListResponse, error)
	GetCampaignDetail(ctx context.Context, organizationID, campaignID uuid.UUID) (appotaadmin.CampaignDetail, error)
	CreateCampaign(ctx context.Context, in appotaadmin.CreateCampaignInput) (appotaadmin.CampaignDetail, error)
	PatchCampaign(ctx context.Context, organizationID, campaignID uuid.UUID, patch appotaadmin.PatchCampaignInput) (appotaadmin.CampaignDetail, error)
	PutCampaignTargets(ctx context.Context, in appotaadmin.PutTargetsInput) error
	ListCampaignTargets(ctx context.Context, organizationID, campaignID uuid.UUID) ([]appotaadmin.CampaignTargetItem, error)
	ListCampaignResults(ctx context.Context, organizationID, campaignID uuid.UUID) ([]appotaadmin.MachineResultItem, error)
	ApproveCampaign(ctx context.Context, organizationID, campaignID, actor uuid.UUID) (appotaadmin.CampaignDetail, error)
	StartCampaign(ctx context.Context, organizationID, campaignID uuid.UUID) (appotaadmin.CampaignDetail, error)
	PauseCampaign(ctx context.Context, organizationID, campaignID uuid.UUID) (appotaadmin.CampaignDetail, error)
	ResumeCampaign(ctx context.Context, organizationID, campaignID uuid.UUID) (appotaadmin.CampaignDetail, error)
	CancelCampaign(ctx context.Context, organizationID, campaignID uuid.UUID) (appotaadmin.CampaignDetail, error)
	RollbackCampaign(ctx context.Context, organizationID, campaignID uuid.UUID, rollbackArtifactID *uuid.UUID) (appotaadmin.CampaignDetail, error)
	// PublishCampaign approves (when draft) and starts rollout (when approved); idempotent for already-running/completed states that reject Start.
	PublishCampaign(ctx context.Context, organizationID, campaignID, actor uuid.UUID) (appotaadmin.CampaignDetail, error)
}

// PaymentsService is the application port for payment operations exposed on the public versioned API.
type PaymentsService interface {
	ListPayments(ctx context.Context, scope listscope.TenantCommerce) (*appcommerceadmin.PaymentsListResponse, error)
}

// OrdersService is the application port for order operations exposed on the public versioned API.
type OrdersService interface {
	ListOrders(ctx context.Context, scope listscope.TenantCommerce) (*appcommerceadmin.OrdersListResponse, error)
}

type ReconciliationAdminService interface {
	ListReconciliationCases(ctx context.Context, scope listscope.TenantCommerce) (*appcommerceadmin.ReconciliationListResponse, error)
	GetReconciliationCase(ctx context.Context, organizationID, caseID uuid.UUID) (appcommerceadmin.ReconciliationCaseItem, error)
	ResolveReconciliationCase(ctx context.Context, in appcommerceadmin.ResolveReconciliationInput) (appcommerceadmin.ReconciliationCaseItem, error)
	ListOrderTimeline(ctx context.Context, organizationID, orderID uuid.UUID, limit, offset int32) (*appcommerceadmin.OrderTimelineResponse, error)
	ListRefundRequests(ctx context.Context, scope listscope.TenantCommerce) (*appcommerceadmin.RefundRequestsListResponse, error)
	GetRefundRequest(ctx context.Context, organizationID, refundRequestID uuid.UUID) (appcommerceadmin.RefundRequestItem, error)
	CreateOrderRefund(ctx context.Context, in appcommerceadmin.CreateOrderRefundInput) (appcommerceadmin.CreateOrderRefundResult, error)
	RefundFromReconciliationCase(ctx context.Context, in appcommerceadmin.RefundFromReconciliationCaseInput) (appcommerceadmin.CreateOrderRefundResult, error)
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
	CashCollectionsExport(ctx context.Context, q listscope.ReportingQuery) ([]appreporting.CashCollectionExportRow, error)
	PaymentSettlement(ctx context.Context, q listscope.ReportingQuery) (*appreporting.PaymentSettlementResponse, error)
	Refunds(ctx context.Context, q listscope.ReportingQuery) (*appreporting.RefundReportResponse, error)
	CashCollectionsReport(ctx context.Context, q listscope.ReportingQuery) (*appreporting.CashCollectionReportResponse, error)
	MachineHealth(ctx context.Context, q listscope.ReportingQuery) (*appreporting.MachineHealthReportResponse, error)
	FailedVends(ctx context.Context, q listscope.ReportingQuery) (*appreporting.FailedVendReportResponse, error)
	ReconciliationQueue(ctx context.Context, q listscope.ReportingQuery) (*appreporting.ReconciliationQueueReportResponse, error)
	VendSummary(ctx context.Context, q listscope.ReportingQuery) (*appreporting.VendSummaryResponse, error)
	StockMovement(ctx context.Context, q listscope.ReportingQuery) (*appreporting.StockMovementReportResponse, error)
	CommandFailures(ctx context.Context, q listscope.ReportingQuery) (*appreporting.CommandFailuresReportResponse, error)
	ReconciliationBI(ctx context.Context, q listscope.ReportingQuery) (*appreporting.ReconciliationBIReportResponse, error)
	ProductPerformance(ctx context.Context, q listscope.ReportingQuery) (*appreporting.ProductPerformanceResponse, error)
	TechnicianFillOperations(ctx context.Context, q listscope.ReportingQuery) (*appreporting.TechnicianFillReportResponse, error)
}

// FinanceService manages immutable finance daily closes (tenant-scoped).
type FinanceService interface {
	CreateDailyClose(ctx context.Context, in appfinance.CreateDailyCloseInput) (*appfinance.DailyCloseView, error)
	GetDailyClose(ctx context.Context, organizationID, closeID uuid.UUID) (*appfinance.DailyCloseView, error)
	ListDailyClose(ctx context.Context, p appfinance.ListDailyCloseParams) (*appfinance.DailyCloseListResponse, error)
}
