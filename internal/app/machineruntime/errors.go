package machineruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrInvalidRuntimeJSON is returned when a client JSON payload is not valid UTF-8 JSON.
var ErrInvalidRuntimeJSON = errors.New("machineruntime: invalid json")

type runtimeSessionStageError struct {
	Op         string
	Stage      string
	SQLName    string
	SQLState   string
	Message    string
	Table      string
	Column     string
	Constraint string
	err        error
}

func (e *runtimeSessionStageError) Error() string {
	if e == nil {
		return "machineruntime: nil stage error"
	}
	op := e.Op
	if op == "" {
		op = "runtime session"
	}
	var b strings.Builder
	b.WriteString(op)
	b.WriteString(": stage=")
	b.WriteString(e.Stage)
	if e.SQLName != "" {
		b.WriteString(" sql=")
		b.WriteString(e.SQLName)
	}
	if e.SQLState != "" {
		b.WriteString(" sqlstate=")
		b.WriteString(e.SQLState)
	}
	if e.Message != "" {
		b.WriteString(" msg=")
		b.WriteString(truncateLog(e.Message, 300))
	}
	if e.Table != "" {
		b.WriteString(" table=")
		b.WriteString(e.Table)
	}
	if e.Column != "" {
		b.WriteString(" column=")
		b.WriteString(e.Column)
	}
	if e.Constraint != "" {
		b.WriteString(" constraint=")
		b.WriteString(e.Constraint)
	}
	if e.err != nil && e.Message == "" {
		b.WriteString(": ")
		b.WriteString(e.err.Error())
	}
	return b.String()
}

func (e *runtimeSessionStageError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func wrapRuntimeStage(op, stage, sqlName string, err error) error {
	if err == nil {
		return nil
	}
	out := &runtimeSessionStageError{Op: op, Stage: stage, SQLName: sqlName, err: err}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		out.SQLState = pgErr.Code
		out.Message = pgErr.Message
		out.Table = pgErr.TableName
		out.Column = pgErr.ColumnName
		out.Constraint = pgErr.ConstraintName
	}
	return out
}

func jsonText(raw json.RawMessage, emptyDefault string) (string, error) {
	if len(raw) == 0 {
		return emptyDefault, nil
	}
	if !utf8.Valid(raw) || !json.Valid(raw) {
		return "", fmt.Errorf("%w", ErrInvalidRuntimeJSON)
	}
	return string(raw), nil
}

func jsonObjectText(raw json.RawMessage) (string, error) {
	return jsonText(raw, "{}")
}

func jsonArrayText(raw json.RawMessage) (string, error) {
	return jsonText(raw, "[]")
}

func nextActionForSQLState(sqlstate, goType string, jsonValid bool) string {
	switch {
	case sqlstate == "22P02":
		return "bind jsonb as text (::text::jsonb); do not change global query mode first"
	case !jsonValid:
		return "reject at service boundary; do not send to Postgres"
	case sqlstate == "23505":
		return "unique current-session constraint; check close_previous_session"
	case sqlstate == "55P03":
		return "lock_machine contention; retry is App-side only, fail-closed"
	default:
		return "inspect stage, sqlstate, and table/column; keep sell path fail-closed"
	}
}

func truncateLog(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func looksLikeSecretKey(k string) bool {
	nk := strings.ToLower(strings.ReplaceAll(k, "-", "_"))
	return strings.Contains(nk, "token") || strings.Contains(nk, "password") ||
		strings.Contains(nk, "authorization") || strings.Contains(nk, "cookie") ||
		strings.Contains(nk, "secret") || strings.Contains(nk, "jwt") ||
		strings.Contains(nk, "private_key")
}

// RuntimeSessionDiag extracts safe persistence diagnostics from a wrapped stage error.
func RuntimeSessionDiag(err error) (stage, sqlName, sqlstate, nextAction string, ok bool) {
	var staged *runtimeSessionStageError
	if !errors.As(err, &staged) || staged == nil {
		if errors.Is(err, ErrInvalidRuntimeJSON) {
			return "validate_json", "", "", nextActionForSQLState("", "string", false), true
		}
		return "", "", "", "", false
	}
	goType := "string"
	return staged.Stage, staged.SQLName, staged.SQLState, nextActionForSQLState(staged.SQLState, goType, true), true
}

func logJSONBindAudit(op string, err error, fields ...string) {
	sqlstate := ""
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		sqlstate = pgErr.Code
	}
	if sqlstate != "22P02" {
		return
	}
	names := []string{"blockers", "hardware_status", "catalog_status", "outbox_status", "recovery_status", "metadata"}
	attrs := []any{
		"op", op,
		"sqlstate", sqlstate,
		"go_type", "string",
		"next_action", nextActionForSQLState(sqlstate, "string", true),
	}
	for i, f := range fields {
		name := "field"
		if i < len(names) {
			name = names[i]
		}
		attrs = append(attrs, name+"_len", len(f), name+"_json_valid", json.Valid([]byte(f)), name+"_utf8_prefix", truncateLog(f, 32))
	}
	slog.Error("runtime_session.json.bind_audit", attrs...)
}
