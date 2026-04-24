package httpserver

import (
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestV1CashCollectionFromDB_reviewFieldsClosed(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	oid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	row := db.CashCollection{
		ID:                   id,
		MachineID:            mid,
		OrganizationID:       oid,
		CollectedAt:          time.Unix(100, 0).UTC(),
		OpenedAt:             time.Unix(100, 0).UTC(),
		ClosedAt:             pgtype.Timestamptz{Time: time.Unix(200, 0).UTC(), Valid: true},
		LifecycleStatus:      "closed",
		AmountMinor:          1250,
		ExpectedAmountMinor:  1200,
		VarianceAmountMinor:  50,
		RequiresReview:       false,
		Currency:             "USD",
		ReconciliationStatus: "mismatch",
	}
	out := v1CashCollectionFromDB(row)
	if out.CountedPhysicalCashMinor != 1250 || out.ExpectedCloudCashMinor != 1200 || out.VarianceMinor != 50 {
		t.Fatalf("physical/cloud/variance mapping: %+v", out)
	}
	if out.ReviewState != "variance_recorded" {
		t.Fatalf("reviewState=%q", out.ReviewState)
	}
}

func TestV1CashCollectionFromDB_reviewStateOpen(t *testing.T) {
	t.Parallel()
	row := db.CashCollection{
		ID:              uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		MachineID:       uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		OrganizationID:  uuid.MustParse("33333333-3333-3333-3333-333333333333"),
		CollectedAt:     time.Unix(100, 0).UTC(),
		OpenedAt:        time.Unix(100, 0).UTC(),
		LifecycleStatus: "open",
		Currency:        "USD",
		ReconciliationStatus: "pending",
	}
	out := v1CashCollectionFromDB(row)
	if out.ReviewState != "open" || out.CountedPhysicalCashMinor != 0 {
		t.Fatalf("open mapping: %+v", out)
	}
}
