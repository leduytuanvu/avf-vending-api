// Package audit persists enterprise audit_events and serves GET /v1/admin/audit/events.
//
// Record is fail-open by historical convention for secondary observability.
// RecordCritical is mandatory for security/commerce mutations: persistence errors propagate unless
// ServiceOpts.CriticalFailOpen is true (AUDIT_CRITICAL_FAIL_OPEN in development/test only).
// RecordCriticalTx always fails closed and must run in the same PostgreSQL transaction as the mutation.
//
// Never store secrets, raw passwords, tokens, or HMAC keys in before/after/metadata (use compliance.SanitizeJSONBytes).
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ServiceOpts configures mandatory audit behavior for pool-level writes.
type ServiceOpts struct {
	// CriticalFailOpen when true swallows RecordCritical persistence errors (never RecordCriticalTx).
	// Allowed only for APP_ENV=development|test (validated in config.Load).
	CriticalFailOpen bool
}

// Service implements enterprise audit_events persistence and listing (RBAC at HTTP layer).
type Service struct {
	q                *db.Queries
	pool             *pgxpool.Pool
	criticalFailOpen bool
}

// NewService wires audit_events reads/writes via sqlc.
func NewService(pool *pgxpool.Pool, opts ...ServiceOpts) *Service {
	if pool == nil {
		panic("audit.NewService: nil pool")
	}
	var o ServiceOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	return &Service{
		q:                db.New(pool),
		pool:             pool,
		criticalFailOpen: o.CriticalFailOpen,
	}
}

var _ compliance.EnterpriseRecorder = (*Service)(nil)

// Record implements compliance.EnterpriseRecorder (fail-open caller semantics).
func (s *Service) Record(ctx context.Context, in compliance.EnterpriseAuditRecord) error {
	if s == nil || s.q == nil {
		return nil
	}
	p, err := s.buildInsertParams(ctx, in)
	if err != nil {
		productionmetrics.RecordAuditWriteFailure("build_insert_params")
		return err
	}
	_, err = s.q.EnterpriseAuditInsertEvent(ctx, p)
	if err != nil {
		productionmetrics.RecordAuditWriteFailure("enterprise_audit_insert")
		return err
	}
	action := strings.TrimSpace(in.Action)
	if action == "" {
		action = "unknown"
	}
	productionmetrics.RecordAuditEvent(action)
	return nil
}

// RecordCritical persists an audit row unless CriticalFailOpen suppresses the error.
func (s *Service) RecordCritical(ctx context.Context, in compliance.EnterpriseAuditRecord) error {
	if s == nil || s.q == nil {
		return nil
	}
	err := s.Record(ctx, in)
	if err == nil || s.criticalFailOpen {
		return nil
	}
	return err
}

// RecordCriticalTx writes audit_events inside tx (always fails closed).
func (s *Service) RecordCriticalTx(ctx context.Context, tx pgx.Tx, in compliance.EnterpriseAuditRecord) error {
	if s == nil || s.q == nil {
		return nil
	}
	if tx == nil {
		return errors.New("audit: RecordCriticalTx requires an open transaction")
	}
	p, err := s.buildInsertParams(ctx, in)
	if err != nil {
		productionmetrics.RecordAuditWriteFailure("build_insert_params_tx")
		return err
	}
	qtx := s.q.WithTx(tx)
	_, err = qtx.EnterpriseAuditInsertEvent(ctx, p)
	if err != nil {
		productionmetrics.RecordAuditWriteFailure("enterprise_audit_insert_tx")
		return err
	}
	action := strings.TrimSpace(in.Action)
	if action == "" {
		action = "unknown"
	}
	productionmetrics.RecordAuditEvent(action)
	return nil
}

func (s *Service) buildInsertParams(ctx context.Context, in compliance.EnterpriseAuditRecord) (db.EnterpriseAuditInsertEventParams, error) {
	in = withTransportMetaDefaults(ctx, in)
	md := sanitizeAuditJSONBytes(in.Metadata)
	if len(md) == 0 || string(md) == "null" {
		md = []byte("{}")
	}
	before := sanitizeAuditJSONBytes(in.BeforeJSON)
	after := sanitizeAuditJSONBytes(in.AfterJSON)
	var beforePtr, afterPtr []byte
	if len(before) > 0 && string(before) != "null" {
		beforePtr = before
	}
	if len(after) > 0 && string(after) != "null" {
		afterPtr = after
	}
	outcome := in.Outcome
	if outcome == "" {
		outcome = compliance.OutcomeSuccess
	}
	var occurred pgtype.Timestamptz
	if in.OccurredAt != nil {
		t := in.OccurredAt.UTC()
		occurred = pgtype.Timestamptz{Time: t, Valid: true}
	}
	return db.EnterpriseAuditInsertEventParams{
		OrganizationID: in.OrganizationID,
		ActorType:      in.ActorType,
		ActorID:        optionalStringPtrToPgText(in.ActorID),
		Action:         in.Action,
		ResourceType:   in.ResourceType,
		ResourceID:     optionalStringPtrToPgText(in.ResourceID),
		MachineID:      optionalUUIDPtrToPg(in.MachineID),
		SiteID:         optionalUUIDPtrToPg(in.SiteID),
		RequestID:      optionalStringPtrToPgText(in.RequestID),
		TraceID:        optionalStringPtrToPgText(in.TraceID),
		IpAddress:      optionalStringPtrToPgText(in.IPAddress),
		UserAgent:      optionalStringPtrToPgText(in.UserAgent),
		BeforeJson:     beforePtr,
		AfterJson:      afterPtr,
		Metadata:       md,
		Outcome:        outcome,
		OccurredAt:     occurred,
	}, nil
}

func optionalUUIDPtrToPg(u *uuid.UUID) pgtype.UUID {
	if u == nil || *u == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *u, Valid: true}
}

func withTransportMetaDefaults(ctx context.Context, in compliance.EnterpriseAuditRecord) compliance.EnterpriseAuditRecord {
	meta := compliance.TransportMetaFromContext(ctx)
	if in.RequestID == nil && strings.TrimSpace(meta.RequestID) != "" {
		in.RequestID = &meta.RequestID
	}
	if in.TraceID == nil && strings.TrimSpace(meta.TraceID) != "" {
		in.TraceID = &meta.TraceID
	}
	if in.IPAddress == nil && strings.TrimSpace(meta.IP) != "" {
		in.IPAddress = &meta.IP
	}
	if in.UserAgent == nil && strings.TrimSpace(meta.UserAgent) != "" {
		in.UserAgent = &meta.UserAgent
	}
	return in
}

var auditSensitiveKeySubstrings = []string{
	"jwt",
	"private_key",
	"privatekey",
	"mqtt_password",
	"mqttpassword",
	"bearer",
	"authorization",
	"password",
	"passwd",
	"secret",
	"token",
	"refresh",
	"hmac",
	"signature",
	"api_key",
	"apikey",
	"client_secret",
	"webhook_secret",
	"card_number",
	"cvv",
}

func sanitizeAuditJSONBytes(b []byte) []byte {
	out := compliance.SanitizeJSONBytes(b)
	var v any
	if err := json.Unmarshal(out, &v); err != nil {
		return out
	}
	sanitizeAuditValue(v)
	next, err := json.Marshal(v)
	if err != nil {
		return out
	}
	return next
}

func sanitizeAuditValue(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if auditKeyLooksSensitive(strings.ToLower(k)) {
				t[k] = "[REDACTED]"
				continue
			}
			sanitizeAuditValue(val)
		}
	case []any:
		for i := range t {
			sanitizeAuditValue(t[i])
		}
	}
}

func auditKeyLooksSensitive(k string) bool {
	for _, s := range auditSensitiveKeySubstrings {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

func optionalStringPtrToPgText(s *string) pgtype.Text {
	if s == nil || *s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// EventListParams filters GET /v1/admin/audit/events.
type EventListParams struct {
	OrganizationID uuid.UUID
	Action         string
	ActorID        string
	ActorType      string
	Outcome        string
	ResourceType   string
	ResourceID     string
	MachineID      string
	From           *time.Time
	To             *time.Time
	Limit          int32
	Offset         int32
}

// EventListItem is one audit_events row for APIs.
type EventListItem struct {
	ID             string          `json:"id"`
	OrganizationID string          `json:"organizationId"`
	ActorType      string          `json:"actorType"`
	ActorID        *string         `json:"actorId,omitempty"`
	Action         string          `json:"action"`
	ResourceType   string          `json:"resourceType"`
	ResourceID     *string         `json:"resourceId,omitempty"`
	MachineID      *string         `json:"machineId,omitempty"`
	SiteID         *string         `json:"siteId,omitempty"`
	RequestID      *string         `json:"requestId,omitempty"`
	TraceID        *string         `json:"traceId,omitempty"`
	IPAddress      *string         `json:"ipAddress,omitempty"`
	UserAgent      *string         `json:"userAgent,omitempty"`
	BeforeJSON     json.RawMessage `json:"beforeJson,omitempty"`
	AfterJSON      json.RawMessage `json:"afterJson,omitempty"`
	Metadata       json.RawMessage `json:"metadata"`
	Outcome        string          `json:"outcome"`
	OccurredAt     time.Time       `json:"occurredAt"`
	CreatedAt      time.Time       `json:"createdAt"`
}

// EventListResponse is paginated audit events.
type EventListResponse struct {
	Items []EventListItem          `json:"items"`
	Meta  listscope.CollectionMeta `json:"meta"`
}

func timeRangeStrings(from, to *time.Time) (fromS, toS string) {
	if from != nil {
		fromS = from.UTC().Format(time.RFC3339Nano)
	}
	if to != nil {
		toS = to.UTC().Format(time.RFC3339Nano)
	}
	return fromS, toS
}

// ListEvents returns tenant-scoped audit rows with filters.
func (s *Service) ListEvents(ctx context.Context, p EventListParams) (*EventListResponse, error) {
	if s == nil || s.q == nil {
		return nil, nil
	}
	if p.OrganizationID == uuid.Nil {
		return nil, listscope.ErrAdminOrganizationRequired
	}
	fromS, toS := timeRangeStrings(p.From, p.To)
	cnt, err := s.q.EnterpriseAuditCountEvents(ctx, db.EnterpriseAuditCountEventsParams{
		OrganizationID: p.OrganizationID,
		Column2:        p.Action,
		Column3:        p.ActorID,
		Column4:        p.ActorType,
		Column5:        p.Outcome,
		Column6:        p.ResourceType,
		Column7:        p.ResourceID,
		Column8:        fromS,
		Column9:        toS,
		Column10:       p.MachineID,
	})
	if err != nil {
		return nil, err
	}
	rows, err := s.q.EnterpriseAuditListEvents(ctx, db.EnterpriseAuditListEventsParams{
		OrganizationID: p.OrganizationID,
		Column2:        p.Action,
		Column3:        p.ActorID,
		Column4:        p.ActorType,
		Column5:        p.Outcome,
		Column6:        p.ResourceType,
		Column7:        p.ResourceID,
		Column8:        fromS,
		Column9:        toS,
		Column10:       p.MachineID,
		Limit:          p.Limit,
		Offset:         p.Offset,
	})
	if err != nil {
		return nil, err
	}
	items := make([]EventListItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, mapAuditEventRow(r))
	}
	return &EventListResponse{
		Items: items,
		Meta: listscope.CollectionMeta{
			Limit:    p.Limit,
			Offset:   p.Offset,
			Returned: len(items),
			Total:    cnt,
		},
	}, nil
}

// GetEventForOrg returns one audit row scoped to organization_id.
func (s *Service) GetEventForOrg(ctx context.Context, orgID, eventID uuid.UUID) (*EventListItem, error) {
	if s == nil || s.q == nil {
		return nil, nil
	}
	if orgID == uuid.Nil || eventID == uuid.Nil {
		return nil, listscope.ErrAdminOrganizationRequired
	}
	row, err := s.q.EnterpriseAuditGetEventForOrg(ctx, db.EnterpriseAuditGetEventForOrgParams{
		ID:             eventID,
		OrganizationID: orgID,
	})
	if err != nil {
		return nil, err
	}
	it := mapAuditEventRow(row)
	return &it, nil
}

func mapAuditEventRow(r db.AuditEvent) EventListItem {
	it := EventListItem{
		ID:             r.ID.String(),
		OrganizationID: r.OrganizationID.String(),
		ActorType:      r.ActorType,
		Action:         r.Action,
		ResourceType:   r.ResourceType,
		Metadata:       json.RawMessage(r.Metadata),
		Outcome:        r.Outcome,
		OccurredAt:     r.OccurredAt.UTC(),
		CreatedAt:      r.CreatedAt.UTC(),
	}
	it.ActorID = pgTextToStrPtr(r.ActorID)
	it.ResourceID = pgTextToStrPtr(r.ResourceID)
	it.MachineID = pgUUIDToStrPtr(r.MachineID)
	it.SiteID = pgUUIDToStrPtr(r.SiteID)
	it.RequestID = pgTextToStrPtr(r.RequestID)
	it.TraceID = pgTextToStrPtr(r.TraceID)
	it.IPAddress = pgTextToStrPtr(r.IpAddress)
	it.UserAgent = pgTextToStrPtr(r.UserAgent)
	if len(r.BeforeJson) > 0 {
		it.BeforeJSON = json.RawMessage(r.BeforeJson)
	}
	if len(r.AfterJson) > 0 {
		it.AfterJSON = json.RawMessage(r.AfterJson)
	}
	return it
}

func pgTextToStrPtr(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	s := t.String
	return &s
}

func pgUUIDToStrPtr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	id := uuid.UUID(u.Bytes)
	if id == uuid.Nil {
		return nil
	}
	s := id.String()
	return &s
}
