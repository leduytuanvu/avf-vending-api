package grpcserver

import (
	"time"

	"github.com/avf/avf-vending-api/internal/app/activation"
	"github.com/avf/avf-vending-api/internal/app/api"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/featureflags"
	appinventory "github.com/avf/avf-vending-api/internal/app/inventoryapp"
	"github.com/avf/avf-vending-api/internal/app/machineruntime"
	appoperator "github.com/avf/avf-vending-api/internal/app/operator"
	"github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/platform/objectstore"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MachinePaymentRuntimeSource resolves machine-facing payment method config for bootstrap and commerce gates.
type MachinePaymentRuntimeSource interface {
	ResolveMachinePaymentMethodsForMachine(cfg *config.Config, featureFlags map[string]bool) platformpayments.MachinePaymentMethodsView
}

// MachineGRPCServicesDeps wires machine-facing gRPC services without importing cmd/bootstrap.
type MachineGRPCServicesDeps struct {
	Activation      *activation.Service
	MachineRuntime  *machineruntime.Service
	MachineQueries  api.InternalMachineQueryService
	FeatureFlags    *featureflags.Service
	SaleCatalog     salecatalog.SnapshotBuilder
	Pool            *pgxpool.Pool
	MQTTBrokerURL   string
	MQTTTopicPrefix string
	Config          *config.Config
	// InventoryLedger writes machine-originated stock movements (required when machine inventory gRPC is registered).
	InventoryLedger appinventory.LedgerRepository
	// EnterpriseAudit is optional; when nil, inventory/operator RPCs skip audit_events writes.
	EnterpriseAudit compliance.EnterpriseRecorder
	// Operator coordinates operator sessions; optional — Heartbeat returns Unavailable when nil.
	Operator *appoperator.Service
	// Commerce orchestrates orders/payments/vends for machine checkout gRPC.
	Commerce appcommerce.Orchestrator
	// PaymentRuntime resolves machine payment method config for bootstrap and commerce gates.
	PaymentRuntime MachinePaymentRuntimeSource
	// TelemetryStore applies vend-success inventory decrements (same Store as HTTP TelemetryStore).
	TelemetryStore *postgres.Store
	// MediaStore issues fresh presigned HTTPS URLs for catalog/media manifests when wired (nil keeps DB URLs).
	MediaStore objectstore.Store
	// MediaPresignTTL is the presigned GET lifetime for MediaStore refresh (typically cfg.Artifacts.DownloadPresignTTL).
	MediaPresignTTL time.Duration
}
