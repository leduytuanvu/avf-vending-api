package reporting

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteSalesSummaryCSV_StableHeaders(t *testing.T) {
	resp := &SalesSummaryResponse{
		OrganizationID: "11111111-1111-1111-1111-111111111111",
		From:           "2026-01-01T00:00:00Z",
		To:             "2026-01-02T00:00:00Z",
		GroupBy:        "none",
		Summary: SalesSummaryRollup{
			GrossTotalMinor:    100,
			SubtotalMinor:      90,
			TaxMinor:           10,
			OrderCount:         2,
			AvgOrderValueMinor: 50,
		},
	}
	var buf bytes.Buffer
	if err := WriteSalesSummaryCSV(&buf, resp); err != nil {
		t.Fatal(err)
	}
	first := strings.Split(buf.String(), "\n")[0]
	wantPrefix := "organization_id,from,to,group_by,row_type,"
	if !strings.HasPrefix(first, wantPrefix) {
		t.Fatalf("header prefix: %q", first)
	}
}

func TestWritePaymentSettlementCSV_StableHeaders(t *testing.T) {
	resp := &PaymentSettlementResponse{
		OrganizationID: "11111111-1111-1111-1111-111111111111",
		From:           "2026-01-01T00:00:00Z",
		To:             "2026-01-02T00:00:00Z",
		Timezone:       "UTC",
	}
	var buf bytes.Buffer
	if err := WritePaymentSettlementCSV(&buf, resp); err != nil {
		t.Fatal(err)
	}
	first := strings.Split(buf.String(), "\n")[0]
	want := "organization_id,from,to,timezone,bucket_start,provider,state,settlement_status,reconciliation_status,payment_count,amount_minor"
	if first != want {
		t.Fatalf("header: %q", first)
	}
}

func TestWritePaymentSettlementCSV_NoSensitivePaymentFields(t *testing.T) {
	resp := &PaymentSettlementResponse{
		OrganizationID: "11111111-1111-1111-1111-111111111111",
		From:           "2026-01-01T00:00:00Z",
		To:             "2026-01-02T00:00:00Z",
		Timezone:       "UTC",
		Items: []PaymentSettlementRow{
			{
				BucketStart:          "2026-01-01T12:00:00Z",
				Provider:             "stripe",
				State:                "captured",
				SettlementStatus:     "settled",
				ReconciliationStatus: "matched",
				PaymentCount:         1,
				AmountMinor:          100,
			},
		},
	}
	var buf bytes.Buffer
	if err := WritePaymentSettlementCSV(&buf, resp); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, forbidden := range []string{"pan", "cvv", "track", "card_number"} {
		if strings.Contains(strings.ToLower(out), forbidden) {
			t.Fatalf("unexpected token %q in settlement CSV", forbidden)
		}
	}
}

func TestWriteRefundsCSV_StableHeaders(t *testing.T) {
	resp := &RefundReportResponse{
		OrganizationID: "11111111-1111-1111-1111-111111111111",
		From:           "2026-01-01T00:00:00Z",
		To:             "2026-01-02T00:00:00Z",
	}
	var buf bytes.Buffer
	if err := WriteRefundsCSV(&buf, resp); err != nil {
		t.Fatal(err)
	}
	first := strings.Split(buf.String(), "\n")[0]
	want := "organization_id,from,to,refund_id,payment_id,order_id,machine_id,amount_minor,currency,state,reason,reconciliation_status,settlement_status,created_at"
	if first != want {
		t.Fatalf("header: %q", first)
	}
}

func TestWriteTechnicianFillOpsCSV_NoTechnicianEmailOrPhoneColumns(t *testing.T) {
	tid := "22222222-2222-2222-2222-222222222222"
	pid := "33333333-3333-3333-3333-333333333333"
	resp := &TechnicianFillReportResponse{
		OrganizationID: "11111111-1111-1111-1111-111111111111",
		From:           "2026-01-01T00:00:00Z",
		To:             "2026-01-02T00:00:00Z",
		Items: []TechnicianFillOpRow{
			{
				InventoryEventID:      "42",
				MachineID:             "44444444-4444-4444-4444-444444444444",
				SiteID:                "55555555-5555-5555-5555-555555555555",
				ProductID:             &pid,
				ProductSku:            "SKU1",
				ProductName:           "Cola",
				EventType:             "restock",
				QuantityDelta:         12,
				TechnicianID:          &tid,
				TechnicianDisplayName: "Pat Technician",
				OccurredAt:            "2026-01-01T10:00:00Z",
			},
		},
	}
	var buf bytes.Buffer
	if err := WriteTechnicianFillOpsCSV(&buf, resp); err != nil {
		t.Fatal(err)
	}
	out := strings.ToLower(buf.String())
	for _, forbidden := range []string{"email", "phone", "pan", "cvv"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("unexpected PII token %q in technician fill CSV", forbidden)
		}
	}
	hdr := strings.Split(buf.String(), "\n")[0]
	if !strings.Contains(hdr, "technician_display_name") {
		t.Fatalf("missing display name column: %q", hdr)
	}
}
