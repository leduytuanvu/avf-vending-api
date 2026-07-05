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

// ResolveMachineRef resolves a path or body ref as either machine UUID or AVF000001-style code.
func ResolveMachineRef(ctx context.Context, pool *pgxpool.Pool, ref string) (machineID uuid.UUID, machineCode string, err error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return uuid.Nil, "", ErrMachineIdentifierRequired
	}
	q := db.New(pool)
	if id, parseErr := uuid.Parse(ref); parseErr == nil && id != uuid.Nil {
		m, qerr := q.GetMachineByID(ctx, id)
		if qerr != nil {
			if qerr == pgx.ErrNoRows {
				return uuid.Nil, "", ErrMachineNotFound
			}
			return uuid.Nil, "", qerr
		}
		return m.ID, strings.TrimSpace(m.Code), nil
	}
	normalized := machineruntime.NormalizeMachineCode(ref)
	if !validActivationMachineCode(normalized) {
		return uuid.Nil, "", ErrInvalidMachineIdentifier
	}
	m, qerr := q.GetMachineByCode(ctx, normalized)
	if qerr != nil {
		if qerr == pgx.ErrNoRows {
			return uuid.Nil, "", ErrMachineNotFound
		}
		return uuid.Nil, "", qerr
	}
	return m.ID, strings.TrimSpace(m.Code), nil
}

// ResolveMachineBody resolves machineId/machineCode fields from a catalog create body.
func ResolveMachineBody(ctx context.Context, pool *pgxpool.Pool, machineID, machineIDSnake, machineCode, machineCodeSnake string) (uuid.UUID, string, error) {
	rawID := strings.TrimSpace(machineID)
	if rawID == "" {
		rawID = strings.TrimSpace(machineIDSnake)
	}
	rawCode := strings.TrimSpace(machineCode)
	if rawCode == "" {
		rawCode = strings.TrimSpace(machineCodeSnake)
	}
	if rawID == "" && rawCode == "" {
		return uuid.Nil, "", ErrMachineIdentifierRequired
	}
	if rawID != "" && rawCode != "" {
		idResolved, _, err := ResolveMachineRef(ctx, pool, rawID)
		if err != nil {
			return uuid.Nil, "", err
		}
		codeResolved, codeOut, err := ResolveMachineRef(ctx, pool, rawCode)
		if err != nil {
			return uuid.Nil, "", err
		}
		if idResolved != codeResolved {
			return uuid.Nil, "", ErrMachineIdentifierConflict
		}
		return idResolved, codeOut, nil
	}
	if rawID != "" {
		return ResolveMachineRef(ctx, pool, rawID)
	}
	return ResolveMachineRef(ctx, pool, rawCode)
}

// ResolveMachineRef resolves a machine ref using the service pool.
func (s *Service) ResolveMachineRef(ctx context.Context, ref string) (uuid.UUID, string, error) {
	if s == nil || s.pool == nil {
		return uuid.Nil, "", errors.New("activation: nil service")
	}
	return ResolveMachineRef(ctx, s.pool, ref)
}

// ResolveMachineBody resolves catalog body identifiers using the service pool.
func (s *Service) ResolveMachineBody(ctx context.Context, machineID, machineIDSnake, machineCode, machineCodeSnake string) (uuid.UUID, string, error) {
	if s == nil || s.pool == nil {
		return uuid.Nil, "", errors.New("activation: nil service")
	}
	return ResolveMachineBody(ctx, s.pool, machineID, machineIDSnake, machineCode, machineCodeSnake)
}
