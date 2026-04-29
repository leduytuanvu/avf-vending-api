// Package adminops implements P1.2 tenant-scoped operational APIs for fleet command troubleshooting and inventory anomalies.
package adminops

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	appdevice "github.com/avf/avf-vending-api/internal/app/device"
	domaindevice "github.com/avf/avf-vending-api/internal/domain/device"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultManualAdjustmentAbsThreshold = 50
	defaultAdjustmentLookbackDays       = 365
)

// Deps wires database access and outbound MQTT dispatch for admin operations.
type Deps struct {
	Pool           *pgxpool.Pool
	RemoteCommands *appdevice.MQTTCommandDispatcher
}

// Service exposes operational workflows used by HTTPS handlers.
type Service struct {
	q               *db.Queries
	pool            *pgxpool.Pool
	remote          *appdevice.MQTTCommandDispatcher
	adjustThreshold int32
	adjustLookback  int64
}

// NewService constructs admin operations helpers (nil RemoteCommands disables mutations that require MQTT).
func NewService(d Deps) (*Service, error) {
	if d.Pool == nil {
		return nil, errors.New("adminops: nil pool")
	}
	s := &Service{
		q:               db.New(d.Pool),
		pool:            d.Pool,
		remote:          d.RemoteCommands,
		adjustThreshold: defaultManualAdjustmentAbsThreshold,
		adjustLookback:  defaultAdjustmentLookbackDays,
	}
	return s, nil
}

// SyncInventoryAnomalies inserts detector rows for supported anomaly kinds (idempotent fingerprints).
func (s *Service) SyncInventoryAnomalies(ctx context.Context, organizationID uuid.UUID) error {
	if s == nil || s.q == nil {
		return errors.New("adminops: nil service")
	}
	if _, err := s.q.AdminOpsInsertDetectedNegativeStockAnomalies(ctx, organizationID); err != nil {
		return err
	}
	if _, err := s.q.AdminOpsInsertDetectedManualAdjustmentAnomalies(ctx, db.AdminOpsInsertDetectedManualAdjustmentAnomaliesParams{
		OrganizationID:         organizationID,
		AdjustmentAbsThreshold: s.adjustThreshold,
		LookbackDays:           s.adjustLookback,
	}); err != nil {
		return err
	}
	_, err := s.q.AdminOpsInsertStaleInventorySyncAnomalies(ctx, organizationID)
	return err
}

// MachineHealth is an operational projection for dashboards (camelCase JSON via HTTPS handlers).
type MachineHealth struct {
	MachineID                 uuid.UUID  `json:"machineId"`
	Status                    string     `json:"status"`
	LastSeenAt                *time.Time `json:"lastSeenAt,omitempty"`
	LastCheckInAt             *time.Time `json:"lastCheckInAt,omitempty"`
	AppVersion                string     `json:"appVersion,omitempty"`
	ConfigVersion             string     `json:"configVersion,omitempty"`
	CatalogVersion            string     `json:"catalogVersion,omitempty"`
	MediaVersion              string     `json:"mediaVersion,omitempty"`
	MqttConnected             *bool      `json:"mqttConnected,omitempty"`
	PendingCommandCount       int64      `json:"pendingCommandCount"`
	FailedCommandCount        int64      `json:"failedCommandCount"`
	InventoryAnomalyCount     int64      `json:"inventoryAnomalyCount"`
	LastErrorCode             string     `json:"lastErrorCode,omitempty"`
	TelemetryFreshnessSeconds *int64     `json:"telemetryFreshnessSeconds,omitempty"`
}

func mapMachineHealth(row db.AdminOpsListMachineHealthRow) MachineHealth {
	out := MachineHealth{
		MachineID:             row.MachineID,
		Status:                row.Status,
		PendingCommandCount:   row.PendingCommandCount,
		FailedCommandCount:    row.FailedCommandCount,
		InventoryAnomalyCount: row.InventoryAnomalyCount,
	}
	out.LastSeenAt = pgTimestamptzPtr(row.LastSeenAt)
	out.LastCheckInAt = pgTimestamptzPtr(row.LastCheckinAt)
	if row.AppVersion.Valid {
		out.AppVersion = row.AppVersion.String
	}
	out.ConfigVersion = row.ConfigVersion
	out.CatalogVersion = row.CatalogVersion
	out.MediaVersion = row.MediaVersion
	if row.MqttConnected.Valid {
		v := row.MqttConnected.Bool
		out.MqttConnected = &v
	}
	out.LastErrorCode = row.LastErrorCode
	if row.TelemetryFreshnessSeconds >= 0 {
		v := row.TelemetryFreshnessSeconds
		out.TelemetryFreshnessSeconds = &v
	}
	return out
}

func mapMachineHealthSingle(row db.AdminOpsGetMachineHealthByIDRow) MachineHealth {
	h := MachineHealth{
		MachineID:             row.MachineID,
		Status:                row.Status,
		PendingCommandCount:   row.PendingCommandCount,
		FailedCommandCount:    row.FailedCommandCount,
		InventoryAnomalyCount: row.InventoryAnomalyCount,
	}
	h.LastSeenAt = pgTimestamptzPtr(row.LastSeenAt)
	h.LastCheckInAt = pgTimestamptzPtr(row.LastCheckinAt)
	if row.AppVersion.Valid {
		h.AppVersion = row.AppVersion.String
	}
	h.ConfigVersion = row.ConfigVersion
	h.CatalogVersion = row.CatalogVersion
	h.MediaVersion = row.MediaVersion
	if row.MqttConnected.Valid {
		v := row.MqttConnected.Bool
		h.MqttConnected = &v
	}
	h.LastErrorCode = row.LastErrorCode
	if row.TelemetryFreshnessSeconds >= 0 {
		v := row.TelemetryFreshnessSeconds
		h.TelemetryFreshnessSeconds = &v
	}
	return h
}

func pgTimestamptzPtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time.UTC()
	return &t
}

// ListMachineHealth returns operational rows for machines in the tenant (paginated).
func (s *Service) ListMachineHealth(ctx context.Context, organizationID uuid.UUID, limit, offset int32) ([]MachineHealth, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("adminops: nil service")
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.q.AdminOpsListMachineHealth(ctx, db.AdminOpsListMachineHealthParams{
		OrganizationID: organizationID,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]MachineHealth, 0, len(rows))
	for _, r := range rows {
		out = append(out, mapMachineHealth(r))
	}
	return out, nil
}

// GetMachineHealth returns health for one machine scoped by organization.
func (s *Service) GetMachineHealth(ctx context.Context, organizationID, machineID uuid.UUID) (MachineHealth, error) {
	if s == nil || s.q == nil {
		return MachineHealth{}, errors.New("adminops: nil service")
	}
	row, err := s.q.AdminOpsGetMachineHealthByID(ctx, db.AdminOpsGetMachineHealthByIDParams{
		OrganizationID: organizationID,
		ID:             machineID,
	})
	if err != nil {
		return MachineHealth{}, err
	}
	return mapMachineHealthSingle(row), nil
}

// TimelineEvent is one merged timeline row for troubleshooting UIs.
type TimelineEvent struct {
	OccurredAt time.Time       `json:"occurredAt"`
	EventKind  string          `json:"eventKind"`
	Title      string          `json:"title"`
	Payload    json.RawMessage `json:"payload"`
	RefID      string          `json:"refId"`
}

// MachineTimeline merges commands, attempts, commerce timelines, and telemetry check-ins.
func (s *Service) MachineTimeline(ctx context.Context, organizationID, machineID uuid.UUID, limit, offset int32) ([]TimelineEvent, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("adminops: nil service")
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.q.AdminOpsMachineTimeline(ctx, db.AdminOpsMachineTimelineParams{
		MachineID:      machineID,
		OrganizationID: organizationID,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		return nil, err
	}
	out := make([]TimelineEvent, 0, len(rows))
	for _, r := range rows {
		ev := TimelineEvent{
			OccurredAt: r.OccurredAt.UTC(),
			EventKind:  r.EventKind,
			Title:      r.Title,
			RefID:      r.RefID,
		}
		if len(r.Payload) > 0 {
			ev.Payload = json.RawMessage(r.Payload)
		} else {
			ev.Payload = json.RawMessage(`{}`)
		}
		out = append(out, ev)
	}
	return out, nil
}

// CommandDetail bundles ledger metadata + attempts for GET …/commands/{commandId}.
type CommandDetail struct {
	Ledger   db.CommandLedger           `json:"ledger"`
	Attempts []db.MachineCommandAttempt `json:"attempts"`
}

// GetCommandDetail loads ledger scoped by organization plus attempts ordered ascending.
func (s *Service) GetCommandDetail(ctx context.Context, organizationID, commandID uuid.UUID) (CommandDetail, error) {
	if s == nil || s.q == nil {
		return CommandDetail{}, errors.New("adminops: nil service")
	}
	ledger, err := s.q.AdminOpsGetCommandLedgerForOrg(ctx, db.AdminOpsGetCommandLedgerForOrgParams{
		ID:             commandID,
		OrganizationID: organizationID,
	})
	if err != nil {
		return CommandDetail{}, err
	}
	atts, err := s.q.AdminOpsListAttemptsForCommand(ctx, commandID)
	if err != nil {
		return CommandDetail{}, err
	}
	return CommandDetail{Ledger: ledger, Attempts: atts}, nil
}

// RetryCommand triggers MQTT replay/dispatch for retryable ledger rows (requires RemoteCommands).
func (s *Service) RetryCommand(ctx context.Context, organizationID, commandID uuid.UUID) (appdevice.RemoteCommandDispatchResult, error) {
	if s == nil || s.remote == nil {
		return appdevice.RemoteCommandDispatchResult{}, appdevice.ErrMQTTCommandPublisherMissing
	}
	return s.remote.AdminRetryLedgerCommand(ctx, organizationID, commandID)
}

// CancelCommand marks pending/sent attempts failed with admin_cancelled (best-effort operator halt).
func (s *Service) CancelCommand(ctx context.Context, organizationID, commandID uuid.UUID) (int64, error) {
	if s == nil || s.q == nil {
		return 0, errors.New("adminops: nil service")
	}
	if _, err := s.q.AdminOpsGetCommandLedgerForOrg(ctx, db.AdminOpsGetCommandLedgerForOrgParams{
		ID:             commandID,
		OrganizationID: organizationID,
	}); err != nil {
		return 0, err
	}
	return s.q.AdminOpsCancelOpenAttemptsForCommand(ctx, commandID)
}

// DispatchMachineCommand issues a new MQTT-backed ledger row when RemoteCommands is wired.
func (s *Service) DispatchMachineCommand(ctx context.Context, organizationID, machineID uuid.UUID, commandType string, payload json.RawMessage, idempotencyKey string) (appdevice.RemoteCommandDispatchResult, error) {
	if s == nil || s.remote == nil {
		return appdevice.RemoteCommandDispatchResult{}, appdevice.ErrMQTTCommandPublisherMissing
	}
	if strings.TrimSpace(commandType) == "" || strings.TrimSpace(idempotencyKey) == "" {
		return appdevice.RemoteCommandDispatchResult{}, appdevice.ErrInvalidArgument
	}
	if _, err := s.q.AdminOpsGetMachineHealthByID(ctx, db.AdminOpsGetMachineHealthByIDParams{
		OrganizationID: organizationID,
		ID:             machineID,
	}); err != nil {
		return appdevice.RemoteCommandDispatchResult{}, err
	}
	desired, err := s.q.AdminOpsGetMachineShadowDesiredJSON(ctx, machineID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return appdevice.RemoteCommandDispatchResult{}, err
	}
	if len(desired) == 0 {
		desired = []byte("{}")
	}
	pl := payload
	if len(pl) == 0 {
		pl = json.RawMessage(`{}`)
	}
	return s.remote.DispatchRemoteMQTTCommand(ctx, appdevice.RemoteCommandDispatchInput{
		Append: domaindevice.AppendCommandInput{
			MachineID:      machineID,
			CommandType:    strings.TrimSpace(commandType),
			Payload:        pl,
			IdempotencyKey: strings.TrimSpace(idempotencyKey),
			DesiredState:   desired,
		},
	})
}

// ListInventoryAnomalies lists persisted anomalies after optional detector refresh.
func (s *Service) ListInventoryAnomalies(ctx context.Context, organizationID uuid.UUID, machineID *uuid.UUID, limit, offset int32, refreshDetectors bool) ([]db.AdminOpsListInventoryAnomaliesByOrgRow, error) {
	if s == nil || s.q == nil {
		return nil, errors.New("adminops: nil service")
	}
	if refreshDetectors {
		if err := s.SyncInventoryAnomalies(ctx, organizationID); err != nil {
			return nil, err
		}
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	filterMachine := machineID != nil && *machineID != uuid.Nil
	return s.q.AdminOpsListInventoryAnomaliesByOrg(ctx, db.AdminOpsListInventoryAnomaliesByOrgParams{
		OrganizationID: organizationID,
		FilterMachine:  filterMachine,
		MachineID:      derefUUID(machineID),
		LimitVal:       limit,
		OffsetVal:      offset,
	})
}

func derefUUID(p *uuid.UUID) uuid.UUID {
	if p == nil {
		return uuid.Nil
	}
	return *p
}

// ResolveInventoryAnomaly closes an open anomaly row (operator acknowledgement).
func (s *Service) ResolveInventoryAnomaly(ctx context.Context, organizationID, anomalyID, actorAccountID uuid.UUID, note string) error {
	if s == nil || s.q == nil {
		return errors.New("adminops: nil service")
	}
	_, err := s.q.AdminOpsResolveInventoryAnomaly(ctx, db.AdminOpsResolveInventoryAnomalyParams{
		ID:             anomalyID,
		OrganizationID: organizationID,
		ResolvedBy:     uuidToPgUUID(actorAccountID),
		ResolutionNote: pgtype.Text{String: strings.TrimSpace(note), Valid: strings.TrimSpace(note) != ""},
	})
	return err
}

func uuidToPgUUID(u uuid.UUID) pgtype.UUID {
	if u == uuid.Nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: u, Valid: true}
}

// InsertInventoryReconcileMarker appends a durable inventory_events reconcile marker.
func (s *Service) InsertInventoryReconcileMarker(ctx context.Context, organizationID, machineID uuid.UUID, reason string, metadata json.RawMessage) (int64, error) {
	if s == nil || s.q == nil {
		return 0, errors.New("adminops: nil service")
	}
	meta := metadata
	if len(meta) == 0 {
		meta = json.RawMessage(`{}`)
	}
	return s.q.AdminInventoryInsertReconcileMarker(ctx, db.AdminInventoryInsertReconcileMarkerParams{
		OrganizationID: organizationID,
		MachineID:      machineID,
		ReasonCode:     pgtype.Text{String: strings.TrimSpace(reason), Valid: strings.TrimSpace(reason) != ""},
		Metadata:       meta,
	})
}
