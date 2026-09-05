package pgjson

import (
	"bytes"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgtype"
)

// TextJSON is optional jsonb sent as UTF-8 text so PostgreSQL can CAST to jsonb.
// Passing []byte makes pgx encode bytea, which yields SQLSTATE 22P02 on json/jsonb columns.
func TextJSON(b []byte) pgtype.Text {
	trim := bytes.TrimSpace(b)
	if len(trim) == 0 || bytes.Equal(trim, []byte("null")) {
		return pgtype.Text{}
	}
	if !json.Valid(trim) {
		return pgtype.Text{String: "{}", Valid: true}
	}
	return pgtype.Text{String: string(trim), Valid: true}
}

// RequiredString is non-null jsonb text (empty becomes {}).
func RequiredString(b []byte) string {
	trim := bytes.TrimSpace(b)
	if len(trim) == 0 || bytes.Equal(trim, []byte("null")) || !json.Valid(trim) {
		return "{}"
	}
	return string(trim)
}

// OptionalString is nullable jsonb text sent as UTF-8; empty becomes SQL NULL via NULLIF(..., ”)::jsonb.
func OptionalString(b []byte) string {
	trim := bytes.TrimSpace(b)
	if len(trim) == 0 || bytes.Equal(trim, []byte("null")) {
		return ""
	}
	if !json.Valid(trim) {
		return "{}"
	}
	return string(trim)
}
