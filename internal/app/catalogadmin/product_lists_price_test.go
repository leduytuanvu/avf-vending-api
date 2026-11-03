package catalogadmin

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestResolveListUnitPrice_explicitZeroWins(t *testing.T) {
	t.Parallel()
	zero := int64(0)
	got, err := resolveListUnitPrice(&zero, pgtype.Int8{Int64: 15000, Valid: true}, 9000, true)
	require.NoError(t, err)
	require.True(t, got.Valid)
	require.Equal(t, int64(0), got.Int64)
}

func TestResolveListUnitPrice_keepsExistingWhenOmitted(t *testing.T) {
	t.Parallel()
	got, err := resolveListUnitPrice(nil, pgtype.Int8{Int64: 15000, Valid: true}, 9000, true)
	require.NoError(t, err)
	require.Equal(t, int64(15000), got.Int64)
}

func TestResolveListUnitPrice_copiesDefaultWhenNew(t *testing.T) {
	t.Parallel()
	got, err := resolveListUnitPrice(nil, pgtype.Int8{}, 9000, true)
	require.NoError(t, err)
	require.Equal(t, int64(9000), got.Int64)
}

func TestResolveListUnitPrice_nullWhenNoDefault(t *testing.T) {
	t.Parallel()
	got, err := resolveListUnitPrice(nil, pgtype.Int8{}, 0, false)
	require.NoError(t, err)
	require.False(t, got.Valid)
}

func TestResolveListUnitPrice_rejectsNegative(t *testing.T) {
	t.Parallel()
	neg := int64(-1)
	_, err := resolveListUnitPrice(&neg, pgtype.Int8{}, 0, false)
	require.ErrorIs(t, err, ErrInvalidArgument)
}
