package featureflags_test

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/app/featureflags"
	"github.com/google/uuid"
)

func TestCanaryHit_bounds(t *testing.T) {
	mid := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	fid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	if featureflags.CanaryHit(mid, fid, 0) {
		t.Fatal("0% should never hit")
	}
	if !featureflags.CanaryHit(mid, fid, 100) {
		t.Fatal("100% should always hit")
	}
}
