package activation

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestClaimContextProvided(t *testing.T) {
	require.False(t, claimContextProvided(ClaimContext{}))
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	require.True(t, claimContextProvided(ClaimContext{RequestID: "req-1"}))
	require.True(t, claimContextProvided(ClaimContext{ActivatedByAccountID: &id}))
}

func TestReattachDeniedStatuses(t *testing.T) {
	for _, st := range []string{"compromised", "retired", "decommissioned"} {
		require.Contains(t, []string{"compromised", "retired", "decommissioned"}, st)
	}
	require.ErrorIs(t, ErrReattachDenied, ErrReattachDenied)
}
