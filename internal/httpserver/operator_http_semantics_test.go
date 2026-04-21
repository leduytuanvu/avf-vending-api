package httpserver

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/domain/fleet"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/google/uuid"
)

func TestParseOperatorLogoutFinalStatus(t *testing.T) {
	cases := []struct {
		raw  string
		want string
		err  bool
	}{
		{"", domainoperator.SessionStatusEnded, false},
		{"  ended  ", domainoperator.SessionStatusEnded, false},
		{"ENDED", domainoperator.SessionStatusEnded, false},
		{"revoked", domainoperator.SessionStatusRevoked, false},
		{"REVOKED", domainoperator.SessionStatusRevoked, false},
		{"ACTIVE", "", true},
	}
	for _, tc := range cases {
		got, err := parseOperatorLogoutFinalStatus(tc.raw)
		if tc.err {
			if !errors.Is(err, domainoperator.ErrInvalidSessionEndStatus) {
				t.Fatalf("%q: want ErrInvalidSessionEndStatus, got %v", tc.raw, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", tc.raw, err)
		}
		if got != tc.want {
			t.Fatalf("%q: got %q want %q", tc.raw, got, tc.want)
		}
	}
}

func TestBuildSetupBootstrapV1_groupsSlotsByCabinet(t *testing.T) {
	machineID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	siteID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	cabID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	layID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	cfgID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	prodID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	assortID := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	now := time.Unix(1700000000, 0).UTC()
	idx := int32(0)
	b := setupapp.MachineBootstrap{
		Machine: fleet.Machine{
			ID:              machineID,
			OrganizationID:  orgID,
			SiteID:          siteID,
			SerialNumber:    "SN-1",
			Name:            "Test",
			Status:          "online",
			CommandSequence: 9,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		Cabinets: []setupapp.CabinetView{
			{ID: cabID, MachineID: machineID, Code: "A", Title: "Left", SortOrder: 1, Metadata: []byte(`{}`), CreatedAt: now, UpdatedAt: now},
		},
		AssortmentProducts: []setupapp.AssortmentProductView{
			{ProductID: prodID, SKU: "sku-1", Name: "Cola", SortOrder: 1, AssortmentID: assortID, AssortmentName: "Primary"},
		},
		CurrentCabinetSlots: []setupapp.CabinetSlotConfigView{
			{
				ConfigID: cfgID, CabinetCode: "A", SlotCode: "S1", SlotIndex: &idx,
				ProductID: &prodID, ProductSKU: "sku-1", ProductName: "Cola",
				MaxQuantity: 6, PriceMinor: 150, EffectiveFrom: now,
				IsCurrent: true, MachineSlotLayout: layID,
			},
		},
	}
	out := buildSetupBootstrapV1(b)
	if len(out.Topology.Cabinets) != 1 {
		t.Fatalf("cabinet count %d", len(out.Topology.Cabinets))
	}
	if len(out.Topology.Cabinets[0].Slots) != 1 {
		t.Fatalf("slot count %d", len(out.Topology.Cabinets[0].Slots))
	}
	if len(out.Catalog.Products) != 1 {
		t.Fatalf("catalog count %d", len(out.Catalog.Products))
	}
	if out.Catalog.Products[0].Sku != "sku-1" {
		t.Fatalf("catalog sku %q", out.Catalog.Products[0].Sku)
	}
}

func TestParseOperatorListLimit(t *testing.T) {
	cases := []struct {
		query string
		want  int32
		err   bool
	}{
		{"", operatorListLimitDefault, false},
		{"limit=", operatorListLimitDefault, false},
		{"limit=10", 10, false},
		{"limit=501", operatorListLimitMax, false},
		{"limit=500", 500, false},
		{"limit=0", 0, true},
		{"limit=-1", 0, true},
		{"limit=abc", 0, true},
	}
	for _, tc := range cases {
		req, err := http.NewRequest(http.MethodGet, "/?"+tc.query, nil)
		if err != nil {
			t.Fatal(err)
		}
		got, lerr := parseOperatorListLimit(req)
		if tc.err {
			if lerr == nil {
				t.Fatalf("query %q: want error", tc.query)
			}
			continue
		}
		if lerr != nil {
			t.Fatalf("query %q: %v", tc.query, lerr)
		}
		if got != tc.want {
			t.Fatalf("query %q: got %d want %d", tc.query, got, tc.want)
		}
	}
}
