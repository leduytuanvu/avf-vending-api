package operator

import (
	"testing"
	"time"

	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMergeMachineTimeline_NewestFirstAndLimit(t *testing.T) {
	t0 := time.Date(2026, 1, 2, 15, 0, 0, 0, time.UTC)
	t1 := t0.Add(-time.Minute)
	sid := uuid.MustParse("77777777-7777-7777-7777-777777777777")
	mid := uuid.MustParse("55555555-5555-5555-5555-555555555555")

	auth := []domainoperator.AuthEvent{
		{ID: 1, MachineID: mid, EventType: domainoperator.AuthEventLoginSuccess, AuthMethod: domainoperator.AuthMethodOIDC, OccurredAt: t1},
	}
	attr := []domainoperator.ActionAttribution{
		{ID: 10, MachineID: mid, ActionOriginType: domainoperator.ActionOriginOperatorSession, ResourceType: "refill_sessions", ResourceID: sid.String(), OccurredAt: t0},
	}
	out := mergeMachineTimeline(auth, attr, nil, 2)
	require.Len(t, out, 2)
	require.Equal(t, "operator_action", out[0].Kind)
	require.Equal(t, "operator_auth", out[1].Kind)
}
