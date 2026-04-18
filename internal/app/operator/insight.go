package operator

import (
	"encoding/json"
	"sort"
	"time"

	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
)

// TimelineItem is a single point on the machine operator timeline (newest items appear first in API lists).
type TimelineItem struct {
	OccurredAt time.Time `json:"occurred_at"`
	Kind       string    `json:"kind"`
	Detail     any       `json:"detail"`
}

// AuthEventView is a JSON-friendly projection of domainoperator.AuthEvent.
type AuthEventView struct {
	ID                int64           `json:"id"`
	OperatorSessionID *string         `json:"operator_session_id,omitempty"`
	EventType         string          `json:"event_type"`
	AuthMethod        string          `json:"auth_method"`
	CorrelationID     *string         `json:"correlation_id,omitempty"`
	Metadata          json.RawMessage `json:"metadata"`
}

// ActionAttributionView projects machine_action_attributions for operators.
type ActionAttributionView struct {
	ID                int64           `json:"id"`
	OperatorSessionID *string         `json:"operator_session_id,omitempty"`
	ActionOriginType  string          `json:"action_origin_type"`
	ResourceType      string          `json:"resource_type"`
	ResourceID        string          `json:"resource_id"`
	CorrelationID     *string         `json:"correlation_id,omitempty"`
	Metadata          json.RawMessage `json:"metadata"`
}

// SessionTimelineView marks session lifecycle points (distinct from auth events, which carry auth_method).
type SessionTimelineView struct {
	SessionID   string  `json:"session_id"`
	Phase       string  `json:"phase"` // started | ended
	ActorType   string  `json:"actor_type"`
	Status      string  `json:"status,omitempty"`
	EndedReason *string `json:"ended_reason,omitempty"`
}

// AuthEventViewFromDomain maps a domain auth event for JSON APIs.
func AuthEventViewFromDomain(e domainoperator.AuthEvent) AuthEventView {
	return authEventView(e)
}

func authEventView(e domainoperator.AuthEvent) AuthEventView {
	v := AuthEventView{
		ID:         e.ID,
		EventType:  e.EventType,
		AuthMethod: e.AuthMethod,
		Metadata:   json.RawMessage(e.Metadata),
	}
	if e.OperatorSessionID != nil {
		s := e.OperatorSessionID.String()
		v.OperatorSessionID = &s
	}
	if e.CorrelationID != nil {
		s := e.CorrelationID.String()
		v.CorrelationID = &s
	}
	if len(v.Metadata) == 0 {
		v.Metadata = []byte("{}")
	}
	return v
}

// ActionAttributionViewFromDomain maps a domain attribution row for JSON APIs.
func ActionAttributionViewFromDomain(a domainoperator.ActionAttribution) ActionAttributionView {
	return actionAttributionView(a)
}

func actionAttributionView(a domainoperator.ActionAttribution) ActionAttributionView {
	v := ActionAttributionView{
		ID:               a.ID,
		ActionOriginType: a.ActionOriginType,
		ResourceType:     a.ResourceType,
		ResourceID:       a.ResourceID,
		Metadata:         json.RawMessage(a.Metadata),
	}
	if a.OperatorSessionID != nil {
		s := a.OperatorSessionID.String()
		v.OperatorSessionID = &s
	}
	if a.CorrelationID != nil {
		s := a.CorrelationID.String()
		v.CorrelationID = &s
	}
	if len(v.Metadata) == 0 {
		v.Metadata = []byte("{}")
	}
	return v
}

func mergeMachineTimeline(
	auth []domainoperator.AuthEvent,
	attr []domainoperator.ActionAttribution,
	sessions []domainoperator.Session,
	maxItems int32,
) []TimelineItem {
	if maxItems <= 0 {
		maxItems = 50
	}
	var items []TimelineItem
	for _, e := range auth {
		items = append(items, TimelineItem{
			OccurredAt: e.OccurredAt,
			Kind:       "operator_auth",
			Detail:     authEventView(e),
		})
	}
	for _, a := range attr {
		items = append(items, TimelineItem{
			OccurredAt: a.OccurredAt,
			Kind:       "operator_action",
			Detail:     actionAttributionView(a),
		})
	}
	for _, s := range sessions {
		items = append(items, TimelineItem{
			OccurredAt: s.StartedAt,
			Kind:       "operator_session",
			Detail: SessionTimelineView{
				SessionID: s.ID.String(),
				Phase:     "started",
				ActorType: s.ActorType,
				Status:    s.Status,
			},
		})
		if s.EndedAt != nil {
			er := s.EndedReason
			items = append(items, TimelineItem{
				OccurredAt: *s.EndedAt,
				Kind:       "operator_session",
				Detail: SessionTimelineView{
					SessionID:   s.ID.String(),
					Phase:       "ended",
					ActorType:   s.ActorType,
					Status:      s.Status,
					EndedReason: er,
				},
			})
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].OccurredAt.Equal(items[j].OccurredAt) {
			return tieBreakKind(items[i].Kind, items[j].Kind)
		}
		return items[i].OccurredAt.After(items[j].OccurredAt)
	})
	if int(maxItems) < len(items) {
		items = items[:maxItems]
	}
	return items
}

func tieBreakKind(a, b string) bool {
	// deterministic ordering when timestamps match
	order := map[string]int{"operator_action": 0, "operator_auth": 1, "operator_session": 2}
	return order[a] < order[b]
}
