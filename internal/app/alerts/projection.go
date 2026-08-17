package alerts

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ProjectInput is the transport-agnostic incident projection request.
type ProjectInput struct {
	MachineID    string
	OccurrenceID string
	Fingerprint  string
	Severity     string
	Code         string
	Title        string
	EventType    string
	Transport    string // grpc|mqtt|http
	Detail       []byte
	OccurredAt   time.Time
}

// ProjectResult summarizes idempotent projection outcomes.
type ProjectResult struct {
	NewOccurrence   bool
	AlertQueued     bool
	OccurrenceCount int64
	TransportDup    bool
}

// IncidentProjector persists occurrences and optionally queues Telegram intents.
type IncidentProjector interface {
	ProjectMachineIncident(ctx context.Context, in ProjectInput, policy Policy) (ProjectResult, error)
}

// ResolveOccurrenceID picks the logical occurrence id without using fingerprint as identity.
func ResolveOccurrenceID(primary string, fallbacks ...string) string {
	if v := strings.TrimSpace(primary); v != "" {
		return v
	}
	for _, f := range fallbacks {
		if v := strings.TrimSpace(f); v != "" {
			return v
		}
	}
	return ""
}

// TelegramAppIdempotencyKey builds the outbox idempotency key for an app occurrence.
func TelegramAppIdempotencyKey(machineID, occurrenceID string) string {
	return fmt.Sprintf("telegram:app:%s:%s", strings.TrimSpace(machineID), strings.TrimSpace(occurrenceID))
}

// TelegramServerIdempotencyKey builds the outbox idempotency key for a server alert.
func TelegramServerIdempotencyKey(occurrenceID string) string {
	return fmt.Sprintf("telegram:server:%s", strings.TrimSpace(occurrenceID))
}

// BuildIncidentFromPayload maps parsed payload fields into a ProjectInput.
func BuildIncidentFromPayload(machineID, transport string, occurrenceID string, severity, code, title, dedupe string, detail []byte, occurredAt time.Time) ProjectInput {
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	if occurrenceID == "" {
		// Degraded legacy path: synthesize a unique occurrence so fingerprint is never the occurrence key.
		occurrenceID = "legacy:" + uuid.NewString()
	}
	et := code
	return ProjectInput{
		MachineID:    machineID,
		OccurrenceID: occurrenceID,
		Fingerprint:  dedupe,
		Severity:     severity,
		Code:         code,
		Title:        title,
		EventType:    et,
		Transport:    transport,
		Detail:       detail,
		OccurredAt:   occurredAt,
	}
}

// ExtractOccurrenceIDFromDetail walks common App envelope shapes for event_id.
func ExtractOccurrenceIDFromDetail(detail []byte) string {
	if len(detail) == 0 {
		return ""
	}
	var root map[string]any
	if err := json.Unmarshal(detail, &root); err != nil {
		return ""
	}
	candidates := []string{"event_id", "eventId", "occurrence_id", "occurrenceId"}
	for _, k := range candidates {
		if v, ok := root[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	if payload, ok := root["payload"].(map[string]any); ok {
		for _, k := range candidates {
			if v, ok := payload[k].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	if attrs, ok := root["attributes"].(map[string]any); ok {
		for _, k := range candidates {
			if v, ok := attrs[k].(string); ok && strings.TrimSpace(v) != "" {
				return strings.TrimSpace(v)
			}
		}
	}
	return ""
}
