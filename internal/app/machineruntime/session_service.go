// Package machineruntime manages Android board attachments and app runtime lifecycle sessions.
package machineruntime

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/netip"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultOnlineThreshold  = 60 * time.Second
	defaultStaleThreshold   = 300 * time.Second
	defaultOfflineThreshold = 600 * time.Second
)

// Deps wires database access.
type Deps struct {
	Pool              *pgxpool.Pool
	OnlineThreshold   time.Duration
	StaleThreshold    time.Duration
	OfflineThreshold  time.Duration
	AssignmentChecker TechnicianAssignmentChecker
}

// TechnicianAssignmentChecker validates technician scope for attach/reattach.
type TechnicianAssignmentChecker interface {
	TechnicianActiveAssignmentExists(ctx context.Context, technicianID, machineID uuid.UUID) (bool, error)
}

// Service coordinates device attachments and runtime app sessions.
type Service struct {
	q                *db.Queries
	pool             *pgxpool.Pool
	onlineThreshold  time.Duration
	staleThreshold   time.Duration
	offlineThreshold time.Duration
	assignments      TechnicianAssignmentChecker
}

// NewService constructs the runtime session service.
func NewService(d Deps) (*Service, error) {
	if d.Pool == nil {
		return nil, errors.New("machineruntime: nil pool")
	}
	on := d.OnlineThreshold
	if on <= 0 {
		on = defaultOnlineThreshold
	}
	st := d.StaleThreshold
	if st <= 0 {
		st = defaultStaleThreshold
	}
	off := d.OfflineThreshold
	if off <= 0 {
		off = defaultOfflineThreshold
	}
	return &Service{
		q:                db.New(d.Pool),
		pool:             d.Pool,
		onlineThreshold:  on,
		staleThreshold:   st,
		offlineThreshold: off,
		assignments:      d.AssignmentChecker,
	}, nil
}

// DeviceIdentity captures Android board / SIM identity at attach time.
type DeviceIdentity struct {
	AndroidID      string
	AndroidSerial  string
	BoardSerial    string
	DeviceSerial   string
	SimSerial      string
	SimICCID       string
	SimOperator    string
	SimCountryISO  string
	Manufacturer   string
	Brand          string
	Model          string
	DeviceModel    string
	Hardware       string
	Product        string
	AndroidRelease string
	SDKInt         *int32
	PackageName    string
	VersionName    string
	VersionCode    *int64
	AppBuildSHA    string
	BootID         string
	NetworkType    string
	NetworkState   string
	IPAddress      string
	UserAgent      string
	Metadata       json.RawMessage
}

// AttachInput binds a board to a physical machine.
type AttachInput struct {
	MachineID           uuid.UUID
	Reason              string
	AttachedByAccountID *uuid.UUID
	OperatorSessionID   *uuid.UUID
	CorrelationID       *uuid.UUID
	Identity            DeviceIdentity
	RequireOperator     bool
	TechnicianID        *uuid.UUID
}

// StartInput opens an app runtime session (machine JWT path).
type StartInput struct {
	MachineID          uuid.UUID
	DeviceAttachmentID *uuid.UUID
	MachineSessionID   *uuid.UUID
	OperatorSessionID  *uuid.UUID
	BootID             string
	AppStartID         string
	AppInstanceID      string
	PackageName        string
	AppVersion         string
	AppBuildSHA        string
	StartReason        string
	NetworkState       string
	MqttState          string
	StorefrontState    string
	Metadata           json.RawMessage
}

// HeartbeatInput updates runtime session liveness.
type HeartbeatInput struct {
	SessionID       uuid.UUID
	MachineID       uuid.UUID
	NetworkState    string
	MqttState       string
	StorefrontState string
	SellReady       bool
	Blockers        json.RawMessage
	HardwareStatus  json.RawMessage
	CatalogStatus   json.RawMessage
	OutboxStatus    json.RawMessage
	RecoveryStatus  json.RawMessage
}

// EndInput closes a runtime app session.
type EndInput struct {
	SessionID uuid.UUID
	MachineID uuid.UUID
	EndReason string
	Status    string
}

// AttachOrReplaceDevice creates a new active attachment, replacing any prior active board.
func (s *Service) AttachOrReplaceDevice(ctx context.Context, in AttachInput) (db.MachineDeviceAttachment, error) {
	if s == nil || s.q == nil {
		return db.MachineDeviceAttachment{}, errors.New("machineruntime: nil service")
	}
	if in.MachineID == uuid.Nil {
		return db.MachineDeviceAttachment{}, errors.New("machineruntime: machine required")
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		reason = "unknown"
	}
	if in.RequireOperator && (in.OperatorSessionID == nil || *in.OperatorSessionID == uuid.Nil) {
		return db.MachineDeviceAttachment{}, errors.New("machineruntime: operator_session_id required")
	}
	if in.TechnicianID != nil && s.assignments != nil {
		ok, err := s.assignments.TechnicianActiveAssignmentExists(ctx, *in.TechnicianID, in.MachineID)
		if err != nil {
			return db.MachineDeviceAttachment{}, err
		}
		if !ok {
			return db.MachineDeviceAttachment{}, errors.New("machineruntime: technician not assigned to machine")
		}
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.MachineDeviceAttachment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)
	row, err := s.attachOrReplaceDeviceTx(ctx, qtx, in)
	if err != nil {
		return db.MachineDeviceAttachment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.MachineDeviceAttachment{}, err
	}
	return row, nil
}

// AttachOrReplaceDeviceInTx attaches within an existing sqlc transaction (e.g. reattach).
func (s *Service) AttachOrReplaceDeviceInTx(ctx context.Context, qtx *db.Queries, in AttachInput) (db.MachineDeviceAttachment, error) {
	if s == nil {
		return db.MachineDeviceAttachment{}, errors.New("machineruntime: nil service")
	}
	return s.attachOrReplaceDeviceTx(ctx, qtx, in)
}

func (s *Service) attachOrReplaceDeviceTx(ctx context.Context, qtx *db.Queries, in AttachInput) (db.MachineDeviceAttachment, error) {
	if in.MachineID == uuid.Nil {
		return db.MachineDeviceAttachment{}, errors.New("machineruntime: machine required")
	}
	if in.RequireOperator && (in.OperatorSessionID == nil || *in.OperatorSessionID == uuid.Nil) {
		return db.MachineDeviceAttachment{}, errors.New("machineruntime: operator_session_id required")
	}
	if in.TechnicianID != nil && s.assignments != nil {
		ok, err := s.assignments.TechnicianActiveAssignmentExists(ctx, *in.TechnicianID, in.MachineID)
		if err != nil {
			return db.MachineDeviceAttachment{}, err
		}
		if !ok {
			return db.MachineDeviceAttachment{}, errors.New("machineruntime: technician not assigned to machine")
		}
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		reason = "unknown"
	}
	var prevID pgtype.UUID
	if cur, err := qtx.GetActiveMachineDeviceAttachment(ctx, in.MachineID); err == nil {
		prevID = pgtype.UUID{Bytes: cur.ID, Valid: true}
		if _, err := qtx.MarkMachineDeviceAttachmentReplaced(ctx, cur.ID); err != nil {
			return db.MachineDeviceAttachment{}, err
		}
		_, _ = qtx.CloseCurrentRuntimeAppSessionForMachine(ctx, db.CloseCurrentRuntimeAppSessionForMachineParams{
			MachineID: in.MachineID,
			Status:    "REPLACED",
			EndReason: pgtype.Text{String: "BOARD_REPLACED", Valid: true},
		})
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return db.MachineDeviceAttachment{}, err
	}

	meta := in.Identity.Metadata
	if meta == nil {
		meta = json.RawMessage("{}")
	}
	var ip *netip.Addr
	if ipStr := strings.TrimSpace(in.Identity.IPAddress); ipStr != "" {
		if parsed, err := netip.ParseAddr(ipStr); err == nil {
			ip = &parsed
		}
	}

	row, err := qtx.InsertMachineDeviceAttachment(ctx, db.InsertMachineDeviceAttachmentParams{
		MachineID:            in.MachineID,
		PreviousAttachmentID: prevID,
		Status:               "active",
		Reason:               reason,
		AttachedByAccountID:  uuidToPg(in.AttachedByAccountID),
		OperatorSessionID:    uuidToPg(in.OperatorSessionID),
		CorrelationID:        uuidToPg(in.CorrelationID),
		AndroidID:            pgText(in.Identity.AndroidID),
		AndroidSerial:        pgText(in.Identity.AndroidSerial),
		BoardSerial:          pgText(in.Identity.BoardSerial),
		DeviceSerial:         pgText(in.Identity.DeviceSerial),
		SimSerial:            pgText(in.Identity.SimSerial),
		SimIccid:             pgText(in.Identity.SimICCID),
		SimOperator:          pgText(in.Identity.SimOperator),
		SimCountryIso:        pgText(in.Identity.SimCountryISO),
		Manufacturer:         pgText(in.Identity.Manufacturer),
		Brand:                pgText(in.Identity.Brand),
		Model:                pgText(in.Identity.Model),
		DeviceModel:          pgText(in.Identity.DeviceModel),
		Hardware:             pgText(in.Identity.Hardware),
		Product:              pgText(in.Identity.Product),
		AndroidRelease:       pgText(in.Identity.AndroidRelease),
		SdkInt:               int32PtrToPg(in.Identity.SDKInt),
		PackageName:          pgText(in.Identity.PackageName),
		VersionName:          pgText(in.Identity.VersionName),
		VersionCode:          int64PtrToPg(in.Identity.VersionCode),
		AppBuildSha:          pgText(in.Identity.AppBuildSHA),
		BootID:               pgText(in.Identity.BootID),
		NetworkType:          pgText(in.Identity.NetworkType),
		NetworkState:         pgText(in.Identity.NetworkState),
		IpAddress:            ip,
		UserAgent:            pgText(in.Identity.UserAgent),
		Metadata:             meta,
	})
	if err != nil {
		return db.MachineDeviceAttachment{}, err
	}
	if err := qtx.UpdateMachineCurrentDeviceAttachment(ctx, db.UpdateMachineCurrentDeviceAttachmentParams{
		ID:                        in.MachineID,
		CurrentDeviceAttachmentID: uuidToPg(&row.ID),
	}); err != nil {
		return db.MachineDeviceAttachment{}, err
	}
	return row, nil
}

// StartRuntimeAppSession opens or idempotently returns an app runtime session.
func (s *Service) StartRuntimeAppSession(ctx context.Context, in StartInput) (db.MachineRuntimeAppSession, error) {
	if s == nil || s.q == nil {
		return db.MachineRuntimeAppSession{}, errors.New("machineruntime: nil service")
	}
	if in.MachineID == uuid.Nil {
		return db.MachineRuntimeAppSession{}, errors.New("machineruntime: machine required")
	}
	startReason := strings.TrimSpace(in.StartReason)
	if startReason == "" {
		startReason = "UNKNOWN"
	}
	if existing, err := s.q.GetMachineRuntimeAppSessionByBootAndStart(ctx, db.GetMachineRuntimeAppSessionByBootAndStartParams{
		MachineID:  in.MachineID,
		BootID:     in.BootID,
		AppStartID: in.AppStartID,
	}); err == nil {
		slog.Info("runtime_session.start.idempotent_hit",
			"machine_id", in.MachineID.String(),
			"session_id", existing.ID.String(),
			"boot_id", in.BootID,
			"app_start_id", in.AppStartID,
		)
		return existing, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return db.MachineRuntimeAppSession{}, wrapRuntimeStage("start runtime session", "lookup_existing", "GetMachineRuntimeAppSessionByBootAndStart", err)
	}

	slog.Info("runtime_session.start.begin",
		"machine_id", in.MachineID.String(),
		"boot_id", in.BootID,
		"app_start_id", in.AppStartID,
		"app_instance_id", in.AppInstanceID,
		"start_reason", startReason,
		"app_version", in.AppVersion,
		"app_build_sha", in.AppBuildSHA,
	)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.MachineRuntimeAppSession{}, wrapRuntimeStage("start runtime session", "begin_tx", "", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)

	endStatus := "ENDED"
	endReason := "SUPERSEDED_BY_NEW_SESSION"
	if startReason == "APP_CRASH_RECOVERY" {
		endReason = "APP_CRASH_DETECTED"
		endStatus = "CRASHED"
	}
	var prevSessID pgtype.UUID
	if _, err := qtx.LockMachineForUpdate(ctx, in.MachineID); err != nil {
		return db.MachineRuntimeAppSession{}, wrapRuntimeStage("start runtime session", "lock_machine", "LockMachineForUpdate", err)
	}
	if closed, err := qtx.CloseCurrentRuntimeAppSessionForMachine(ctx, db.CloseCurrentRuntimeAppSessionForMachineParams{
		MachineID: in.MachineID,
		Status:    endStatus,
		EndReason: pgtype.Text{String: endReason, Valid: true},
	}); err != nil {
		return db.MachineRuntimeAppSession{}, wrapRuntimeStage("start runtime session", "close_previous_session", "CloseCurrentRuntimeAppSessionForMachine", err)
	} else if len(closed) > 0 {
		prevSessID = pgtype.UUID{Bytes: closed[0].ID, Valid: true}
	}

	meta := in.Metadata
	if meta == nil {
		meta = json.RawMessage("{}")
	}
	sf := strings.TrimSpace(in.StorefrontState)
	if sf == "" {
		sf = "INITIALIZING"
	}
	blockersText, err := jsonArrayText(json.RawMessage("[]"))
	if err != nil {
		return db.MachineRuntimeAppSession{}, err
	}
	hwText, err := jsonObjectText(json.RawMessage("{}"))
	if err != nil {
		return db.MachineRuntimeAppSession{}, err
	}
	catText, err := jsonObjectText(json.RawMessage("{}"))
	if err != nil {
		return db.MachineRuntimeAppSession{}, err
	}
	outboxText, err := jsonObjectText(json.RawMessage("{}"))
	if err != nil {
		return db.MachineRuntimeAppSession{}, err
	}
	recText, err := jsonObjectText(json.RawMessage("{}"))
	if err != nil {
		return db.MachineRuntimeAppSession{}, err
	}
	metaText, err := jsonObjectText(meta)
	if err != nil {
		return db.MachineRuntimeAppSession{}, err
	}
	row, err := qtx.StartMachineRuntimeAppSession(ctx, db.StartMachineRuntimeAppSessionParams{
		MachineID:                in.MachineID,
		DeviceAttachmentID:       uuidToPg(in.DeviceAttachmentID),
		MachineSessionID:         uuidToPg(in.MachineSessionID),
		OperatorSessionID:        uuidToPg(in.OperatorSessionID),
		PreviousRuntimeSessionID: prevSessID,
		BootID:                   in.BootID,
		AppStartID:               in.AppStartID,
		AppInstanceID:            in.AppInstanceID,
		PackageName:              in.PackageName,
		AppVersion:               in.AppVersion,
		AppBuildSha:              in.AppBuildSHA,
		StartReason:              startReason,
		Status:                   "ONLINE",
		LastNetworkState:         in.NetworkState,
		LastMqttState:            in.MqttState,
		StorefrontState:          sf,
		SellReady:                false,
		Blockers:                 blockersText,
		HardwareStatus:           hwText,
		CatalogStatus:            catText,
		OutboxStatus:             outboxText,
		RecoveryStatus:           recText,
		Metadata:                 metaText,
	})
	if err != nil {
		logJSONBindAudit("start", err, blockersText, hwText, catText, outboxText, recText, metaText)
		return db.MachineRuntimeAppSession{}, wrapRuntimeStage("start runtime session", "insert_runtime_session", "StartMachineRuntimeAppSession", err)
	}
	now := time.Now().UTC()
	if err := qtx.UpdateMachineCurrentRuntimeAppSession(ctx, db.UpdateMachineCurrentRuntimeAppSessionParams{
		ID:                         in.MachineID,
		CurrentRuntimeAppSessionID: uuidToPg(&row.ID),
	}); err != nil {
		return db.MachineRuntimeAppSession{}, wrapRuntimeStage("start runtime session", "update_machine_current_session", "UpdateMachineCurrentRuntimeAppSession", err)
	}
	if err := qtx.UpdateMachineOnlineStatus(ctx, db.UpdateMachineOnlineStatusParams{
		ID:           in.MachineID,
		OnlineStatus: "online",
		LastSeenAt:   pgtype.Timestamptz{Time: now, Valid: true},
	}); err != nil {
		return db.MachineRuntimeAppSession{}, wrapRuntimeStage("start runtime session", "update_machine_online", "UpdateMachineOnlineStatus", err)
	}
	if err := s.projectRuntimeSnapshot(ctx, qtx, in.MachineID, row, "online"); err != nil {
		return db.MachineRuntimeAppSession{}, wrapRuntimeStage("start runtime session", "update_machine_snapshot", "UpdateMachineCurrentSnapshotRuntime", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return db.MachineRuntimeAppSession{}, wrapRuntimeStage("start runtime session", "commit", "", err)
	}
	slog.Info("runtime_session.start.ok",
		"machine_id", in.MachineID.String(),
		"session_id", row.ID.String(),
		"boot_id", in.BootID,
		"app_start_id", in.AppStartID,
		"start_reason", startReason,
	)
	return row, nil
}

// HeartbeatRuntimeAppSession updates session and machine online projection.
func (s *Service) HeartbeatRuntimeAppSession(ctx context.Context, in HeartbeatInput) (db.MachineRuntimeAppSession, error) {
	if s == nil || s.q == nil {
		return db.MachineRuntimeAppSession{}, errors.New("machineruntime: nil service")
	}
	blockers, err := jsonArrayText(in.Blockers)
	if err != nil {
		return db.MachineRuntimeAppSession{}, err
	}
	hw, err := jsonObjectText(in.HardwareStatus)
	if err != nil {
		return db.MachineRuntimeAppSession{}, err
	}
	cat, err := jsonObjectText(in.CatalogStatus)
	if err != nil {
		return db.MachineRuntimeAppSession{}, err
	}
	outbox, err := jsonObjectText(in.OutboxStatus)
	if err != nil {
		return db.MachineRuntimeAppSession{}, err
	}
	rec, err := jsonObjectText(in.RecoveryStatus)
	if err != nil {
		return db.MachineRuntimeAppSession{}, err
	}
	row, err := s.q.HeartbeatMachineRuntimeAppSession(ctx, db.HeartbeatMachineRuntimeAppSessionParams{
		ID:               in.SessionID,
		LastHeartbeatAt:  pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		Status:           "ONLINE",
		LastNetworkState: in.NetworkState,
		LastMqttState:    in.MqttState,
		StorefrontState:  in.StorefrontState,
		SellReady:        in.SellReady,
		Blockers:         blockers,
		HardwareStatus:   hw,
		CatalogStatus:    cat,
		OutboxStatus:     outbox,
		RecoveryStatus:   rec,
		MachineID:        in.MachineID,
	})
	if err != nil {
		logJSONBindAudit("heartbeat", err, blockers, hw, cat, outbox, rec, "")
		return db.MachineRuntimeAppSession{}, wrapRuntimeStage("heartbeat runtime session", "heartbeat", "HeartbeatMachineRuntimeAppSession", err)
	}
	if row.MachineID != in.MachineID {
		return db.MachineRuntimeAppSession{}, errors.New("machineruntime: session machine mismatch")
	}
	online, _ := s.ComputeMachineOnlineStatus(ctx, in.MachineID)
	_ = s.q.UpdateMachineOnlineStatus(ctx, db.UpdateMachineOnlineStatusParams{
		ID:           in.MachineID,
		OnlineStatus: online,
		LastSeenAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	_ = s.projectRuntimeSnapshot(ctx, s.q, in.MachineID, row, online)
	return row, nil
}

// EndRuntimeAppSession closes an app runtime session.
func (s *Service) EndRuntimeAppSession(ctx context.Context, in EndInput) (db.MachineRuntimeAppSession, error) {
	if s == nil || s.q == nil {
		return db.MachineRuntimeAppSession{}, errors.New("machineruntime: nil service")
	}
	status := strings.TrimSpace(in.Status)
	if status == "" {
		status = "ENDED"
	}
	row, err := s.q.EndMachineRuntimeAppSession(ctx, db.EndMachineRuntimeAppSessionParams{
		ID:        in.SessionID,
		Status:    status,
		EndReason: pgtype.Text{String: in.EndReason, Valid: in.EndReason != ""},
		EndedAt:   pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		MachineID: in.MachineID,
	})
	if err != nil {
		return db.MachineRuntimeAppSession{}, wrapRuntimeStage("end runtime session", "end", "EndMachineRuntimeAppSession", err)
	}
	if row.MachineID != in.MachineID {
		return db.MachineRuntimeAppSession{}, errors.New("machineruntime: session machine mismatch")
	}
	sess, err := s.q.GetCurrentMachineRuntimeAppSession(ctx, in.MachineID)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = s.q.UpdateMachineCurrentRuntimeAppSession(ctx, db.UpdateMachineCurrentRuntimeAppSessionParams{
			ID:                         in.MachineID,
			CurrentRuntimeAppSessionID: pgtype.UUID{},
		})
	} else if err == nil && sess.ID == row.ID {
		_ = s.q.UpdateMachineCurrentRuntimeAppSession(ctx, db.UpdateMachineCurrentRuntimeAppSessionParams{
			ID:                         in.MachineID,
			CurrentRuntimeAppSessionID: pgtype.UUID{},
		})
	}
	_ = s.projectRuntimeSnapshot(ctx, s.q, in.MachineID, row, "offline")
	return row, nil
}

// ForceEndRuntimeAppSession admin-closes an app runtime session after ownership check.
func (s *Service) ForceEndRuntimeAppSession(ctx context.Context, machineID, sessionID uuid.UUID, reason string) (db.MachineRuntimeAppSession, error) {
	if s == nil || s.q == nil {
		return db.MachineRuntimeAppSession{}, errors.New("machineruntime: nil service")
	}
	sess, err := s.q.GetMachineRuntimeAppSessionByID(ctx, sessionID)
	if err != nil {
		return db.MachineRuntimeAppSession{}, err
	}
	if sess.MachineID != machineID {
		return db.MachineRuntimeAppSession{}, errors.New("machineruntime: session not on machine")
	}
	if strings.TrimSpace(reason) == "" {
		reason = "ADMIN_FORCE_END"
	}
	return s.EndRuntimeAppSession(ctx, EndInput{
		SessionID: sessionID,
		MachineID: machineID,
		EndReason: reason,
		Status:    "ENDED",
	})
}

// MarkRuntimeAppSessionStale flags a session stale without ending it.
func (s *Service) MarkRuntimeAppSessionStale(ctx context.Context, machineID, sessionID uuid.UUID) (db.MachineRuntimeAppSession, error) {
	if s == nil || s.q == nil {
		return db.MachineRuntimeAppSession{}, errors.New("machineruntime: nil service")
	}
	row, err := s.q.MarkMachineRuntimeAppSessionStale(ctx, db.MarkMachineRuntimeAppSessionStaleParams{
		ID:        sessionID,
		MachineID: machineID,
	})
	if err != nil {
		return db.MachineRuntimeAppSession{}, err
	}
	if row.MachineID != machineID {
		return db.MachineRuntimeAppSession{}, errors.New("machineruntime: session not on machine")
	}
	online, _ := s.ComputeMachineOnlineStatus(ctx, machineID)
	_ = s.projectRuntimeSnapshot(ctx, s.q, machineID, row, online)
	return row, nil
}

// GetCurrentRuntimeAppSession returns the open app runtime session for a machine.
func (s *Service) GetCurrentRuntimeAppSession(ctx context.Context, machineID uuid.UUID) (db.MachineRuntimeAppSession, error) {
	return s.q.GetCurrentMachineRuntimeAppSession(ctx, machineID)
}

// ListRuntimeAppSessionHistory lists historical app runtime sessions.
func (s *Service) ListRuntimeAppSessionHistory(ctx context.Context, machineID uuid.UUID, limit, offset int32) ([]db.MachineRuntimeAppSession, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.q.ListMachineRuntimeAppSessionHistory(ctx, db.ListMachineRuntimeAppSessionHistoryParams{
		MachineID: machineID,
		LimitVal:  limit,
		OffsetVal: offset,
	})
}

// GetActiveDeviceAttachment returns the active board attachment.
func (s *Service) GetActiveDeviceAttachment(ctx context.Context, machineID uuid.UUID) (db.MachineDeviceAttachment, error) {
	return s.q.GetActiveMachineDeviceAttachment(ctx, machineID)
}

// ListDeviceAttachments lists attachment history.
func (s *Service) ListDeviceAttachments(ctx context.Context, machineID uuid.UUID, limit, offset int32) ([]db.MachineDeviceAttachment, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.q.ListMachineDeviceAttachments(ctx, db.ListMachineDeviceAttachmentsParams{
		MachineID: machineID,
		LimitVal:  limit,
		OffsetVal: offset,
	})
}

// TouchMQTTSeen records MQTT activity on the current runtime session.
func (s *Service) TouchMQTTSeen(ctx context.Context, machineID uuid.UUID, mqttState string) error {
	sess, err := s.q.GetCurrentMachineRuntimeAppSession(ctx, machineID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := s.q.TouchRuntimeAppSessionMQTT(ctx, db.TouchRuntimeAppSessionMQTTParams{
		ID:             sess.ID,
		LastMqttSeenAt: pgtype.Timestamptz{Time: now, Valid: true},
		LastMqttState:  mqttState,
	}); err != nil {
		return err
	}
	if err := s.q.UpdateMachineOnlineStatus(ctx, db.UpdateMachineOnlineStatusParams{
		ID:           machineID,
		OnlineStatus: "online",
		LastSeenAt:   pgtype.Timestamptz{Time: now, Valid: true},
	}); err != nil {
		return err
	}
	sess.LastMqttSeenAt = pgtype.Timestamptz{Time: now, Valid: true}
	sess.LastMqttState = mqttState
	return s.projectRuntimeSnapshot(ctx, s.q, machineID, sess, "online")
}

func (s *Service) projectRuntimeSnapshot(ctx context.Context, q *db.Queries, machineID uuid.UUID, sess db.MachineRuntimeAppSession, online string) error {
	if q == nil {
		return nil
	}
	var attachID pgtype.UUID
	if sess.DeviceAttachmentID.Valid {
		attachID = sess.DeviceAttachmentID
	}
	var started, heartbeat pgtype.Timestamptz
	started = pgtype.Timestamptz{Time: sess.StartedAt.UTC(), Valid: true}
	if sess.LastHeartbeatAt.Valid {
		heartbeat = sess.LastHeartbeatAt
	}
	blockers, err := jsonArrayText(sess.Blockers)
	if err != nil {
		return err
	}
	var sessID pgtype.UUID
	if !sess.EndedAt.Valid {
		sessID = pgtype.UUID{Bytes: sess.ID, Valid: true}
	}
	if err := q.UpdateMachineCurrentSnapshotRuntime(ctx, db.UpdateMachineCurrentSnapshotRuntimeParams{
		MachineID:                  machineID,
		CurrentDeviceAttachmentID:  attachID,
		CurrentRuntimeAppSessionID: sessID,
		OnlineStatus:               online,
		RuntimeSessionStatus:       sess.Status,
		RuntimeStartReason:         sess.StartReason,
		RuntimeStartedAt:           started,
		RuntimeLastHeartbeatAt:     heartbeat,
		LastMqttState:              sess.LastMqttState,
		StorefrontState:            sess.StorefrontState,
		SellReady:                  sess.SellReady,
		Blockers:                   blockers,
	}); err != nil {
		logJSONBindAudit("snapshot", err, blockers, "", "", "", "", "")
		return err
	}
	return nil
}

// ComputeMachineOnlineStatus derives online/stale/offline from last signals.
func (s *Service) ComputeMachineOnlineStatus(ctx context.Context, machineID uuid.UUID) (string, error) {
	sess, err := s.q.GetCurrentMachineRuntimeAppSession(ctx, machineID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "unknown", nil
	}
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	last := sess.StartedAt
	if sess.LastHeartbeatAt.Valid {
		last = sess.LastHeartbeatAt.Time
	}
	if sess.LastMqttSeenAt.Valid && sess.LastMqttSeenAt.Time.After(last) {
		last = sess.LastMqttSeenAt.Time
	}
	if sess.LastCheckInAt.Valid && sess.LastCheckInAt.Time.After(last) {
		last = sess.LastCheckInAt.Time
	}
	age := now.Sub(last)
	switch {
	case age <= s.onlineThreshold:
		return "online", nil
	case age <= s.staleThreshold:
		return "stale", nil
	case age <= s.offlineThreshold:
		return "offline", nil
	default:
		if sess.EndedAt.Valid {
			return "offline", nil
		}
		return "offline", nil
	}
}

func uuidToPg(id *uuid.UUID) pgtype.UUID {
	if id == nil || *id == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func pgText(v string) pgtype.Text {
	v = strings.TrimSpace(v)
	if v == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v, Valid: true}
}

func int32PtrToPg(v *int32) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *v, Valid: true}
}

func int64PtrToPg(v *int64) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *v, Valid: true}
}

func defaultJSON(v json.RawMessage) json.RawMessage {
	if v == nil {
		return json.RawMessage("{}")
	}
	return v
}
