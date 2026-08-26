package grpcserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/activation"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestMapActivationErrorMQTTProvisioningUnavailable(t *testing.T) {
	t.Parallel()
	wrapped := fmt.Errorf("%w: emqxadmin: HTTP 401", activation.ErrMQTTProvisioning)
	got := mapActivationError(wrapped)
	st, ok := status.FromError(got)
	if !ok {
		t.Fatal("expected grpc status")
	}
	if st.Code() != codes.Unavailable {
		t.Fatalf("code=%s", st.Code())
	}
	if st.Message() != "mqtt_provisioning_failed" {
		t.Fatalf("message=%q", st.Message())
	}
	if !errors.Is(wrapped, activation.ErrMQTTProvisioning) {
		t.Fatal("wrapped error must remain errors.Is MQTT provisioning")
	}
}

func TestMapActivationErrorInvalidArgument(t *testing.T) {
	t.Parallel()
	got := mapActivationError(activation.ErrInvalid)
	st, ok := status.FromError(got)
	if !ok {
		t.Fatal("expected grpc status")
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("code=%s", st.Code())
	}
	if st.Message() != "activation_invalid" {
		t.Fatalf("message=%q", st.Message())
	}
}

func TestMapActivationErrorMachineNotEligible(t *testing.T) {
	t.Parallel()
	got := mapActivationError(activation.ErrMachineNotEligible)
	st, ok := status.FromError(got)
	if !ok {
		t.Fatal("expected grpc status")
	}
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("code=%s", st.Code())
	}
	if st.Message() != "machine_not_eligible" {
		t.Fatalf("message=%q", st.Message())
	}
}

func TestMapActivationErrorJSONBindFailedPrecondition(t *testing.T) {
	t.Parallel()
	pgErr := &pgconn.PgError{
		Code:    "22P02",
		Message: "invalid input syntax for type json",
	}
	got := mapActivationError(pgErr)
	st, ok := status.FromError(got)
	if !ok {
		t.Fatal("expected grpc status")
	}
	if st.Code() != codes.FailedPrecondition {
		t.Fatalf("code=%s", st.Code())
	}
	if st.Message() != "activation_storage_json_invalid" {
		t.Fatalf("message=%q", st.Message())
	}
	if strings.Contains(st.Message(), "invalid input") {
		t.Fatal("public status must not include the original postgres message")
	}
}

func TestMapActivationErrorUnknownRemainsInternal(t *testing.T) {
	t.Parallel()
	original := errors.New("db exploded")
	got := mapActivationError(original)
	st, ok := status.FromError(got)
	if !ok {
		t.Fatal("expected grpc status")
	}
	if st.Code() != codes.Internal {
		t.Fatalf("code=%s", st.Code())
	}
	if st.Message() != "internal" {
		t.Fatalf("message=%q", st.Message())
	}
	if strings.Contains(st.Message(), "db exploded") {
		t.Fatal("public status must not include the original error")
	}
}

func captureClaimActivationLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := claimActivationTestLogger
	claimActivationTestLogger = slog.New(slog.NewJSONHandler(&buf, nil))
	t.Cleanup(func() { claimActivationTestLogger = prev })
	return &buf
}

func TestLogUnmappedClaimActivationError_IncludesOriginalOmitsSecret(t *testing.T) {
	buf := captureClaimActivationLogs(t)

	ctx := withGRPCRequestMeta(context.Background(), GRPCRequestMeta{
		RequestID:     "req-diag-1",
		CorrelationID: "corr-diag-1",
	})
	ctx = metadata.NewIncomingContext(ctx, metadata.Pairs("x-request-id", "req-diag-1"))
	original := errors.New("db exploded: unique constraint ux_machine_sessions_one_active")
	logUnmappedClaimActivationError(ctx, original, time.Now().Add(-1500*time.Millisecond))

	out := buf.String()
	if !strings.Contains(out, "machine activation claim failed") {
		t.Fatalf("missing event name in log: %s", out)
	}
	if !strings.Contains(out, "db exploded") {
		t.Fatalf("original error missing from log: %s", out)
	}
	if !strings.Contains(out, "req-diag-1") {
		t.Fatalf("request_id missing from log: %s", out)
	}
	if !strings.Contains(out, "corr-diag-1") {
		t.Fatalf("correlation_id missing from log: %s", out)
	}
	if !strings.Contains(out, "MachineActivationService/ClaimActivation") {
		t.Fatalf("grpc_method missing from log: %s", out)
	}
}

func TestLogUnmappedClaimActivationError_SkipsMappedSentinels(t *testing.T) {
	buf := captureClaimActivationLogs(t)
	ctx := context.Background()
	logUnmappedClaimActivationError(ctx, activation.ErrInvalid, time.Now())
	logUnmappedClaimActivationError(ctx, activation.ErrMachineNotEligible, time.Now())
	logUnmappedClaimActivationError(ctx, fmt.Errorf("%w: emqxadmin: HTTP 401", activation.ErrMQTTProvisioning), time.Now())
	if buf.Len() != 0 {
		t.Fatalf("mapped errors must not emit the internal diagnostic log: %s", buf.String())
	}
}

func TestLogUnmappedClaimActivationError_IncludesPgConstraintNotDetail(t *testing.T) {
	buf := captureClaimActivationLogs(t)
	pgErr := &pgconn.PgError{
		Code:           "23505",
		Message:        "duplicate key value violates unique constraint",
		Detail:         "Key (refresh_token_hash)=(\\xdeadbeef) already exists.",
		Hint:           "hash-hint-should-not-log",
		Where:          "SQL statement containing secret-where",
		InternalQuery:  "SELECT secret_hash FROM machine_credentials",
		TableName:      "machine_sessions",
		ConstraintName: "ux_machine_sessions_one_active",
		ColumnName:     "refresh_token_hash",
		SchemaName:     "public",
		DataTypeName:   "bytea",
	}
	logUnmappedClaimActivationError(context.Background(), pgErr, time.Now())
	out := buf.String()
	if !strings.Contains(out, `"sqlstate":"23505"`) && !strings.Contains(out, "23505") {
		t.Fatalf("sqlstate missing: %s", out)
	}
	if !strings.Contains(out, "ux_machine_sessions_one_active") {
		t.Fatalf("constraint missing: %s", out)
	}
	if !strings.Contains(out, "machine_sessions") {
		t.Fatalf("table missing: %s", out)
	}
	leaks := []string{"deadbeef", "hash-hint-should-not-log", "secret-where", "secret_hash", "SELECT"}
	for _, leak := range leaks {
		if strings.Contains(out, leak) {
			t.Fatalf("unsafe pg field leaked %q: %s", leak, out)
		}
	}
}

func TestClaimActivationLogErrorText_RedactsSixDigitActivationCode(t *testing.T) {
	t.Parallel()
	// Matches activation.randomActivationCode / validActivationCode: exactly six digits.
	const plaintext = "482917"
	got := claimActivationLogErrorText(fmt.Errorf("claim failed around token %s in wrapper", plaintext))
	if strings.Contains(got, plaintext) {
		t.Fatal("six-digit activation plaintext must be redacted from diagnostic error text")
	}
	if !strings.Contains(got, "[REDACTED_ACTIVATION_CODE]") {
		t.Fatalf("expected redaction marker, got %q", got)
	}
	// SQLSTATE (5 digits) and machine codes (AVF + digits) must remain.
	kept := claimActivationLogErrorText(errors.New("sqlstate 23505 machine AVF000001"))
	if !strings.Contains(kept, "23505") {
		t.Fatalf("SQLSTATE must not be redacted: %q", kept)
	}
	if !strings.Contains(kept, "AVF000001") {
		t.Fatalf("machine code must not be redacted: %q", kept)
	}
}
