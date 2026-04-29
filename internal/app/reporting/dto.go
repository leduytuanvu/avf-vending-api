package reporting

// SalesSummaryResponse is GET /v1/reports/sales-summary.
type SalesSummaryResponse struct {
	OrganizationID string              `json:"organizationId"`
	From           string              `json:"from"`
	To             string              `json:"to"`
	GroupBy        string              `json:"groupBy"`
	Summary        SalesSummaryRollup  `json:"summary"`
	Breakdown      []SalesBreakdownRow `json:"breakdown"`
}

// SalesSummaryRollup aggregates orders in the reporting window.
type SalesSummaryRollup struct {
	GrossTotalMinor    int64 `json:"grossTotalMinor"`
	SubtotalMinor      int64 `json:"subtotalMinor"`
	TaxMinor           int64 `json:"taxMinor"`
	OrderCount         int64 `json:"orderCount"`
	AvgOrderValueMinor int64 `json:"avgOrderValueMinor"`
}

// SalesBreakdownRow is one grouping row; only the dimension fields matching groupBy are set.
type SalesBreakdownRow struct {
	BucketStart     *string `json:"bucketStart,omitempty"`
	SiteID          *string `json:"siteId,omitempty"`
	MachineID       *string `json:"machineId,omitempty"`
	PaymentProvider *string `json:"paymentProvider,omitempty"`
	ProductID       *string `json:"productId,omitempty"`
	ProductSku      *string `json:"productSku,omitempty"`
	ProductName     *string `json:"productName,omitempty"`
	OrderCount      int64   `json:"orderCount"`
	TotalMinor      int64   `json:"totalMinor"`
	SubtotalMinor   int64   `json:"subtotalMinor"`
	TaxMinor        int64   `json:"taxMinor"`
	SuccessVends    int64   `json:"successVends,omitempty"`
	FailedVends     int64   `json:"failedVends,omitempty"`
}

// PaymentsSummaryResponse is GET /v1/reports/payments-summary.
type PaymentsSummaryResponse struct {
	OrganizationID string                 `json:"organizationId"`
	From           string                 `json:"from"`
	To             string                 `json:"to"`
	GroupBy        string                 `json:"groupBy"`
	Summary        PaymentsSummaryRollup  `json:"summary"`
	Breakdown      []PaymentsBreakdownRow `json:"breakdown"`
}

// PaymentsSummaryRollup aggregates payment rows joined to orders in the window.
type PaymentsSummaryRollup struct {
	AuthorizedCount       int64 `json:"authorizedCount"`
	CapturedCount         int64 `json:"capturedCount"`
	FailedCount           int64 `json:"failedCount"`
	RefundedCount         int64 `json:"refundedCount"`
	CapturedAmountMinor   int64 `json:"capturedAmountMinor"`
	AuthorizedAmountMinor int64 `json:"authorizedAmountMinor"`
	FailedAmountMinor     int64 `json:"failedAmountMinor"`
	RefundedAmountMinor   int64 `json:"refundedAmountMinor"`
}

// PaymentsBreakdownRow supports day, payment_method (provider), or status grouping.
type PaymentsBreakdownRow struct {
	BucketStart  *string `json:"bucketStart,omitempty"`
	Provider     *string `json:"provider,omitempty"`
	State        *string `json:"state,omitempty"`
	PaymentCount int64   `json:"paymentCount"`
	AmountMinor  int64   `json:"amountMinor"`
}

// FleetHealthResponse is GET /v1/reports/fleet-health.
type FleetHealthResponse struct {
	OrganizationID             string                    `json:"organizationId"`
	From                       string                    `json:"from"`
	To                         string                    `json:"to"`
	MachineSummary             FleetMachineHealthSummary `json:"machineSummary"`
	MachinesByStatus           []FleetStatusCountRow     `json:"machinesByStatus"`
	IncidentsByStatus          []FleetStatusCountRow     `json:"incidentsByStatus"`
	MachineIncidentsBySeverity []FleetSeverityCountRow   `json:"machineIncidentsBySeverity"`
}

// FleetMachineHealthSummary maps machine.status into operational buckets for dashboards.
type FleetMachineHealthSummary struct {
	Total   int64 `json:"total"`
	Online  int64 `json:"online"`
	Offline int64 `json:"offline"`
	Fault   int64 `json:"fault"`
	Warn    int64 `json:"warn"`
	Retired int64 `json:"retired"`
}

// FleetStatusCountRow is a status label with a count (machines or incidents table).
type FleetStatusCountRow struct {
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

// FleetSeverityCountRow rolls up telemetry machine_incidents by severity.
type FleetSeverityCountRow struct {
	Severity string `json:"severity"`
	Count    int64  `json:"count"`
}

// InventoryExceptionsResponse is GET /v1/reports/inventory-exceptions.
type InventoryExceptionsResponse struct {
	OrganizationID string                      `json:"organizationId"`
	From           string                      `json:"from"`
	To             string                      `json:"to"`
	ExceptionKind  string                      `json:"exceptionKind"`
	Meta           InventoryExceptionsListMeta `json:"meta"`
	Items          []InventoryExceptionItem    `json:"items"`
}

// InventoryExceptionsListMeta paginates exception rows (cap enforced server-side).
type InventoryExceptionsListMeta struct {
	Limit    int32 `json:"limit"`
	Offset   int32 `json:"offset"`
	Returned int   `json:"returned"`
	Total    int64 `json:"total"`
}

// InventoryExceptionItem is a slot-level projection needing refill or restock attention.
type InventoryExceptionItem struct {
	MachineID           string  `json:"machineId"`
	MachineName         string  `json:"machineName"`
	MachineSerialNumber string  `json:"machineSerialNumber"`
	MachineStatus       string  `json:"machineStatus"`
	PlanogramID         string  `json:"planogramId"`
	PlanogramName       string  `json:"planogramName"`
	SlotIndex           int32   `json:"slotIndex"`
	CurrentQuantity     int32   `json:"currentQuantity"`
	MaxQuantity         int32   `json:"maxQuantity"`
	ProductID           *string `json:"productId,omitempty"`
	ProductSku          *string `json:"productSku,omitempty"`
	ProductName         *string `json:"productName,omitempty"`
	OutOfStock          bool    `json:"outOfStock"`
	LowStock            bool    `json:"lowStock"`
	AttentionNeeded     bool    `json:"attentionNeeded"`
}

// PaymentSettlementResponse groups provider settlement state by business date.
type PaymentSettlementResponse struct {
	OrganizationID string                      `json:"organizationId"`
	From           string                      `json:"from"`
	To             string                      `json:"to"`
	Timezone       string                      `json:"timezone"`
	Items          []PaymentSettlementRow      `json:"items"`
	Meta           InventoryExceptionsListMeta `json:"meta"`
}

type PaymentSettlementRow struct {
	BucketStart          string `json:"bucketStart"`
	Provider             string `json:"provider"`
	State                string `json:"state"`
	SettlementStatus     string `json:"settlementStatus"`
	ReconciliationStatus string `json:"reconciliationStatus"`
	PaymentCount         int64  `json:"paymentCount"`
	AmountMinor          int64  `json:"amountMinor"`
}

type RefundReportResponse struct {
	OrganizationID string                      `json:"organizationId"`
	From           string                      `json:"from"`
	To             string                      `json:"to"`
	Meta           InventoryExceptionsListMeta `json:"meta"`
	Items          []RefundReportItem          `json:"items"`
}

type RefundReportItem struct {
	RefundID             string  `json:"refundId"`
	PaymentID            string  `json:"paymentId"`
	OrderID              string  `json:"orderId"`
	MachineID            string  `json:"machineId"`
	AmountMinor          int64   `json:"amountMinor"`
	Currency             string  `json:"currency"`
	State                string  `json:"state"`
	Reason               *string `json:"reason,omitempty"`
	ReconciliationStatus string  `json:"reconciliationStatus"`
	SettlementStatus     string  `json:"settlementStatus"`
	CreatedAt            string  `json:"createdAt"`
}

type CashCollectionReportResponse struct {
	OrganizationID string                      `json:"organizationId"`
	From           string                      `json:"from"`
	To             string                      `json:"to"`
	Meta           InventoryExceptionsListMeta `json:"meta"`
	Items          []CashCollectionExportRow   `json:"items"`
}

type MachineHealthReportResponse struct {
	OrganizationID string                      `json:"organizationId"`
	From           string                      `json:"from"`
	To             string                      `json:"to"`
	Meta           InventoryExceptionsListMeta `json:"meta"`
	Items          []MachineHealthReportItem   `json:"items"`
}

type MachineHealthReportItem struct {
	MachineID    string  `json:"machineId"`
	SiteID       string  `json:"siteId"`
	SiteName     string  `json:"siteName"`
	SerialNumber string  `json:"serialNumber"`
	MachineName  string  `json:"machineName"`
	Status       string  `json:"status"`
	LastSeenAt   *string `json:"lastSeenAt,omitempty"`
	Offline      bool    `json:"offline"`
}

type FailedVendReportResponse struct {
	OrganizationID string                      `json:"organizationId"`
	From           string                      `json:"from"`
	To             string                      `json:"to"`
	Meta           InventoryExceptionsListMeta `json:"meta"`
	Items          []FailedVendReportItem      `json:"items"`
}

type FailedVendReportItem struct {
	VendSessionID string  `json:"vendSessionId"`
	OrderID       string  `json:"orderId"`
	MachineID     string  `json:"machineId"`
	SlotIndex     int32   `json:"slotIndex"`
	ProductID     string  `json:"productId"`
	FailureReason *string `json:"failureReason,omitempty"`
	StartedAt     *string `json:"startedAt,omitempty"`
	CompletedAt   *string `json:"completedAt,omitempty"`
	CreatedAt     string  `json:"createdAt"`
	TotalMinor    int64   `json:"totalMinor"`
	Currency      string  `json:"currency"`
	OrderStatus   string  `json:"orderStatus"`
}

type ReconciliationQueueReportResponse struct {
	OrganizationID string                      `json:"organizationId"`
	From           string                      `json:"from"`
	To             string                      `json:"to"`
	Meta           InventoryExceptionsListMeta `json:"meta"`
	Items          []ReconciliationQueueItem   `json:"items"`
}

// VendSummaryResponse aggregates vend lifecycle counts plus paginated failed vend rows (same window/filters).
type VendSummaryResponse struct {
	OrganizationID string                   `json:"organizationId"`
	From           string                   `json:"from"`
	To             string                   `json:"to"`
	Summary        VendCountsSummary        `json:"summary"`
	FailedVends    VendFailedVendsSubreport `json:"failedVends"`
}

type VendCountsSummary struct {
	SuccessCount    int64 `json:"successCount"`
	FailedCount     int64 `json:"failedCount"`
	InProgressCount int64 `json:"inProgressCount"`
}

// VendFailedVendsSubreport paginates failed vend drill-down alongside totals.
type VendFailedVendsSubreport struct {
	Meta  InventoryExceptionsListMeta `json:"meta"`
	Items []FailedVendReportItem      `json:"items"`
}

// StockMovementReportResponse lists inventory_events rows for BI movement tracing.
type StockMovementReportResponse struct {
	OrganizationID string                      `json:"organizationId"`
	From           string                      `json:"from"`
	To             string                      `json:"to"`
	Meta           InventoryExceptionsListMeta `json:"meta"`
	Items          []StockMovementRow          `json:"items"`
}

type StockMovementRow struct {
	InventoryEventID string  `json:"inventoryEventId"`
	MachineID        string  `json:"machineId"`
	SiteID           string  `json:"siteId"`
	ProductID        *string `json:"productId,omitempty"`
	ProductSku       string  `json:"productSku"`
	ProductName      string  `json:"productName"`
	EventType        string  `json:"eventType"`
	SlotCode         *string `json:"slotCode,omitempty"`
	QuantityDelta    int32   `json:"quantityDelta"`
	QuantityBefore   *int32  `json:"quantityBefore,omitempty"`
	QuantityAfter    *int32  `json:"quantityAfter,omitempty"`
	OccurredAt       string  `json:"occurredAt"`
}

// TechnicianFillReportResponse lists technician / refill / operator-attributed inventory operations (no payment data).
type TechnicianFillReportResponse struct {
	OrganizationID string                      `json:"organizationId"`
	From           string                      `json:"from"`
	To             string                      `json:"to"`
	Meta           InventoryExceptionsListMeta `json:"meta"`
	Items          []TechnicianFillOpRow       `json:"items"`
}

// TechnicianFillOpRow is one inventory_events row for operational fill / restock reporting.
type TechnicianFillOpRow struct {
	InventoryEventID      string  `json:"inventoryEventId"`
	MachineID             string  `json:"machineId"`
	SiteID                string  `json:"siteId"`
	ProductID             *string `json:"productId,omitempty"`
	ProductSku            string  `json:"productSku"`
	ProductName           string  `json:"productName"`
	EventType             string  `json:"eventType"`
	SlotCode              *string `json:"slotCode,omitempty"`
	QuantityDelta         int32   `json:"quantityDelta"`
	QuantityBefore        *int32  `json:"quantityBefore,omitempty"`
	QuantityAfter         *int32  `json:"quantityAfter,omitempty"`
	OperatorSessionID     *string `json:"operatorSessionId,omitempty"`
	TechnicianID          *string `json:"technicianId,omitempty"`
	TechnicianDisplayName string  `json:"technicianDisplayName,omitempty"`
	RefillSessionID       *string `json:"refillSessionId,omitempty"`
	OccurredAt            string  `json:"occurredAt"`
}

// CommandFailuresReportResponse lists terminal unsuccessful machine command attempts (no raw payloads).
type CommandFailuresReportResponse struct {
	OrganizationID string                      `json:"organizationId"`
	From           string                      `json:"from"`
	To             string                      `json:"to"`
	Meta           InventoryExceptionsListMeta `json:"meta"`
	Items          []CommandFailureRow         `json:"items"`
}

type CommandFailureRow struct {
	AttemptID     string `json:"attemptId"`
	CommandID     string `json:"commandId"`
	MachineID     string `json:"machineId"`
	SiteID        string `json:"siteId"`
	AttemptNo     int32  `json:"attemptNo"`
	SentAt        string `json:"sentAt"`
	Status        string `json:"status"`
	TimeoutReason string `json:"timeoutReason,omitempty"`
}

// ReconciliationBIReportResponse is admin reconciliation BI (open + closed summaries + scoped cases).
type ReconciliationBIReportResponse struct {
	OrganizationID string                      `json:"organizationId"`
	From           string                      `json:"from"`
	To             string                      `json:"to"`
	Scope          string                      `json:"scope"`
	Summary        ReconciliationBISummary     `json:"summary"`
	Meta           InventoryExceptionsListMeta `json:"meta"`
	Items          []ReconciliationBIItem      `json:"items"`
}

type ReconciliationBISummary struct {
	OpenCases   int64 `json:"openCases"`
	ClosedCases int64 `json:"closedCases"`
}

type ReconciliationBIItem struct {
	ID              string  `json:"id"`
	CaseType        string  `json:"caseType"`
	Status          string  `json:"status"`
	Severity        string  `json:"severity"`
	OrderID         *string `json:"orderId,omitempty"`
	PaymentID       *string `json:"paymentId,omitempty"`
	VendSessionID   *string `json:"vendSessionId,omitempty"`
	RefundID        *string `json:"refundId,omitempty"`
	Provider        *string `json:"provider,omitempty"`
	Reason          string  `json:"reason"`
	FirstDetectedAt string  `json:"firstDetectedAt"`
	LastDetectedAt  string  `json:"lastDetectedAt"`
	ResolvedAt      *string `json:"resolvedAt,omitempty"`
}

// ProductPerformanceResponse is vend and revenue attribution by product (admin /reports/products).
type ProductPerformanceResponse struct {
	OrganizationID string                  `json:"organizationId"`
	From           string                  `json:"from"`
	To             string                  `json:"to"`
	Items          []ProductPerformanceRow `json:"items"`
}

type ProductPerformanceRow struct {
	ProductID             string `json:"productId"`
	ProductSku            string `json:"productSku"`
	ProductName           string `json:"productName"`
	SuccessVends          int64  `json:"successVends"`
	FailedVends           int64  `json:"failedVends"`
	AllocatedRevenueMinor int64  `json:"allocatedRevenueMinor"`
}

type ReconciliationQueueItem struct {
	ID              string  `json:"id"`
	CaseType        string  `json:"caseType"`
	Status          string  `json:"status"`
	Severity        string  `json:"severity"`
	OrderID         *string `json:"orderId,omitempty"`
	PaymentID       *string `json:"paymentId,omitempty"`
	VendSessionID   *string `json:"vendSessionId,omitempty"`
	RefundID        *string `json:"refundId,omitempty"`
	Provider        *string `json:"provider,omitempty"`
	Reason          string  `json:"reason"`
	FirstDetectedAt string  `json:"firstDetectedAt"`
	LastDetectedAt  string  `json:"lastDetectedAt"`
}
