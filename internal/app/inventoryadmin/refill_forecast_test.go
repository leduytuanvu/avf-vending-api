package inventoryadmin

import (
	"math"
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestComputeUrgency_emptyStock_critical(t *testing.T) {
	got := ComputeUrgency(0, 10, 0.5, ptrFloat(5))
	require.Equal(t, UrgencyCritical, got)
}

func TestComputeUrgency_daysToEmpty_criticalHighMedium(t *testing.T) {
	require.Equal(t, UrgencyCritical, ComputeUrgency(5, 10, 0.5, ptrFloat(0.5)))
	require.Equal(t, UrgencyHigh, ComputeUrgency(5, 10, 0.5, ptrFloat(2)))
	require.Equal(t, UrgencyMedium, ComputeUrgency(5, 10, 0.5, ptrFloat(5)))
	require.Equal(t, UrgencyLow, ComputeUrgency(5, 10, 0.5, ptrFloat(30)))
}

func TestComputeUrgency_noDays_usesFillRatio(t *testing.T) {
	require.Equal(t, UrgencyHigh, ComputeUrgency(2, 100, 0.02, nil))
	require.Equal(t, UrgencyMedium, ComputeUrgency(10, 100, 0.10, nil))
	require.Equal(t, UrgencyLow, ComputeUrgency(50, 100, 0.5, nil))
}

func TestComputeUrgency_maxQuantityZero(t *testing.T) {
	require.Equal(t, UrgencyMedium, ComputeUrgency(3, 0, 1.0, ptrFloat(10)))
}

func TestBuildRefillForecastItem_zeroSales_noDivideByZero_noDays(t *testing.T) {
	pid := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	row := db.InventoryAdminRefillForecastSlotsRow{
		MachineID:       uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		MachineName:     "M1",
		SiteID:          uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
		SiteName:        "S",
		PlanogramID:     uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"),
		PlanogramName:   "P",
		SlotIndex:       0,
		ProductID:       pgtype.UUID{Bytes: pid, Valid: true},
		CurrentQuantity: 8,
		MaxQuantity:     10,
		UnitsSoldWindow: 0,
	}
	it := buildRefillForecastItem(row, 14.0, pid)
	require.Equal(t, float64(0), it.DailyVelocity)
	require.Nil(t, it.DaysToEmpty)
	require.Equal(t, int32(2), it.SuggestedRefillQuantity)
	require.False(t, math.IsInf(it.DailyVelocity, 0))
}

func TestBuildRefillForecastItem_daysToEmpty_and_refillCap(t *testing.T) {
	pid := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	row := db.InventoryAdminRefillForecastSlotsRow{
		MachineID:       uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
		MachineName:     "M1",
		SiteID:          uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"),
		SiteName:        "S",
		PlanogramID:     uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd"),
		PlanogramName:   "P",
		SlotIndex:       1,
		ProductID:       pgtype.UUID{Bytes: pid, Valid: true},
		CurrentQuantity: 7,
		MaxQuantity:     10,
		UnitsSoldWindow: 14,
	}
	it := buildRefillForecastItem(row, 14.0, pid)
	require.NotNil(t, it.DaysToEmpty)
	require.InDelta(t, 7.0, *it.DaysToEmpty, 1e-9)
	require.Equal(t, int32(3), it.SuggestedRefillQuantity)
}

func TestPassesRefillFilters_daysThreshold(t *testing.T) {
	th := 5.0
	p := RefillForecastParams{DaysThreshold: &th}
	ok := RefillForecastItem{CurrentQuantity: 10, FillRatio: 0.5, DaysToEmpty: ptrFloat(4)}
	require.True(t, passesRefillFilters(ok, p))
	bad := RefillForecastItem{CurrentQuantity: 10, FillRatio: 0.5, DaysToEmpty: ptrFloat(10)}
	require.False(t, passesRefillFilters(bad, p))
	empty := RefillForecastItem{CurrentQuantity: 0, FillRatio: 0, DaysToEmpty: ptrFloat(100)}
	require.True(t, passesRefillFilters(empty, p))
}

func TestPassesRefillFilters_urgencyFilter(t *testing.T) {
	p := RefillForecastParams{UrgencyFilter: UrgencyHigh}
	require.True(t, passesRefillFilters(RefillForecastItem{Urgency: UrgencyHigh}, p))
	require.False(t, passesRefillFilters(RefillForecastItem{Urgency: UrgencyLow}, p))
}

func ptrFloat(f float64) *float64 {
	return &f
}
