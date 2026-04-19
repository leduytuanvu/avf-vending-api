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
	BucketStart       *string `json:"bucketStart,omitempty"`
	SiteID            *string `json:"siteId,omitempty"`
	MachineID         *string `json:"machineId,omitempty"`
	PaymentProvider   *string `json:"paymentProvider,omitempty"`
	OrderCount        int64   `json:"orderCount"`
	TotalMinor        int64   `json:"totalMinor"`
	SubtotalMinor     int64   `json:"subtotalMinor"`
	TaxMinor          int64   `json:"taxMinor"`
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
	AuthorizedCount        int64 `json:"authorizedCount"`
	CapturedCount          int64 `json:"capturedCount"`
	FailedCount            int64 `json:"failedCount"`
	RefundedCount          int64 `json:"refundedCount"`
	CapturedAmountMinor    int64 `json:"capturedAmountMinor"`
	AuthorizedAmountMinor int64 `json:"authorizedAmountMinor"`
	FailedAmountMinor      int64 `json:"failedAmountMinor"`
	RefundedAmountMinor    int64 `json:"refundedAmountMinor"`
}

// PaymentsBreakdownRow supports day, payment_method (provider), or status grouping.
type PaymentsBreakdownRow struct {
	BucketStart   *string `json:"bucketStart,omitempty"`
	Provider      *string `json:"provider,omitempty"`
	State         *string `json:"state,omitempty"`
	PaymentCount  int64   `json:"paymentCount"`
	AmountMinor   int64   `json:"amountMinor"`
}

// FleetHealthResponse is GET /v1/reports/fleet-health.
type FleetHealthResponse struct {
	OrganizationID              string                        `json:"organizationId"`
	From                        string                        `json:"from"`
	To                          string                        `json:"to"`
	MachineSummary              FleetMachineHealthSummary     `json:"machineSummary"`
	MachinesByStatus            []FleetStatusCountRow         `json:"machinesByStatus"`
	IncidentsByStatus           []FleetStatusCountRow         `json:"incidentsByStatus"`
	MachineIncidentsBySeverity  []FleetSeverityCountRow       `json:"machineIncidentsBySeverity"`
}

// FleetMachineHealthSummary maps machine.status into operational buckets for dashboards.
type FleetMachineHealthSummary struct {
	Total    int64 `json:"total"`
	Online   int64 `json:"online"`
	Offline  int64 `json:"offline"`
	Fault    int64 `json:"fault"`
	Warn     int64 `json:"warn"`
	Retired  int64 `json:"retired"`
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
