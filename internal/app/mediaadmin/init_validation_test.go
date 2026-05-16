package mediaadmin

import (
	"testing"

	"github.com/google/uuid"
)

var testScopeID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

func TestValidateImageContentType(t *testing.T) {
	t.Parallel()

	t.Run("org required", func(t *testing.T) {
		t.Parallel()
		_, _, err := validateImageContentType(uuid.Nil, "image/png")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("content type required", func(t *testing.T) {
		t.Parallel()
		_, _, err := validateImageContentType(testScopeID, "  ")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("must be image", func(t *testing.T) {
		t.Parallel()
		_, _, err := validateImageContentType(testScopeID, "application/octet-stream")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rejects unsupported image mime", func(t *testing.T) {
		t.Parallel()
		_, _, err := validateImageContentType(testScopeID, "image/gif")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("strips MIME parameters", func(t *testing.T) {
		t.Parallel()
		o, ct, err := validateImageContentType(testScopeID, "image/png; charset=binary")
		if err != nil {
			t.Fatal(err)
		}
		if o != testScopeID || ct != "image/png" {
			t.Fatalf("got org=%v ct=%q", o, ct)
		}
	})

	t.Run("normalizes case", func(t *testing.T) {
		t.Parallel()
		o, ct, err := validateImageContentType(testScopeID, "image/PNG")
		if err != nil {
			t.Fatal(err)
		}
		if o != testScopeID || ct != "image/png" {
			t.Fatalf("got org=%v ct=%q", o, ct)
		}
	})
}
