package listscope

import (
	"time"

	"github.com/google/uuid"
)

// ReportingQuery carries validated parameters for read-only GET /v1/reports/* analytics.
type ReportingQuery struct {
	IsPlatformAdmin bool
	OrganizationID  uuid.UUID
	From            time.Time
	To              time.Time
	Timezone        string
	GroupBy         string
	ExceptionKind   string
	Limit           int32
	Offset          int32
	// SiteIDFilter and MachineIDFilter narrow cash-collection exports (unset = no extra filter).
	SiteIDFilter    uuid.UUID
	MachineIDFilter uuid.UUID
	// ProductIDFilter narrows analytics where applicable (unset = uuid.Nil sentinel).
	ProductIDFilter uuid.UUID
	// ReconciliationScope selects reconciliation cases: open | closed | all (admin reconciliation BI report).
	ReconciliationScope string
	// InventoryReportKind selects GET .../reports/inventory shape: low_stock | movement.
	InventoryReportKind string
}
