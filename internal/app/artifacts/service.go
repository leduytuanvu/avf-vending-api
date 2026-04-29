package artifacts

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/platform/objectstore"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const metaSHA256Hex = "sha256hex"
const metaOriginalFilename = "originalfilename"

var objectStorageFailures = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "avf",
	Subsystem: "object_storage",
	Name:      "failures_total",
	Help:      "Object storage operation failures for backend artifacts and media processing.",
}, []string{"operation", "reason"})

func recordObjectStorageFailure(operation, reason string) {
	operation = strings.TrimSpace(operation)
	reason = strings.TrimSpace(reason)
	if operation == "" {
		operation = "unknown"
	}
	if reason == "" {
		reason = "unknown"
	}
	objectStorageFailures.WithLabelValues(operation, reason).Inc()
}

// Deps wires object storage for backend-managed artifacts.
type Deps struct {
	Store              objectstore.Store
	MaxUploadBytes     int64
	DownloadPresignTTL time.Duration
	ListMaxKeys        int32
}

// Service manages canonical artifact objects under objectstore.BackendArtifactObjectKey.
type Service struct {
	store              objectstore.Store
	maxUploadBytes     int64
	downloadPresignTTL time.Duration
	listMaxKeys        int32
}

// NewService constructs an artifact service. Store must be non-nil.
func NewService(d Deps) *Service {
	if d.Store == nil {
		panic("artifacts.NewService: nil Store")
	}
	max := d.MaxUploadBytes
	if max <= 0 {
		max = 100 << 20
	}
	ttl := d.DownloadPresignTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	lk := d.ListMaxKeys
	if lk <= 0 {
		lk = 500
	}
	if lk > 1000 {
		lk = 1000
	}
	return &Service{
		store:              d.Store,
		maxUploadBytes:     max,
		downloadPresignTTL: ttl,
		listMaxKeys:        lk,
	}
}

// Store returns the backing object store (catalog media reuse).
func (s *Service) Store() objectstore.Store {
	if s == nil {
		return nil
	}
	return s.store
}

// MaxUploadBytes returns the configured artifact payload ceiling (reuse as catalog image cap).
func (s *Service) MaxUploadBytes() int64 {
	if s == nil {
		return 0
	}
	return s.maxUploadBytes
}

// DownloadPresignTTL returns presigned GET TTL used for catalog URLs when wired.
func (s *Service) DownloadPresignTTL() time.Duration {
	if s == nil {
		return 0
	}
	return s.downloadPresignTTL
}

// ReserveArtifact allocates a new artifact id (no object written yet).
func (s *Service) ReserveArtifact(_ context.Context) (uuid.UUID, error) {
	return uuid.New(), nil
}

// PutContent streams exactly size bytes into the canonical artifact key, validating SHA-256 and content type.
func (s *Service) PutContent(ctx context.Context, organizationID, artifactID uuid.UUID, body io.Reader, size int64, contentType, sha256Hex, originalFilename string) error {
	if organizationID == uuid.Nil || artifactID == uuid.Nil {
		return fmt.Errorf("%w: organization_id and artifact_id are required", ErrInvalidArgument)
	}
	if size <= 0 {
		return fmt.Errorf("%w: content length must be > 0", ErrInvalidArgument)
	}
	if size > s.maxUploadBytes {
		return fmt.Errorf("%w: payload exceeds max_upload_bytes", ErrInvalidArgument)
	}
	ct, err := validateContentType(contentType)
	if err != nil {
		return err
	}
	want, err := parseSHA256Hex(sha256Hex)
	if err != nil {
		return err
	}
	key := objectstore.BackendArtifactObjectKey(organizationID, artifactID)

	h := sha256.New()
	limited := io.LimitReader(body, size+1)
	tee := io.TeeReader(limited, h)

	meta := map[string]string{metaSHA256Hex: strings.ToLower(strings.TrimSpace(sha256Hex))}
	if fn := sanitizeOriginalFilename(originalFilename); fn != "" {
		meta[metaOriginalFilename] = fn
	}

	if err := s.store.PutWithUserMetadata(ctx, key, tee, size, ct, meta); err != nil {
		recordObjectStorageFailure("put_artifact", "store_put")
		return err
	}
	sum := h.Sum(nil)
	if subtle.ConstantTimeCompare(sum, want) != 1 {
		_ = s.store.Delete(ctx, key)
		recordObjectStorageFailure("put_artifact", "checksum_mismatch")
		return ErrChecksumMismatch
	}
	extra, err := io.Copy(io.Discard, limited)
	if extra > 0 {
		_ = s.store.Delete(ctx, key)
		recordObjectStorageFailure("put_artifact", "trailing_bytes")
		return ErrTrailingBytes
	}
	if err != nil && !errors.Is(err, io.EOF) {
		recordObjectStorageFailure("put_artifact", "read_error")
		return err
	}
	return nil
}

func parseSHA256Hex(s string) ([]byte, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if len(s) != 64 {
		return nil, fmt.Errorf("%w: X-Artifact-SHA256 must be 64 hex characters", ErrInvalidArgument)
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid sha256 hex", ErrInvalidArgument)
	}
	return b, nil
}

func validateContentType(ct string) (string, error) {
	ct = strings.TrimSpace(strings.ToLower(ct))
	if ct == "" {
		return "", fmt.Errorf("%w: content-type is required", ErrInvalidArgument)
	}
	if len(ct) > 128 {
		return "", fmt.Errorf("%w: content-type too long", ErrInvalidArgument)
	}
	allowed := map[string]struct{}{
		"application/octet-stream": {},
		"application/gzip":         {},
		"application/x-gzip":       {},
		"application/zip":          {},
		"application/x-tar":        {},
		"application/x-compress":   {},
		"text/plain":               {},
		"application/json":         {},
	}
	if _, ok := allowed[ct]; !ok {
		return "", fmt.Errorf("%w: unsupported content-type for artifacts", ErrInvalidArgument)
	}
	return ct, nil
}

func sanitizeOriginalFilename(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	base := s
	if i := strings.LastIndexAny(s, `/\`); i >= 0 {
		base = s[i+1:]
	}
	base = strings.TrimSpace(base)
	if len(base) > 255 {
		base = base[:255]
	}
	if base == "" || base == "." {
		return ""
	}
	return base
}

// ArtifactInfo is returned for describe and list enrichment.
type ArtifactInfo struct {
	OrganizationID   uuid.UUID
	ArtifactID       uuid.UUID
	Size             int64
	ContentType      string
	LastModifiedUTC  time.Time
	ETag             string
	SHA256Hex        string
	OriginalFilename string
	ObjectKey        string
}

// GetInfo returns metadata for an artifact if the object exists.
func (s *Service) GetInfo(ctx context.Context, organizationID, artifactID uuid.UUID) (ArtifactInfo, error) {
	key := objectstore.BackendArtifactObjectKey(organizationID, artifactID)
	meta, err := s.store.Head(ctx, key)
	if err != nil {
		return ArtifactInfo{}, mapStoreError(err)
	}
	org, art, ok := objectstore.ParseBackendArtifactKey(meta.Key)
	if !ok || org != organizationID || art != artifactID {
		return ArtifactInfo{}, ErrNotFound
	}
	info := ArtifactInfo{
		OrganizationID:  organizationID,
		ArtifactID:      artifactID,
		Size:            meta.Size,
		ContentType:     meta.ContentType,
		LastModifiedUTC: meta.LastModified.UTC(),
		ETag:            meta.ETag,
		ObjectKey:       meta.Key,
	}
	if meta.UserMetadata != nil {
		info.SHA256Hex = meta.UserMetadata[metaSHA256Hex]
		info.OriginalFilename = meta.UserMetadata[metaOriginalFilename]
	}
	return info, nil
}

// ListArtifacts lists artifact payload objects for an organization (best-effort Head for metadata).
func (s *Service) ListArtifacts(ctx context.Context, organizationID uuid.UUID) ([]ArtifactInfo, error) {
	if organizationID == uuid.Nil {
		return nil, fmt.Errorf("%w: organization_id is required", ErrInvalidArgument)
	}
	prefix := objectstore.BackendArtifactOrgPrefix(organizationID)
	rows, err := s.store.ListPrefix(ctx, prefix, s.listMaxKeys)
	if err != nil {
		recordObjectStorageFailure("list_artifacts", "store_list")
		return nil, err
	}
	out := make([]ArtifactInfo, 0, len(rows))
	for _, row := range rows {
		org, art, ok := objectstore.ParseBackendArtifactKey(row.Key)
		if !ok || org != organizationID {
			continue
		}
		info := ArtifactInfo{
			OrganizationID:  org,
			ArtifactID:      art,
			Size:            row.Size,
			ContentType:     row.ContentType,
			LastModifiedUTC: row.LastModified.UTC(),
			ETag:            row.ETag,
			ObjectKey:       row.Key,
		}
		// ListObjectsV2 does not return user metadata; enrich with Head (bounded).
		full, err := s.store.Head(ctx, row.Key)
		if err == nil && full.UserMetadata != nil {
			info.SHA256Hex = full.UserMetadata[metaSHA256Hex]
			info.OriginalFilename = full.UserMetadata[metaOriginalFilename]
			if full.ContentType != "" {
				info.ContentType = full.ContentType
			}
			if !full.LastModified.IsZero() {
				info.LastModifiedUTC = full.LastModified.UTC()
			}
			if full.Size > 0 {
				info.Size = full.Size
			}
		}
		out = append(out, info)
	}
	return out, nil
}

// DeleteArtifact removes the canonical artifact object.
func (s *Service) DeleteArtifact(ctx context.Context, organizationID, artifactID uuid.UUID) error {
	key := objectstore.BackendArtifactObjectKey(organizationID, artifactID)
	err := s.store.Delete(ctx, key)
	if err != nil {
		recordObjectStorageFailure("delete_artifact", "store_delete")
		return mapStoreError(err)
	}
	return nil
}

// PresignDownload returns a time-limited GET URL for the artifact payload.
func (s *Service) PresignDownload(ctx context.Context, organizationID, artifactID uuid.UUID) (objectstore.SignedHTTP, time.Time, error) {
	key := objectstore.BackendArtifactObjectKey(organizationID, artifactID)
	_, err := s.store.Head(ctx, key)
	if err != nil {
		recordObjectStorageFailure("presign_download", "store_head")
		return objectstore.SignedHTTP{}, time.Time{}, mapStoreError(err)
	}
	signed, err := s.store.PresignGet(ctx, key, s.downloadPresignTTL)
	if err != nil {
		recordObjectStorageFailure("presign_download", "store_presign")
		return objectstore.SignedHTTP{}, time.Time{}, err
	}
	exp := time.Now().UTC().Add(s.downloadPresignTTL)
	return signed, exp, nil
}

func mapStoreError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "nosuchkey") || strings.Contains(msg, "statuscode: 404") || strings.Contains(msg, "notfound") {
		return ErrNotFound
	}
	return err
}
