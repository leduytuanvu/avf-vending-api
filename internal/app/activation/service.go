package activation

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/machineruntime"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/emqxadmin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrInvalid is returned for all public claim failures (constant time messaging at HTTP layer).
	ErrInvalid      = errors.New("activation: invalid or unusable code")
	ErrNotFound     = errors.New("activation: not found")
	ErrUnauthorized = errors.New("activation: unauthorized")
	// ErrRefreshInvalid is returned for unusable machine refresh tokens.
	ErrRefreshInvalid = errors.New("activation: invalid refresh token")
	// ErrMachineNotEligible is returned when the machine cannot authenticate (retired / bad state).
	ErrMachineNotEligible = errors.New("activation: machine not eligible")
	// ErrMQTTProvisioning is returned when EMQX is configured but per-machine MQTT provisioning fails.
	ErrMQTTProvisioning = errors.New("activation: mqtt provisioning failed")
)

// Service manages kiosk activation codes.
type Service struct {
	pool    *pgxpool.Pool
	issuer  *plauth.SessionIssuer
	pepper  []byte
	audit   compliance.EnterpriseRecorder
	emqx    *emqxadmin.Client
	runtime *machineruntime.Service
}

// NewService constructs an activation service.
func NewService(pool *pgxpool.Pool, issuer *plauth.SessionIssuer, pepper []byte, audit compliance.EnterpriseRecorder) *Service {
	if pool == nil || issuer == nil || len(pepper) == 0 {
		panic("activation.NewService: pool, issuer, and pepper are required")
	}
	return &Service{pool: pool, issuer: issuer, pepper: pepper, audit: audit}
}

// SetEMQXClient enables per-machine MQTT user provisioning during activation claim.
func (s *Service) SetEMQXClient(c *emqxadmin.Client) {
	if s == nil {
		return
	}
	s.emqx = c
}

// SetMachineRuntime wires device attachment + runtime session lifecycle for reattach flows.
func (s *Service) SetMachineRuntime(rt *machineruntime.Service) {
	if s == nil {
		return
	}
	s.runtime = rt
}

// CreateInput is an admin create request.
type CreateInput struct {
	MachineID        uuid.UUID
	ExpiresInMinutes int32
	MaxUses          int32
	Notes            string
}

// CreateResult includes the plaintext code once.
type CreateResult struct {
	PlaintextCode string
	ID            uuid.UUID
	MachineID     uuid.UUID
	ExpiresAt     time.Time
	MaxUses       int32
	RemainingUses int32
	Status        string
}

// CreateCode generates and stores a hashed activation code.
func (s *Service) CreateCode(ctx context.Context, in CreateInput) (CreateResult, error) {
	if in.MachineID == uuid.Nil {
		return CreateResult{}, fmt.Errorf("activation: machine required")
	}
	if in.ExpiresInMinutes <= 0 {
		in.ExpiresInMinutes = 1440
	}
	if in.MaxUses <= 0 {
		in.MaxUses = 1
	}
	plain, err := randomActivationCode()
	if err != nil {
		return CreateResult{}, err
	}
	hash := hashActivationCode(s.pepper, plain)
	exp := time.Now().UTC().Add(time.Duration(in.ExpiresInMinutes) * time.Minute)
	var notes pgtype.Text
	if strings.TrimSpace(in.Notes) != "" {
		notes = pgtype.Text{String: strings.TrimSpace(in.Notes), Valid: true}
	}
	row, err := db.New(s.pool).InsertMachineActivationCode(ctx, db.InsertMachineActivationCodeParams{
		MachineID: in.MachineID,
		CodeHash:  hash,
		MaxUses:   in.MaxUses,
		Uses:      0,
		ExpiresAt: exp,
		Notes:     notes,
		Status:    "active",
	})
	if err != nil {
		return CreateResult{}, err
	}
	return CreateResult{
		PlaintextCode: plain,
		ID:            row.ID,
		MachineID:     row.MachineID,
		ExpiresAt:     row.ExpiresAt.UTC(),
		MaxUses:       row.MaxUses,
		RemainingUses: row.MaxUses - row.Uses,
		Status:        row.Status,
	}, nil
}

// ListRow is safe for admin list (no plaintext, no hash).
type ListRow struct {
	ID            uuid.UUID
	MachineID     uuid.UUID
	ExpiresAt     time.Time
	MaxUses       int32
	Uses          int32
	RemainingUses int32
	Status        string
	Notes         string
	CreatedAt     time.Time
}

// ListCodes returns activation rows for a machine.
func (s *Service) ListCodes(ctx context.Context, machineID uuid.UUID) ([]ListRow, error) {
	_, err := db.New(s.pool).GetMachineByID(ctx, machineID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rows, err := db.New(s.pool).ListMachineActivationCodesForMachine(ctx, machineID)
	if err != nil {
		return nil, err
	}
	out := make([]ListRow, 0, len(rows))
	for _, r := range rows {
		n := ""
		if r.Notes.Valid {
			n = r.Notes.String
		}
		rem := r.MaxUses - r.Uses
		if rem < 0 {
			rem = 0
		}
		st := r.Status
		if time.Now().UTC().After(r.ExpiresAt) && st == "active" {
			st = "expired"
		}
		out = append(out, ListRow{
			ID:            r.ID,
			MachineID:     r.MachineID,
			ExpiresAt:     r.ExpiresAt.UTC(),
			MaxUses:       r.MaxUses,
			Uses:          r.Uses,
			RemainingUses: rem,
			Status:        st,
			Notes:         n,
			CreatedAt:     r.CreatedAt.UTC(),
		})
	}
	return out, nil
}

// RevokeCode marks a code revoked.
func (s *Service) RevokeCode(ctx context.Context, machineID, codeID uuid.UUID) error {
	_, err := db.New(s.pool).RevokeMachineActivationCode(ctx, db.RevokeMachineActivationCodeParams{
		ID:        codeID,
		MachineID: machineID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// ListAllCodes returns activation rows for the single-company admin list.
func (s *Service) ListAllCodes(ctx context.Context, limit, offset int32) ([]ListRow, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	q := db.New(s.pool)
	total, err := q.CountMachineActivationCodesAll(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := q.ListMachineActivationCodesPaged(ctx, db.ListMachineActivationCodesPagedParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]ListRow, 0, len(rows))
	for _, r := range rows {
		n := ""
		if r.Notes.Valid {
			n = r.Notes.String
		}
		rem := r.MaxUses - r.Uses
		if rem < 0 {
			rem = 0
		}
		st := r.Status
		if time.Now().UTC().After(r.ExpiresAt) && st == "active" {
			st = "expired"
		}
		out = append(out, ListRow{
			ID:            r.ID,
			MachineID:     r.MachineID,
			ExpiresAt:     r.ExpiresAt.UTC(),
			MaxUses:       r.MaxUses,
			Uses:          r.Uses,
			RemainingUses: rem,
			Status:        st,
			Notes:         n,
			CreatedAt:     r.CreatedAt.UTC(),
		})
	}
	return out, total, nil
}

// RevokeCodeByID revokes a code by id.
func (s *Service) RevokeCodeByID(ctx context.Context, codeID uuid.UUID) error {
	_, err := db.New(s.pool).RevokeMachineActivationCodeActive(ctx, codeID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// DeviceFingerprint is submitted on claim.
type DeviceFingerprint struct {
	AndroidID    string `json:"androidId"`
	SerialNumber string `json:"serialNumber"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	PackageName  string `json:"packageName"`
	VersionName  string `json:"versionName"`
	VersionCode  int    `json:"versionCode"`
}

// ClaimInput is the public claim body.
type ClaimInput struct {
	ActivationCode    string
	DeviceFingerprint DeviceFingerprint
	// ClientIP and UserAgent are optional hints for machine_activation_claims (HTTP may populate both).
	ClientIP  string
	UserAgent string
	// Optional accountability context (headers/body); persisted via extended insert when any field set.
	ClaimContext
}

// ClaimResult is returned on successful claim (and idempotent replay).
type ClaimResult struct {
	MachineID         uuid.UUID
	SiteID            uuid.UUID
	MachineName       string
	MachineToken      string
	TokenExpiresAt    time.Time
	RefreshToken      string
	RefreshExpiresAt  time.Time
	MQTTBrokerURL     string
	MQTTTopicPrefix   string
	MQTTTopicLayout   string
	MQTTUsername      string
	MQTTPassword      string
	BootstrapPath     string
	BootstrapRequired bool
}

func (s *Service) refreshTTL() time.Duration {
	if s == nil || s.issuer == nil {
		return 720 * time.Hour
	}
	if ttl := s.issuer.MachineRefreshTokenTTL(); ttl > 0 {
		return ttl
	}
	return 720 * time.Hour
}

func (s *Service) refreshGracePeriod() time.Duration {
	return 60 * time.Second
}

func sessionAllowsRefresh(sess db.MachineSession, now time.Time, grace time.Duration) (graceReplay bool, ok bool) {
	if !sess.ExpiresAt.After(now) {
		return false, false
	}
	if sess.Status == "active" && !sess.RevokedAt.Valid {
		return false, true
	}
	if sess.Status == "revoked" && sess.RevokedAt.Valid {
		revokedAt := sess.RevokedAt.Time
		if now.Sub(revokedAt) <= grace {
			return true, true
		}
	}
	return false, false
}

func machineEligibleForRuntime(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "maintenance", "retired", "decommissioned", "suspended", "compromised":
		return false
	default:
		return true
	}
}

func activationCodeIDPG(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: id != uuid.Nil}
}

func claimContextProvided(cc ClaimContext) bool {
	return cc.ActivatedByAccountID != nil ||
		cc.OperatorSessionID != nil ||
		strings.TrimSpace(cc.RequestID) != "" ||
		cc.CorrelationID != nil ||
		strings.TrimSpace(cc.AppVersion) != "" ||
		strings.TrimSpace(cc.BootID) != "" ||
		strings.TrimSpace(cc.DeviceSerial) != "" ||
		strings.TrimSpace(cc.Reason) != "" ||
		strings.TrimSpace(cc.ActivationSource) != ""
}

func insertActivationClaimRecord(
	ctx context.Context,
	qtx *db.Queries,
	codeID uuid.UUID,
	machineID uuid.UUID,
	fpHash []byte,
	ip, ua, result, failure string,
	cc ClaimContext,
) error {
	if claimContextProvided(cc) {
		params := db.InsertMachineActivationClaimExtendedParams{
			ActivationCodeID: activationCodeIDPG(codeID),
			MachineID:        machineID,
			FingerprintHash:  fpHash,
			IpAddress:        ip,
			UserAgent:        ua,
			Result:           result,
			FailureReason:    failure,
			AppVersion:       pgtype.Text{String: strings.TrimSpace(cc.AppVersion), Valid: strings.TrimSpace(cc.AppVersion) != ""},
			BootID:           pgtype.Text{String: strings.TrimSpace(cc.BootID), Valid: strings.TrimSpace(cc.BootID) != ""},
			DeviceSerial:     pgtype.Text{String: strings.TrimSpace(cc.DeviceSerial), Valid: strings.TrimSpace(cc.DeviceSerial) != ""},
			Reason:           pgtype.Text{String: strings.TrimSpace(cc.Reason), Valid: strings.TrimSpace(cc.Reason) != ""},
			ActivationSource: pgtype.Text{String: strings.TrimSpace(cc.ActivationSource), Valid: strings.TrimSpace(cc.ActivationSource) != ""},
			RequestID:        pgtype.Text{String: strings.TrimSpace(cc.RequestID), Valid: strings.TrimSpace(cc.RequestID) != ""},
		}
		if cc.ActivatedByAccountID != nil {
			params.ActivatedByAccountID = pgtype.UUID{Bytes: *cc.ActivatedByAccountID, Valid: true}
		}
		if cc.OperatorSessionID != nil {
			params.OperatorSessionID = pgtype.UUID{Bytes: *cc.OperatorSessionID, Valid: true}
		}
		if cc.CorrelationID != nil {
			params.CorrelationID = pgtype.UUID{Bytes: *cc.CorrelationID, Valid: true}
		}
		if strings.TrimSpace(cc.ActivationSource) == "" && result == "succeeded" {
			params.ActivationSource = pgtype.Text{String: "activation_code", Valid: true}
		}
		_, err := qtx.InsertMachineActivationClaimExtended(ctx, params)
		return err
	}
	_, err := qtx.InsertMachineActivationClaim(ctx, db.InsertMachineActivationClaimParams{
		ActivationCodeID: activationCodeIDPG(codeID),
		MachineID:        machineID,
		FingerprintHash:  fpHash,
		IpAddress:        ip,
		UserAgent:        ua,
		Result:           result,
		FailureReason:    failure,
	})
	return err
}

func machineEligibleForClaim(m db.Machine) bool {
	if !machineEligibleForRuntime(m.Status) {
		return false
	}
	if m.CredentialRevokedAt.Valid {
		return false
	}
	return true
}

func (s *Service) ensureActiveMachineCredential(ctx context.Context, q *db.Queries, scopeID, machineID uuid.UUID, ver int64) (db.MachineCredential, error) {
	row, err := q.GetMachineCredentialByMachineAndVersion(ctx, db.GetMachineCredentialByMachineAndVersionParams{
		MachineID:         machineID,
		CredentialVersion: ver,
	})
	if err == nil {
		switch strings.ToLower(strings.TrimSpace(row.Status)) {
		case "active":
			return row, nil
		default:
			return db.MachineCredential{}, ErrMachineNotEligible
		}
	}
	if err != pgx.ErrNoRows {
		return db.MachineCredential{}, err
	}
	return q.InsertMachineCredential(ctx, db.InsertMachineCredentialParams{
		MachineID:         machineID,
		CredentialVersion: ver,
		SecretHash:        nil,
		Status:            "active",
	})
}

// provisionMachineRefreshSession ensures the machine always receives a usable refresh session on
// activation/claim/reclaim. Any pre-existing active sessions are revoked, then a fresh refresh token is
// minted and persisted. Returns the non-empty plaintext refresh token, its expiry, and the session id for
// JWT binding. It never returns an access-only (blank refresh) result for an eligible machine.
func (s *Service) provisionMachineRefreshSession(ctx context.Context, q *db.Queries, machineID, scopeID uuid.UUID, m db.Machine, cred db.MachineCredential) (plainRefresh string, refreshExp time.Time, sessionID uuid.UUID, err error) {
	has, err := q.HasActiveMachineSession(ctx, machineID)
	if err != nil {
		return "", time.Time{}, uuid.Nil, err
	}
	if has {
		if err := q.RevokeAllMachineSessionsForMachine(ctx, machineID); err != nil {
			return "", time.Time{}, uuid.Nil, err
		}
	}
	raw, hash, err := plauth.NewRefreshToken()
	if err != nil {
		return "", time.Time{}, uuid.Nil, err
	}
	refreshJTI := uuid.NewString()
	exp := time.Now().UTC().Add(s.refreshTTL())
	sess, err := q.InsertMachineSession(ctx, db.InsertMachineSessionParams{
		MachineID:         machineID,
		CredentialID:      cred.ID,
		RefreshTokenHash:  hash,
		AccessTokenJti:    pgtype.Text{},
		RefreshTokenJti:   refreshJTI,
		CredentialVersion: m.CredentialVersion,
		Status:            "active",
		ExpiresAt:         exp,
		UserAgent:         pgtype.Text{},
		IpAddress:         pgtype.Text{},
	})
	if err != nil {
		return "", time.Time{}, uuid.Nil, err
	}
	return raw, exp, sess.ID, nil
}

func (s *Service) recordActivationRejectedAudit(ctx context.Context, scopeID, machineID uuid.UUID, meta map[string]any) {
	if s == nil || s.audit == nil || scopeID == uuid.Nil {
		return
	}
	md, _ := json.Marshal(meta)
	if len(md) == 0 {
		md = []byte("{}")
	}
	mid := machineID.String()
	_ = s.audit.Record(ctx, compliance.EnterpriseAuditRecord{
		ActorType:    compliance.ActorMachine,
		ActorID:      &mid,
		Action:       compliance.ActionMachineActivationRejected,
		ResourceType: "machine_activation_code",
		ResourceID:   &mid,
		Metadata:     md,
	})
}

func (s *Service) recordRefreshFailureAudit(ctx context.Context, scopeID uuid.UUID, machineID uuid.UUID, reason string) {
	if s == nil || s.audit == nil || scopeID == uuid.Nil {
		return
	}
	var actorID *string
	var resourceID *string
	if machineID != uuid.Nil {
		sid := machineID.String()
		actorID = &sid
		resourceID = &sid
	}
	md, _ := json.Marshal(map[string]any{"reason": reason})
	_ = s.audit.Record(ctx, compliance.EnterpriseAuditRecord{
		ActorType:    compliance.ActorMachine,
		ActorID:      actorID,
		Action:       compliance.ActionMachineAuthFailed,
		ResourceType: "machine",
		ResourceID:   resourceID,
		Metadata:     md,
	})
}

func (s *Service) deliverActivationClaim(ctx context.Context, tx pgx.Tx, row db.MachineActivationCode, mqttBrokerURL, mqttTopicPrefix, mqttTopicLayout string, auditMeta map[string]any) (ClaimResult, error) {
	qtx := db.New(tx)
	m, err := qtx.GetMachineByID(ctx, row.MachineID)
	if err != nil {
		return ClaimResult{}, err
	}
	if !machineEligibleForClaim(m) {
		return ClaimResult{}, ErrMachineNotEligible
	}
	cred, err := s.ensureActiveMachineCredential(ctx, qtx, uuid.Nil, row.MachineID, m.CredentialVersion)
	if err != nil {
		return ClaimResult{}, err
	}
	plainRefresh, refreshExp, sessionID, err := s.provisionMachineRefreshSession(ctx, qtx, row.MachineID, uuid.Nil, m, cred)
	if err != nil {
		return ClaimResult{}, err
	}
	tok, exp, err := s.issuer.IssueMachineAccessJWT(row.MachineID, m.SiteID, m.CredentialVersion, sessionID)
	if err != nil {
		return ClaimResult{}, err
	}
	meta := auditMeta
	if meta == nil {
		meta = map[string]any{}
	}
	meta["session_id"] = sessionID.String()
	md, _ := json.Marshal(meta)
	md = compliance.SanitizeJSONBytes(md)
	if len(md) == 0 || string(md) == "null" {
		md = []byte("{}")
	}
	mid := row.MachineID
	actorStr := mid.String()
	var sitePtr *uuid.UUID
	if m.SiteID != uuid.Nil {
		sid := m.SiteID
		sitePtr = &sid
	}
	if s.audit != nil {
		if err := s.audit.RecordCriticalTx(ctx, tx, compliance.EnterpriseAuditRecord{
			ActorType:    compliance.ActorMachine,
			ActorID:      &actorStr,
			Action:       compliance.ActionMachineActivationClaimed,
			ResourceType: "machine",
			ResourceID:   &actorStr,
			MachineID:    &mid,
			SiteID:       sitePtr,
			Metadata:     md,
		}); err != nil {
			return ClaimResult{}, err
		}
	}
	mqttUser, mqttPass, err := s.provisionMachineMQTT(ctx, qtx, row.MachineID)
	if err != nil {
		return ClaimResult{}, err
	}
	if s.emqx != nil && (mqttUser == "" || mqttPass == "") {
		return ClaimResult{}, ErrMQTTProvisioning
	}
	if err := tx.Commit(ctx); err != nil {
		if s.emqx != nil && mqttUser != "" {
			_ = s.emqx.DeleteUser(context.WithoutCancel(ctx), mqttUser)
		}
		return ClaimResult{}, err
	}
	return ClaimResult{
		MachineID:         row.MachineID,
		SiteID:            m.SiteID,
		MachineName:       m.Name,
		MachineToken:      tok,
		TokenExpiresAt:    exp.UTC(),
		RefreshToken:      plainRefresh,
		RefreshExpiresAt:  refreshExp,
		MQTTBrokerURL:     mqttBrokerURL,
		MQTTTopicPrefix:   mqttTopicPrefix,
		MQTTTopicLayout:   mqttTopicLayout,
		MQTTUsername:      mqttUser,
		MQTTPassword:      mqttPass,
		BootstrapPath:     fmt.Sprintf("/v1/setup/machines/%s/bootstrap", row.MachineID),
		BootstrapRequired: true,
	}, nil
}

// Claim exchanges a valid activation code for a machine token.
func (s *Service) Claim(ctx context.Context, in ClaimInput, mqttBrokerURL, mqttTopicPrefix, mqttTopicLayout string) (ClaimResult, error) {
	code := normalizeActivationCode(in.ActivationCode)
	if code == "" {
		return ClaimResult{}, ErrInvalid
	}
	fpJSON, err := json.Marshal(in.DeviceFingerprint)
	if err != nil {
		return ClaimResult{}, err
	}
	fpHash := sha256.Sum256(fpJSON)
	ip := strings.TrimSpace(in.ClientIP)
	ua := strings.TrimSpace(in.UserAgent)

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ClaimResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := db.New(tx)
	row, err := qtx.GetMachineActivationCodeByHashForUpdate(ctx, hashActivationCode(s.pepper, code))
	if err != nil {
		if err == pgx.ErrNoRows {
			return ClaimResult{}, ErrInvalid
		}
		return ClaimResult{}, err
	}
	now := time.Now().UTC()
	if strings.EqualFold(strings.TrimSpace(row.Status), "revoked") || now.After(row.ExpiresAt) {
		return ClaimResult{}, ErrInvalid
	}

	prevSucc, err := qtx.GetSucceededMachineActivationClaimByCodeAndFingerprint(ctx, db.GetSucceededMachineActivationClaimByCodeAndFingerprintParams{
		ActivationCodeID: activationCodeIDPG(row.ID),
		FingerprintHash:  fpHash[:],
	})
	if err == nil {
		return s.deliverActivationClaim(ctx, tx, row, mqttBrokerURL, mqttTopicPrefix, mqttTopicLayout, map[string]any{
			"idempotent_replay":   true,
			"activation_claim_id": prevSucc.ID.String(),
		})
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ClaimResult{}, err
	}

	if !strings.EqualFold(strings.TrimSpace(row.Status), "active") {
		return ClaimResult{}, ErrInvalid
	}

	nSucc, err := qtx.CountSucceededMachineActivationClaims(ctx, activationCodeIDPG(row.ID))
	if err != nil {
		return ClaimResult{}, err
	}
	if nSucc >= int64(row.MaxUses) {
		if ierr := insertActivationClaimRecord(ctx, qtx, row.ID, row.MachineID, fpHash[:], ip, ua, "rejected", "max_uses_exhausted", in.ClaimContext); ierr != nil {
			return ClaimResult{}, ierr
		}
		if err := tx.Commit(ctx); err != nil {
			return ClaimResult{}, err
		}
		s.recordActivationRejectedAudit(ctx, uuid.Nil, row.MachineID, map[string]any{
			"reason":                "max_uses_exhausted",
			"activation_code_id":    row.ID.String(),
			"succeeded_claim_count": nSucc,
			"max_uses":              row.MaxUses,
		})
		return ClaimResult{}, ErrInvalid
	}

	m, err := qtx.GetMachineByID(ctx, row.MachineID)
	if err != nil {
		return ClaimResult{}, err
	}
	if !machineEligibleForClaim(m) {
		if ierr := insertActivationClaimRecord(ctx, qtx, row.ID, row.MachineID, fpHash[:], ip, ua, "failed", "machine_not_eligible", in.ClaimContext); ierr != nil {
			return ClaimResult{}, ierr
		}
		if err := tx.Commit(ctx); err != nil {
			return ClaimResult{}, err
		}
		s.recordActivationRejectedAudit(ctx, uuid.Nil, row.MachineID, map[string]any{
			"reason":             "machine_not_eligible",
			"activation_code_id": row.ID.String(),
		})
		return ClaimResult{}, ErrMachineNotEligible
	}

	if err := insertActivationClaimRecord(ctx, qtx, row.ID, row.MachineID, fpHash[:], ip, ua, "succeeded", "", in.ClaimContext); err != nil {
		return ClaimResult{}, err
	}

	row, err = qtx.RefreshMachineActivationCodeAggregate(ctx, db.RefreshMachineActivationCodeAggregateParams{
		ID:                     row.ID,
		ClaimedFingerprintHash: fpHash[:],
	})
	if err != nil {
		return ClaimResult{}, err
	}

	return s.deliverActivationClaim(ctx, tx, row, mqttBrokerURL, mqttTopicPrefix, mqttTopicLayout, map[string]any{
		"first_claim": true,
	})
}

// RefreshInput exchanges a refresh token for rotated credentials.
type RefreshInput struct {
	RefreshToken string
}

// RefreshMachineSession rotates the opaque refresh token and issues a new access token without bumping credential_version.
func (s *Service) RefreshMachineSession(ctx context.Context, in RefreshInput, mqttBrokerURL, mqttTopicPrefix, mqttTopicLayout string) (ClaimResult, error) {
	raw := strings.TrimSpace(in.RefreshToken)
	if raw == "" {
		return ClaimResult{}, ErrRefreshInvalid
	}
	hash := plauth.HashRefreshToken(raw)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ClaimResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)
	sess, err := qtx.GetMachineSessionByRefreshHashForUpdate(ctx, hash)
	if err != nil {
		if err == pgx.ErrNoRows {
			s.recordRefreshFailureAudit(ctx, uuid.Nil, uuid.Nil, "unknown_refresh_hash")
			return ClaimResult{}, ErrRefreshInvalid
		}
		return ClaimResult{}, err
	}
	now := time.Now().UTC()
	grace := s.refreshGracePeriod()
	graceReplay, sessionOK := sessionAllowsRefresh(sess, now, grace)
	if !sessionOK {
		if sess.Status != "active" || sess.RevokedAt.Valid {
			s.recordRefreshFailureAudit(ctx, uuid.Nil, sess.MachineID, "session_revoked")
		} else {
			s.recordRefreshFailureAudit(ctx, uuid.Nil, sess.MachineID, "session_expired")
		}
		return ClaimResult{}, ErrRefreshInvalid
	}
	if !graceReplay {
		if err := qtx.MarkMachineSessionUsedByID(ctx, sess.ID); err != nil {
			return ClaimResult{}, err
		}
	} else if s.audit != nil {
		md, _ := json.Marshal(map[string]any{"session_id": sess.ID.String(), "grace_replay": true})
		md = compliance.SanitizeJSONBytes(md)
		mid := sess.MachineID
		actorStr := mid.String()
		_ = s.audit.Record(ctx, compliance.EnterpriseAuditRecord{
			ActorType:    compliance.ActorMachine,
			ActorID:      &actorStr,
			Action:       compliance.ActionMachineTokenRefreshed,
			ResourceType: "machine",
			ResourceID:   &actorStr,
			MachineID:    &mid,
			Metadata:     md,
		})
	}
	m, err := qtx.GetMachineByIDForUpdate(ctx, sess.MachineID)
	if err != nil {
		if err == pgx.ErrNoRows {
			s.recordRefreshFailureAudit(ctx, uuid.Nil, sess.MachineID, "machine_missing")
			return ClaimResult{}, ErrRefreshInvalid
		}
		return ClaimResult{}, err
	}
	if uuid.Nil != uuid.Nil {
		s.recordRefreshFailureAudit(ctx, uuid.Nil, sess.MachineID, "scope_mismatch")
		return ClaimResult{}, ErrRefreshInvalid
	}
	if !machineEligibleForClaim(m) {
		s.recordRefreshFailureAudit(ctx, uuid.Nil, m.ID, "machine_ineligible")
		return ClaimResult{}, ErrMachineNotEligible
	}
	if m.CredentialVersion != sess.CredentialVersion {
		s.recordRefreshFailureAudit(ctx, uuid.Nil, m.ID, "credential_version_stale")
		return ClaimResult{}, ErrRefreshInvalid
	}
	cred, err := qtx.GetMachineCredentialByMachineAndVersion(ctx, db.GetMachineCredentialByMachineAndVersionParams{
		MachineID:         m.ID,
		CredentialVersion: m.CredentialVersion,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			s.recordRefreshFailureAudit(ctx, uuid.Nil, m.ID, "credential_missing")
			return ClaimResult{}, ErrRefreshInvalid
		}
		return ClaimResult{}, err
	}
	if strings.ToLower(strings.TrimSpace(cred.Status)) != "active" {
		s.recordRefreshFailureAudit(ctx, uuid.Nil, m.ID, "credential_inactive")
		return ClaimResult{}, ErrRefreshInvalid
	}
	if cred.ID != sess.CredentialID {
		s.recordRefreshFailureAudit(ctx, uuid.Nil, m.ID, "credential_id_mismatch")
		return ClaimResult{}, ErrRefreshInvalid
	}
	if graceReplay {
		if err := qtx.RevokeAllMachineSessionsForMachine(ctx, sess.MachineID); err != nil {
			return ClaimResult{}, err
		}
	} else if err := qtx.RevokeMachineSessionByID(ctx, sess.ID); err != nil {
		return ClaimResult{}, err
	}
	newPlain, newHash, err := plauth.NewRefreshToken()
	if err != nil {
		return ClaimResult{}, err
	}
	rexp := time.Now().UTC().Add(s.refreshTTL())
	newSess, err := qtx.InsertMachineSession(ctx, db.InsertMachineSessionParams{
		MachineID:         m.ID,
		CredentialID:      cred.ID,
		RefreshTokenHash:  newHash,
		AccessTokenJti:    pgtype.Text{},
		RefreshTokenJti:   uuid.NewString(),
		CredentialVersion: m.CredentialVersion,
		Status:            "active",
		ExpiresAt:         rexp,
		UserAgent:         pgtype.Text{},
		IpAddress:         pgtype.Text{},
	})
	if err != nil {
		return ClaimResult{}, err
	}
	tok, exp, err := s.issuer.IssueMachineAccessJWT(m.ID, m.SiteID, m.CredentialVersion, newSess.ID)
	if err != nil {
		return ClaimResult{}, err
	}
	if s.audit != nil {
		md, _ := json.Marshal(map[string]any{"session_id": newSess.ID.String()})
		md = compliance.SanitizeJSONBytes(md)
		if len(md) == 0 || string(md) == "null" {
			md = []byte("{}")
		}
		mid := m.ID
		actorStr := mid.String()
		var sitePtr *uuid.UUID
		if m.SiteID != uuid.Nil {
			sid := m.SiteID
			sitePtr = &sid
		}
		if err := s.audit.RecordCriticalTx(ctx, tx, compliance.EnterpriseAuditRecord{
			ActorType:    compliance.ActorMachine,
			ActorID:      &actorStr,
			Action:       compliance.ActionMachineTokenRefreshed,
			ResourceType: "machine",
			ResourceID:   &actorStr,
			MachineID:    &mid,
			SiteID:       sitePtr,
			Metadata:     md,
		}); err != nil {
			return ClaimResult{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return ClaimResult{}, err
	}
	return ClaimResult{
		MachineID:         m.ID,
		SiteID:            m.SiteID,
		MachineName:       m.Name,
		MachineToken:      tok,
		TokenExpiresAt:    exp.UTC(),
		RefreshToken:      newPlain,
		RefreshExpiresAt:  rexp,
		MQTTBrokerURL:     mqttBrokerURL,
		MQTTTopicPrefix:   mqttTopicPrefix,
		MQTTTopicLayout:   mqttTopicLayout,
		BootstrapPath:     fmt.Sprintf("/v1/setup/machines/%s/bootstrap", m.ID),
		BootstrapRequired: false,
	}, nil
}

func normalizeActivationCode(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

func randomMQTTPassword() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", buf), nil
}

func (s *Service) emqxConfigured() bool {
	return s != nil && s.emqx != nil
}

func mqttSecretRef(version int64) string {
	return fmt.Sprintf("emqx:v1:%d", version)
}

func (s *Service) provisionMachineMQTT(ctx context.Context, q *db.Queries, machineID uuid.UUID) (string, string, error) {
	if s == nil || machineID == uuid.Nil {
		return "", "", nil
	}
	if !s.emqxConfigured() {
		return "", "", nil
	}
	password, err := randomMQTTPassword()
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrMQTTProvisioning, err)
	}
	username := machineID.String()
	if err := s.emqx.UpsertUser(ctx, username, password); err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrMQTTProvisioning, err)
	}
	if q != nil {
		if err := q.UpsertMachineMQTTCredentials(ctx, db.UpsertMachineMQTTCredentialsParams{
			MachineID:       machineID,
			MqttBrokerShard: "default",
			Username:        pgtype.Text{String: username, Valid: true},
			SecretRef:       pgtype.Text{String: mqttSecretRef(time.Now().UTC().UnixNano()), Valid: true},
		}); err != nil {
			return "", "", fmt.Errorf("%w: persist metadata: %v", ErrMQTTProvisioning, err)
		}
	}
	return username, password, nil
}

// RevokeMachineMQTT deletes the broker user and marks credential metadata revoked.
func (s *Service) RevokeMachineMQTT(ctx context.Context, machineID uuid.UUID) error {
	if s == nil || machineID == uuid.Nil || !s.emqxConfigured() {
		return nil
	}
	if err := s.emqx.DeleteUser(ctx, machineID.String()); err != nil {
		return err
	}
	return db.New(s.pool).RevokeMachineMQTTCredentials(ctx, machineID)
}

// RotateMachineMQTT issues a new broker password and updates metadata.
func (s *Service) RotateMachineMQTT(ctx context.Context, machineID uuid.UUID) (string, string, error) {
	return s.provisionMachineMQTT(ctx, db.New(s.pool), machineID)
}

func hashActivationCode(pepper []byte, code string) []byte {
	mac := hmac.New(sha256.New, pepper)
	_, _ = mac.Write([]byte(normalizeActivationCode(code)))
	return mac.Sum(nil)
}

func randomActivationCode() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	a := binary.BigEndian.Uint32(b[0:4]) % 0xffffff
	c := binary.BigEndian.Uint32(b[4:8]) % 0xffffff
	return fmt.Sprintf("AVF-%06X-%06X", a, c), nil
}
