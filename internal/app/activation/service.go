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

	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
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
)

// Service manages kiosk activation codes.
type Service struct {
	pool   *pgxpool.Pool
	issuer *plauth.SessionIssuer
	pepper []byte
}

// NewService constructs an activation service.
func NewService(pool *pgxpool.Pool, issuer *plauth.SessionIssuer, pepper []byte) *Service {
	if pool == nil || issuer == nil || len(pepper) == 0 {
		panic("activation.NewService: pool, issuer, and pepper are required")
	}
	return &Service{pool: pool, issuer: issuer, pepper: pepper}
}

// CreateInput is an admin create request.
type CreateInput struct {
	MachineID        uuid.UUID
	OrganizationID   uuid.UUID
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
	if in.MachineID == uuid.Nil || in.OrganizationID == uuid.Nil {
		return CreateResult{}, fmt.Errorf("activation: machine and organization required")
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
		MachineID:      in.MachineID,
		OrganizationID: in.OrganizationID,
		CodeHash:       hash,
		MaxUses:        in.MaxUses,
		Uses:           0,
		ExpiresAt:      exp,
		Notes:          notes,
		Status:         "active",
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

// ListCodes returns activation rows for a machine (admin; org must match).
func (s *Service) ListCodes(ctx context.Context, machineID, organizationID uuid.UUID) ([]ListRow, error) {
	mOrg, err := db.New(s.pool).GetMachineOrganizationID(ctx, machineID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if mOrg != organizationID {
		return nil, ErrUnauthorized
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
func (s *Service) RevokeCode(ctx context.Context, machineID, organizationID, codeID uuid.UUID) error {
	_, err := db.New(s.pool).RevokeMachineActivationCode(ctx, db.RevokeMachineActivationCodeParams{
		ID:             codeID,
		MachineID:      machineID,
		OrganizationID: organizationID,
	})
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
}

// ClaimResult is returned on successful claim (and idempotent replay).
type ClaimResult struct {
	MachineID       uuid.UUID
	OrganizationID  uuid.UUID
	SiteID          uuid.UUID
	MachineName     string
	MachineToken    string
	TokenExpiresAt  time.Time
	MQTTBrokerURL   string
	MQTTTopicPrefix string
	BootstrapPath   string
}

// Claim exchanges a valid activation code for a machine token.
func (s *Service) Claim(ctx context.Context, in ClaimInput, mqttBrokerURL, mqttTopicPrefix string) (ClaimResult, error) {
	code := normalizeActivationCode(in.ActivationCode)
	if code == "" {
		return ClaimResult{}, ErrInvalid
	}
	fpJSON, err := json.Marshal(in.DeviceFingerprint)
	if err != nil {
		return ClaimResult{}, err
	}
	fpHash := sha256.Sum256(fpJSON)
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
	if row.Status == "revoked" || now.After(row.ExpiresAt) {
		return ClaimResult{}, ErrInvalid
	}
	if len(row.ClaimedFingerprintHash) > 0 {
		if !hmac.Equal(row.ClaimedFingerprintHash, fpHash[:]) {
			return ClaimResult{}, ErrInvalid
		}
		// Idempotent success path (token re-issued).
		m, err := qtx.GetMachineByID(ctx, row.MachineID)
		if err != nil {
			return ClaimResult{}, err
		}
		tok, exp, err := s.issuer.IssueMachineAccessJWT(row.MachineID, row.OrganizationID, m.SiteID)
		if err != nil {
			return ClaimResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return ClaimResult{}, err
		}
		return ClaimResult{
			MachineID:       row.MachineID,
			OrganizationID:  row.OrganizationID,
			SiteID:          m.SiteID,
			MachineName:     m.Name,
			MachineToken:    tok,
			TokenExpiresAt:  exp.UTC(),
			MQTTBrokerURL:   mqttBrokerURL,
			MQTTTopicPrefix: mqttTopicPrefix,
			BootstrapPath:   fmt.Sprintf("/v1/setup/machines/%s/bootstrap", row.MachineID),
		}, nil
	}
	if row.Status != "active" || row.Uses >= row.MaxUses {
		return ClaimResult{}, ErrInvalid
	}
	_, err = qtx.MarkActivationCodeUsed(ctx, db.MarkActivationCodeUsedParams{
		ID:                     row.ID,
		ClaimedFingerprintHash: fpHash[:],
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return ClaimResult{}, ErrInvalid
		}
		return ClaimResult{}, err
	}
	m, err := qtx.GetMachineByID(ctx, row.MachineID)
	if err != nil {
		return ClaimResult{}, err
	}
	tok, exp, err := s.issuer.IssueMachineAccessJWT(row.MachineID, row.OrganizationID, m.SiteID)
	if err != nil {
		return ClaimResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ClaimResult{}, err
	}
	return ClaimResult{
		MachineID:       row.MachineID,
		OrganizationID:  row.OrganizationID,
		SiteID:          m.SiteID,
		MachineName:     m.Name,
		MachineToken:    tok,
		TokenExpiresAt:  exp.UTC(),
		MQTTBrokerURL:   mqttBrokerURL,
		MQTTTopicPrefix: mqttTopicPrefix,
		BootstrapPath:   fmt.Sprintf("/v1/setup/machines/%s/bootstrap", row.MachineID),
	}, nil
}

func normalizeActivationCode(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
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
