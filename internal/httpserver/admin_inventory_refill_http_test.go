package httpserver

import (
	"net/http/httptest"
	"testing"

	appinventoryadmin "github.com/avf/avf-vending-api/internal/app/inventoryadmin"
	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestParseInventoryRefillForecastQuery_velocityDaysInvalid(t *testing.T) {
	org := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	req := httptest.NewRequest("GET", "/v1/admin/inventory/refill-suggestions?velocity_days=abc", nil)
	_, err := parseInventoryRefillForecastQuery(req, org, nil, false)
	require.ErrorIs(t, err, listscope.ErrInvalidListQuery)
}

func TestParseInventoryRefillForecastQuery_urgencyInvalid(t *testing.T) {
	org := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	req := httptest.NewRequest("GET", "/v1/admin/inventory/refill-suggestions?urgency=nope", nil)
	_, err := parseInventoryRefillForecastQuery(req, org, nil, false)
	require.ErrorIs(t, err, listscope.ErrInvalidListQuery)
}

func TestParseInventoryRefillForecastQuery_machinePathMismatch(t *testing.T) {
	org := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	pathMid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	queryMid := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	req := httptest.NewRequest("GET", "/?machine_id="+queryMid.String(), nil)
	_, err := parseInventoryRefillForecastQuery(req, org, &pathMid, false)
	require.ErrorIs(t, err, listscope.ErrInvalidListQuery)
}

func TestParseInventoryRefillForecastQuery_machinePathMatchesQuery(t *testing.T) {
	org := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	pathMid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	req := httptest.NewRequest("GET", "/?machine_id="+pathMid.String()+"&velocity_days=30", nil)
	p, err := parseInventoryRefillForecastQuery(req, org, &pathMid, false)
	require.NoError(t, err)
	require.NotNil(t, p.MachineID)
	require.Equal(t, pathMid, *p.MachineID)
	require.Equal(t, 30, p.VelocityWindowDays)
	require.False(t, p.LowStockOnly)
}

func TestParseInventoryRefillForecastQuery_lowStockFlag(t *testing.T) {
	org := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	req := httptest.NewRequest("GET", "/", nil)
	p, err := parseInventoryRefillForecastQuery(req, org, nil, true)
	require.NoError(t, err)
	require.True(t, p.LowStockOnly)
}

func TestParseInventoryRefillForecastQuery_urgencyLowercases(t *testing.T) {
	org := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	req := httptest.NewRequest("GET", "/?urgency=HIGH", nil)
	p, err := parseInventoryRefillForecastQuery(req, org, nil, false)
	require.NoError(t, err)
	require.Equal(t, appinventoryadmin.UrgencyHigh, p.UrgencyFilter)
}
