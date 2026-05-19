package testfixtures

import (
	"testing"

	"github.com/google/uuid"
)

// AssertResourceUUIDV7 checks a generated internal resource UUID (v7, non-nil, parseable).
func AssertResourceUUIDV7(t *testing.T, got uuid.UUID) {
	t.Helper()
	if got == uuid.Nil {
		t.Fatal("resource UUID is nil")
	}
	if got.Version() != 7 {
		t.Fatalf("resource UUID version = %d, want 7", got.Version())
	}
	if _, err := uuid.Parse(got.String()); err != nil {
		t.Fatalf("resource UUID not parseable: %v", err)
	}
}
