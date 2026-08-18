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
	got, ok := out[0].(pgtype.FlatArray[uuid.UUID])
	if !ok {
		t.Fatalf("expected FlatArray, got %T", out[0])
	}
	if len(got) != 1 || got[0] != ids[0] {
		t.Fatalf("unexpected array %#v", got)
	}
	if out[1] != "ok" || out[2] != int32(1) {
		t.Fatalf("non-uuid args mutated: %#v", out)
	}
}

func TestRewriteUUIDSliceArgs_empty(t *testing.T) {
	t.Parallel()
	out := RewriteUUIDSliceArgs([]any{[]uuid.UUID{}})
	got, ok := out[0].(pgtype.FlatArray[uuid.UUID])
	if !ok {
		t.Fatalf("expected FlatArray, got %T", out[0])
	}
	if len(got) != 0 {
		t.Fatalf("expected empty FlatArray, got %#v", got)
	}
}
