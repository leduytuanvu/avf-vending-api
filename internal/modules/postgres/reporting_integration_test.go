package postgres_test

import (
	"context"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/listscope"
	appreporting "github.com/avf/avf-vending-api/internal/app/reporting"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestReportingSalesAndPaymentsAggregatesMatchSeededData(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	orderID := id.NewUUIDV7()
	paymentID := id.NewUUIDV7()
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	_, err := pool.Exec(ctx, `
INSERT INTO sites (id, name, address, timezone, code, contact_info, status)
VALUES ($1, 'Reporting Test Site', '{}'::jsonb, 'UTC', $2, '{}'::jsonb, 'active')`, siteID, "RPT-"+siteID.String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, site_id, serial_number, code, cabinet_type, name, status)
VALUES ($1, $2, $3, $4, 'ambient', 'Reporting Test Machine', 'active')`, machineID, siteID, "SN-"+machineID.String(), "M-"+machineID.String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO orders (id, machine_id, status, currency, subtotal_minor, tax_minor, total_minor, idempotency_key, created_at, updated_at)
VALUES ($1, $2, 'completed', 'USD', 900, 100, 1000, $3, $4, $4)`, orderID, machineID, "order-"+orderID.String(), from.Add(time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO payments (id, order_id, provider, state, amount_minor, currency, idempotency_key, created_at, updated_at, reconciliation_status, settlement_status)
VALUES ($1, $2, 'cash', 'captured', 1000, 'USD', $3, $4, $4, 'matched', 'settled')`, paymentID, orderID, "payment-"+paymentID.String(), from.Add(time.Hour))
	require.NoError(t, err)

	svc := appreporting.NewService(db.New(pool))
	q := listscope.ReportingQuery{From: from, To: to, GroupBy: "none", Timezone: "UTC"}
	sales, err := svc.SalesSummary(ctx, q)
	require.NoError(t, err)
	require.Equal(t, int64(1), sales.Summary.OrderCount)
	require.Equal(t, int64(1000), sales.Summary.GrossTotalMinor)
	require.Equal(t, int64(900), sales.Summary.SubtotalMinor)
	require.Equal(t, int64(100), sales.Summary.TaxMinor)

	payments, err := svc.PaymentSettlement(ctx, q)
	require.NoError(t, err)
	require.Len(t, payments.Items, 1)
	require.Equal(t, "cash", payments.Items[0].Provider)
	require.Equal(t, "captured", payments.Items[0].State)
	require.Equal(t, "settled", payments.Items[0].SettlementStatus)
	require.Equal(t, int64(1), payments.Items[0].PaymentCount)
	require.Equal(t, int64(1000), payments.Items[0].AmountMinor)
}

func TestReportingSalesTotalsRespectProductFilterWhenNoMatchingOrderLines(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	svc := appreporting.NewService(db.New(pool))
	q := listscope.ReportingQuery{
		From:            from,
		To:              to,
		GroupBy:         "none",
		Timezone:        "UTC",
		ProductIDFilter: uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"),
	}
	sales, err := svc.SalesSummary(ctx, q)
	require.NoError(t, err)
	require.Equal(t, int64(0), sales.Summary.OrderCount)
}

func TestReportingTechnicianFillOpsSeededAndFiltered(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	productID := id.NewUUIDV7()
	otherProductID := id.NewUUIDV7()
	techID := id.NewUUIDV7()
	from := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	occurred := from.Add(3 * time.Hour)

	_, err := pool.Exec(ctx, `
INSERT INTO sites (id, name, address, timezone, code, contact_info, status)
VALUES ($1, 'Site', '{}'::jsonb, 'UTC', $2, '{}'::jsonb, 'active')`, siteID, "S-"+siteID.String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, site_id, serial_number, code, cabinet_type, name, status)
VALUES ($1, $2, $3, $4, 'ambient', 'M1', 'active')`, machineID, siteID, "SN-"+machineID.String(), "C-"+machineID.String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO products (id, sku, name)
VALUES ($1, 'SKU-A', 'Product A'), ($2, 'SKU-B', 'Product B')`, productID, otherProductID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO technicians (id, display_name, email)
VALUES ($1, 'Tech One', 'tech@example.test')`, techID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
INSERT INTO inventory_events (
    machine_id, product_id, event_type, slot_code,
    quantity_delta, quantity_before, quantity_after, technician_id, occurred_at
) VALUES ($1, $2, 'restock', 'A1', 10, 0, 10, $3, $4), ($1, $5, 'restock', 'B2', 5, 0, 5, $3, $4)`,
		machineID, productID, techID, occurred, otherProductID)
	require.NoError(t, err)

	svc := appreporting.NewService(db.New(pool))
	base := listscope.ReportingQuery{
		From:     from,
		To:       to,
		Timezone: "UTC",
		Limit:    50,
		Offset:   0,
	}

	all, err := svc.TechnicianFillOperations(ctx, base)
	require.NoError(t, err)
	require.Equal(t, int64(2), all.Meta.Total)
	require.Len(t, all.Items, 2)

	filtered := base
	filtered.ProductIDFilter = productID
	one, err := svc.TechnicianFillOperations(ctx, filtered)
	require.NoError(t, err)
	require.Equal(t, int64(1), one.Meta.Total)
	require.Len(t, one.Items, 1)
	require.Equal(t, productID.String(), *one.Items[0].ProductID)
}
