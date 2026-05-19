package mediaadmin

import (
	"context"
	"fmt"
	"strings"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

// MediaManifestEntry is the cache-validation surface for machines/offline clients.
// Identity for deduplication is (mediaId, variant, sha256, version) — not downloadUrl.
type MediaManifestEntry struct {
	MediaID     uuid.UUID `json:"mediaId"`
	Variant     string    `json:"variant"`
	SHA256      string    `json:"sha256"`
	Version     int32     `json:"version"`
	SizeBytes   int64     `json:"sizeBytes"`
	MimeType    string    `json:"mimeType"`
	Width       int32     `json:"width,omitempty"`
	Height      int32     `json:"height,omitempty"`
	DownloadURL string    `json:"downloadUrl"`
}

// MediaService is the production-facing abstraction for product-image media:
// presigned upload init, finalize with integrity checks + variant generation,
// and deterministic manifests (metadata in PostgreSQL, blobs in object storage only).
//
// CompleteUpload paths validate object size (configured max), Content-Type (image/* or octet-stream),
// and payload sniffing inside the variant pipeline; signed URLs are never written to audit logs.
type MediaService interface {
	InitUpload(ctx context.Context, companyID uuid.UUID, filename, contentType, purpose string) (*InitUploadResult, error)
	CompleteUpload(ctx context.Context, companyID, mediaID uuid.UUID) (*db.MediaAsset, error)
	CompleteUploadWithOptions(ctx context.Context, companyID, mediaID uuid.UUID, opts CompleteUploadOptions) (*db.MediaAsset, error)
	MediaManifest(ctx context.Context, companyID, mediaID uuid.UUID) ([]MediaManifestEntry, error)
}

// Compile-time check that Service implements MediaService.
var _ MediaService = (*Service)(nil)

// MediaManifest returns presigned manifest rows for all persisted variants (original, thumb, display).
// Requires asset status ready.
func (s *Service) MediaManifest(ctx context.Context, companyID, mediaID uuid.UUID) ([]MediaManifestEntry, error) {
	if s == nil {
		return nil, ErrNotConfigured
	}
	if mediaID == uuid.Nil {
		return nil, ErrInvalidArgument
	}
	a, err := s.GetAsset(ctx, companyID, mediaID)
	if err != nil {
		return nil, err
	}
	if a.Status != "ready" {
		return nil, fmt.Errorf("%w: manifest requires ready media", ErrInvalidArgument)
	}
	variants, err := s.ListVariantsForAssets(ctx, []uuid.UUID{mediaID})
	if err != nil {
		return nil, err
	}
	out := make([]MediaManifestEntry, 0, len(variants))
	for _, v := range variants {
		url, err := s.PresignedDownloadURL(ctx, v.ObjectKey)
		if err != nil {
			return nil, err
		}
		e := MediaManifestEntry{
			MediaID:     mediaID,
			Variant:     v.Variant,
			Version:     v.Version,
			DownloadURL: url,
		}
		if v.Sha256.Valid {
			e.SHA256 = strings.TrimSpace(v.Sha256.String)
		}
		if v.SizeBytes.Valid {
			e.SizeBytes = v.SizeBytes.Int64
		}
		if v.MimeType.Valid {
			e.MimeType = strings.TrimSpace(v.MimeType.String)
		}
		if v.Width.Valid {
			e.Width = v.Width.Int32
		}
		if v.Height.Valid {
			e.Height = v.Height.Int32
		}
		out = append(out, e)
	}
	return out, nil
}
