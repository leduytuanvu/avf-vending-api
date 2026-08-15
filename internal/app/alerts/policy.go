// Package alerts defines the stable incident grouping and notification policy.
package alerts

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// RepeatMode controls how often Telegram intents are created for a grouped incident.
type RepeatMode string

const (
	// RepeatEvery queues a Telegram intent for every new logical occurrence (default).
	RepeatEvery RepeatMode = "every"
	// RepeatAggregate queues the first alert immediately, then summary alerts after cooldown
	// using last_alerted_at (never discards occurrence rows).
	RepeatAggregate RepeatMode = "aggregate"
)

// Incident contains the fields used to decide whether a machine incident should alert.
type Incident struct {
	MachineID    string
	OccurrenceID string
	Severity     string
	Code         string
	Title        string
	DedupeKey    string
	Detail       []byte
	Transport    string
	EventType    string
	OccurredAt   time.Time
	Source       string // app|server — set by backend, never trusted from device for bot routing
}

// Decision is the normalized incident and its notification decision.
type Decision struct {
	Incident
	GroupKey    string
	ShouldAlert bool
}

// Policy controls which severities notify and how frequently a group may notify.
type Policy struct {
	Cooldown   time.Duration
	RepeatMode RepeatMode
}

// DefaultPolicy provides every-occurrence paging for high/critical incidents.
func DefaultPolicy() Policy {
	return Policy{Cooldown: 15 * time.Minute, RepeatMode: RepeatEvery}
}

// NormalizeRepeatMode coerces env strings; empty/unknown → every.
func NormalizeRepeatMode(v string) RepeatMode {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case string(RepeatAggregate), "agg", "cooldown":
		return RepeatAggregate
	default:
		return RepeatEvery
	}
}

// DecideForOccurrence decides whether a NEW logical occurrence should create a Telegram intent.
// previousLastAlerted is the group's last_alerted_at (nil if never alerted). It must NOT be updated_at.
// isNewOccurrence false means transport replay — never alert again for the same occurrence.
func (p Policy) DecideForOccurrence(in Incident, isNewOccurrence bool, previousLastAlerted *time.Time, now time.Time) Decision {
	in.Severity = NormalizeSeverity(in.Severity)
	in.Code = strings.TrimSpace(in.Code)
	in.Title = strings.TrimSpace(in.Title)
	in.DedupeKey = strings.TrimSpace(in.DedupeKey)
	in.OccurrenceID = strings.TrimSpace(in.OccurrenceID)
	if in.DedupeKey == "" {
		in.DedupeKey = Fingerprint(in)
	}
	if p.Cooldown <= 0 {
		p.Cooldown = DefaultPolicy().Cooldown
	}
	if p.RepeatMode == "" {
		p.RepeatMode = RepeatEvery
	}

	should := alertsForSeverity(in.Severity) && isNewOccurrence
	if should && p.RepeatMode == RepeatAggregate {
		if previousLastAlerted != nil && now.UTC().Sub(previousLastAlerted.UTC()) < p.Cooldown {
			should = false
		}
	}

	return Decision{
		Incident:    in,
		GroupKey:    in.Code,
		ShouldAlert: should,
	}
}

// Decide is retained for tests/compat. Prefer DecideForOccurrence.
// When previousUpdatedAt is non-nil it is treated as last_alerted_at for aggregate mode only;
// every mode always alerts when severity qualifies (caller must pass isNewOccurrence semantics).
func (p Policy) Decide(in Incident, previousUpdatedAt *time.Time, now time.Time) Decision {
	mode := p.RepeatMode
	if mode == "" {
		mode = RepeatEvery
	}
	p.RepeatMode = mode
	// Legacy callers used updated_at as cooldown clock. In every mode we ignore it so
	// continuous repeats are not starved; treat as new occurrence.
	if mode == RepeatEvery {
		return p.DecideForOccurrence(in, true, nil, now)
	}
	return p.DecideForOccurrence(in, true, previousUpdatedAt, now)
}

// NormalizeSeverity accepts Cursor and device aliases while preserving the database's simple vocabulary.
func NormalizeSeverity(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "critical", "fatal", "panic":
		return "critical"
	case "high", "error", "err":
		return "high"
	case "medium", "warning", "warn":
		return "medium"
	case "low", "info", "information", "debug":
		return "low"
	default:
		return "medium"
	}
}

func alertsForSeverity(severity string) bool {
	return severity == "high" || severity == "critical"
}

// Fingerprint returns a stable group key when a device did not provide fingerprint/dedupe_key.
// Incidents are already scoped by machine in storage, so code and title are sufficient grouping inputs.
func Fingerprint(in Incident) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(in.Code) + "\n" + strings.TrimSpace(in.Title)))
	return hex.EncodeToString(sum[:])
}

// IsProjectableIncidentEventType reports whether a telemetry event_type should project into
// machine_incident_occurrences / machine_incidents / Telegram app alerts.
func IsProjectableIncidentEventType(eventType string) bool {
	t := strings.ToLower(strings.TrimSpace(eventType))
	switch t {
	case "incident_hardware_fault",
		"incident_payment_mismatch",
		"incident_peripheral_disconnected",
		"incident_serial_fault",
		"incident_process_crash",
		"incident_timeout",
		"incident_anr",
		"incident_sell_gate_blocked",
		"incident_runtime_error",
		"incident":
		return true
	}
	if strings.HasPrefix(t, "incident_") {
		return true
	}
	if strings.HasPrefix(t, "incident.") || strings.HasPrefix(t, "alert.") {
		return true
	}
	if t == "telemetry.incident" || strings.HasPrefix(t, "telemetry.incident.") {
		return true
	}
	return false
}

// AlertSource constants for trusted bot routing (set by backend only).
const (
	SourceApp    = "app"
	SourceServer = "server"
)
