package catalogbootstrap

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	platformcloudinary "github.com/avf/avf-vending-api/internal/platform/cloudinary"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ImageDownloader fetches source image bytes.
type ImageDownloader struct {
	Client *http.Client
}

func NewImageDownloader() *ImageDownloader {
	return &ImageDownloader{Client: &http.Client{Timeout: 60 * time.Second}}
}

func (d *ImageDownloader) Download(ctx context.Context, url string) ([]byte, string, error) {
	if d == nil || d.Client == nil {
		return nil, "", fmt.Errorf("downloader not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "avf-catalog-bootstrap/1.0")
	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 6<<20))
	if err != nil {
		return nil, "", err
	}
	ct := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	return body, ct, nil
}

// CloudinaryUploadResult is persisted upload metadata.
type CloudinaryUploadResult struct {
	MediaID      uuid.UUID
	PublicID     string
	AssetID      string
	SecureURL    string
	ThumbnailURL string
	Format       string
	Width        int
	Height       int
	Bytes        int64
	SHA256       string
	MIME         string
}

// UploadProductImage downloads source URL and uploads to Cloudinary, inserting media_assets.
func UploadProductImage(
	ctx context.Context,
	q *db.Queries,
	uploader *platformcloudinary.Uploader,
	downloader *ImageDownloader,
	companyID uuid.UUID,
	p ProductRecord,
	appEnv string,
) (*CloudinaryUploadResult, error) {
	if uploader == nil {
		return nil, fmt.Errorf("cloudinary uploader not configured")
	}
	body, mime, err := downloader.Download(ctx, p.SourceImageURL)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	if mime == "" {
		mime = "image/png"
	}
	sum := sha256.Sum256(body)
	checksumHex := hex.EncodeToString(sum[:])

	publicID := strings.TrimSpace(p.CloudinaryPubID)
	if publicID == "" {
		publicID = "import-" + p.SKU
	}

	up, err := uploader.UploadProductImage(ctx, platformcloudinary.UploadInput{
		Reader:      bytes.NewReader(body),
		Filename:    publicID + ".img",
		ContentType: mime,
		SizeBytes:   int64(len(body)),
		PublicID:    publicID,
		Purpose:     "product_image",
		ActorID:     uuid.Nil,
		AppEnv:      appEnv,
	})
	if err != nil {
		return nil, fmt.Errorf("cloudinary upload: %w", err)
	}

	mediaID := id.NewUUIDV7()
	displayURL := strings.TrimSpace(up.DisplayURL)
	thumbURL := strings.TrimSpace(up.ThumbnailURL)
	if thumbURL == "" {
		thumbURL = displayURL
	}
	_, err = q.MediaAdminInsertCloudinaryAsset(ctx, db.MediaAdminInsertCloudinaryAssetParams{
		ID:                mediaID,
		Kind:              "product_image",
		OriginalFilename:  pgtype.Text{String: p.SKU + ".img", Valid: true},
		ObjectKey:         pgtype.Text{String: displayURL, Valid: displayURL != ""},
		OriginalObjectKey: displayURL,
		ThumbObjectKey:    thumbURL,
		DisplayObjectKey:  displayURL,
		SourceType:        "cloudinary",
		StorageProvider:   "cloudinary",
		ProviderPublicID:  pgtype.Text{String: up.PublicID, Valid: up.PublicID != ""},
		ProviderAssetID:   pgtype.Text{String: up.AssetID, Valid: up.AssetID != ""},
		OriginalUrl:       pgtype.Text{String: displayURL, Valid: displayURL != ""},
		MimeType:          pgtype.Text{String: mime, Valid: mime != ""},
		SizeBytes:         pgtype.Int8{Int64: up.Bytes, Valid: up.Bytes > 0},
		Sha256:            pgtype.Text{String: checksumHex, Valid: checksumHex != ""},
		Width:             pgtype.Int4{Int32: int32(up.Width), Valid: up.Width > 0},
		Height:            pgtype.Int4{Int32: int32(up.Height), Valid: up.Height > 0},
		CreatedBy:         pgtype.UUID{},
		Status:            "ready",
	})
	if err != nil {
		return nil, fmt.Errorf("insert media_assets: %w", err)
	}
	_ = companyID // reserved for future org-scoped cache bump
	return &CloudinaryUploadResult{
		MediaID:      mediaID,
		PublicID:     up.PublicID,
		AssetID:      up.AssetID,
		SecureURL:    displayURL,
		ThumbnailURL: thumbURL,
		Format:       up.Format,
		Width:        up.Width,
		Height:       up.Height,
		Bytes:        up.Bytes,
		SHA256:       checksumHex,
		MIME:         mime,
	}, nil
}
