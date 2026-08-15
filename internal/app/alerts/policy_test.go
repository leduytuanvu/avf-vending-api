package alerts

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPolicyEveryModeAlertsNewOccurrenceDespiteRecentLastSeen(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-1 * time.Minute)
	policy := Policy{Cooldown: 15 * time.Minute, RepeatMode: RepeatEvery}

	decision := policy.DecideForOccurrence(Incident{
		Severity:     "error",
		Code:         "incident_timeout",
		OccurrenceID: "incident_timeout:2",
		DedupeKey:    "TCN_TIMEOUT",
	}, true, &recent, now)

	require.Equal(t, "high", decision.Severity)
	require.True(t, decision.ShouldAlert, "every mode must alert each new occurrence")
}

func TestPolicyEveryModeDoesNotAlertTransportReplay(t *testing.T) {
	policy := DefaultPolicy()
	decision := policy.DecideForOccurrence(Incident{
		Severity: "critical", Code: "incident_anr", OccurrenceID: "a",
	}, false, nil, time.Now())
	require.False(t, decision.ShouldAlert)
}

func TestPolicyAggregateUsesLastAlertedNotUpdatedAt(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	lastAlerted := now.Add(-5 * time.Minute)
	policy := Policy{Cooldown: 15 * time.Minute, RepeatMode: RepeatAggregate}
	decision := policy.DecideForOccurrence(Incident{
		Severity: "high", Code: "incident_timeout", OccurrenceID: "b", DedupeKey: "fp",
	}, true, &lastAlerted, now)
	require.False(t, decision.ShouldAlert)
}

func TestPolicyAlertsNewHighIncident(t *testing.T) {
	decision := DefaultPolicy().Decide(Incident{Severity: "critical", Code: "disk.full"}, nil, time.Now())
	require.True(t, decision.ShouldAlert)
	require.NotEmpty(t, decision.DedupeKey)
}

func TestIsProjectableIncidentEventType(t *testing.T) {
	require.True(t, IsProjectableIncidentEventType("incident_hardware_fault"))
	require.True(t, IsProjectableIncidentEventType("incident_runtime_error"))
	require.True(t, IsProjectableIncidentEventType("incident.door"))
	require.False(t, IsProjectableIncidentEventType("vend_result"))
	require.False(t, IsProjectableIncidentEventType("payment_status_transition"))
}

func TestTelegramAppIdempotencyKey(t *testing.T) {
	require.Equal(t, "telegram:app:m1:occ-1", TelegramAppIdempotencyKey("m1", "occ-1"))
	require.Equal(t, "telegram:server:occ-9", TelegramServerIdempotencyKey("occ-9"))
}
