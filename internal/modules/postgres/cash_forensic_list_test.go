package postgres

import (
	"testing"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/google/uuid"
)

func TestCashLedgerFetchLimit(t *testing.T) {
	t.Parallel()
	if got := cashLedgerFetchLimit(50, 0); got != 50 {
		t.Fatalf("offset 0: got %d", got)
	}
	if got := cashLedgerFetchLimit(50, 50); got != 100 {
		t.Fatalf("page 2 window: got %d", got)
	}
}

func TestSliceCashLedgerPage(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 11, 2, 8, 0, 0, 0, time.UTC)
	ids := []string{
		"00000000-0000-0000-0000-000000000001",
		"00000000-0000-0000-0000-000000000002",
		"00000000-0000-0000-0000-000000000003",
		"00000000-0000-0000-0000-000000000004",
		"00000000-0000-0000-0000-000000000005",
	}
	items := make([]appcommerce.CashLedgerRow, len(ids))
	for i, id := range ids {
		items[i] = appcommerce.CashLedgerRow{
			ID:               uuid.MustParse(id),
			OccurredAtDevice: base.Add(time.Duration(i) * time.Minute),
		}
	}

	page1 := sliceCashLedgerPage(items, 0, 2)
	if len(page1) != 2 || page1[0].ID != items[0].ID || page1[1].ID != items[1].ID {
		t.Fatalf("page 1: %+v", page1)
	}
	page2 := sliceCashLedgerPage(items, 2, 2)
	if len(page2) != 2 || page2[0].ID != items[2].ID || page2[1].ID != items[3].ID {
		t.Fatalf("page 2: %+v", page2)
	}
	if page1[0].ID == page2[0].ID || page1[1].ID == page2[0].ID {
		t.Fatal("page 1 and page 2 share rows")
	}
	empty := sliceCashLedgerPage(items, 50, 2)
	if len(empty) != 0 {
		t.Fatalf("offset past end: %+v", empty)
	}
}
