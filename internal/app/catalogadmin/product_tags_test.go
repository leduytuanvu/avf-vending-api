package catalogadmin

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNormalizeDedupeProductTagIDs(t *testing.T) {
	t.Parallel()
	a := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	b := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	out, err := normalizeDedupeProductTagIDs([]uuid.UUID{a, b, a})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{a, b}, out)

	_, err = normalizeDedupeProductTagIDs([]uuid.UUID{uuid.Nil})
	require.ErrorIs(t, err, ErrInvalidArgument)
}
