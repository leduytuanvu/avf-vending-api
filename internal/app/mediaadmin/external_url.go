package mediaadmin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/id"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	externalDownloadStrategy = "download_when_online_use_local_when_offline"
	externalMediaVersion       = 1
)

// RegisterExternalProductImageInput registers a hosted HTTPS image URL as a ready media_assets row.
type RegisterExternalProductImageInput struct {
	CompanyID   uuid.UUID
	URL         string
	Purpose     string
	Filename    string
	ContentType string
}

// ExternalProductImageResult is returned after external URL registration.
type ExternalProductImageResult struct {
	MediaID      uuid.UUID
	SourceType   string
	URL          string
	DisplayURL   string
	ThumbnailURL string
	ContentType  string
	Filename     string
	Status       string
	CacheKey     string
	Version      int32
	CreatedAt    time.Time
	Replay       bool
}

// ExternalConfigured reports whether external product image URL registration is enabled.
func (s *Service) ExternalConfigured() bool {
	return s != nil && s.external.Enabled
}

// UploadConfigured reports whether presigned object-storage upload is available.
func (s *Service) UploadConfigured() bool {
	return s != nil && s.store != nil
}

// ExternalImageCacheKey returns a deterministic offline cache key for an external URL.
func ExternalImageCacheKey(normalizedURL string, version int) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(normalizedURL)))
	return fmt.Sprintf("external-image:%s:v%d", hex.EncodeToString(sum[:]), version)
}

func externalMediaObjectKey(mediaID uuid.UUID, variant string) string {
	return fmt.Sprintf("external/%s/%s", mediaID.String(), variant)
}

// RegisterExternalProductImage validates and stores an external product image URL.
func (s *Service) RegisterExternalProductImage(ctx context.Context, in RegisterExternalProductImageInput) (*ExternalProductImageResult, error) {
	if s == nil || s.pool == nil {
		return nil, ErrNotConfigured
	}
	if !s.external.Enabled {
		return nil, ErrExternalNotConfigured
	}
	if in.CompanyID == uuid.Nil {
		return nil, fmt.Errorf("%w: company_id", ErrInvalidArgument)
	}
	purpose := strings.TrimSpace(strings.ToLower(in.Purpose))
	if purpose == "" {
		purpose = "product_image"
	}
	if purpose != "product_image" {
		return nil, fmt.Errorf("%w: unsupported purpose %q", ErrInvalidArgument, purpose)
	}

	normalized, err := validateExternalProductImageURL(in.URL, s.external)
	if err != nil {
		return nil, err
	}
	filename := strings.TrimSpace(in.Filename)
	if filename == "" {
		filename = path.Base(normalized.Path)
	}
	contentType := normalizeMIMEHeader(in.ContentType)
	if contentType == "" {
		contentType = inferMIMEFromFilename(filename)
	}
	if contentType == "" {
		return nil, fmt.Errorf("%w: content_type required or inferrable from filename", ErrInvalidArgument)
	}
	if err := validateRasterUploadMIME(contentType); err != nil {
		return nil, err
	}
	if err := validateExternalFilenameExtension(filename); err != nil {
		return nil, err
	}

	q := db.New(s.pool)
	if existing, err := q.MediaAdminGetAssetByOriginalURL(ctx, normalized.String()); err == nil {
		return mapExternalAssetResult(existing, normalized.String(), true), nil
	} else if err != pgx.ErrNoRows {
		return nil, err
	}

	if err := probeExternalImageHEAD(ctx, normalized.String(), contentType, s.external); err != nil {
		return nil, err
	}

	mediaID := id.NewUUIDV7()
	ok := externalMediaObjectKey(mediaID, "original")
	tk := externalMediaObjectKey(mediaID, "thumb")
	dk := externalMediaObjectKey(mediaID, "display")
	createdBy := pgtype.UUID{}
	if p, ok := plauth.PrincipalFromContext(ctx); ok {
		if uid, err := uuid.Parse(strings.TrimSpace(p.Subject)); err == nil && uid != uuid.Nil {
			createdBy = pgtype.UUID{Bytes: uid, Valid: true}
		}
	}
	fnPg := pgtype.Text{String: filename, Valid: filename != ""}
	row, err := q.MediaAdminInsertAsset(ctx, db.MediaAdminInsertAssetParams{
		ID:                mediaID,
		Kind:              "product_image",
		OriginalFilename:  fnPg,
		ObjectKey:         pgtype.Text{String: dk, Valid: true},
		OriginalObjectKey: ok,
		ThumbObjectKey:    tk,
		DisplayObjectKey:  dk,
		SourceType:        "external",
		OriginalUrl:       pgtype.Text{String: normalized.String(), Valid: true},
		MimeType:          pgtype.Text{String: contentType, Valid: true},
		CreatedBy:         createdBy,
		Status:            "ready",
	})
	if err != nil {
		return nil, err
	}
	s.bumpCache(ctx, in.CompanyID)
	mid := row.ID.String()
	s.auditRecord(ctx, in.CompanyID, compliance.ActionMediaCreated, "media.asset", &mid, map[string]any{
		"phase":       "external_url",
		"kind":        "product_image",
		"status":      "ready",
		"source_type": "external",
		"url":         normalized.String(),
	})
	return mapExternalAssetResult(row, normalized.String(), false), nil
}

func mapExternalAssetResult(row db.MediaAsset, normalizedURL string, replay bool) *ExternalProductImageResult {
	url := normalizedURL
	if row.OriginalUrl.Valid && strings.TrimSpace(row.OriginalUrl.String) != "" {
		url = strings.TrimSpace(row.OriginalUrl.String)
	}
	ct := ""
	if row.MimeType.Valid {
		ct = strings.TrimSpace(row.MimeType.String)
	}
	fn := ""
	if row.OriginalFilename.Valid {
		fn = strings.TrimSpace(row.OriginalFilename.String)
	}
	ver := externalMediaVersion
	if row.ObjectVersion > 0 {
		ver = int(row.ObjectVersion)
	}
	return &ExternalProductImageResult{
		MediaID:      row.ID,
		SourceType:   "external_url",
		URL:          url,
		DisplayURL:   url,
		ThumbnailURL: url,
		ContentType:  ct,
		Filename:     fn,
		Status:       row.Status,
		CacheKey:     ExternalImageCacheKey(url, ver),
		Version:      int32(ver),
		CreatedAt:    row.CreatedAt.UTC(),
		Replay:       replay,
	}
}

func validateExternalProductImageURL(raw string, cfg config.ExternalProductImageConfig) (*url.URL, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, fmt.Errorf("%w: url is required", ErrInvalidArgument)
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("%w: url must be absolute with a host", ErrInvalidArgument)
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	switch scheme {
	case "https":
	case "http":
		if cfg.RequireHTTPS {
			return nil, fmt.Errorf("%w: url must use https", ErrInvalidArgument)
		}
	default:
		return nil, fmt.Errorf("%w: unsupported url scheme %q", ErrInvalidArgument, scheme)
	}
	if u.User != nil {
		return nil, fmt.Errorf("%w: url must not embed credentials", ErrInvalidArgument)
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host == "" {
		return nil, fmt.Errorf("%w: url host is required", ErrInvalidArgument)
	}
	if err := rejectSSRFHost(host); err != nil {
		return nil, err
	}
	if len(cfg.AllowedHosts) == 0 {
		return nil, fmt.Errorf("%w: external image host allowlist is empty", ErrInvalidArgument)
	}
	allowed := false
	for _, h := range cfg.AllowedHosts {
		h = strings.ToLower(strings.TrimSpace(h))
		if h != "" && host == h {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("%w: host %q is not allowlisted", ErrInvalidArgument, host)
	}
	u.Fragment = ""
	u.RawQuery = strings.TrimSpace(u.RawQuery)
	return u, nil
}

func rejectSSRFHost(host string) error {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return fmt.Errorf("%w: empty host", ErrInvalidArgument)
	}
	switch h {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0", "metadata.google.internal", "169.254.169.254":
		return fmt.Errorf("%w: host %q is not allowed", ErrInvalidArgument, host)
	}
	if strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".internal") {
		return fmt.Errorf("%w: host %q is not allowed", ErrInvalidArgument, host)
	}
	if ip := net.ParseIP(h); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("%w: host %q is not allowed", ErrInvalidArgument, host)
		}
	}
	return nil
}

func inferMIMEFromFilename(name string) string {
	ext := strings.ToLower(strings.TrimSpace(path.Ext(name)))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	default:
		return ""
	}
}

func validateExternalFilenameExtension(name string) error {
	ext := strings.ToLower(strings.TrimSpace(path.Ext(name)))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
		return nil
	case "":
		return fmt.Errorf("%w: filename extension required (.png, .jpg, .jpeg, .webp)", ErrInvalidArgument)
	default:
		return fmt.Errorf("%w: unsupported filename extension %q", ErrInvalidArgument, ext)
	}
}

func probeExternalImageHEAD(ctx context.Context, imageURL, expectedMIME string, cfg config.ExternalProductImageConfig) error {
	timeout := cfg.RemoteTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("%w: too many redirects", ErrInvalidArgument)
			}
			if _, err := validateExternalProductImageURL(req.URL.String(), cfg); err != nil {
				return err
			}
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, imageURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: remote url probe failed: %v", ErrInvalidArgument, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotFound {
		_ = resp.Body.Close()
		reqGet, gerr := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
		if gerr != nil {
			return gerr
		}
		reqGet.Header.Set("Range", "bytes=0-0")
		resp, err = client.Do(reqGet)
		if err != nil {
			return fmt.Errorf("%w: remote url probe failed: %v", ErrInvalidArgument, err)
		}
		defer func() { _ = resp.Body.Close() }()
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("%w: remote url returned HTTP %d", ErrInvalidArgument, resp.StatusCode)
	}
	if cl := resp.ContentLength; cl > 0 && cfg.MaxBytes > 0 && cl > cfg.MaxBytes {
		return fmt.Errorf("%w: remote content length exceeds limit", ErrInvalidArgument)
	}
	if ct := normalizeMIMEHeader(resp.Header.Get("Content-Type")); ct != "" {
		if err := validateRasterUploadMIME(ct); err != nil {
			return err
		}
		if expectedMIME != "" && ct != expectedMIME {
			// Allow image/jpeg vs image/jpg mismatch only when both are jpeg family.
			if !(strings.HasPrefix(expectedMIME, "image/jpeg") && strings.HasPrefix(ct, "image/jpeg")) {
				return fmt.Errorf("%w: remote content-type %q does not match expected %q", ErrInvalidArgument, ct, expectedMIME)
			}
		}
	}
	if resp.Body != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 512))
	}
	return nil
}
