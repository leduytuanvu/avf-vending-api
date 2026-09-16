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
		Code:         "incident_sales_locked",
		OccurrenceID: "incident_sales_locked:2",
		DedupeKey:    "SALES_LOCKED",
	}, true, &recent, now)

	require.Equal(t, "high", decision.Severity)
	require.True(t, decision.ShouldAlert, "every mode must alert each new approved occurrence")
}

func TestPolicyEveryModeDoesNotAlertTransportReplay(t *testing.T) {
	policy := DefaultPolicy()
	decision := policy.DecideForOccurrence(Incident{
		Severity: "critical", Code: "incident_app_crashed", OccurrenceID: "a",
	}, false, nil, time.Now())
	require.False(t, decision.ShouldAlert)
}

func TestPolicyAggregateUsesLastAlertedNotUpdatedAt(t *testing.T) {
	now := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	lastAlerted := now.Add(-5 * time.Minute)
	policy := Policy{Cooldown: 15 * time.Minute, RepeatMode: RepeatAggregate}
	decision := policy.DecideForOccurrence(Incident{
		Severity: "high", Code: "incident_network_lost", OccurrenceID: "b", DedupeKey: "fp",
	}, true, &lastAlerted, now)
	require.False(t, decision.ShouldAlert)
}

func TestPolicyAlertsNewHighIncident(t *testing.T) {
	decision := DefaultPolicy().Decide(Incident{Severity: "critical", Code: "incident_sales_locked"}, nil, time.Now())
	require.True(t, decision.ShouldAlert)
	require.NotEmpty(t, decision.DedupeKey)
}

func TestShouldPageTelegram_approvedOnly(t *testing.T) {
	require.True(t, ShouldPageTelegram("incident_network_lost"))
	require.True(t, ShouldPageTelegram("incident_network_recovered"))
	require.False(t, ShouldPageTelegram("incident_runtime_error"))
	require.False(t, ShouldPageTelegram("incident_hardware_fault"))
	require.False(t, ShouldPageTelegram("disk.full"))
}

func TestPolicyLegacyHighIncidentNotPaged(t *testing.T) {
	decision := DefaultPolicy().DecideForOccurrence(Incident{
		Severity: "critical", Code: "incident_runtime_error", OccurrenceID: "x",
	}, true, nil, time.Now())
	require.False(t, decision.ShouldAlert)
}

func TestPolicyRecoveryMediumSeverityPages(t *testing.T) {
	decision := DefaultPolicy().DecideForOccurrence(Incident{
		Severity: "medium", Code: "incident_network_recovered", OccurrenceID: "r1",
	}, true, nil, time.Now())
	require.True(t, decision.ShouldAlert)
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
