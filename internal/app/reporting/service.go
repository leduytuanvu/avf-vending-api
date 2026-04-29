package reporting

import (
	"context"
	"fmt"
	"strconv"
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

func reportingTimezone(q listscope.ReportingQuery) string {
	if q.Timezone == "" {
		return "UTC"
	}
	return q.Timezone
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

func timeStringPtr(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := rfc3339(t.Time)
	return &s
}

// SalesSummary returns order rollups and a single breakdown dimension controlled by groupBy.
func (s *Service) SalesSummary(ctx context.Context, q listscope.ReportingQuery) (*SalesSummaryResponse, error) {
	filtered := reportingFiltersActive(q)
	var grossTotal, subtotal, taxMinor, orderCount int64
	if filtered {
		ft, err := s.q.ReportingSalesTotalsFiltered(ctx, db.ReportingSalesTotalsFilteredParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
			Column4:        q.SiteIDFilter,
			Column5:        q.MachineIDFilter,
			Column6:        q.ProductIDFilter,
		})
		if err != nil {
			return nil, fmt.Errorf("reporting sales totals: %w", err)
		}
		grossTotal, subtotal, taxMinor, orderCount = ft.GrossTotalMinor, ft.SubtotalMinor, ft.TaxMinor, ft.OrderCount
	} else {
		tot, err := s.q.ReportingSalesTotals(ctx, db.ReportingSalesTotalsParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
		})
		if err != nil {
			return nil, fmt.Errorf("reporting sales totals: %w", err)
		}
		grossTotal, subtotal, taxMinor, orderCount = tot.GrossTotalMinor, tot.SubtotalMinor, tot.TaxMinor, tot.OrderCount
	}
	avg := int64(0)
	if orderCount > 0 {
		avg = grossTotal / orderCount
	}
	out := &SalesSummaryResponse{
		OrganizationID: uuidStr(q.OrganizationID),
		From:           rfc3339(q.From),
		To:             rfc3339(q.To),
		GroupBy:        q.GroupBy,
		Summary: SalesSummaryRollup{
			GrossTotalMinor:    grossTotal,
			SubtotalMinor:      subtotal,
			TaxMinor:           taxMinor,
			OrderCount:         orderCount,
			AvgOrderValueMinor: avg,
		},
		Breakdown: nil,
	}
	switch q.GroupBy {
	case "day":
		if filtered {
			rows, err := s.q.ReportingSalesByDayFiltered(ctx, db.ReportingSalesByDayFilteredParams{
				OrganizationID: q.OrganizationID,
				Column2:        q.From,
				Column3:        q.To,
				Column4:        q.SiteIDFilter,
				Column5:        q.MachineIDFilter,
				Column6:        q.ProductIDFilter,
				Column7:        reportingTimezone(q),
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
		} else {
			rows, err := s.q.ReportingSalesByDay(ctx, db.ReportingSalesByDayParams{
				OrganizationID: q.OrganizationID,
				Column2:        q.From,
				Column3:        q.To,
				Column4:        reportingTimezone(q),
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
		}
	case "site":
		if filtered {
			rows, err := s.q.ReportingSalesBySiteFiltered(ctx, db.ReportingSalesBySiteFilteredParams{
				OrganizationID: q.OrganizationID,
				Column2:        q.From,
				Column3:        q.To,
				Column4:        q.SiteIDFilter,
				Column5:        q.MachineIDFilter,
				Column6:        q.ProductIDFilter,
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
		} else {
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
		}
	case "machine":
		if filtered {
			rows, err := s.q.ReportingSalesByMachineFiltered(ctx, db.ReportingSalesByMachineFilteredParams{
				OrganizationID: q.OrganizationID,
				Column2:        q.From,
				Column3:        q.To,
				Column4:        q.SiteIDFilter,
				Column5:        q.MachineIDFilter,
				Column6:        q.ProductIDFilter,
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
		} else {
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
		}
	case "payment_method":
		if filtered {
			rows, err := s.q.ReportingSalesByPaymentProviderFiltered(ctx, db.ReportingSalesByPaymentProviderFilteredParams{
				OrganizationID: q.OrganizationID,
				Column2:        q.From,
				Column3:        q.To,
				Column4:        q.SiteIDFilter,
				Column5:        q.MachineIDFilter,
				Column6:        q.ProductIDFilter,
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
		} else {
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
		}
	case "product":
		rows, err := s.q.ReportingSalesByProduct(ctx, db.ReportingSalesByProductParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
			Column4:        q.SiteIDFilter,
			Column5:        q.MachineIDFilter,
			Column6:        q.ProductIDFilter,
		})
		if err != nil {
			return nil, fmt.Errorf("reporting sales by product: %w", err)
		}
		for _, r := range rows {
			pid := uuidStr(r.ProductID)
			pn := r.ProductName
			ps := r.ProductSku
			out.Breakdown = append(out.Breakdown, SalesBreakdownRow{
				ProductID:     &pid,
				ProductName:   &pn,
				ProductSku:    &ps,
				OrderCount:    r.SuccessVends + r.FailedVends,
				TotalMinor:    r.AllocatedRevenueMinor,
				SuccessVends:  r.SuccessVends,
				FailedVends:   r.FailedVends,
				SubtotalMinor: 0,
				TaxMinor:      0,
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
	filtered := reportingFiltersActive(q)
	var tot db.ReportingPaymentsTotalsRow
	if filtered {
		ft, err := s.q.ReportingPaymentsTotalsFiltered(ctx, db.ReportingPaymentsTotalsFilteredParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
			Column4:        q.SiteIDFilter,
			Column5:        q.MachineIDFilter,
			Column6:        q.ProductIDFilter,
		})
		if err != nil {
			return nil, fmt.Errorf("reporting payments totals: %w", err)
		}
		tot = db.ReportingPaymentsTotalsRow{
			AuthorizedCount:       ft.AuthorizedCount,
			CapturedCount:         ft.CapturedCount,
			FailedCount:           ft.FailedCount,
			RefundedCount:         ft.RefundedCount,
			CapturedAmountMinor:   ft.CapturedAmountMinor,
			AuthorizedAmountMinor: ft.AuthorizedAmountMinor,
			FailedAmountMinor:     ft.FailedAmountMinor,
			RefundedAmountMinor:   ft.RefundedAmountMinor,
		}
	} else {
		var err error
		tot, err = s.q.ReportingPaymentsTotals(ctx, db.ReportingPaymentsTotalsParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
		})
		if err != nil {
			return nil, fmt.Errorf("reporting payments totals: %w", err)
		}
	}
	out := &PaymentsSummaryResponse{
		OrganizationID: uuidStr(q.OrganizationID),
		From:           rfc3339(q.From),
		To:             rfc3339(q.To),
		GroupBy:        q.GroupBy,
		Summary: PaymentsSummaryRollup{
			AuthorizedCount:       tot.AuthorizedCount,
			CapturedCount:         tot.CapturedCount,
			FailedCount:           tot.FailedCount,
			RefundedCount:         tot.RefundedCount,
			CapturedAmountMinor:   tot.CapturedAmountMinor,
			AuthorizedAmountMinor: tot.AuthorizedAmountMinor,
			FailedAmountMinor:     tot.FailedAmountMinor,
			RefundedAmountMinor:   tot.RefundedAmountMinor,
		},
	}
	switch q.GroupBy {
	case "day":
		if filtered {
			rows, err := s.q.ReportingPaymentsByDayFiltered(ctx, db.ReportingPaymentsByDayFilteredParams{
				OrganizationID: q.OrganizationID,
				Column2:        q.From,
				Column3:        q.To,
				Column4:        q.SiteIDFilter,
				Column5:        q.MachineIDFilter,
				Column6:        q.ProductIDFilter,
				Column7:        reportingTimezone(q),
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
		} else {
			rows, err := s.q.ReportingPaymentsByDay(ctx, db.ReportingPaymentsByDayParams{
				OrganizationID: q.OrganizationID,
				Column2:        q.From,
				Column3:        q.To,
				Column4:        reportingTimezone(q),
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
		}
	case "payment_method":
		byProv := map[string]PaymentsBreakdownRow{}
		if filtered {
			rows, err := s.q.ReportingPaymentsByMethodAndStateFiltered(ctx, db.ReportingPaymentsByMethodAndStateFilteredParams{
				OrganizationID: q.OrganizationID,
				Column2:        q.From,
				Column3:        q.To,
				Column4:        q.SiteIDFilter,
				Column5:        q.MachineIDFilter,
				Column6:        q.ProductIDFilter,
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
		} else {
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
		}
		for _, v := range byProv {
			out.Breakdown = append(out.Breakdown, v)
		}
	case "status":
		byState := map[string]PaymentsBreakdownRow{}
		if filtered {
			rows, err := s.q.ReportingPaymentsByMethodAndStateFiltered(ctx, db.ReportingPaymentsByMethodAndStateFilteredParams{
				OrganizationID: q.OrganizationID,
				Column2:        q.From,
				Column3:        q.To,
				Column4:        q.SiteIDFilter,
				Column5:        q.MachineIDFilter,
				Column6:        q.ProductIDFilter,
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
		} else {
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
		case "retired", "decommissioned":
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
	var cnt int64
	var items []InventoryExceptionItem
	if reportingFiltersActive(q) {
		cnt, err = s.q.ReportingInventoryExceptionsFilteredCount(ctx, db.ReportingInventoryExceptionsFilteredCountParams{
			OrganizationID: q.OrganizationID,
			Column2:        includeOut,
			Column3:        includeLow,
			Column4:        q.SiteIDFilter,
			Column5:        q.MachineIDFilter,
			Column6:        q.ProductIDFilter,
		})
		if err != nil {
			return nil, fmt.Errorf("reporting inventory exceptions count: %w", err)
		}
		rows, ferr := s.q.ReportingInventoryExceptionsFiltered(ctx, db.ReportingInventoryExceptionsFilteredParams{
			OrganizationID: q.OrganizationID,
			Column2:        includeOut,
			Column3:        includeLow,
			Column4:        q.SiteIDFilter,
			Column5:        q.MachineIDFilter,
			Column6:        q.ProductIDFilter,
			Limit:          q.Limit,
			Offset:         q.Offset,
		})
		if ferr != nil {
			return nil, fmt.Errorf("reporting inventory exceptions: %w", ferr)
		}
		items = make([]InventoryExceptionItem, 0, len(rows))
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
	} else {
		cnt, err = s.q.ReportingInventoryExceptionsCount(ctx, db.ReportingInventoryExceptionsCountParams{
			OrganizationID: q.OrganizationID,
			Column2:        includeOut,
			Column3:        includeLow,
		})
		if err != nil {
			return nil, fmt.Errorf("reporting inventory exceptions count: %w", err)
		}
		rows, ferr := s.q.ReportingInventoryExceptions(ctx, db.ReportingInventoryExceptionsParams{
			OrganizationID: q.OrganizationID,
			Column2:        includeOut,
			Column3:        includeLow,
			Limit:          q.Limit,
			Offset:         q.Offset,
		})
		if ferr != nil {
			return nil, fmt.Errorf("reporting inventory exceptions: %w", ferr)
		}
		items = make([]InventoryExceptionItem, 0, len(rows))
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

// CashCollectionsExport returns cash_collections rows for an organization within [from,to), honoring optional site/machine filters (uuid.Nil = unset).
func (s *Service) CashCollectionsExport(ctx context.Context, q listscope.ReportingQuery) ([]CashCollectionExportRow, error) {
	rows, err := s.q.ReportingCashCollectionsForOrganization(ctx, db.ReportingCashCollectionsForOrganizationParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
		Column4:        q.SiteIDFilter,
		Column5:        q.MachineIDFilter,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting cash collections export: %w", err)
	}
	out := make([]CashCollectionExportRow, 0, len(rows))
	for _, r := range rows {
		item := CashCollectionExportRow{
			CollectionID:         r.ID.String(),
			MachineID:            r.MachineID.String(),
			SiteID:               r.SiteID.String(),
			SiteName:             r.SiteName,
			MachineSerialNumber:  r.MachineSerialNumber,
			CollectedAt:          r.CollectedAt,
			OpenedAt:             r.OpenedAt,
			LifecycleStatus:      r.LifecycleStatus,
			AmountMinor:          r.AmountMinor,
			ExpectedAmountMinor:  r.ExpectedAmountMinor,
			VarianceAmountMinor:  r.VarianceAmountMinor,
			Currency:             r.Currency,
			ReconciliationStatus: r.ReconciliationStatus,
			CreatedAt:            r.CreatedAt,
		}
		if r.ClosedAt.Valid {
			ts := r.ClosedAt.Time
			item.ClosedAt = &ts
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *Service) PaymentSettlement(ctx context.Context, q listscope.ReportingQuery) (*PaymentSettlementResponse, error) {
	var items []PaymentSettlementRow
	if reportingFiltersActive(q) {
		rows, err := s.q.ReportingPaymentSettlementFiltered(ctx, db.ReportingPaymentSettlementFilteredParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
			Column4:        q.SiteIDFilter,
			Column5:        q.MachineIDFilter,
			Column6:        q.ProductIDFilter,
			Column7:        reportingTimezone(q),
		})
		if err != nil {
			return nil, fmt.Errorf("reporting payment settlement: %w", err)
		}
		items = make([]PaymentSettlementRow, 0, len(rows))
		for _, r := range rows {
			items = append(items, PaymentSettlementRow{
				BucketStart:          rfc3339(r.BucketStart),
				Provider:             r.Provider,
				State:                r.State,
				SettlementStatus:     r.SettlementStatus,
				ReconciliationStatus: r.ReconciliationStatus,
				PaymentCount:         r.PaymentCount,
				AmountMinor:          r.AmountMinor,
			})
		}
	} else {
		rows, err := s.q.ReportingPaymentSettlement(ctx, db.ReportingPaymentSettlementParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
			Column4:        reportingTimezone(q),
		})
		if err != nil {
			return nil, fmt.Errorf("reporting payment settlement: %w", err)
		}
		items = make([]PaymentSettlementRow, 0, len(rows))
		for _, r := range rows {
			items = append(items, PaymentSettlementRow{
				BucketStart:          rfc3339(r.BucketStart),
				Provider:             r.Provider,
				State:                r.State,
				SettlementStatus:     r.SettlementStatus,
				ReconciliationStatus: r.ReconciliationStatus,
				PaymentCount:         r.PaymentCount,
				AmountMinor:          r.AmountMinor,
			})
		}
	}
	return &PaymentSettlementResponse{
		OrganizationID: uuidStr(q.OrganizationID),
		From:           rfc3339(q.From),
		To:             rfc3339(q.To),
		Timezone:       reportingTimezone(q),
		Items:          items,
		Meta: InventoryExceptionsListMeta{
			Limit:    int32(len(items)),
			Offset:   0,
			Returned: len(items),
			Total:    int64(len(items)),
		},
	}, nil
}

func (s *Service) Refunds(ctx context.Context, q listscope.ReportingQuery) (*RefundReportResponse, error) {
	total, err := s.q.ReportingRefundsCount(ctx, db.ReportingRefundsCountParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting refunds count: %w", err)
	}
	rows, err := s.q.ReportingRefunds(ctx, db.ReportingRefundsParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
		Limit:          q.Limit,
		Offset:         q.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting refunds: %w", err)
	}
	items := make([]RefundReportItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, RefundReportItem{
			RefundID:             uuidStr(r.RefundID),
			PaymentID:            uuidStr(r.PaymentID),
			OrderID:              uuidStr(r.OrderID),
			MachineID:            uuidStr(r.MachineID),
			AmountMinor:          r.AmountMinor,
			Currency:             r.Currency,
			State:                r.State,
			Reason:               textPtr(r.Reason),
			ReconciliationStatus: r.ReconciliationStatus,
			SettlementStatus:     r.SettlementStatus,
			CreatedAt:            rfc3339(r.CreatedAt),
		})
	}
	return &RefundReportResponse{
		OrganizationID: uuidStr(q.OrganizationID),
		From:           rfc3339(q.From),
		To:             rfc3339(q.To),
		Meta:           reportMeta(q, len(items), total),
		Items:          items,
	}, nil
}

func (s *Service) CashCollectionsReport(ctx context.Context, q listscope.ReportingQuery) (*CashCollectionReportResponse, error) {
	total, err := s.q.ReportingCashCollectionsCount(ctx, db.ReportingCashCollectionsCountParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
		Column4:        q.SiteIDFilter,
		Column5:        q.MachineIDFilter,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting cash collections count: %w", err)
	}
	rows, err := s.CashCollectionsExport(ctx, q)
	if err != nil {
		return nil, err
	}
	if q.Offset > 0 {
		if int(q.Offset) >= len(rows) {
			rows = nil
		} else {
			rows = rows[q.Offset:]
		}
	}
	if q.Limit > 0 && int(q.Limit) < len(rows) {
		rows = rows[:q.Limit]
	}
	return &CashCollectionReportResponse{
		OrganizationID: uuidStr(q.OrganizationID),
		From:           rfc3339(q.From),
		To:             rfc3339(q.To),
		Meta:           reportMeta(q, len(rows), total),
		Items:          rows,
	}, nil
}

func (s *Service) MachineHealth(ctx context.Context, q listscope.ReportingQuery) (*MachineHealthReportResponse, error) {
	cutoff := q.To.Add(-15 * time.Minute)
	filtered := reportingFiltersActive(q)
	var total int64
	var err error
	var items []MachineHealthReportItem
	if filtered {
		total, err = s.q.ReportingMachineHealthFilteredCount(ctx, db.ReportingMachineHealthFilteredCountParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.SiteIDFilter,
			Column3:        q.MachineIDFilter,
			Column4:        q.ProductIDFilter,
			Column5:        q.From,
			Column6:        q.To,
		})
		if err != nil {
			return nil, fmt.Errorf("reporting machine health count: %w", err)
		}
		rows, ferr := s.q.ReportingMachineHealthFiltered(ctx, db.ReportingMachineHealthFilteredParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.SiteIDFilter,
			Column3:        q.MachineIDFilter,
			Column4:        q.ProductIDFilter,
			Column5:        q.From,
			Column6:        q.To,
			Column7:        cutoff,
			Limit:          q.Limit,
			Offset:         q.Offset,
		})
		if ferr != nil {
			return nil, fmt.Errorf("reporting machine health: %w", ferr)
		}
		items = make([]MachineHealthReportItem, 0, len(rows))
		for _, r := range rows {
			items = append(items, MachineHealthReportItem{
				MachineID:    uuidStr(r.MachineID),
				SiteID:       uuidStr(r.SiteID),
				SiteName:     r.SiteName,
				SerialNumber: r.SerialNumber,
				MachineName:  r.MachineName,
				Status:       r.Status,
				LastSeenAt:   timeStringPtr(r.LastSeenAt),
				Offline:      r.Offline,
			})
		}
	} else {
		total, err = s.q.ReportingMachineHealthCount(ctx, q.OrganizationID)
		if err != nil {
			return nil, fmt.Errorf("reporting machine health count: %w", err)
		}
		rows, ferr := s.q.ReportingMachineHealth(ctx, db.ReportingMachineHealthParams{
			OrganizationID: q.OrganizationID,
			Column2:        cutoff,
			Limit:          q.Limit,
			Offset:         q.Offset,
		})
		if ferr != nil {
			return nil, fmt.Errorf("reporting machine health: %w", ferr)
		}
		items = make([]MachineHealthReportItem, 0, len(rows))
		for _, r := range rows {
			items = append(items, MachineHealthReportItem{
				MachineID:    uuidStr(r.MachineID),
				SiteID:       uuidStr(r.SiteID),
				SiteName:     r.SiteName,
				SerialNumber: r.SerialNumber,
				MachineName:  r.MachineName,
				Status:       r.Status,
				LastSeenAt:   timeStringPtr(r.LastSeenAt),
				Offline:      r.Offline,
			})
		}
	}
	return &MachineHealthReportResponse{
		OrganizationID: uuidStr(q.OrganizationID),
		From:           rfc3339(q.From),
		To:             rfc3339(q.To),
		Meta:           reportMeta(q, len(items), total),
		Items:          items,
	}, nil
}

func (s *Service) FailedVends(ctx context.Context, q listscope.ReportingQuery) (*FailedVendReportResponse, error) {
	filtered := reportingFiltersActive(q)
	var total int64
	var err error
	var items []FailedVendReportItem
	if filtered {
		total, err = s.q.ReportingFailedVendsCountFiltered(ctx, db.ReportingFailedVendsCountFilteredParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
			Column4:        q.SiteIDFilter,
			Column5:        q.MachineIDFilter,
			Column6:        q.ProductIDFilter,
		})
		if err != nil {
			return nil, fmt.Errorf("reporting failed vends count: %w", err)
		}
		fr, ferr := s.q.ReportingFailedVendsFiltered(ctx, db.ReportingFailedVendsFilteredParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
			Column4:        q.SiteIDFilter,
			Column5:        q.MachineIDFilter,
			Column6:        q.ProductIDFilter,
			Limit:          q.Limit,
			Offset:         q.Offset,
		})
		if ferr != nil {
			return nil, fmt.Errorf("reporting failed vends: %w", ferr)
		}
		items = make([]FailedVendReportItem, 0, len(fr))
		for _, r := range fr {
			items = append(items, FailedVendReportItem{
				VendSessionID: uuidStr(r.VendSessionID),
				OrderID:       uuidStr(r.OrderID),
				MachineID:     uuidStr(r.MachineID),
				SlotIndex:     r.SlotIndex,
				ProductID:     uuidStr(r.ProductID),
				FailureReason: textPtr(r.FailureReason),
				StartedAt:     timeStringPtr(r.StartedAt),
				CompletedAt:   timeStringPtr(r.CompletedAt),
				CreatedAt:     rfc3339(r.CreatedAt),
				TotalMinor:    r.TotalMinor,
				Currency:      r.Currency,
				OrderStatus:   r.OrderStatus,
			})
		}
	} else {
		total, err = s.q.ReportingFailedVendsCount(ctx, db.ReportingFailedVendsCountParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
		})
		if err != nil {
			return nil, fmt.Errorf("reporting failed vends count: %w", err)
		}
		rows, ferr := s.q.ReportingFailedVends(ctx, db.ReportingFailedVendsParams{
			OrganizationID: q.OrganizationID,
			Column2:        q.From,
			Column3:        q.To,
			Limit:          q.Limit,
			Offset:         q.Offset,
		})
		if ferr != nil {
			return nil, fmt.Errorf("reporting failed vends: %w", ferr)
		}
		items = make([]FailedVendReportItem, 0, len(rows))
		for _, r := range rows {
			items = append(items, FailedVendReportItem{
				VendSessionID: uuidStr(r.VendSessionID),
				OrderID:       uuidStr(r.OrderID),
				MachineID:     uuidStr(r.MachineID),
				SlotIndex:     r.SlotIndex,
				ProductID:     uuidStr(r.ProductID),
				FailureReason: textPtr(r.FailureReason),
				StartedAt:     timeStringPtr(r.StartedAt),
				CompletedAt:   timeStringPtr(r.CompletedAt),
				CreatedAt:     rfc3339(r.CreatedAt),
				TotalMinor:    r.TotalMinor,
				Currency:      r.Currency,
				OrderStatus:   r.OrderStatus,
			})
		}
	}
	return &FailedVendReportResponse{
		OrganizationID: uuidStr(q.OrganizationID),
		From:           rfc3339(q.From),
		To:             rfc3339(q.To),
		Meta:           reportMeta(q, len(items), total),
		Items:          items,
	}, nil
}

func (s *Service) ReconciliationQueue(ctx context.Context, q listscope.ReportingQuery) (*ReconciliationQueueReportResponse, error) {
	total, err := s.q.ReportingReconciliationQueueCount(ctx, db.ReportingReconciliationQueueCountParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting reconciliation queue count: %w", err)
	}
	rows, err := s.q.ReportingReconciliationQueue(ctx, db.ReportingReconciliationQueueParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
		Limit:          q.Limit,
		Offset:         q.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting reconciliation queue: %w", err)
	}
	items := make([]ReconciliationQueueItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, ReconciliationQueueItem{
			ID:              uuidStr(r.ID),
			CaseType:        r.CaseType,
			Status:          r.Status,
			Severity:        r.Severity,
			OrderID:         uuidPtr(r.OrderID),
			PaymentID:       uuidPtr(r.PaymentID),
			VendSessionID:   uuidPtr(r.VendSessionID),
			RefundID:        uuidPtr(r.RefundID),
			Provider:        textPtr(r.Provider),
			Reason:          r.Reason,
			FirstDetectedAt: rfc3339(r.FirstDetectedAt),
			LastDetectedAt:  rfc3339(r.LastDetectedAt),
		})
	}
	return &ReconciliationQueueReportResponse{
		OrganizationID: uuidStr(q.OrganizationID),
		From:           rfc3339(q.From),
		To:             rfc3339(q.To),
		Meta:           reportMeta(q, len(items), total),
		Items:          items,
	}, nil
}

func (s *Service) VendSummary(ctx context.Context, q listscope.ReportingQuery) (*VendSummaryResponse, error) {
	tot, err := s.q.ReportingVendSummaryTotals(ctx, db.ReportingVendSummaryTotalsParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
		Column4:        q.SiteIDFilter,
		Column5:        q.MachineIDFilter,
		Column6:        q.ProductIDFilter,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting vend summary totals: %w", err)
	}
	failed, err := s.FailedVends(ctx, q)
	if err != nil {
		return nil, err
	}
	return &VendSummaryResponse{
		OrganizationID: uuidStr(q.OrganizationID),
		From:           rfc3339(q.From),
		To:             rfc3339(q.To),
		Summary: VendCountsSummary{
			SuccessCount:    tot.SuccessCount,
			FailedCount:     tot.FailedCount,
			InProgressCount: tot.InProgressCount,
		},
		FailedVends: VendFailedVendsSubreport{
			Meta:  failed.Meta,
			Items: failed.Items,
		},
	}, nil
}

func (s *Service) StockMovement(ctx context.Context, q listscope.ReportingQuery) (*StockMovementReportResponse, error) {
	total, err := s.q.ReportingStockMovementCount(ctx, db.ReportingStockMovementCountParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
		Column4:        q.SiteIDFilter,
		Column5:        q.MachineIDFilter,
		Column6:        q.ProductIDFilter,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting stock movement count: %w", err)
	}
	rows, err := s.q.ReportingStockMovement(ctx, db.ReportingStockMovementParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
		Column4:        q.SiteIDFilter,
		Column5:        q.MachineIDFilter,
		Column6:        q.ProductIDFilter,
		Limit:          q.Limit,
		Offset:         q.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting stock movement: %w", err)
	}
	items := make([]StockMovementRow, 0, len(rows))
	for _, r := range rows {
		slot := textPtr(r.SlotCode)
		items = append(items, StockMovementRow{
			InventoryEventID: strconv.FormatInt(r.InventoryEventID, 10),
			MachineID:        uuidStr(r.MachineID),
			SiteID:           uuidStr(r.SiteID),
			ProductID:        uuidPtr(r.ProductID),
			ProductSku:       r.ProductSku,
			ProductName:      r.ProductName,
			EventType:        r.EventType,
			SlotCode:         slot,
			QuantityDelta:    r.QuantityDelta,
			QuantityBefore:   pgInt32Ptr(r.QuantityBefore),
			QuantityAfter:    pgInt32Ptr(r.QuantityAfter),
			OccurredAt:       rfc3339(r.OccurredAt),
		})
	}
	return &StockMovementReportResponse{
		OrganizationID: uuidStr(q.OrganizationID),
		From:           rfc3339(q.From),
		To:             rfc3339(q.To),
		Meta:           reportMeta(q, len(items), total),
		Items:          items,
	}, nil
}

func (s *Service) TechnicianFillOperations(ctx context.Context, q listscope.ReportingQuery) (*TechnicianFillReportResponse, error) {
	total, err := s.q.ReportingTechnicianFillOpsCount(ctx, db.ReportingTechnicianFillOpsCountParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
		Column4:        q.SiteIDFilter,
		Column5:        q.MachineIDFilter,
		Column6:        q.ProductIDFilter,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting technician fill ops count: %w", err)
	}
	rows, err := s.q.ReportingTechnicianFillOps(ctx, db.ReportingTechnicianFillOpsParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
		Column4:        q.SiteIDFilter,
		Column5:        q.MachineIDFilter,
		Column6:        q.ProductIDFilter,
		Limit:          q.Limit,
		Offset:         q.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting technician fill ops: %w", err)
	}
	items := make([]TechnicianFillOpRow, 0, len(rows))
	for _, r := range rows {
		items = append(items, TechnicianFillOpRow{
			InventoryEventID:      strconv.FormatInt(r.InventoryEventID, 10),
			MachineID:             uuidStr(r.MachineID),
			SiteID:                uuidStr(r.SiteID),
			ProductID:             uuidPtr(r.ProductID),
			ProductSku:            r.ProductSku,
			ProductName:           r.ProductName,
			EventType:             r.EventType,
			SlotCode:              textPtr(r.SlotCode),
			QuantityDelta:         r.QuantityDelta,
			QuantityBefore:        pgInt32Ptr(r.QuantityBefore),
			QuantityAfter:         pgInt32Ptr(r.QuantityAfter),
			OperatorSessionID:     uuidPtr(r.OperatorSessionID),
			TechnicianID:          uuidPtr(r.TechnicianID),
			TechnicianDisplayName: r.TechnicianDisplayName,
			RefillSessionID:       uuidPtr(r.RefillSessionID),
			OccurredAt:            rfc3339(r.OccurredAt),
		})
	}
	return &TechnicianFillReportResponse{
		OrganizationID: uuidStr(q.OrganizationID),
		From:           rfc3339(q.From),
		To:             rfc3339(q.To),
		Meta:           reportMeta(q, len(items), total),
		Items:          items,
	}, nil
}

func (s *Service) CommandFailures(ctx context.Context, q listscope.ReportingQuery) (*CommandFailuresReportResponse, error) {
	total, err := s.q.ReportingCommandFailuresCount(ctx, db.ReportingCommandFailuresCountParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
		Column4:        q.SiteIDFilter,
		Column5:        q.MachineIDFilter,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting command failures count: %w", err)
	}
	rows, err := s.q.ReportingCommandFailures(ctx, db.ReportingCommandFailuresParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
		Column4:        q.SiteIDFilter,
		Column5:        q.MachineIDFilter,
		Limit:          q.Limit,
		Offset:         q.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting command failures: %w", err)
	}
	items := make([]CommandFailureRow, 0, len(rows))
	for _, r := range rows {
		items = append(items, CommandFailureRow{
			AttemptID:     uuidStr(r.AttemptID),
			CommandID:     uuidStr(r.CommandID),
			MachineID:     uuidStr(r.MachineID),
			SiteID:        uuidStr(r.SiteID),
			AttemptNo:     r.AttemptNo,
			SentAt:        rfc3339(r.SentAt),
			Status:        r.Status,
			TimeoutReason: r.TimeoutReason,
		})
	}
	return &CommandFailuresReportResponse{
		OrganizationID: uuidStr(q.OrganizationID),
		From:           rfc3339(q.From),
		To:             rfc3339(q.To),
		Meta:           reportMeta(q, len(items), total),
		Items:          items,
	}, nil
}

func (s *Service) ReconciliationBI(ctx context.Context, q listscope.ReportingQuery) (*ReconciliationBIReportResponse, error) {
	scope := q.ReconciliationScope
	if scope == "" {
		scope = "all"
	}
	summary, err := s.q.ReportingReconciliationSummary(ctx, db.ReportingReconciliationSummaryParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting reconciliation summary: %w", err)
	}
	total, err := s.q.ReportingReconciliationCasesCount(ctx, db.ReportingReconciliationCasesCountParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
		Column4:        scope,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting reconciliation cases count: %w", err)
	}
	rows, err := s.q.ReportingReconciliationCases(ctx, db.ReportingReconciliationCasesParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
		Column4:        scope,
		Limit:          q.Limit,
		Offset:         q.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting reconciliation cases: %w", err)
	}
	items := make([]ReconciliationBIItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, ReconciliationBIItem{
			ID:              uuidStr(r.ID),
			CaseType:        r.CaseType,
			Status:          r.Status,
			Severity:        r.Severity,
			OrderID:         uuidPtr(r.OrderID),
			PaymentID:       uuidPtr(r.PaymentID),
			VendSessionID:   uuidPtr(r.VendSessionID),
			RefundID:        uuidPtr(r.RefundID),
			Provider:        textPtr(r.Provider),
			Reason:          r.Reason,
			FirstDetectedAt: rfc3339(r.FirstDetectedAt),
			LastDetectedAt:  rfc3339(r.LastDetectedAt),
			ResolvedAt:      timeStringPtr(r.ResolvedAt),
		})
	}
	return &ReconciliationBIReportResponse{
		OrganizationID: uuidStr(q.OrganizationID),
		From:           rfc3339(q.From),
		To:             rfc3339(q.To),
		Scope:          scope,
		Summary: ReconciliationBISummary{
			OpenCases:   summary.OpenCount,
			ClosedCases: summary.ClosedCount,
		},
		Meta:  reportMeta(q, len(items), total),
		Items: items,
	}, nil
}

func (s *Service) ProductPerformance(ctx context.Context, q listscope.ReportingQuery) (*ProductPerformanceResponse, error) {
	rows, err := s.q.ReportingSalesByProduct(ctx, db.ReportingSalesByProductParams{
		OrganizationID: q.OrganizationID,
		Column2:        q.From,
		Column3:        q.To,
		Column4:        q.SiteIDFilter,
		Column5:        q.MachineIDFilter,
		Column6:        q.ProductIDFilter,
	})
	if err != nil {
		return nil, fmt.Errorf("reporting product performance: %w", err)
	}
	items := make([]ProductPerformanceRow, 0, len(rows))
	for _, r := range rows {
		items = append(items, ProductPerformanceRow{
			ProductID:             uuidStr(r.ProductID),
			ProductSku:            r.ProductSku,
			ProductName:           r.ProductName,
			SuccessVends:          r.SuccessVends,
			FailedVends:           r.FailedVends,
			AllocatedRevenueMinor: r.AllocatedRevenueMinor,
		})
	}
	return &ProductPerformanceResponse{
		OrganizationID: uuidStr(q.OrganizationID),
		From:           rfc3339(q.From),
		To:             rfc3339(q.To),
		Items:          items,
	}, nil
}

func pgInt32Ptr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	x := v.Int32
	return &x
}

func reportMeta(q listscope.ReportingQuery, returned int, total int64) InventoryExceptionsListMeta {
	return InventoryExceptionsListMeta{
		Limit:    q.Limit,
		Offset:   q.Offset,
		Returned: returned,
		Total:    total,
	}
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
