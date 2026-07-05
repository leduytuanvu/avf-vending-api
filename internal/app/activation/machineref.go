package activation

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/machineruntime"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrMachineIdentifierRequired is returned when no machine id or code was provided.
	ErrMachineIdentifierRequired = errors.New("activation: machine identifier required")
	// ErrInvalidMachineIdentifier is returned when the ref is neither a UUID nor a valid AVF code.
	ErrInvalidMachineIdentifier = errors.New("activation: invalid machine identifier")
	// ErrMachineNotFound is returned when the identifier does not match a machine row.
	ErrMachineNotFound = errors.New("activation: machine not found")
	// ErrMachineIdentifierConflict is returned when body id and code resolve to different machines.
	ErrMachineIdentifierConflict = errors.New("activation: machine identifier conflict")
)

var activationMachineCodePattern = regexp.MustCompile(`^AVF[0-9]{6}$`)

func validActivationMachineCode(code string) bool {
	return activationMachineCodePattern.MatchString(machineruntime.NormalizeMachineCode(code))
}

// MachineIdentityRef is the resolved canonical machine identity for activation admin routes.
type MachineIdentityRef struct {
	MachineID   uuid.UUID
	MachineCode string
}

// ResolveMachineRef resolves a path or body ref as either machine UUID or AVF000001-style code.
func ResolveMachineRef(ctx context.Context, pool *pgxpool.Pool, ref string) (MachineIdentityRef, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return MachineIdentityRef{}, ErrMachineIdentifierRequired
	}
	q := db.New(pool)
	if id, parseErr := uuid.Parse(ref); parseErr == nil && id != uuid.Nil {
		m, qerr := q.GetMachineByID(ctx, id)
		if qerr != nil {
			if qerr == pgx.ErrNoRows {
				return MachineIdentityRef{}, ErrMachineNotFound
			}
			return MachineIdentityRef{}, qerr
		}
		return MachineIdentityRef{MachineID: m.ID, MachineCode: strings.TrimSpace(m.Code)}, nil
	}
	normalized := machineruntime.NormalizeMachineCode(ref)
	if !validActivationMachineCode(normalized) {
		return MachineIdentityRef{}, ErrInvalidMachineIdentifier
	}
	m, qerr := q.GetMachineByCode(ctx, normalized)
	if qerr != nil {
		if qerr == pgx.ErrNoRows {
			return MachineIdentityRef{}, ErrMachineNotFound
		}
		return MachineIdentityRef{}, qerr
	}
	return MachineIdentityRef{MachineID: m.ID, MachineCode: strings.TrimSpace(m.Code)}, nil
}

// ResolveMachineBody resolves machineId/machineCode fields from a catalog create body.
func ResolveMachineBody(ctx context.Context, pool *pgxpool.Pool, machineID, machineIDSnake, machineCode, machineCodeSnake string) (MachineIdentityRef, error) {
	rawID := strings.TrimSpace(machineID)
	if rawID == "" {
		rawID = strings.TrimSpace(machineIDSnake)
	}
	rawCode := strings.TrimSpace(machineCode)
	if rawCode == "" {
		rawCode = strings.TrimSpace(machineCodeSnake)
	}
	if rawID == "" && rawCode == "" {
		return MachineIdentityRef{}, ErrMachineIdentifierRequired
	}
	if rawID != "" && rawCode != "" {
		idResolved, err := ResolveMachineRef(ctx, pool, rawID)
		if err != nil {
			return MachineIdentityRef{}, err
		}
		codeResolved, err := ResolveMachineRef(ctx, pool, rawCode)
		if err != nil {
			return MachineIdentityRef{}, err
		}
		if idResolved.MachineID != codeResolved.MachineID {
			return MachineIdentityRef{}, ErrMachineIdentifierConflict
		}
		return codeResolved, nil
	}
	if rawID != "" {
		return ResolveMachineRef(ctx, pool, rawID)
	}
	return ResolveMachineRef(ctx, pool, rawCode)
}

// ResolveMachineRef resolves a machine ref using the service pool.
func (s *Service) ResolveMachineRef(ctx context.Context, ref string) (MachineIdentityRef, error) {
	if s == nil || s.pool == nil {
		return MachineIdentityRef{}, errors.New("activation: nil service")
	}
	return ResolveMachineRef(ctx, s.pool, ref)
}

// ResolveMachineBody resolves catalog body identifiers using the service pool.
func (s *Service) ResolveMachineBody(ctx context.Context, machineID, machineIDSnake, machineCode, machineCodeSnake string) (MachineIdentityRef, error) {
	if s == nil || s.pool == nil {
		return MachineIdentityRef{}, errors.New("activation: nil service")
	}
	return ResolveMachineBody(ctx, s.pool, machineID, machineIDSnake, machineCode, machineCodeSnake)
}
