package catalogadmin

import (
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

func TestPickWinner_prefersHigherTier(t *testing.T) {
	t.Parallel()
	id1 := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	id2 := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	orgLow := pricedCandidate{
		tier:  1,
		book:  db.PriceBook{ID: id1, Priority: 100, EffectiveFrom: t0},
		price: 100,
	}
	machHigh := pricedCandidate{
		tier:  3,
		book:  db.PriceBook{ID: id2, Priority: 0, EffectiveFrom: t0},
		price: 200,
	}
	w := pickWinner([]pricedCandidate{orgLow, machHigh})
	if w == nil || w.tier != 3 || w.price != 200 {
		t.Fatalf("got %+v", w)
	}
}

func TestPickWinner_sameTierHigherPriority(t *testing.T) {
	t.Parallel()
	id1 := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	id2 := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	a := pricedCandidate{tier: 2, book: db.PriceBook{ID: id1, Priority: 1, EffectiveFrom: t0}, price: 50}
	b := pricedCandidate{tier: 2, book: db.PriceBook{ID: id2, Priority: 5, EffectiveFrom: t0}, price: 60}
	w := pickWinner([]pricedCandidate{a, b})
	if w == nil || w.book.Priority != 5 {
		t.Fatalf("got %+v", w)
	}
}
