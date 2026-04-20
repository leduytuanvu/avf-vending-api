package reporting

import (
	"context"
	"fmt"
	"time"
	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Service runs read-only reporting queries (sqlc-backed).
type Service struct {
	q *db.Queries
}

// NewService constructs a reporting service.
func NewService(q *db.Queries) *Service {
	if q == nil {
		panic("reporting.NewService: nil queries")
	}
	return &Service{q: q}
}

func rfc3339(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func uuidStr(id uuid.UUID) string {
	return id.String()
}

func textPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

func uuidPtr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	id := uuid.UUID(u.Bytes)
	s := id.String()
	return &s
}

// SalesSummary returns order rollups and a single breakdown dimension controlled by groupBy.
func (s *Service) SalesSummary(ctx context.Context, q listscope.ReportingQuery) (*SalesSummaryResponse, error) {
	p := db.ReportingSalesTotalsParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
	}
	tot, err := s.q.ReportingSalesTotals(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("reporting sales totals: %w", err)
	}
	avg := int64(0)
	if tot.OrderCount > 0 {
		avg = tot.GrossTotalMinor / tot.OrderCount
	}
	out := &SalesSummaryResponse{
		OrganizationID: uuidStr(q.OrganizationID),
		From:           rfc3339(q.From),
		To:             rfc3339(q.To),
		GroupBy:        q.GroupBy,
		Summary: SalesSummaryRollup{
			GrossTotalMinor:    tot.GrossTotalMinor,
			SubtotalMinor:      tot.SubtotalMinor,
			TaxMinor:           tot.TaxMinor,
			OrderCount:         tot.OrderCount,
			AvgOrderValueMinor: avg,
		},
		Breakdown: nil,
	}
	switch q.GroupBy {
	case "day":
		rows, err := s.q.ReportingSalesByDay(ctx, db.ReportingSalesByDayParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
		})
		if err != nil {
			return nil, fmt.Errorf("reporting sales by day: %w", err)
		}
		for _, r := range rows {
			bs := rfc3339(r.BucketStart)
			out.Breakdown = append(out.Breakdown, SalesBreakdownRow{
				BucketStart:   &bs,
				OrderCount:    r.OrderCount,
				TotalMinor:    r.TotalMinor,
				SubtotalMinor: r.SubtotalMinor,
				TaxMinor:      r.TaxMinor,
			})
		}
	case "site":
		rows, err := s.q.ReportingSalesBySite(ctx, db.ReportingSalesBySiteParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
		})
		if err != nil {
			return nil, fmt.Errorf("reporting sales by site: %w", err)
		}
		for _, r := range rows {
			sid := uuidStr(r.SiteID)
			out.Breakdown = append(out.Breakdown, SalesBreakdownRow{
				SiteID:        &sid,
				OrderCount:    r.OrderCount,
				TotalMinor:    r.TotalMinor,
				SubtotalMinor: r.SubtotalMinor,
				TaxMinor:      r.TaxMinor,
			})
		}
	case "machine":
		rows, err := s.q.ReportingSalesByMachine(ctx, db.ReportingSalesByMachineParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
		})
		if err != nil {
			return nil, fmt.Errorf("reporting sales by machine: %w", err)
		}
		for _, r := range rows {
			mid := uuidStr(r.MachineID)
			out.Breakdown = append(out.Breakdown, SalesBreakdownRow{
				MachineID:     &mid,
				OrderCount:    r.OrderCount,
				TotalMinor:    r.TotalMinor,
				SubtotalMinor: r.SubtotalMinor,
				TaxMinor:      r.TaxMinor,
			})
		}
	case "payment_method":
		rows, err := s.q.ReportingSalesByPaymentProvider(ctx, db.ReportingSalesByPaymentProviderParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
		})
		if err != nil {
			return nil, fmt.Errorf("reporting sales by payment provider: %w", err)
		}
		for _, r := range rows {
			pr := r.Provider
			out.Breakdown = append(out.Breakdown, SalesBreakdownRow{
				PaymentProvider: &pr,
				OrderCount:      r.OrderCount,
				TotalMinor:      r.TotalMinor,
				SubtotalMinor:   r.SubtotalMinor,
				TaxMinor:        r.TaxMinor,
			})
		}
	case "none":
		// totals only
	default:
		return nil, fmt.Errorf("reporting: unsupported sales group_by")
	}
	return out, nil
}

// PaymentsSummary returns payment rollups and a breakdown controlled by groupBy.
func (s *Service) PaymentsSummary(ctx context.Context, q listscope.ReportingQuery) (*PaymentsSummaryResponse, error) {
	p := db.ReportingPaymentsTotalsParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
	}
	tot, err := s.q.ReportingPaymentsTotals(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("reporting payments totals: %w", err)
	}
	out := &PaymentsSummaryResponse{
		OrganizationID: uuidStr(q.OrganizationID),
		From:           rfc3339(q.From),
		To:             rfc3339(q.To),
		GroupBy:        q.GroupBy,
		Summary: PaymentsSummaryRollup{
			AuthorizedCount:        tot.AuthorizedCount,
			CapturedCount:          tot.CapturedCount,
			FailedCount:            tot.FailedCount,
			RefundedCount:          tot.RefundedCount,
			CapturedAmountMinor:    tot.CapturedAmountMinor,
			AuthorizedAmountMinor: tot.AuthorizedAmountMinor,
			FailedAmountMinor:      tot.FailedAmountMinor,
			RefundedAmountMinor:    tot.RefundedAmountMinor,
		},
	}
	switch q.GroupBy {
	case "day":
		rows, err := s.q.ReportingPaymentsByDay(ctx, db.ReportingPaymentsByDayParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
		})
		if err != nil {
			return nil, fmt.Errorf("reporting payments by day: %w", err)
		}
		for _, r := range rows {
			bs := rfc3339(r.BucketStart)
			out.Breakdown = append(out.Breakdown, PaymentsBreakdownRow{
				BucketStart:  &bs,
				PaymentCount: r.PaymentCount,
				AmountMinor:  r.AmountMinor,
			})
		}
	case "payment_method":
		byProv := map[string]PaymentsBreakdownRow{}
		rows, err := s.q.ReportingPaymentsByMethodAndState(ctx, db.ReportingPaymentsByMethodAndStateParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
		})
		if err != nil {
			return nil, fmt.Errorf("reporting payments by method: %w", err)
		}
		for _, r := range rows {
			cur := byProv[r.Provider]
			pr := r.Provider
			cur.Provider = &pr
			cur.PaymentCount += r.PaymentCount
			cur.AmountMinor += r.AmountMinor
			byProv[r.Provider] = cur
		}
		for _, v := range byProv {
			out.Breakdown = append(out.Breakdown, v)
		}
	case "status":
		byState := map[string]PaymentsBreakdownRow{}
		rows, err := s.q.ReportingPaymentsByMethodAndState(ctx, db.ReportingPaymentsByMethodAndStateParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
		})
		if err != nil {
			return nil, fmt.Errorf("reporting payments by status: %w", err)
		}
		for _, r := range rows {
			cur := byState[r.State]
			st := r.State
			cur.State = &st
			cur.PaymentCount += r.PaymentCount
			cur.AmountMinor += r.AmountMinor
			byState[r.State] = cur
		}
		for _, v := range byState {
			out.Breakdown = append(out.Breakdown, v)
		}
	case "none":
	default:
		return nil, fmt.Errorf("reporting: unsupported payments group_by")
	}
	return out, nil
}

// FleetHealth returns machine posture and incident rollups for the reporting window.
func (s *Service) FleetHealth(ctx context.Context, q listscope.ReportingQuery) (*FleetHealthResponse, error) {
	machineRows, err := s.q.ReportingFleetMachinesByStatus(ctx, q.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("reporting fleet machines: %w", err)
	}
	summary := FleetMachineHealthSummary{}
	byStatus := make([]FleetStatusCountRow, 0, len(machineRows))
	for _, r := range machineRows {
		byStatus = append(byStatus, FleetStatusCountRow{Status: r.Status, Count: r.MachineCount})
		summary.Total += r.MachineCount
		switch r.Status {
		case "online":
			summary.Online += r.MachineCount
		case "offline":
			summary.Offline += r.MachineCount
		case "maintenance":
			summary.Fault += r.MachineCount
		case "provisioning":
			summary.Warn += r.MachineCount
		case "retired":
			summary.Retired += r.MachineCount
		default:
			summary.Fault += r.MachineCount
		}
	}
	incP := db.ReportingFleetIncidentsByStatusParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
	}
	incRows, err := s.q.ReportingFleetIncidentsByStatus(ctx, incP)
	if err != nil {
		return nil, fmt.Errorf("reporting fleet incidents: %w", err)
	}
	incBy := make([]FleetStatusCountRow, 0, len(incRows))
	for _, r := range incRows {
		incBy = append(incBy, FleetStatusCountRow{Status: r.Status, Count: r.IncidentCount})
	}
	sevP := db.ReportingFleetMachineIncidentsBySeverityParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
	}
	sevRows, err := s.q.ReportingFleetMachineIncidentsBySeverity(ctx, sevP)
	if err != nil {
		return nil, fmt.Errorf("reporting machine incidents: %w", err)
	}
	sev := make([]FleetSeverityCountRow, 0, len(sevRows))
	for _, r := range sevRows {
		sev = append(sev, FleetSeverityCountRow{Severity: r.Severity, Count: r.IncidentCount})
	}
	return &FleetHealthResponse{
		OrganizationID:             uuidStr(q.OrganizationID),
		From:                       rfc3339(q.From),
		To:                         rfc3339(q.To),
		MachineSummary:             summary,
		MachinesByStatus:           byStatus,
		IncidentsByStatus:          incBy,
		MachineIncidentsBySeverity: sev,
	}, nil
}

// InventoryExceptions lists current slot projections that are out of stock or below 15% of configured max.
func (s *Service) InventoryExceptions(ctx context.Context, q listscope.ReportingQuery) (*InventoryExceptionsResponse, error) {
	includeOut, includeLow, err := exceptionFilters(q.ExceptionKind)
	if err != nil {
		return nil, err
	}
	cnt, err := s.q.ReportingInventoryExceptionsCount(ctx, db.ReportingInventoryExceptionsCountParams{
		OrganizationID: q.OrganizationID,
		Column2:        includeOut,
		Column3:        includeLow,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting inventory exceptions count: %w", err)
	}
	rows, err := s.q.ReportingInventoryExceptions(ctx, db.ReportingInventoryExceptionsParams{
		OrganizationID: q.OrganizationID,
		Column2:        includeOut,
		Column3:        includeLow,
		Limit:          q.Limit,
		Offset:         q.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting inventory exceptions: %w", err)
	}
	items := make([]InventoryExceptionItem, 0, len(rows))
	for _, r := range rows {
		low := r.LowStock.Valid && r.LowStock.Bool
		items = append(items, InventoryExceptionItem{
			MachineID:           uuidStr(r.MachineID),
			MachineName:         r.MachineName,
			MachineSerialNumber: r.MachineSerialNumber,
			MachineStatus:       r.MachineStatus,
			PlanogramID:         uuidStr(r.PlanogramID),
			PlanogramName:       r.PlanogramName,
			SlotIndex:           r.SlotIndex,
			CurrentQuantity:     r.CurrentQuantity,
			MaxQuantity:         r.MaxQuantity,
			ProductID:           uuidPtr(r.ProductID),
			ProductSku:          textPtr(r.ProductSku),
			ProductName:         textPtr(r.ProductName),
			OutOfStock:          r.OutOfStock,
			LowStock:            low,
			AttentionNeeded:     r.OutOfStock || low,
		})
	}
	return &InventoryExceptionsResponse{
		OrganizationID: uuidStr(q.OrganizationID),
		From:           rfc3339(q.From),
		To:             rfc3339(q.To),
		ExceptionKind:  q.ExceptionKind,
		Meta: InventoryExceptionsListMeta{
			Limit:    q.Limit,
			Offset:   q.Offset,
			Returned: len(items),
			Total:    cnt,
		},
		Items: items,
	}, nil
}

func exceptionFilters(kind string) (includeOutOfStock, includeLowStock bool, err error) {
	switch kind {
	case "all":
		return true, true, nil
	case "out_of_stock":
		return true, false, nil
	case "low_stock":
		return false, true, nil
	default:
		return false, false, fmt.Errorf("reporting: invalid exception_kind")
	}
}
