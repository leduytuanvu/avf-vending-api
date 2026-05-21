package mediaadmin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	platformcloudinary "github.com/avf/avf-vending-api/internal/platform/cloudinary"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ProductImageFileUploader uploads validated image bytes to a remote provider (Cloudinary).
type ProductImageFileUploader interface {
	UploadProductImage(ctx context.Context, in platformcloudinary.UploadInput) (platformcloudinary.UploadResult, error)
}

// UploadProductImageFileInput is the application-layer multipart upload request.
type UploadProductImageFileInput struct {
	CompanyID   uuid.UUID
	Filename    string
	ContentType string
	SizeBytes   int64
	Reader      io.Reader
	Purpose     string
	AltText     string
	ProductID   uuid.UUID
	IsPrimary   bool
}

// UploadProductImageFileResult is returned after a successful server-side upload.
type UploadProductImageFileResult struct {
	MediaID      uuid.UUID
	Provider     string
	SourceType   string
	Status       string
	Filename     string
	ContentType  string
	SizeBytes    int64
	Width        int32
	Height       int32
	Checksum     string
	DisplayURL   string
	ThumbnailURL string
	Version      int32
	CreatedAt    time.Time
	ProductID    uuid.UUID
	Attached     bool
	IsPrimary    bool
}

// CloudinaryConfigured reports whether server-side Cloudinary upload is available.
func (s *Service) CloudinaryConfigured() bool {
	return s != nil && s.cloudinary != nil && s.uploadCfg.CloudinaryConfigured()
}

// UploadProductImageFile validates, uploads to Cloudinary, persists media_assets, optionally binds product.
func (s *Service) UploadProductImageFile(ctx context.Context, in UploadProductImageFileInput) (*UploadProductImageFileResult, error) {
	if s == nil || s.pool == nil {
		return nil, ErrNotConfigured
	}
	if !s.CloudinaryConfigured() {
		return nil, ErrCloudinaryNotConfigured
	}
	if in.CompanyID == uuid.Nil {
		return nil, fmt.Errorf("%w: company_id", ErrInvalidArgument)
	}
	if in.Reader == nil {
		return nil, invalidImageFile("file is required", s.uploadCfg.AllowedTypes, int(s.uploadCfg.MaxBytes>>20))
	}
	purpose := strings.TrimSpace(strings.ToLower(in.Purpose))
	if purpose == "" {
		purpose = "product_image"
	}
	if purpose != "product_image" {
		return nil, fmt.Errorf("%w: unsupported purpose %q", ErrInvalidArgument, purpose)
	}

	limited := io.LimitReader(in.Reader, s.uploadCfg.MaxBytes+1)
	peek, err := readImageHeader(limited, 512)
	if err != nil {
		return nil, invalidImageFile("unable to read uploaded file", s.uploadCfg.AllowedTypes, int(s.uploadCfg.MaxBytes>>20))
	}
	size := in.SizeBytes
	if size <= 0 {
		size = int64(len(peek))
	}
	if err := validateProductImageFile(in.Filename, in.ContentType, size, peek, s.uploadCfg); err != nil {
		return nil, err
	}
	contentType := normalizeMIMEHeader(in.ContentType)
	if contentType == "" {
		contentType = inferMIMEFromFilename(in.Filename)
	}

	hasher := sha256.New()
	body := io.TeeReader(io.MultiReader(bytes.NewReader(peek), limited), hasher)

	mediaID := id.NewUUIDV7()
	actorID := uuid.Nil
	if p, ok := plauth.PrincipalFromContext(ctx); ok {
		if uid, err := uuid.Parse(strings.TrimSpace(p.Subject)); err == nil {
			actorID = uid
		}
	}
	upResult, err := s.cloudinary.UploadProductImage(ctx, platformcloudinary.UploadInput{
		Reader:      body,
		Filename:    in.Filename,
		ContentType: contentType,
		SizeBytes:   size,
		PublicID:    mediaID.String(),
		Purpose:     purpose,
		ActorID:     actorID,
		AppEnv:      s.appEnv,
	})
	if err != nil {
		return nil, fmt.Errorf("cloudinary upload: %w", err)
	}

	checksumHex := hex.EncodeToString(hasher.Sum(nil))
	checksum := "sha256:" + checksumHex

	displayURL := strings.TrimSpace(upResult.DisplayURL)
	thumbURL := strings.TrimSpace(upResult.ThumbnailURL)
	if thumbURL == "" {
		thumbURL = displayURL
	}
	fnPg := pgtype.Text{String: strings.TrimSpace(in.Filename), Valid: strings.TrimSpace(in.Filename) != ""}
	createdBy := pgtype.UUID{}
	if actorID != uuid.Nil {
		createdBy = pgtype.UUID{Bytes: actorID, Valid: true}
	}
	q := db.New(s.pool)
	row, err := q.MediaAdminInsertCloudinaryAsset(ctx, db.MediaAdminInsertCloudinaryAssetParams{
		ID:                mediaID,
		Kind:              "product_image",
		OriginalFilename:  fnPg,
		ObjectKey:         pgtype.Text{String: displayURL, Valid: displayURL != ""},
		OriginalObjectKey: displayURL,
		ThumbObjectKey:    thumbURL,
		DisplayObjectKey:  displayURL,
		SourceType:        "cloudinary",
		StorageProvider:   "cloudinary",
		ProviderPublicID:  pgtype.Text{String: strings.TrimSpace(upResult.PublicID), Valid: strings.TrimSpace(upResult.PublicID) != ""},
		ProviderAssetID:   pgtype.Text{String: strings.TrimSpace(upResult.AssetID), Valid: strings.TrimSpace(upResult.AssetID) != ""},
		OriginalUrl:       pgtype.Text{String: displayURL, Valid: displayURL != ""},
		MimeType:          pgtype.Text{String: contentType, Valid: contentType != ""},
		SizeBytes:         pgtype.Int8{Int64: upResult.Bytes, Valid: upResult.Bytes > 0},
		Sha256:            pgtype.Text{String: checksumHex, Valid: checksumHex != ""},
		Width:             pgtype.Int4{Int32: int32(upResult.Width), Valid: upResult.Width > 0},
		Height:            pgtype.Int4{Int32: int32(upResult.Height), Valid: upResult.Height > 0},
		CreatedBy:         createdBy,
		Status:            "ready",
	})
	if err != nil {
		return nil, err
	}
	s.bumpCache(ctx, in.CompanyID)
	mid := row.ID.String()
	s.auditRecord(ctx, in.CompanyID, compliance.ActionMediaCreated, "media.asset", &mid, map[string]any{
		"phase":              "cloudinary_upload",
		"kind":               "product_image",
		"status":             "ready",
		"source_type":        "cloudinary",
		"storage_provider":   "cloudinary",
		"provider_public_id": upResult.PublicID,
	})

	out := &UploadProductImageFileResult{
		MediaID:      row.ID,
		Provider:     "cloudinary",
		SourceType:   "uploaded_file",
		Status:       row.Status,
		Filename:     strings.TrimSpace(in.Filename),
		ContentType:  contentType,
		SizeBytes:    upResult.Bytes,
		Width:        row.Width.Int32,
		Height:       row.Height.Int32,
		Checksum:     checksum,
		DisplayURL:   displayURL,
		ThumbnailURL: thumbURL,
		Version:      int32(upResult.Version),
		CreatedAt:    row.CreatedAt.UTC(),
	}
	if in.ProductID != uuid.Nil && in.IsPrimary {
		out.ProductID = in.ProductID
		if _, err := s.BindProductPrimaryMedia(ctx, in.CompanyID, in.ProductID, row.ID); err != nil {
			return nil, err
		}
		out.Attached = true
		out.IsPrimary = true
	}
	return out, nil
}
