package pgjson

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestTextJSON_EmptyIsNull(t *testing.T) {
	got := TextJSON(nil)
	if got.Valid {
		t.Fatalf("empty should be NULL, got %#v", got)
	}
}

func TestTextJSON_ValidObject(t *testing.T) {
	got := TextJSON([]byte(`{"k":1}`))
	want := pgtype.Text{String: `{"k":1}`, Valid: true}
	if got != want {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestRequiredString_InvalidBecomesObject(t *testing.T) {
	if got := RequiredString([]byte("not-json")); got != "{}" {
		t.Fatalf("got %q", got)
	}
	if got := RequiredString([]byte(`{"ok":true}`)); got != `{"ok":true}` {
		t.Fatalf("got %q", got)
	}
}
