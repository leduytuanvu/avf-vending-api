package pricingengine

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestDedupeUUIDs_dropsNilAndDuplicates(t *testing.T) {
	t.Parallel()
	a := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	b := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	out := dedupeUUIDs([]uuid.UUID{a, a, uuid.Nil, b, b})
	require.Equal(t, []uuid.UUID{a, b}, out)
}

func TestLoadProductPromoMetaBatch_emptyInput(t *testing.T) {
	t.Parallel()
	out, err := LoadProductPromoMetaBatch(nil, nil, nil)
	require.NoError(t, err)
	require.Empty(t, out)
}
