package postgres_test

import (
	"context"
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

	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	orderID := uuid.New()
	paymentID := uuid.New()
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	_, err := pool.Exec(ctx, `
INSERT INTO organizations (id, name, slug, status, default_timezone)
VALUES ($1, 'Reporting Test Org', $2, 'active', 'UTC')`, orgID, "reporting-test-"+orgID.String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO sites (id, organization_id, name, address, timezone, code, contact_info, status)
VALUES ($1, $2, 'Reporting Test Site', '{}'::jsonb, 'UTC', $3, '{}'::jsonb, 'active')`, siteID, orgID, "RPT-"+siteID.String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, serial_number, code, cabinet_type, name, status)
VALUES ($1, $2, $3, $4, $5, 'ambient', 'Reporting Test Machine', 'active')`, machineID, orgID, siteID, "SN-"+machineID.String(), "M-"+machineID.String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO orders (id, organization_id, machine_id, status, currency, subtotal_minor, tax_minor, total_minor, idempotency_key, created_at, updated_at)
VALUES ($1, $2, $3, 'completed', 'USD', 900, 100, 1000, $4, $5, $5)`, orderID, orgID, machineID, "order-"+orderID.String(), from.Add(time.Hour))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO payments (id, order_id, provider, state, amount_minor, currency, idempotency_key, created_at, updated_at, reconciliation_status, settlement_status)
VALUES ($1, $2, 'cash', 'captured', 1000, 'USD', $3, $4, $4, 'matched', 'settled')`, paymentID, orderID, "payment-"+paymentID.String(), from.Add(time.Hour))
	require.NoError(t, err)

	svc := appreporting.NewService(db.New(pool))
	q := listscope.ReportingQuery{OrganizationID: orgID, From: from, To: to, GroupBy: "none", Timezone: "UTC"}
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

	orgID := uuid.New()
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)

	_, err := pool.Exec(ctx, `
INSERT INTO organizations (id, name, slug, status, default_timezone)
VALUES ($1, 'Reporting Filter Org', $2, 'active', 'UTC')`, orgID, "reporting-filter-"+orgID.String())
	require.NoError(t, err)

	svc := appreporting.NewService(db.New(pool))
	q := listscope.ReportingQuery{
		OrganizationID:  orgID,
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

	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	productID := uuid.New()
	otherProductID := uuid.New()
	techID := uuid.New()
	from := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	occurred := from.Add(3 * time.Hour)

	_, err := pool.Exec(ctx, `
INSERT INTO organizations (id, name, slug, status, default_timezone)
VALUES ($1, 'Fill Report Org', $2, 'active', 'UTC')`, orgID, "fill-rpt-"+orgID.String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO sites (id, organization_id, name, address, timezone, code, contact_info, status)
VALUES ($1, $2, 'Site', '{}'::jsonb, 'UTC', $3, '{}'::jsonb, 'active')`, siteID, orgID, "S-"+siteID.String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, serial_number, code, cabinet_type, name, status)
VALUES ($1, $2, $3, $4, $5, 'ambient', 'M1', 'active')`, machineID, orgID, siteID, "SN-"+machineID.String(), "C-"+machineID.String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO products (id, organization_id, sku, name)
VALUES ($1, $2, 'SKU-A', 'Product A'), ($3, $2, 'SKU-B', 'Product B')`, productID, orgID, otherProductID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO technicians (id, organization_id, display_name, email)
VALUES ($1, $2, 'Tech One', 'tech@example.test')`, techID, orgID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `
INSERT INTO inventory_events (
    organization_id, machine_id, product_id, event_type, slot_code,
    quantity_delta, quantity_before, quantity_after, technician_id, occurred_at
) VALUES ($1, $2, $3, 'restock', 'A1', 10, 0, 10, $4, $5), ($1, $2, $6, 'restock', 'B2', 5, 0, 5, $4, $5)`,
		orgID, machineID, productID, techID, occurred, otherProductID)
	require.NoError(t, err)

	svc := appreporting.NewService(db.New(pool))
	base := listscope.ReportingQuery{
		OrganizationID: orgID,
		From:           from,
		To:             to,
		Timezone:       "UTC",
		Limit:          50,
		Offset:         0,
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

	otherOrg := uuid.New()
	siteOther := uuid.New()
	_, err = pool.Exec(ctx, `
INSERT INTO organizations (id, name, slug, status, default_timezone)
VALUES ($1, 'Other Org', $2, 'active', 'UTC')`, otherOrg, "other-"+otherOrg.String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO sites (id, organization_id, name, address, timezone, code, contact_info, status)
VALUES ($1, $2, 'S2', '{}'::jsonb, 'UTC', $3, '{}'::jsonb, 'active')`, siteOther, otherOrg, "S-"+siteOther.String())
	require.NoError(t, err)
	mOther := uuid.New()
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, serial_number, code, cabinet_type, name, status)
VALUES ($1, $2, $3, $4, $5, 'ambient', 'M2', 'active')`, mOther, otherOrg, siteOther, "SN2-"+mOther.String(), "MC2-"+mOther.String())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO inventory_events (
    organization_id, machine_id, event_type, quantity_delta, quantity_before, quantity_after, occurred_at
) VALUES ($1, $2, 'adjustment', 1, 0, 1, $3)`, otherOrg, mOther, occurred)
	require.NoError(t, err)

	iso, err := svc.TechnicianFillOperations(ctx, base)
	require.NoError(t, err)
	require.Equal(t, int64(2), iso.Meta.Total, "cross-org rows must not affect tenant totals")
}
