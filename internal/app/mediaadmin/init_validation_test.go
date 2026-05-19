package mediaadmin

import (
	"testing"

	"github.com/google/uuid"
)

var testFixtureCompanyUUID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

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
		_, _, err := validateImageContentType(testFixtureCompanyUUID, "  ")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("must be image", func(t *testing.T) {
		t.Parallel()
		_, _, err := validateImageContentType(testFixtureCompanyUUID, "application/octet-stream")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rejects unsupported image mime", func(t *testing.T) {
		t.Parallel()
		_, _, err := validateImageContentType(testFixtureCompanyUUID, "image/gif")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("strips MIME parameters", func(t *testing.T) {
		t.Parallel()
		o, ct, err := validateImageContentType(testFixtureCompanyUUID, "image/png; charset=binary")
		if err != nil {
			t.Fatal(err)
		}
		if o != testFixtureCompanyUUID || ct != "image/png" {
			t.Fatalf("got org=%v ct=%q", o, ct)
		}
	})

	t.Run("normalizes case", func(t *testing.T) {
		t.Parallel()
		o, ct, err := validateImageContentType(testFixtureCompanyUUID, "image/PNG")
		if err != nil {
			t.Fatal(err)
		}
		if o != testFixtureCompanyUUID || ct != "image/png" {
			t.Fatalf("got org=%v ct=%q", o, ct)
		}
	})
}

func TestValidateHeadContentTypeForImageUpload(t *testing.T) {
	t.Parallel()
	t.Run("rejects pdf", func(t *testing.T) {
		t.Parallel()
		if err := validateHeadContentTypeForImageUpload("application/pdf"); err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("allows octet-stream", func(t *testing.T) {
		t.Parallel()
		if err := validateHeadContentTypeForImageUpload("application/octet-stream"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("allows empty", func(t *testing.T) {
		t.Parallel()
		if err := validateHeadContentTypeForImageUpload(""); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("allows png", func(t *testing.T) {
		t.Parallel()
		if err := validateHeadContentTypeForImageUpload("image/png"); err != nil {
			t.Fatal(err)
		}
	})
}
