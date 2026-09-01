package auth

import (
	"errors"
	"strings"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func emailStringFromPG(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func emailPGFromOptional(raw string) (pgtype.Text, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return pgtype.Text{}, nil
	}
	normalized, err := normalizeEmail(v)
	if err != nil {
		return pgtype.Text{}, err
	}
	return pgtype.Text{String: normalized, Valid: true}, nil
}

func accountLabel(acct db.PlatformAuthAccount) string {
	if strings.TrimSpace(acct.Username) != "" {
		return acct.Username
	}
	return emailStringFromPG(acct.Email)
}

func mapAuthUniqueViolation(err error) error {
	var pe *pgconn.PgError
	if !errors.As(err, &pe) || pe.Code != "23505" {
		return err
	}
	cn := strings.ToLower(strings.TrimSpace(pe.ConstraintName))
	if strings.Contains(cn, "username") {
		return ErrConflictDuplicateUsername
	}
	if strings.Contains(cn, "email") {
		return ErrConflictDuplicateEmail
	}
	return ErrConflictDuplicateEmail
}
