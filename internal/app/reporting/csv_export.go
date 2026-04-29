package reporting

import (
	"encoding/csv"
	"io"
	"strconv"
	"time"
)

// WriteSalesSummaryCSV writes UTF-8 CSV with stable headers aligned to SalesSummary JSON dimensions.
func WriteSalesSummaryCSV(w io.Writer, resp *SalesSummaryResponse) error {
	cw := csv.NewWriter(w)
	cw.UseCRLF = false
	header := []string{
		"organization_id",
		"from",
		"to",
		"group_by",
		"row_type",
		"bucket_start",
		"site_id",
		"machine_id",
		"payment_provider",
		"product_id",
		"product_name",
		"product_sku",
		"order_count",
		"total_minor",
		"subtotal_minor",
		"tax_minor",
		"success_vends",
		"failed_vends",
		"gross_total_minor",
		"summary_order_count",
		"avg_order_value_minor",
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	summaryRow := []string{
		resp.OrganizationID,
		resp.From,
		resp.To,
		resp.GroupBy,
		"SUMMARY",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		strconv.FormatInt(resp.Summary.OrderCount, 10),
		strconv.FormatInt(resp.Summary.GrossTotalMinor, 10),
		strconv.FormatInt(resp.Summary.SubtotalMinor, 10),
		strconv.FormatInt(resp.Summary.TaxMinor, 10),
		"",
		"",
		strconv.FormatInt(resp.Summary.GrossTotalMinor, 10),
		strconv.FormatInt(resp.Summary.OrderCount, 10),
		strconv.FormatInt(resp.Summary.AvgOrderValueMinor, 10),
	}
	if err := cw.Write(summaryRow); err != nil {
		return err
	}
	for _, b := range resp.Breakdown {
		row := []string{
			resp.OrganizationID,
			resp.From,
			resp.To,
			resp.GroupBy,
			"BREAKDOWN",
			valOrEmpty(b.BucketStart),
			valOrEmpty(b.SiteID),
			valOrEmpty(b.MachineID),
			valOrEmpty(b.PaymentProvider),
			valOrEmpty(b.ProductID),
			valOrEmpty(b.ProductName),
			valOrEmpty(b.ProductSku),
			strconv.FormatInt(b.OrderCount, 10),
			strconv.FormatInt(b.TotalMinor, 10),
			strconv.FormatInt(b.SubtotalMinor, 10),
			strconv.FormatInt(b.TaxMinor, 10),
			strconv.FormatInt(b.SuccessVends, 10),
			strconv.FormatInt(b.FailedVends, 10),
			"",
			"",
			"",
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WritePaymentsSummaryCSV writes UTF-8 CSV aligned to PaymentsSummary JSON.
func WritePaymentsSummaryCSV(w io.Writer, resp *PaymentsSummaryResponse) error {
	cw := csv.NewWriter(w)
	cw.UseCRLF = false
	header := []string{
		"organization_id",
		"from",
		"to",
		"group_by",
		"row_type",
		"bucket_start",
		"provider",
		"state",
		"payment_count",
		"amount_minor",
		"authorized_count",
		"captured_count",
		"failed_count",
		"refunded_count",
		"captured_amount_minor",
		"authorized_amount_minor",
		"failed_amount_minor",
		"refunded_amount_minor",
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	s := resp.Summary
	sumRow := []string{
		resp.OrganizationID,
		resp.From,
		resp.To,
		resp.GroupBy,
		"SUMMARY",
		"",
		"",
		"",
		"",
		"",
		strconv.FormatInt(s.AuthorizedCount, 10),
		strconv.FormatInt(s.CapturedCount, 10),
		strconv.FormatInt(s.FailedCount, 10),
		strconv.FormatInt(s.RefundedCount, 10),
		strconv.FormatInt(s.CapturedAmountMinor, 10),
		strconv.FormatInt(s.AuthorizedAmountMinor, 10),
		strconv.FormatInt(s.FailedAmountMinor, 10),
		strconv.FormatInt(s.RefundedAmountMinor, 10),
	}
	if err := cw.Write(sumRow); err != nil {
		return err
	}
	for _, b := range resp.Breakdown {
		row := []string{
			resp.OrganizationID,
			resp.From,
			resp.To,
			resp.GroupBy,
			"BREAKDOWN",
			valOrEmpty(b.BucketStart),
			valOrEmpty(b.Provider),
			valOrEmpty(b.State),
			strconv.FormatInt(b.PaymentCount, 10),
			strconv.FormatInt(b.AmountMinor, 10),
			"", "", "", "", "", "", "", "",
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteCashCollectionsCSV writes UTF-8 CSV for cross-machine cash_collections in a reporting window.
func WriteCashCollectionsCSV(w io.Writer, organizationID, fromRFC3339, toRFC3339 string, rows []CashCollectionExportRow) error {
	cw := csv.NewWriter(w)
	cw.UseCRLF = false
	header := []string{
		"organization_id",
		"from",
		"to",
		"collection_id",
		"machine_id",
		"site_id",
		"site_name",
		"machine_serial_number",
		"collected_at",
		"opened_at",
		"closed_at",
		"lifecycle_status",
		"amount_minor",
		"expected_amount_minor",
		"variance_amount_minor",
		"currency",
		"reconciliation_status",
		"created_at",
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, r := range rows {
		row := []string{
			organizationID,
			fromRFC3339,
			toRFC3339,
			r.CollectionID,
			r.MachineID,
			r.SiteID,
			r.SiteName,
			r.MachineSerialNumber,
			formatRFC3339(r.CollectedAt),
			formatRFC3339(r.OpenedAt),
			formatRFC3339Ptr(r.ClosedAt),
			r.LifecycleStatus,
			strconv.FormatInt(r.AmountMinor, 10),
			strconv.FormatInt(r.ExpectedAmountMinor, 10),
			strconv.FormatInt(r.VarianceAmountMinor, 10),
			r.Currency,
			r.ReconciliationStatus,
			formatRFC3339(r.CreatedAt),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WritePaymentSettlementCSV writes provider settlement rows with stable headers.
func WritePaymentSettlementCSV(w io.Writer, resp *PaymentSettlementResponse) error {
	cw := csv.NewWriter(w)
	cw.UseCRLF = false
	header := []string{"organization_id", "from", "to", "timezone", "bucket_start", "provider", "state", "settlement_status", "reconciliation_status", "payment_count", "amount_minor"}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, r := range resp.Items {
		row := []string{
			resp.OrganizationID,
			resp.From,
			resp.To,
			resp.Timezone,
			r.BucketStart,
			r.Provider,
			r.State,
			r.SettlementStatus,
			r.ReconciliationStatus,
			strconv.FormatInt(r.PaymentCount, 10),
			strconv.FormatInt(r.AmountMinor, 10),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteRefundsCSV writes refund report rows with stable headers.
func WriteRefundsCSV(w io.Writer, resp *RefundReportResponse) error {
	cw := csv.NewWriter(w)
	cw.UseCRLF = false
	header := []string{"organization_id", "from", "to", "refund_id", "payment_id", "order_id", "machine_id", "amount_minor", "currency", "state", "reason", "reconciliation_status", "settlement_status", "created_at"}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, r := range resp.Items {
		row := []string{
			resp.OrganizationID,
			resp.From,
			resp.To,
			r.RefundID,
			r.PaymentID,
			r.OrderID,
			r.MachineID,
			strconv.FormatInt(r.AmountMinor, 10),
			r.Currency,
			r.State,
			valOrEmpty(r.Reason),
			r.ReconciliationStatus,
			r.SettlementStatus,
			r.CreatedAt,
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteInventoryExceptionsCSV exports slot-level inventory exceptions (no payment data).
func WriteInventoryExceptionsCSV(w io.Writer, resp *InventoryExceptionsResponse) error {
	cw := csv.NewWriter(w)
	cw.UseCRLF = false
	header := []string{
		"organization_id", "from", "to", "machine_id", "machine_name", "machine_serial_number",
		"machine_status", "planogram_id", "planogram_name", "slot_index", "current_quantity",
		"max_quantity", "product_id", "product_sku", "product_name", "out_of_stock", "low_stock", "attention_needed",
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, r := range resp.Items {
		row := []string{
			resp.OrganizationID,
			resp.From,
			resp.To,
			r.MachineID,
			r.MachineName,
			r.MachineSerialNumber,
			r.MachineStatus,
			r.PlanogramID,
			r.PlanogramName,
			strconv.FormatInt(int64(r.SlotIndex), 10),
			strconv.FormatInt(int64(r.CurrentQuantity), 10),
			strconv.FormatInt(int64(r.MaxQuantity), 10),
			valOrEmpty(r.ProductID),
			valOrEmpty(r.ProductSku),
			valOrEmpty(r.ProductName),
			strconv.FormatBool(r.OutOfStock),
			strconv.FormatBool(r.LowStock),
			strconv.FormatBool(r.AttentionNeeded),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteMachineHealthCSV exports machine uptime / last-seen rows (no credentials).
func WriteMachineHealthCSV(w io.Writer, resp *MachineHealthReportResponse) error {
	cw := csv.NewWriter(w)
	cw.UseCRLF = false
	header := []string{"organization_id", "from", "to", "machine_id", "site_id", "site_name", "serial_number", "machine_name", "status", "last_seen_at", "offline"}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, r := range resp.Items {
		row := []string{
			resp.OrganizationID,
			resp.From,
			resp.To,
			r.MachineID,
			r.SiteID,
			r.SiteName,
			r.SerialNumber,
			r.MachineName,
			r.Status,
			valOrEmpty(r.LastSeenAt),
			strconv.FormatBool(r.Offline),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteStockMovementCSV exports inventory_events lines (operational telemetry, not card data).
func WriteStockMovementCSV(w io.Writer, resp *StockMovementReportResponse) error {
	cw := csv.NewWriter(w)
	cw.UseCRLF = false
	header := []string{
		"organization_id", "from", "to", "inventory_event_id", "machine_id", "site_id",
		"product_id", "product_sku", "product_name", "event_type", "slot_code",
		"quantity_delta", "quantity_before", "quantity_after", "occurred_at",
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, r := range resp.Items {
		row := []string{
			resp.OrganizationID,
			resp.From,
			resp.To,
			r.InventoryEventID,
			r.MachineID,
			r.SiteID,
			valOrEmpty(r.ProductID),
			r.ProductSku,
			r.ProductName,
			r.EventType,
			valOrEmpty(r.SlotCode),
			strconv.FormatInt(int64(r.QuantityDelta), 10),
			formatInt32Ptr(r.QuantityBefore),
			formatInt32Ptr(r.QuantityAfter),
			r.OccurredAt,
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteTechnicianFillOpsCSV exports fill/restock operations (ids + display name only; no email/phone/PAN).
func WriteTechnicianFillOpsCSV(w io.Writer, resp *TechnicianFillReportResponse) error {
	cw := csv.NewWriter(w)
	cw.UseCRLF = false
	header := []string{
		"organization_id", "from", "to",
		"inventory_event_id", "machine_id", "site_id",
		"product_id", "product_sku", "product_name", "event_type", "slot_code",
		"quantity_delta", "quantity_before", "quantity_after",
		"operator_session_id", "technician_id", "technician_display_name", "refill_session_id",
		"occurred_at",
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, r := range resp.Items {
		row := []string{
			resp.OrganizationID,
			resp.From,
			resp.To,
			r.InventoryEventID,
			r.MachineID,
			r.SiteID,
			valOrEmpty(r.ProductID),
			r.ProductSku,
			r.ProductName,
			r.EventType,
			valOrEmpty(r.SlotCode),
			strconv.FormatInt(int64(r.QuantityDelta), 10),
			formatInt32Ptr(r.QuantityBefore),
			formatInt32Ptr(r.QuantityAfter),
			valOrEmpty(r.OperatorSessionID),
			valOrEmpty(r.TechnicianID),
			r.TechnicianDisplayName,
			valOrEmpty(r.RefillSessionID),
			r.OccurredAt,
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func formatInt32Ptr(p *int32) string {
	if p == nil {
		return ""
	}
	return strconv.FormatInt(int64(*p), 10)
}

// WriteProductPerformanceCSV exports product vend/revenue aggregates (allocated revenue split).
func WriteProductPerformanceCSV(w io.Writer, resp *ProductPerformanceResponse) error {
	cw := csv.NewWriter(w)
	cw.UseCRLF = false
	header := []string{"organization_id", "from", "to", "product_id", "product_sku", "product_name", "success_vends", "failed_vends", "allocated_revenue_minor"}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, r := range resp.Items {
		row := []string{
			resp.OrganizationID,
			resp.From,
			resp.To,
			r.ProductID,
			r.ProductSku,
			r.ProductName,
			strconv.FormatInt(r.SuccessVends, 10),
			strconv.FormatInt(r.FailedVends, 10),
			strconv.FormatInt(r.AllocatedRevenueMinor, 10),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteReconciliationBICSV exports reconciliation case metadata (IDs only, no PAN or provider secrets).
func WriteReconciliationBICSV(w io.Writer, resp *ReconciliationBIReportResponse) error {
	cw := csv.NewWriter(w)
	cw.UseCRLF = false
	header := []string{
		"organization_id", "from", "to", "scope", "open_cases", "closed_cases",
		"id", "case_type", "status", "severity",
		"order_id", "payment_id", "vend_session_id", "refund_id", "provider",
		"reason", "first_detected_at", "last_detected_at", "resolved_at",
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	sumRow := []string{
		resp.OrganizationID,
		resp.From,
		resp.To,
		resp.Scope,
		strconv.FormatInt(resp.Summary.OpenCases, 10),
		strconv.FormatInt(resp.Summary.ClosedCases, 10),
		"", "", "", "", "", "", "", "", "", "", "", "", "",
	}
	if err := cw.Write(sumRow); err != nil {
		return err
	}
	for _, r := range resp.Items {
		row := []string{
			resp.OrganizationID,
			resp.From,
			resp.To,
			resp.Scope,
			"", "",
			r.ID,
			r.CaseType,
			r.Status,
			r.Severity,
			valOrEmpty(r.OrderID),
			valOrEmpty(r.PaymentID),
			valOrEmpty(r.VendSessionID),
			valOrEmpty(r.RefundID),
			valOrEmpty(r.Provider),
			r.Reason,
			r.FirstDetectedAt,
			r.LastDetectedAt,
			valOrEmpty(r.ResolvedAt),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteCommandFailuresCSV exports terminal command attempt outcomes (no raw payloads).
func WriteCommandFailuresCSV(w io.Writer, resp *CommandFailuresReportResponse) error {
	cw := csv.NewWriter(w)
	cw.UseCRLF = false
	header := []string{"organization_id", "from", "to", "attempt_id", "command_id", "machine_id", "site_id", "attempt_no", "sent_at", "status", "timeout_reason"}
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, r := range resp.Items {
		row := []string{
			resp.OrganizationID,
			resp.From,
			resp.To,
			r.AttemptID,
			r.CommandID,
			r.MachineID,
			r.SiteID,
			strconv.FormatInt(int64(r.AttemptNo), 10),
			r.SentAt,
			r.Status,
			r.TimeoutReason,
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteVendSummaryCSV exports vend totals plus failed vend rows (no payment instrument fields).
func WriteVendSummaryCSV(w io.Writer, resp *VendSummaryResponse) error {
	cw := csv.NewWriter(w)
	cw.UseCRLF = false
	header := []string{
		"organization_id", "from", "to", "row_type",
		"success_count", "failed_count", "in_progress_count",
		"vend_session_id", "order_id", "machine_id", "slot_index", "product_id",
		"failure_reason", "total_minor", "currency", "order_status", "created_at",
	}
	if err := cw.Write(header); err != nil {
		return err
	}
	sum := []string{
		resp.OrganizationID,
		resp.From,
		resp.To,
		"SUMMARY",
		strconv.FormatInt(resp.Summary.SuccessCount, 10),
		strconv.FormatInt(resp.Summary.FailedCount, 10),
		strconv.FormatInt(resp.Summary.InProgressCount, 10),
		"", "", "", "", "", "", "", "", "", "",
	}
	if err := cw.Write(sum); err != nil {
		return err
	}
	for _, r := range resp.FailedVends.Items {
		row := []string{
			resp.OrganizationID,
			resp.From,
			resp.To,
			"FAILED_VEND",
			"", "", "",
			r.VendSessionID,
			r.OrderID,
			r.MachineID,
			strconv.FormatInt(int64(r.SlotIndex), 10),
			r.ProductID,
			valOrEmpty(r.FailureReason),
			strconv.FormatInt(r.TotalMinor, 10),
			r.Currency,
			r.OrderStatus,
			r.CreatedAt,
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// CashCollectionExportRow is one exported cash_collections row with machine/site labels.
type CashCollectionExportRow struct {
	CollectionID         string
	MachineID            string
	SiteID               string
	SiteName             string
	MachineSerialNumber  string
	CollectedAt          time.Time
	OpenedAt             time.Time
	ClosedAt             *time.Time
	LifecycleStatus      string
	AmountMinor          int64
	ExpectedAmountMinor  int64
	VarianceAmountMinor  int64
	Currency             string
	ReconciliationStatus string
	CreatedAt            time.Time
}

func valOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func formatRFC3339(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func formatRFC3339Ptr(ts *time.Time) string {
	if ts == nil {
		return ""
	}
	return ts.UTC().Format(time.RFC3339Nano)
}
