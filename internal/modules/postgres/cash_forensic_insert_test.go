package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestCashMovementInsertResult(t *testing.T) {
	t.Parallel()
	inserted, err := cashMovementInsertResult(nil)
	if err != nil || !inserted {
		t.Fatalf("nil err should be insert, inserted=%v err=%v", inserted, err)
	}
	inserted, err = cashMovementInsertResult(pgx.ErrNoRows)
	if err != nil || inserted {
		t.Fatalf("ErrNoRows should be duplicate, inserted=%v err=%v", inserted, err)
	}
	boom := errors.New("db down")
	inserted, err = cashMovementInsertResult(boom)
	if inserted || !errors.Is(err, boom) {
		t.Fatalf("other err should propagate, inserted=%v err=%v", inserted, err)
	}
}
