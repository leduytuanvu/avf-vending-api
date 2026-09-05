package pgjson_test

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/platform/pgjson"
)

func TestOptionalString_emptyIsNullBind(t *testing.T) {
	t.Parallel()
	if pgjson.OptionalString(nil) != "" {
		t.Fatalf("expected empty string for nil snapshot")
	}
	if pgjson.OptionalString([]byte("  ")) != "" {
		t.Fatalf("expected empty string for blank snapshot")
	}
}

func TestOptionalString_validJSON(t *testing.T) {
	t.Parallel()
	got := pgjson.OptionalString([]byte(`{"total_minor":2000}`))
	if got != `{"total_minor":2000}` {
		t.Fatalf("got %q", got)
	}
}
