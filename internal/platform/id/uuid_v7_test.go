package id

import (
	"bytes"
	"testing"

	"github.com/google/uuid"
)

func TestNewUUIDV7NotNil(t *testing.T) {
	got := NewUUIDV7()
	if got == uuid.Nil {
		t.Fatal("NewUUIDV7() returned nil UUID")
	}
}

func TestNewUUIDV7Version7(t *testing.T) {
	got := NewUUIDV7()
	if got.Version() != 7 {
		t.Fatalf("Version() = %d, want 7", got.Version())
	}
}

func TestNewUUIDV7StringParses(t *testing.T) {
	raw := NewUUIDV7String()
	parsed, err := uuid.Parse(raw)
	if err != nil {
		t.Fatalf("uuid.Parse(%q): %v", raw, err)
	}
	if parsed.Version() != 7 {
		t.Fatalf("parsed version = %d, want 7", parsed.Version())
	}
}

func TestNewUUIDV7Unique(t *testing.T) {
	const n = 256
	seen := make(map[uuid.UUID]struct{}, n)
	for range n {
		u := NewUUIDV7()
		if _, dup := seen[u]; dup {
			t.Fatalf("duplicate UUID %s", u)
		}
		seen[u] = struct{}{}
	}
}

func TestNewUUIDV7TimeSortable(t *testing.T) {
	prev := NewUUIDV7()
	for range 32 {
		next := NewUUIDV7()
		if bytes.Compare(prev[:], next[:]) >= 0 {
			t.Fatalf("expected monotonic v7 ordering: prev=%s next=%s", prev, next)
		}
		prev = next
	}
}
