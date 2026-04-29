package inventoryadmin

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

const (
	defaultVelocityWindowDays = 14
	minVelocityWindowDays     = 7
	maxVelocityWindowDays     = 90

	UrgencyCritical = "critical"
	UrgencyHigh     = "high"
	UrgencyMedium   = "medium"
	UrgencyLow      = "low"
)

// RefillForecastParams drives refill / low-stock list endpoints (tenant-scoped by OrganizationID).
type RefillForecastParams struct {
	OrganizationID uuid.UUID
	SiteID         *uuid.UUID
	MachineID      *uuid.UUID
	ProductID      *uuid.UUID

	VelocityWindowDays int
	LowStockOnly       bool

	DaysThreshold *float64
	UrgencyFilter string

	Limit  int32
	Offset int32
}

// RefillForecastResponse is GET /v1/admin/inventory/refill-suggestions (and low-stock).
type RefillForecastResponse struct {
	OrganizationID     string               `json:"organizationId"`
	VelocityWindowDays int                  `json:"velocityWindowDays"`
	WindowStart        string               `json:"windowStart"`
	WindowEnd          string               `json:"windowEnd"`
	Items              []RefillForecastItem `json:"items"`
	Meta               RefillForecastMeta   `json:"meta"`
}

// RefillForecastMeta pagination matches admin collection style (total after filters).
type RefillForecastMeta struct {
	Limit    int32 `json:"limit"`
	Offset   int32 `json:"offset"`
	Returned int   `json:"returned"`
	Total    int64 `json:"total"`
}

// RefillForecastItem is one slot-level suggestion row.
type RefillForecastItem struct {
	MachineID               string   `json:"machineId"`
	MachineName             string   `json:"machineName"`
	SiteID                  string   `json:"siteId"`
	SiteName                string   `json:"siteName"`
	PlanogramID             string   `json:"planogramId"`
	PlanogramName           string   `json:"planogramName"`
	SlotIndex               int32    `json:"slotIndex"`
	ProductID               string   `json:"productId"`
	ProductSku              string   `json:"productSku,omitempty"`
	ProductName             string   `json:"productName,omitempty"`
	CurrentQuantity         int32    `json:"currentQuantity"`
	MaxQuantity             int32    `json:"maxQuantity"`
	UnitsSoldInWindow       int64    `json:"unitsSoldInWindow"`
	DailyVelocity           float64  `json:"dailyVelocity"`
	DaysToEmpty             *float64 `json:"daysToEmpty,omitempty"`
	FillRatio               float64  `json:"fillRatio"`
	SuggestedRefillQuantity int32    `json:"suggestedRefillQuantity"`
	Urgency                 string   `json:"urgency"`
}

// ListRefillForecast loads slot rows with vend velocity, applies forecasting rules, filters, and paginates in-memory.
func (s *Service) ListRefillForecast(ctx context.Context, p RefillForecastParams) (*RefillForecastResponse, error) {
	if s == nil || s.q == nil {
		return nil, fmt.Errorf("inventoryadmin: nil service")
	}
	wd := p.VelocityWindowDays
	if wd <= 0 {
		wd = defaultVelocityWindowDays
	}
	if wd < minVelocityWindowDays {
		wd = minVelocityWindowDays
	}
	if wd > maxVelocityWindowDays {
		wd = maxVelocityWindowDays
	}
	windowDays := float64(wd)
	windowEnd := time.Now().UTC()
	windowStart := windowEnd.AddDate(0, 0, -wd)

	site := uuid.Nil
	if p.SiteID != nil {
		site = *p.SiteID
	}
	machine := uuid.Nil
	if p.MachineID != nil {
		machine = *p.MachineID
	}
	product := uuid.Nil
	if p.ProductID != nil {
		product = *p.ProductID
	}

	rows, err := s.q.InventoryAdminRefillForecastSlots(ctx, db.InventoryAdminRefillForecastSlotsParams{
		OrganizationID: p.OrganizationID,
		Column2:        windowStart,
		Column3:        windowEnd,
		Column4:        site,
		Column5:        machine,
		Column6:        product,
		Column7:        p.LowStockOnly,
	})
	if err != nil {
		return nil, fmt.Errorf("inventoryadmin refill forecast query: %w", err)
	}

	items := make([]RefillForecastItem, 0, len(rows))
	for _, row := range rows {
		pid := uuid.UUID(row.ProductID.Bytes)
		it := buildRefillForecastItem(row, windowDays, pid)
		if !passesRefillFilters(it, p) {
			continue
		}
		items = append(items, it)
	}

	sort.SliceStable(items, func(i, j int) bool {
		ri := urgencyRank(items[i].Urgency)
		rj := urgencyRank(items[j].Urgency)
		if ri != rj {
			return ri < rj
		}
		di := safeDays(items[i].DaysToEmpty)
		dj := safeDays(items[j].DaysToEmpty)
		if di != dj {
			return di < dj
		}
		if items[i].FillRatio != items[j].FillRatio {
			return items[i].FillRatio < items[j].FillRatio
		}
		return items[i].MachineName < items[j].MachineName
	})

	total := int64(len(items))
	lim := p.Limit
	if lim <= 0 {
		lim = 50
	}
	if lim > 500 {
		lim = 500
	}
	off := p.Offset
	if off < 0 {
		off = 0
	}
	start := int(off)
	if start > len(items) {
		start = len(items)
	}
	end := start + int(lim)
	if end > len(items) {
		end = len(items)
	}
	page := items[start:end]

	return &RefillForecastResponse{
		OrganizationID:     p.OrganizationID.String(),
		VelocityWindowDays: wd,
		WindowStart:        windowStart.UTC().Format(time.RFC3339Nano),
		WindowEnd:          windowEnd.UTC().Format(time.RFC3339Nano),
		Items:              page,
		Meta: RefillForecastMeta{
			Limit:    lim,
			Offset:   off,
			Returned: len(page),
			Total:    total,
		},
	}, nil
}

func buildRefillForecastItem(row db.InventoryAdminRefillForecastSlotsRow, windowDays float64, productID uuid.UUID) RefillForecastItem {
	cur := row.CurrentQuantity
	maxQ := row.MaxQuantity
	sold := row.UnitsSoldWindow

	daily := 0.0
	if windowDays > 0 {
		daily = float64(sold) / windowDays
	}

	fill := 1.0
	if maxQ > 0 {
		fill = float64(cur) / float64(maxQ)
		if fill > 1 {
			fill = 1
		}
	}

	var days *float64
	if cur <= 0 {
		z := 0.0
		days = &z
	} else if sold > 0 && windowDays > 0 {
		est := float64(cur) * windowDays / float64(sold)
		if math.IsInf(est, 0) || math.IsNaN(est) {
			days = nil
		} else {
			days = &est
		}
	}

	refill := int32(0)
	if maxQ > cur {
		refill = maxQ - cur
	}

	u := ComputeUrgency(cur, maxQ, fill, days)

	sku := ""
	if row.ProductSku.Valid {
		sku = row.ProductSku.String
	}
	pname := ""
	if row.ProductName.Valid {
		pname = row.ProductName.String
	}

	return RefillForecastItem{
		MachineID:               row.MachineID.String(),
		MachineName:             row.MachineName,
		SiteID:                  row.SiteID.String(),
		SiteName:                row.SiteName,
		PlanogramID:             row.PlanogramID.String(),
		PlanogramName:           row.PlanogramName,
		SlotIndex:               row.SlotIndex,
		ProductID:               productID.String(),
		ProductSku:              sku,
		ProductName:             pname,
		CurrentQuantity:         cur,
		MaxQuantity:             maxQ,
		UnitsSoldInWindow:       sold,
		DailyVelocity:           math.Round(daily*1e6) / 1e6,
		DaysToEmpty:             days,
		FillRatio:               math.Round(fill*1e4) / 1e4,
		SuggestedRefillQuantity: refill,
		Urgency:                 u,
	}
}

func passesRefillFilters(it RefillForecastItem, p RefillForecastParams) bool {
	if uf := strings.TrimSpace(strings.ToLower(p.UrgencyFilter)); uf != "" {
		if it.Urgency != uf {
			return false
		}
	}
	if p.DaysThreshold != nil {
		t := *p.DaysThreshold
		include := it.CurrentQuantity <= 0 ||
			it.FillRatio < 0.15 ||
			(it.DaysToEmpty != nil && *it.DaysToEmpty <= t)
		if !include {
			return false
		}
	}
	return true
}

// ComputeUrgency is exported for tests; tiers align days-to-empty and fill ratio.
func ComputeUrgency(current, maxQ int32, fillRatio float64, daysToEmpty *float64) string {
	if current <= 0 {
		return UrgencyCritical
	}
	if maxQ <= 0 {
		return UrgencyMedium
	}
	if daysToEmpty != nil {
		if *daysToEmpty <= 1 {
			return UrgencyCritical
		}
		if *daysToEmpty <= 3 {
			return UrgencyHigh
		}
		if *daysToEmpty <= 7 {
			return UrgencyMedium
		}
	}
	if fillRatio < 0.05 {
		return UrgencyHigh
	}
	if fillRatio < 0.15 {
		return UrgencyMedium
	}
	return UrgencyLow
}

func urgencyRank(u string) int {
	switch strings.ToLower(strings.TrimSpace(u)) {
	case UrgencyCritical:
		return 0
	case UrgencyHigh:
		return 1
	case UrgencyMedium:
		return 2
	default:
		return 3
	}
}

func safeDays(d *float64) float64 {
	if d == nil {
		return 1e12
	}
	return *d
}
