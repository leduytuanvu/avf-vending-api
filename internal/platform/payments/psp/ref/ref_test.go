package ref

import (
	"testing"

	"github.com/google/uuid"
)

func TestGenerateFromUUID_LengthAndCharset(t *testing.T) {
	u := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	got := GenerateFromUUID(u)
	if len(got) == 0 || len(got) > MaxLength {
		t.Fatalf("GenerateFromUUID length = %d, want 1..%d", len(got), MaxLength)
	}
	for _, c := range got {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z')) {
			t.Fatalf("non URL-safe / non-crockford char %q in %q", c, got)
		}
		switch c {
		case 'I', 'L', 'O', 'U':
			t.Fatalf("forbidden crockford char %q in %q", c, got)
		}
	}
}

func TestGenerateFromUUID_Deterministic(t *testing.T) {
	u := uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
	a := GenerateFromUUID(u)
	b := GenerateFromUUID(u)
	if a != b {
		t.Fatalf("not deterministic: %q vs %q", a, b)
	}
}

func TestValidate(t *testing.T) {
	if err := Validate(""); err == nil {
		t.Fatal("expected error for empty")
	}
	if err := Validate(string(make([]byte, MaxLength+1))); err == nil {
		t.Fatal("expected error for too long")
	}
	if err := Validate("ORDER_OK_001"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := Validate(string(make([]byte, MaxLength))); err != nil {
		t.Fatalf("exact max length should pass: %v", err)
	}
}
