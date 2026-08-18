package pgxutil

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestRewriteUUIDSliceArgs(t *testing.T) {
	t.Parallel()
	ids := []uuid.UUID{uuid.MustParse("11111111-1111-1111-1111-111111111111")}
	out := RewriteUUIDSliceArgs([]any{ids, "ok", int32(1)})
	got, ok := out[0].(string)
	if !ok {
		t.Fatalf("expected text literal string, got %T", out[0])
	}
	if got != "{11111111-1111-1111-1111-111111111111}" {
		t.Fatalf("unexpected literal %q", got)
	}
	if out[1] != "ok" || out[2] != int32(1) {
		t.Fatalf("non-uuid args mutated: %#v", out)
	}
}

func TestRewriteUUIDSliceArgs_empty(t *testing.T) {
	t.Parallel()
	out := RewriteUUIDSliceArgs([]any{[]uuid.UUID{}})
	got, ok := out[0].(string)
	if !ok {
		t.Fatalf("expected text literal string, got %T", out[0])
	}
	if got != "{}" {
		t.Fatalf("expected empty array literal, got %#v", got)
	}
}

func TestRewriteUUIDSliceArgs_nil(t *testing.T) {
	t.Parallel()
	var ids []uuid.UUID
	out := RewriteUUIDSliceArgs([]any{ids})
	got, ok := out[0].(string)
	if !ok {
		t.Fatalf("expected text literal string, got %T", out[0])
	}
	if got != "{}" {
		t.Fatalf("expected empty array literal for nil slice, got %#v", got)
	}
}

func TestRewriteUUIDSliceArgs_flatArray(t *testing.T) {
	t.Parallel()
	ids := pgtype.FlatArray[uuid.UUID]{
		uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		uuid.MustParse("22222222-2222-2222-2222-222222222222"),
	}
	out := RewriteUUIDSliceArgs([]any{ids})
	got, ok := out[0].(string)
	if !ok {
		t.Fatalf("expected text literal string, got %T", out[0])
	}
	want := "{11111111-1111-1111-1111-111111111111,22222222-2222-2222-2222-222222222222}"
	if got != want {
		t.Fatalf("unexpected literal %q", got)
	}
}

func TestRewriteUUIDSliceArgs_emptyArgs(t *testing.T) {
	t.Parallel()
	if RewriteUUIDSliceArgs(nil) != nil {
		t.Fatal("nil args should stay nil")
	}
	if len(RewriteUUIDSliceArgs([]any{})) != 0 {
		t.Fatal("empty args should stay empty")
	}
}
