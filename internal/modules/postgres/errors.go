package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrMachineOrganizationMismatch is returned when a row's machine_id does not belong to the expected organization_id.
var ErrMachineOrganizationMismatch = errors.New("postgres: machine does not belong to organization")

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
