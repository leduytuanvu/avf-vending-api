package adminops

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// OpsOverview aggregates machine operational state for admin dashboards.
type OpsOverview struct {
	MachineID             uuid.UUID       `json:"machineId"`
	Status                string          `json:"status"`
	CredentialVersion     int64           `json:"credentialVersion"`
	Health                MachineHealth   `json:"health"`
	RuntimeSession        *RuntimeSession `json:"runtimeSession,omitempty"`
	LastActivationClaim   json.RawMessage `json:"lastActivationClaim,omitempty"`
	ActiveOperatorSession json.RawMessage `json:"activeOperatorSession,omitempty"`
}

// RuntimeSession is a safe admin view of machine_sessions (no token hash).
type RuntimeSession struct {
	SessionID         uuid.UUID  `json:"sessionId"`
	CredentialVersion int64      `json:"credentialVersion"`
	Status            string     `json:"status"`
	IssuedAt          time.Time  `json:"issuedAt"`
	ExpiresAt         time.Time  `json:"expiresAt"`
	LastUsedAt        *time.Time `json:"lastUsedAt,omitempty"`
	RevokedAt         *time.Time `json:"revokedAt,omitempty"`
}

// UnifiedTimelineItem is one merged enterprise timeline row.
type UnifiedTimelineItem struct {
	ID                string          `json:"id"`
	OccurredAt        time.Time       `json:"occurredAt"`
	ReceivedAt        time.Time       `json:"receivedAt"`
	EventType         string          `json:"eventType"`
	Severity          string          `json:"severity"`
	MachineID         uuid.UUID       `json:"machineId"`
	ActorType         string          `json:"actorType"`
	ActorAccountID    *uuid.UUID      `json:"actorAccountId,omitempty"`
	OperatorSessionID *uuid.UUID      `json:"operatorSessionId,omitempty"`
	MachineSessionID  *uuid.UUID      `json:"machineSessionId,omitempty"`
	ResourceType      string          `json:"resourceType"`
	ResourceID        string          `json:"resourceId"`
	RequestID         string          `json:"requestId,omitempty"`
	CorrelationID     *uuid.UUID      `json:"correlationId,omitempty"`
	Reason            string          `json:"reason,omitempty"`
	ErrorCode         string          `json:"errorCode,omitempty"`
	Summary           string          `json:"summary"`
	Metadata          json.RawMessage `json:"metadata"`
	Source            string          `json:"source"`
	Category          string          `json:"category,omitempty"`
	EventID           string          `json:"eventId,omitempty"`
	OrderID           *uuid.UUID      `json:"orderId,omitempty"`
	PaymentID         *uuid.UUID      `json:"paymentId,omitempty"`
	VendAttemptID     *uuid.UUID      `json:"vendAttemptId,omitempty"`
}

// TimelineFilter optional query filters for unified timeline.
type TimelineFilter struct {
	From              *time.Time
	To                *time.Time
	OperatorSessionID *uuid.UUID
	OrderID           *uuid.UUID
	PaymentID         *uuid.UUID
	Limit             int32
}

// GetMachineOpsOverview returns operational snapshot for one machine.
func (s *Service) GetMachineOpsOverview(ctx context.Context, companyID, machineID uuid.UUID) (OpsOverview, error) {
	if s == nil || s.q == nil {
		return OpsOverview{}, errors.New("adminops: nil service")
	}
	health, err := s.GetMachineHealth(ctx, companyID, machineID)
	if err != nil {
		return OpsOverview{}, err
	}
	out := OpsOverview{
		MachineID:         machineID,
		Status:            health.Status,
		CredentialVersion: 0,
		Health:            health,
	}
	m, err := s.q.GetMachineByID(ctx, machineID)
	if err == nil {
		out.Status = m.Status
		out.CredentialVersion = m.CredentialVersion
	}
	if sess, err := s.q.GetCurrentMachineRuntimeSession(ctx, machineID); err == nil {
		out.RuntimeSession = mapRuntimeSession(sess)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return OpsOverview{}, err
	}
	if claim, err := s.q.GetLastSucceededActivationClaimForMachine(ctx, machineID); err == nil {
		b, _ := json.Marshal(map[string]any{
			"id":               claim.ID.String(),
			"claimedAt":        claim.ClaimedAt.UTC().Format(time.RFC3339Nano),
			"activationSource": claim.ActivationSource.String,
			"appVersion":       claim.AppVersion.String,
		})
		out.LastActivationClaim = b
	}
	if op, err := s.q.GetActiveOperatorSessionForMachine(ctx, machineID); err == nil {
		b, _ := json.Marshal(map[string]any{
			"id":        op.ID.String(),
			"actorType": op.ActorType,
			"status":    op.Status,
			"startedAt": op.StartedAt.UTC().Format(time.RFC3339Nano),
		})
		out.ActiveOperatorSession = b
	}
	return out, nil
}

// UnifiedMachineTimeline merges lifecycle, activation, attribution, and runtime session events.
func (s *Service) UnifiedMachineTimeline(ctx context.Context, companyID, machineID uuid.UUID, f TimelineFilter) ([]UnifiedTimelineItem, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("adminops: nil service")
	}
	lim := f.Limit
	if lim <= 0 || lim > 500 {
		lim = 50
	}
	var from, to pgtype.Timestamptz
	if f.From != nil {
		from = pgtype.Timestamptz{Time: f.From.UTC(), Valid: true}
	}
	if f.To != nil {
		to = pgtype.Timestamptz{Time: f.To.UTC(), Valid: true}
	}
	var opSess pgtype.UUID
	if f.OperatorSessionID != nil {
		opSess = pgtype.UUID{Bytes: *f.OperatorSessionID, Valid: true}
	}
	var orderID, paymentID pgtype.UUID
	if f.OrderID != nil {
		orderID = pgtype.UUID{Bytes: *f.OrderID, Valid: true}
	}
	if f.PaymentID != nil {
		paymentID = pgtype.UUID{Bytes: *f.PaymentID, Valid: true}
	}
	rows, err := s.q.AdminUnifiedMachineTimeline(ctx, db.AdminUnifiedMachineTimelineParams{
		MachineID:         pgtype.UUID{Bytes: machineID, Valid: true},
		FromTs:            from,
		ToTs:              to,
		OperatorSessionID: opSess,
		OrderID:           orderID,
		PaymentID:         paymentID,
		Lim:               lim,
	})
	if err != nil {
		return nil, err
	}
	out := make([]UnifiedTimelineItem, 0, len(rows))
	for _, r := range rows {
		item := UnifiedTimelineItem{
			ID:           r.OccurredAt.UTC().Format(time.RFC3339Nano) + ":" + r.EventType,
			OccurredAt:   r.OccurredAt.UTC(),
			ReceivedAt:   r.ReceivedAt.UTC(),
			EventType:    r.EventType,
			Severity:     r.Severity,
			ActorType:    r.ActorType,
			ResourceType: r.ResourceType,
			Summary:      r.Summary,
			Source:       r.Source,
			Category:     r.Category,
			EventID:      r.EventID,
		}
		if r.MachineID.Valid {
			item.MachineID = uuid.UUID(r.MachineID.Bytes)
		}
		if r.ResourceID.Valid {
			item.ResourceID = r.ResourceID.String
		}
		if r.RequestID.Valid {
			item.RequestID = r.RequestID.String
		}
		if r.Reason.Valid {
			item.Reason = r.Reason.String
		}
		if r.ErrorCode.Valid {
			item.ErrorCode = r.ErrorCode.String
		}
		if r.ActorAccountID.Valid {
			id := uuid.UUID(r.ActorAccountID.Bytes)
			item.ActorAccountID = &id
		}
		if r.OperatorSessionID.Valid {
			id := uuid.UUID(r.OperatorSessionID.Bytes)
			item.OperatorSessionID = &id
		}
		if r.MachineSessionID.Valid {
			id := uuid.UUID(r.MachineSessionID.Bytes)
			item.MachineSessionID = &id
		}
		if r.CorrelationID.Valid {
			id := uuid.UUID(r.CorrelationID.Bytes)
			item.CorrelationID = &id
		}
		if r.OrderID.Valid {
			id := uuid.UUID(r.OrderID.Bytes)
			item.OrderID = &id
		}
		if r.PaymentID.Valid {
			id := uuid.UUID(r.PaymentID.Bytes)
			item.PaymentID = &id
		}
		if r.VendSessionID.Valid {
			id := uuid.UUID(r.VendSessionID.Bytes)
			item.VendAttemptID = &id
		}
		if len(r.Metadata) > 0 {
			item.Metadata = json.RawMessage(r.Metadata)
		} else {
			item.Metadata = json.RawMessage(`{}`)
		}
		out = append(out, item)
	}
	return out, nil
}

func mapRuntimeSession(sess db.GetCurrentMachineRuntimeSessionRow) *RuntimeSession {
	rs := &RuntimeSession{
		SessionID:         sess.ID,
		CredentialVersion: sess.CredentialVersion,
		Status:            sess.Status,
		IssuedAt:          sess.IssuedAt.UTC(),
		ExpiresAt:         sess.ExpiresAt.UTC(),
	}
	if sess.LastUsedAt.Valid {
		t := sess.LastUsedAt.Time.UTC()
		rs.LastUsedAt = &t
	}
	if sess.RevokedAt.Valid {
		t := sess.RevokedAt.Time.UTC()
		rs.RevokedAt = &t
	}
	return rs
}
