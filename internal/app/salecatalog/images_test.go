package salecatalog

import (
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestProductImageContentHashUsesMediaAssetSHA(t *testing.T) {
	t.Parallel()
	first := productImageContentHash(db.RuntimeListProductImagesForProductsRow{
		StorageKey:  "legacy-key",
		AssetSha256: pgtype.Text{String: "aaaaaaaa", Valid: true},
	})
	second := productImageContentHash(db.RuntimeListProductImagesForProductsRow{
		StorageKey:  "legacy-key",
		AssetSha256: pgtype.Text{String: "bbbbbbbb", Valid: true},
	})
	if first != "sha256:aaaaaaaa" {
		t.Fatalf("unexpected first hash: %s", first)
	}
	if second != "sha256:bbbbbbbb" {
		t.Fatalf("unexpected second hash: %s", second)
	}
	if first == second {
		t.Fatal("content hash should change when media asset sha changes")
	}
}

func TestProductImageContentHash_usesProjectionHashWithoutAssetSHA(t *testing.T) {
	t.Parallel()
	row := db.RuntimeListProductImagesForProductsRow{
		StorageKey:  "k",
		ContentHash: pgtype.Text{String: "projsha256", Valid: true},
		CreatedAt:   time.Now().UTC(),
	}
	if got := productImageContentHash(row); got != "projsha256" {
		t.Fatalf("got %q want projsha256", got)
	}
}

func TestProductImageContentHash_prefersAssetSHA256(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	row := db.RuntimeListProductImagesForProductsRow{
		ID:           uuid.New(),
		ProductID:    pid,
		StorageKey:   "k",
		ContentHash:  pgtype.Text{String: "sha256:legacy", Valid: true},
		CreatedAt:    time.Now().UTC(),
		MediaAssetID: pgtype.UUID{Bytes: uuid.New(), Valid: true},
		AssetSha256:  pgtype.Text{String: "beefcafe", Valid: true},
	}
	if got := productImageContentHash(row); got != "sha256:beefcafe" {
		t.Fatalf("got %q", got)
	}
}

func TestProductImageEtag_prefersAssetEtag(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	row := db.RuntimeListProductImagesForProductsRow{
		CreatedAt:   at,
		AssetEtag:   pgtype.Text{String: `W/"from-db"`, Valid: true},
		ContentHash: pgtype.Text{String: "sha256:x", Valid: true},
	}
	if got := productImageEtag(row, "sha256:x"); got != `W/"from-db"` {
		t.Fatalf("got %q", got)
	}
}
