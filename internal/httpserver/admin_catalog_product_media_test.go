package httpserver

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestHostedMediaURL_cloudinaryDisplayObjectKey(t *testing.T) {
	t.Parallel()
	asset := db.MediaAsset{
		SourceType:       "cloudinary",
		DisplayObjectKey: "https://res.cloudinary.com/demo/image/upload/v1/sample.jpg",
	}
	got := hostedMediaURL(asset)
	if got != "https://res.cloudinary.com/demo/image/upload/v1/sample.jpg" {
		t.Fatalf("hostedMediaURL() = %q", got)
	}
}

func TestHostedMediaURL_prefersOriginalURL(t *testing.T) {
	t.Parallel()
	asset := db.MediaAsset{
		SourceType:       "external",
		OriginalUrl:      pgtype.Text{String: "https://cdn.example.com/a.jpg", Valid: true},
		DisplayObjectKey: "https://cdn.example.com/b.jpg",
	}
	got := hostedMediaURL(asset)
	if got != "https://cdn.example.com/a.jpg" {
		t.Fatalf("hostedMediaURL() = %q", got)
	}
}

func TestMapMediaVariantsForDoc_cloudinaryWithoutVariants(t *testing.T) {
	t.Parallel()
	asset := db.MediaAsset{
		ID:               uuid.New(),
		SourceType:       "cloudinary",
		Status:           "ready",
		ObjectVersion:    3,
		DisplayObjectKey: "https://res.cloudinary.com/demo/image/upload/v1/sample.jpg",
		ThumbObjectKey:   "https://res.cloudinary.com/demo/image/upload/c_thumb/sample.jpg",
		MimeType:         pgtype.Text{String: "image/jpeg", Valid: true},
		SizeBytes:        pgtype.Int8{Int64: 12345, Valid: true},
	}
	variants := mapMediaVariantsForDoc(t.Context(), nil, asset, nil)
	if len(variants) != 2 {
		t.Fatalf("len(variants) = %d, want 2", len(variants))
	}
	if variants[0].Variant != "display" || variants[0].DownloadURL == "" {
		t.Fatalf("display variant missing: %+v", variants[0])
	}
	if variants[1].Variant != "thumb" {
		t.Fatalf("thumb variant = %+v", variants[1])
	}
}

func TestMapMediaVariantDownloadURL_usesHTTPKeyDirectly(t *testing.T) {
	t.Parallel()
	got := mapMediaVariantDownloadURL(t.Context(), nil, "https://cdn.example.com/x.png")
	if got != "https://cdn.example.com/x.png" {
		t.Fatalf("mapMediaVariantDownloadURL() = %q", got)
	}
}
