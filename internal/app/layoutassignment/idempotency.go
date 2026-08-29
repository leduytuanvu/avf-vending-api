package layoutassignment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrIdempotencyKeyConflict is returned when an idempotency key was reused with a different payload.
var ErrIdempotencyKeyConflict = errors.New("layoutassignment: idempotency key conflict")

// ComputeAssignServerLayoutRequestHash returns a deterministic SHA-256 hex digest of the idempotent
// assignment payload (machine, layout version, optional org version, expected revision).
func ComputeAssignServerLayoutRequestHash(in AssignServerLayoutInput) string {
	var orgVer string
	if in.OrgLayoutVersionID != nil {
		orgVer = in.OrgLayoutVersionID.String()
	}
	var expectedRev *int32
	if in.ExpectedCurrentRevision != nil {
		v := *in.ExpectedCurrentRevision
		expectedRev = &v
	}
	canon := struct {
		MachineID               string `json:"machine_id"`
		LayoutVersionID         string `json:"layout_version_id"`
		OrgLayoutVersionID      string `json:"org_layout_version_id,omitempty"`
		ExpectedCurrentRevision *int32 `json:"expected_current_revision,omitempty"`
	}{
		MachineID:               in.MachineID.String(),
		LayoutVersionID:         in.LayoutVersionID.String(),
		OrgLayoutVersionID:      orgVer,
		ExpectedCurrentRevision: expectedRev,
	}
	b, _ := json.Marshal(canon)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func assignServerLayoutScopeID(machineID uuid.UUID) string {
	return machineID.String()
}

func (s *Service) beginAssignServerLayoutIdempotency(ctx context.Context, in AssignServerLayoutInput) (AssignServerLayoutResult, bool, error) {
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" {
		return AssignServerLayoutResult{}, false, nil
	}
	hash := ComputeAssignServerLayoutRequestHash(in)
	scopeID := assignServerLayoutScopeID(in.MachineID)

	q := pgxutil.NewQueries(s.Pool)
	row, err := q.GetLayoutAssignmentIdempotency(ctx, db.GetLayoutAssignmentIdempotencyParams{
		ScopeID:        scopeID,
		IdempotencyKey: key,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return AssignServerLayoutResult{}, false, nil
		}
		return AssignServerLayoutResult{}, false, err
	}
	if !strings.EqualFold(strings.TrimSpace(row.RequestHash), hash) {
		return AssignServerLayoutResult{}, false, ErrIdempotencyKeyConflict
	}
	var out AssignServerLayoutResult
	if err := json.Unmarshal(row.ResponseJson, &out); err != nil {
		return AssignServerLayoutResult{}, false, err
	}
	return out, true, nil
}

func (s *Service) storeAssignServerLayoutIdempotency(ctx context.Context, in AssignServerLayoutInput, out AssignServerLayoutResult) error {
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" {
		return nil
	}
	resp, err := json.Marshal(out)
	if err != nil {
		return err
	}
	q := pgxutil.NewQueries(s.Pool)
	err = q.InsertLayoutAssignmentIdempotency(ctx, db.InsertLayoutAssignmentIdempotencyParams{
		ScopeID:        assignServerLayoutScopeID(in.MachineID),
		IdempotencyKey: key,
		RequestHash:    ComputeAssignServerLayoutRequestHash(in),
		ResponseJson:   resp,
	})
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		replayed, replay, rerr := s.beginAssignServerLayoutIdempotency(ctx, in)
		if rerr != nil {
			return rerr
		}
		if !replay {
			return ErrIdempotencyKeyConflict
		}
		if replayed.AssignmentID != out.AssignmentID {
			return ErrIdempotencyKeyConflict
		}
		return nil
	}
	return err
}
