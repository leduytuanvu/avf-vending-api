package mediaadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/avf/avf-vending-api/internal/platform/objectstore"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const s3UserMetaSHA256 = metaSHA256Hex

// Service manages media_assets rows and S3 objects for the enterprise media pipeline.
type Service struct {
	pool        *pgxpool.Pool
	store       objectstore.Store
	audit       compliance.EnterpriseRecorder
	variants    VariantGenerator
	putTTL      time.Duration
	maxBytes    int64
	cache       CatalogMediaCacheBumper
	external    config.ExternalProductImageConfig
	uploadCfg   config.MediaUploadConfig
	cloudinary  ProductImageFileUploader
	appEnv      string
	remoteProbe func(ctx context.Context, imageURL, expectedMIME string, cfg config.ExternalProductImageConfig) error
}

// NewService returns a media pipeline service. Store may be nil when only external URL registration is enabled.
func NewService(d Deps) (*Service, error) {
	if d.Pool == nil {
		return nil, ErrNotConfigured
	}
	hasUpload := d.Store != nil
	hasExternal := d.External.Enabled
	hasCloudinary := d.Cloudinary != nil && d.Upload.CloudinaryConfigured()
	if !hasUpload && !hasExternal && !hasCloudinary {
		return nil, ErrNotConfigured
	}
	ttl := d.PresignPutTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	maxB := d.MaxUploadBytes
	if maxB <= 0 {
		maxB = 10 << 20
	}
	v := d.Variants
	if v == nil {
		v = WebPVariantGenerator{ThumbMax: d.ThumbMaxPixels, DisplayMax: d.DisplayMaxPixels}
	}
	return &Service{
		pool:        d.Pool,
		store:       d.Store,
		audit:       d.Audit,
		variants:    v,
		putTTL:      ttl,
		maxBytes:    maxB,
		cache:       d.Cache,
		external:    d.External,
		uploadCfg:   d.Upload,
		cloudinary:  d.Cloudinary,
		appEnv:      strings.TrimSpace(d.AppEnv),
		remoteProbe: d.RemoteProbe,
	}, nil
}

func (s *Service) bumpCache(ctx context.Context, org uuid.UUID) {
	if s == nil || s.cache == nil || org == uuid.Nil {
		return
	}
	s.cache.BumpCompanyMedia(ctx, org)
}

func strPtrTrim(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

func (s *Service) auditRecord(ctx context.Context, org uuid.UUID, action, resType string, resID *string, md map[string]any) {
	if s == nil || s.audit == nil || org == uuid.Nil {
		return
	}
	meta := compliance.TransportMetaFromContext(ctx)
	actorType, actorID := compliance.ActorSystem, ""
	if p, ok := plauth.PrincipalFromContext(ctx); ok {
		actorType, actorID = p.Actor()
	}
	var mdBytes []byte
	if len(md) > 0 {
		b, _ := json.Marshal(md)
		mdBytes = compliance.SanitizeJSONBytes(b)
	}
	if len(mdBytes) == 0 || string(mdBytes) == "null" {
		mdBytes = []byte("{}")
	}
	var aidPtr *string
	if actorID != "" {
		aidPtr = &actorID
	}
	_ = s.audit.Record(ctx, compliance.EnterpriseAuditRecord{
		ActorType:    actorType,
		ActorID:      aidPtr,
		Action:       action,
		ResourceType: resType,
		ResourceID:   resID,
		RequestID:    strPtrTrim(meta.RequestID),
		TraceID:      strPtrTrim(meta.TraceID),
		IPAddress:    strPtrTrim(meta.IP),
		UserAgent:    strPtrTrim(meta.UserAgent),
		Metadata:     mdBytes,
		Outcome:      compliance.OutcomeSuccess,
	})
}

// MaxUploadBytes returns configured multipart upload limit for Cloudinary uploads.
func (s *Service) MaxUploadBytes() int64 {
	if s == nil || s.uploadCfg.MaxBytes <= 0 {
		return 5 << 20
	}
	return s.uploadCfg.MaxBytes
}

// CloudinaryImageCacheKey builds offline cache key for Cloudinary-hosted product images.
func CloudinaryImageCacheKey(mediaID uuid.UUID, version int, checksum string) string {
	ch := strings.TrimSpace(checksum)
	if strings.HasPrefix(ch, "sha256:") {
		ch = strings.TrimPrefix(ch, "sha256:")
	}
	if version <= 0 {
		version = 1
	}
	return fmt.Sprintf("%s:%d:%s", mediaID.String(), version, ch)
}

// InitUploadResult is returned from POST /v1/admin/media/uploads.
type InitUploadResult struct {
	MediaID       uuid.UUID
	UploadURL     string
	UploadMethod  string
	UploadHeaders map[string][]string
	ExpiresAt     time.Time
	CompletePath  string
	OriginalKey   string
	ThumbKey      string
	DisplayKey    string
	Status        string
}

// InitUpload creates a pending media_assets row and a presigned PUT for the original object.
func (s *Service) InitUpload(ctx context.Context, companyID uuid.UUID, filename string, contentType string, purpose string) (*InitUploadResult, error) {
	if s == nil {
		return nil, ErrNotConfigured
	}
	if s.store == nil {
		return nil, ErrUploadNotConfigured
	}
	companyID, ct, err := validateImageContentType(companyID, contentType)
	if err != nil {
		return nil, err
	}
	purpose = strings.TrimSpace(strings.ToLower(purpose))
	if purpose == "" {
		purpose = "product_image"
	}
	if purpose != "product_image" {
		return nil, fmt.Errorf("%w: unsupported purpose %q", ErrInvalidArgument, purpose)
	}
	mediaID := id.NewUUIDV7()
	ok := objectstore.MediaAssetOriginalKey(companyID, mediaID)
	tk := objectstore.MediaAssetThumbWebpKey(companyID, mediaID)
	dk := objectstore.MediaAssetDisplayWebpKey(companyID, mediaID)
	q := db.New(s.pool)
	createdBy := pgtype.UUID{}
	if p, ok := plauth.PrincipalFromContext(ctx); ok {
		if uid, err := uuid.Parse(strings.TrimSpace(p.Subject)); err == nil && uid != uuid.Nil {
			createdBy = pgtype.UUID{Bytes: uid, Valid: true}
		}
	}
	fn := strings.TrimSpace(filename)
	fnPg := pgtype.Text{}
	if fn != "" {
		fnPg = pgtype.Text{String: fn, Valid: true}
	}
	row, err := q.MediaAdminInsertAsset(ctx, db.MediaAdminInsertAssetParams{
		ID:                mediaID,
		Kind:              "product_image",
		OriginalFilename:  fnPg,
		ObjectKey:         pgtype.Text{String: ok, Valid: true},
		OriginalObjectKey: ok,
		ThumbObjectKey:    tk,
		DisplayObjectKey:  dk,
		SourceType:        "upload",
		OriginalUrl:       pgtype.Text{},
		MimeType:          pgtype.Text{String: ct, Valid: true},
		CreatedBy:         createdBy,
		Status:            "pending",
	})
	if err != nil {
		return nil, err
	}
	signed, err := s.store.PresignPut(ctx, ok, ct, s.putTTL)
	if err != nil {
		if _, derr := q.MediaAdminDeletePendingAsset(ctx, row.ID); derr != nil {
			return nil, fmt.Errorf("presign put failed: %w (cleanup pending asset failed: %v)", err, derr)
		}
		return nil, fmt.Errorf("presign put: %w", err)
	}
	mid := row.ID.String()
	s.auditRecord(ctx, companyID, compliance.ActionMediaCreated, "media.asset", &mid, map[string]any{
		"phase":     "init_upload",
		"kind":      "product_image",
		"status":    "pending",
		"mime_type": ct,
	})
	exp := time.Now().UTC().Add(s.putTTL)
	return &InitUploadResult{
		MediaID:       row.ID,
		UploadURL:     signed.URL,
		UploadMethod:  signed.Method,
		UploadHeaders: signed.Headers,
		ExpiresAt:     exp,
		CompletePath:  "/v1/admin/media/uploads/" + row.ID.String() + "/complete",
		OriginalKey:   ok,
		ThumbKey:      tk,
		DisplayKey:    dk,
		Status:        "pending",
	}, nil
}

func validateImageContentType(companyID uuid.UUID, contentType string) (uuid.UUID, string, error) {
	if companyID == uuid.Nil {
		return uuid.Nil, "", fmt.Errorf("%w: company_id", ErrInvalidArgument)
	}
	ct := normalizeMIMEHeader(contentType)
	if ct == "" {
		return uuid.Nil, "", fmt.Errorf("%w: content_type required", ErrInvalidArgument)
	}
	if err := validateRasterUploadMIME(ct); err != nil {
		return uuid.Nil, "", err
	}
	return companyID, ct, nil
}

func normalizeMIMEHeader(mt string) string {
	s := strings.TrimSpace(strings.ToLower(mt))
	if i := strings.IndexByte(s, ';'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

func validateRasterUploadMIME(ct string) error {
	switch ct {
	case "image/jpeg", "image/png", "image/webp":
		return nil
	default:
		return fmt.Errorf("%w: content_type must be one of image/jpeg, image/png, image/webp", ErrInvalidArgument)
	}
}

// validateHeadContentTypeForImageUpload enforces image/* (subset validated separately) or application/octet-stream
// so clients can upload valid images before magic-byte sniffing in the variant pipeline.
func validateHeadContentTypeForImageUpload(raw string) error {
	ct := normalizeMIMEHeader(raw)
	if ct == "" || ct == "application/octet-stream" {
		return nil
	}
	if !strings.HasPrefix(ct, "image/") {
		return fmt.Errorf("%w: stored object Content-Type must be image/* or application/octet-stream (got %q)", ErrInvalidArgument, ct)
	}
	return validateRasterUploadMIME(ct)
}

// CompleteUploadOptions carries optional client-supplied integrity hints validated against the uploaded bytes.
type CompleteUploadOptions struct {
	SizeBytes   *int64
	SHA256Hex   string
	ContentType string
}

// CompleteUpload finalizes a pending asset: generates variants, records SHA256/size, marks ready.
func (s *Service) CompleteUpload(ctx context.Context, companyID, mediaID uuid.UUID) (*db.MediaAsset, error) {
	return s.CompleteUploadWithOptions(ctx, companyID, mediaID, CompleteUploadOptions{})
}

// CompleteUploadWithOptions is like CompleteUpload but validates optional client claims against measured bytes/hashes.
func (s *Service) CompleteUploadWithOptions(ctx context.Context, companyID, mediaID uuid.UUID, opts CompleteUploadOptions) (*db.MediaAsset, error) {
	if s == nil {
		return nil, ErrNotConfigured
	}
	if s.store == nil {
		return nil, ErrUploadNotConfigured
	}
	if companyID == uuid.Nil || mediaID == uuid.Nil {
		return nil, ErrInvalidArgument
	}
	q := db.New(s.pool)
	asset, err := q.MediaAdminGetAssetForOrg(ctx, mediaID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if asset.Status != "pending" {
		return nil, fmt.Errorf("%w: asset not pending", ErrConflict)
	}
	head, err := s.store.Head(ctx, asset.OriginalObjectKey)
	if err != nil {
		return nil, err
	}
	if head.Size <= 0 {
		return nil, fmt.Errorf("%w: original object empty", ErrInvalidArgument)
	}
	if s.maxBytes > 0 && head.Size > s.maxBytes {
		return nil, fmt.Errorf("%w: original exceeds max bytes", ErrInvalidArgument)
	}
	origMIME := strings.TrimSpace(head.ContentType)
	if origMIME == "" && asset.MimeType.Valid {
		origMIME = strings.TrimSpace(asset.MimeType.String)
	}
	if strings.TrimSpace(origMIME) != "" {
		if err := validateHeadContentTypeForImageUpload(origMIME); err != nil {
			return nil, err
		}
	}
	if ct := strings.TrimSpace(opts.ContentType); ct != "" {
		if normalizeMIMEHeader(ct) != normalizeMIMEHeader(origMIME) {
			return nil, fmt.Errorf("%w: contentType does not match uploaded object", ErrInvalidArgument)
		}
	}
	va, err := s.variants.GenerateWebPVariants(ctx, s.store, companyID, mediaID, asset.OriginalObjectKey, s.maxBytes)
	if err != nil {
		_, _ = q.MediaAdminMarkAssetFailed(ctx, db.MediaAdminMarkAssetFailedParams{FailedReason: pgtype.Text{String: err.Error(), Valid: true},
			ID: mediaID,
		})
		mid := mediaID.String()
		s.auditRecord(ctx, companyID, compliance.ActionMediaProcessingFailed, "media.asset", &mid, map[string]any{
			"phase": "complete_upload",
			"error": err.Error(),
		})
		return nil, err
	}
	if opts.SizeBytes != nil && *opts.SizeBytes != va.OriginalBytes {
		return nil, fmt.Errorf("%w: sizeBytes does not match uploaded object", ErrInvalidArgument)
	}
	if hx := strings.TrimSpace(opts.SHA256Hex); hx != "" {
		if normalizeSHA256Hex(hx) != normalizeSHA256Hex(va.OriginalSHA256Hex) {
			return nil, fmt.Errorf("%w: sha256 does not match uploaded object", ErrInvalidArgument)
		}
	}
	dhead, err := s.store.Head(ctx, asset.DisplayObjectKey)
	if err != nil {
		return nil, err
	}
	sha := normalizeSHA256Hex(va.DisplaySHA256Hex)
	if sha == "" && dhead.UserMetadata != nil {
		sha = normalizeSHA256Hex(dhead.UserMetadata[s3UserMetaSHA256])
	}
	etag := ""
	if sha != "" {
		etag = `W/"` + sha + `"`
	} else if e := strings.TrimSpace(dhead.ETag); e != "" {
		etag = `W/"` + strings.Trim(e, `"`) + `"`
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)

	if _, err := qtx.MediaAdminUpdateAssetReady(ctx, db.MediaAdminUpdateAssetReadyParams{MimeType: pgtype.Text{String: webpContentType, Valid: true},
		SizeBytes: pgtype.Int8{Int64: va.DisplayBytes, Valid: true},
		Sha256:    pgtype.Text{String: sha, Valid: sha != ""},
		Width:     pgtype.Int4{Int32: int32(va.DisplayWidth), Valid: va.DisplayWidth > 0},
		Height:    pgtype.Int4{Int32: int32(va.DisplayHeight), Valid: va.DisplayHeight > 0},
		Etag:      pgtype.Text{String: etag, Valid: etag != ""},
		ID:        mediaID,
	}); err != nil {
		return nil, err
	}
	canonical, err := qtx.MediaAdminEnsureCanonicalObjectKey(ctx, mediaID)
	if err != nil {
		return nil, err
	}
	if err := s.persistMediaVariants(ctx, qtx, canonical, va, origMIME); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	s.bumpCache(ctx, companyID)
	mid := mediaID.String()
	s.auditRecord(ctx, companyID, compliance.ActionMediaVariantGenerated, "media.asset", &mid, map[string]any{
		"phase":           "complete_upload",
		"variants":        []string{"original", "thumb", "display"},
		"thumb_mime":      webpContentType,
		"display_mime":    webpContentType,
		"display_bytes":   va.DisplayBytes,
		"thumb_bytes":     va.ThumbBytes,
		"thumb_sha256":    va.ThumbSHA256Hex,
		"display_sha256":  va.DisplaySHA256Hex,
		"display_width":   va.DisplayWidth,
		"display_height":  va.DisplayHeight,
		"original_sha256": va.OriginalSHA256Hex,
	})
	s.auditRecord(ctx, companyID, compliance.ActionMediaUploaded, "media.asset", &mid, map[string]any{"phase": "complete", "size_bytes": va.DisplayBytes})
	return &canonical, nil
}

func (s *Service) persistMediaVariants(ctx context.Context, qtx *db.Queries, asset db.MediaAsset, va VariantArtifacts, originalMIME string) error {
	if err := qtx.MediaAdminDeleteVariantsForAsset(ctx, asset.ID); err != nil {
		return err
	}
	ver := asset.ObjectVersion
	ins := func(variant, objectKey, mime string, w, h int, sz int64, shaHex string) error {
		sh := normalizeSHA256Hex(shaHex)
		var shaPg pgtype.Text
		if sh != "" {
			shaPg = pgtype.Text{String: sh, Valid: true}
		}
		var wPg, hPg pgtype.Int4
		if w > 0 {
			wPg = pgtype.Int4{Int32: int32(w), Valid: true}
		}
		if h > 0 {
			hPg = pgtype.Int4{Int32: int32(h), Valid: true}
		}
		var szPg pgtype.Int8
		if sz >= 0 {
			szPg = pgtype.Int8{Int64: sz, Valid: true}
		}
		mt := strings.TrimSpace(mime)
		var mimePg pgtype.Text
		if mt != "" {
			mimePg = pgtype.Text{String: mt, Valid: true}
		}
		_, err := qtx.MediaAdminInsertMediaVariant(ctx, db.MediaAdminInsertMediaVariantParams{
			MediaAssetID: asset.ID,
			Variant:      variant,
			ObjectKey:    objectKey,
			MimeType:     mimePg,
			Width:        wPg,
			Height:       hPg,
			SizeBytes:    szPg,
			Sha256:       shaPg,
			Version:      ver,
		})
		return err
	}
	om := normalizeMIMEHeader(originalMIME)
	if om == "" {
		om = "application/octet-stream"
	}
	if err := ins("original", asset.OriginalObjectKey, om, va.OriginalWidth, va.OriginalHeight, va.OriginalBytes, va.OriginalSHA256Hex); err != nil {
		return err
	}
	if err := ins("thumb", asset.ThumbObjectKey, webpContentType, va.ThumbWidth, va.ThumbHeight, va.ThumbBytes, va.ThumbSHA256Hex); err != nil {
		return err
	}
	return ins("display", asset.DisplayObjectKey, webpContentType, va.DisplayWidth, va.DisplayHeight, va.DisplayBytes, va.DisplaySHA256Hex)
}

// GetAsset returns one media asset for the company.
func (s *Service) GetAsset(ctx context.Context, companyID, mediaID uuid.UUID) (db.MediaAsset, error) {
	if s == nil {
		return db.MediaAsset{}, ErrNotConfigured
	}
	q := db.New(s.pool)
	a, err := q.MediaAdminGetAssetForOrg(ctx, mediaID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return db.MediaAsset{}, ErrNotFound
		}
		return db.MediaAsset{}, err
	}
	if a.Status == "deleted" || a.Status == "archived" {
		return db.MediaAsset{}, ErrNotFound
	}
	return a, nil
}

// ListAssetsPage returns a page of non-deleted assets.
func (s *Service) ListAssetsPage(ctx context.Context, companyID uuid.UUID, limit, offset int32) ([]db.MediaAsset, int64, error) {
	if s == nil {
		return nil, 0, ErrNotConfigured
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	q := db.New(s.pool)
	total, err := q.MediaAdminCountAssetsForOrg(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := q.MediaAdminListAssetsForOrg(ctx, db.MediaAdminListAssetsForOrgParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// DeleteAsset soft-deletes the asset, removes product_image bindings, and deletes objects best-effort.
func (s *Service) DeleteAsset(ctx context.Context, companyID, mediaID uuid.UUID) error {
	if s == nil {
		return ErrNotConfigured
	}
	q := db.New(s.pool)
	asset, err := q.MediaAdminGetAssetForOrg(ctx, mediaID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ErrNotFound
		}
		return err
	}
	if asset.Status == "deleted" || asset.Status == "archived" {
		return ErrNotFound
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)
	binds, err := qtx.MediaAdminListProductImagesForAsset(ctx, pgtype.UUID{Bytes: mediaID, Valid: true})
	if err != nil {
		return err
	}
	for _, b := range binds {
		if b.IsPrimary {
			if _, err := qtx.CatalogWriteClearProductPrimaryImage(ctx, b.ProductID); err != nil {
				return err
			}
		}
	}
	if err := qtx.MediaAdminArchiveProductImagesForMediaAsset(ctx, pgtype.UUID{Bytes: mediaID, Valid: true}); err != nil {
		return err
	}
	if _, err := qtx.MediaAdminSoftDeleteAsset(ctx, mediaID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if s.store != nil {
		_ = s.store.Delete(ctx, asset.OriginalObjectKey)
		_ = s.store.Delete(ctx, asset.ThumbObjectKey)
		_ = s.store.Delete(ctx, asset.DisplayObjectKey)
	}
	s.bumpCache(ctx, companyID)
	mid := mediaID.String()
	s.auditRecord(ctx, companyID, compliance.ActionMediaDeleted, "media.asset", &mid, map[string]any{"media_id": mid})
	return nil
}

// normalizeProductMediaSourceType maps media_assets.source_type values onto product_media.source_type
// (upload | external | import). Cloudinary and other hosted pipelines store URLs as external.
func normalizeProductMediaSourceType(assetSourceType string) string {
	switch strings.ToLower(strings.TrimSpace(assetSourceType)) {
	case "upload", "external", "import":
		return strings.ToLower(strings.TrimSpace(assetSourceType))
	case "cloudinary", "uploaded_file", "external_url":
		return "external"
	default:
		return "upload"
	}
}

func upsertPrimaryProductMediaProjection(ctx context.Context, qtx *db.Queries, productID uuid.UUID, img db.ProductImage, asset db.MediaAsset, thumbURL, dispURL string) error {
	var sz int64
	if asset.SizeBytes.Valid {
		sz = asset.SizeBytes.Int64
	}
	ok := func(k string) pgtype.Text {
		k = strings.TrimSpace(k)
		if k == "" {
			return pgtype.Text{}
		}
		return pgtype.Text{String: k, Valid: true}
	}
	sourceType := normalizeProductMediaSourceType(asset.SourceType)
	origURL := pgtype.Text{}
	if asset.OriginalUrl.Valid {
		if s := strings.TrimSpace(asset.OriginalUrl.String); s != "" {
			origURL = pgtype.Text{String: s, Valid: true}
		}
	}
	_, err := qtx.CatalogWriteUpsertProductMediaProjection(ctx, db.CatalogWriteUpsertProductMediaProjectionParams{
		ID:                img.ID,
		ProductID:         productID,
		MediaRole:         "primary",
		SourceType:        sourceType,
		OriginalUrl:       origURL,
		OriginalObjectKey: ok(asset.OriginalObjectKey),
		ThumbObjectKey:    ok(asset.ThumbObjectKey),
		DisplayObjectKey:  ok(asset.DisplayObjectKey),
		ThumbUrl:          pgtype.Text{String: thumbURL, Valid: thumbURL != ""},
		DisplayUrl:        pgtype.Text{String: dispURL, Valid: dispURL != ""},
		MimeType:          asset.MimeType,
		Width:             asset.Width,
		Height:            asset.Height,
		SizeBytes:         sz,
		ContentHash:       img.ContentHash,
		MediaVersion:      img.MediaVersion,
	})
	return err
}

// BindProductPrimaryMediaTx binds ready media inside an existing transaction (shared with catalog writes).
func (s *Service) BindProductPrimaryMediaTx(ctx context.Context, tx pgx.Tx, companyID, productID, mediaID uuid.UUID) (*db.Product, error) {
	if s == nil {
		return nil, ErrNotConfigured
	}
	qtx := db.New(tx)
	return s.bindProductPrimaryMediaWithQ(ctx, qtx, companyID, productID, mediaID)
}

// BindProductPrimaryMedia binds a ready asset as the sole primary image for a product.
func (s *Service) BindProductPrimaryMedia(ctx context.Context, companyID, productID, mediaID uuid.UUID) (*db.Product, error) {
	if s == nil {
		return nil, ErrNotConfigured
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	prod, err := s.BindProductPrimaryMediaTx(ctx, tx, companyID, productID, mediaID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.bumpCache(ctx, companyID)
	pid := productID.String()
	mid := mediaID.String()
	s.auditRecord(ctx, companyID, compliance.ActionMediaBoundToProduct, "catalog.product", &pid, map[string]any{"product_id": pid, "media_id": mid})
	s.auditRecord(ctx, companyID, compliance.ActionMediaBound, "catalog.product", &pid, map[string]any{"product_id": pid, "media_id": mid})
	return prod, nil
}

func (s *Service) bindProductPrimaryMediaWithQ(ctx context.Context, qtx *db.Queries, companyID, productID, mediaID uuid.UUID) (*db.Product, error) {
	asset, err := qtx.MediaAdminGetAssetForOrg(ctx, mediaID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if asset.Status != "ready" {
		return nil, fmt.Errorf("%w: media not ready", ErrConflict)
	}
	if _, err := qtx.CatalogAdminGetProduct(ctx, productID); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var thumbURL, dispURL, storageKey string
	switch asset.SourceType {
	case "external", "cloudinary":
		ext := ""
		if asset.OriginalUrl.Valid {
			ext = strings.TrimSpace(asset.OriginalUrl.String)
		}
		if ext == "" && asset.SourceType == "cloudinary" {
			ext = strings.TrimSpace(asset.DisplayObjectKey)
		}
		if ext == "" {
			return nil, fmt.Errorf("%w: hosted media missing display url", ErrInvalidArgument)
		}
		dispURL = ext
		thumbURL = strings.TrimSpace(asset.ThumbObjectKey)
		if thumbURL == "" || !strings.HasPrefix(strings.ToLower(thumbURL), "http") {
			thumbURL = ext
		}
		storageKey = asset.DisplayObjectKey
		if storageKey == "" {
			storageKey = ext
		}
	default:
		if s.store == nil {
			return nil, ErrUploadNotConfigured
		}
		thumbSigned, err := s.store.PresignGet(ctx, asset.ThumbObjectKey, s.putTTL)
		if err != nil {
			return nil, err
		}
		dispSigned, err := s.store.PresignGet(ctx, asset.DisplayObjectKey, s.putTTL)
		if err != nil {
			return nil, err
		}
		thumbURL = thumbSigned.URL
		dispURL = dispSigned.URL
		storageKey = asset.DisplayObjectKey
	}
	ch := ""
	if asset.Sha256.Valid {
		ch = strings.TrimSpace(asset.Sha256.String)
	}
	if ch != "" && !strings.HasPrefix(ch, "sha256:") {
		ch = "sha256:" + ch
	}
	if _, err := qtx.CatalogWriteClearProductPrimaryImage(ctx, productID); err != nil {
		return nil, err
	}
	if err := qtx.CatalogWriteArchiveAllProductImagesForProduct(ctx, productID); err != nil {
		return nil, err
	}
	img, err := qtx.CatalogWriteInsertProductImageWithMedia(ctx, db.CatalogWriteInsertProductImageWithMediaParams{
		ProductID:    productID,
		StorageKey:   storageKey,
		CdnUrl:       pgtype.Text{String: dispURL, Valid: true},
		ThumbCdnUrl:  pgtype.Text{String: thumbURL, Valid: true},
		ContentHash:  pgtype.Text{String: ch, Valid: ch != ""},
		Width:        asset.Width,
		Height:       asset.Height,
		MimeType:     asset.MimeType,
		AltText:      "",
		SortOrder:    0,
		IsPrimary:    true,
		MediaAssetID: pgtype.UUID{Bytes: mediaID, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	prod, err := qtx.CatalogWriteSetProductPrimaryImage(ctx, db.CatalogWriteSetProductPrimaryImageParams{
		PrimaryImageID: pgtype.UUID{Bytes: img.ID, Valid: true},
		ID:             productID,
	})
	if err != nil {
		return nil, err
	}
	if err := upsertPrimaryProductMediaProjection(ctx, qtx, productID, img, asset, thumbURL, dispURL); err != nil {
		return nil, err
	}
	_ = companyID
	return &prod, nil
}

// UnbindProductMedia removes a product_image row bound to mediaID for the product.
func (s *Service) UnbindProductMedia(ctx context.Context, companyID, productID, mediaID uuid.UUID) (*db.Product, error) {
	if s == nil {
		return nil, ErrNotConfigured
	}
	q := db.New(s.pool)
	imgID, err := q.MediaAdminFindProductImageBinding(ctx, db.MediaAdminFindProductImageBindingParams{
		ProductID:    productID,
		MediaAssetID: pgtype.UUID{Bytes: mediaID, Valid: true},
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)
	pimg, err := qtx.CatalogAdminGetPrimaryProductImageForOrg(ctx, productID)
	clearPrimary := err == nil && pimg.ID == imgID
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	if clearPrimary {
		if _, err := qtx.CatalogWriteClearProductPrimaryImage(ctx, productID); err != nil {
			return nil, err
		}
	}
	if _, err := qtx.CatalogWriteArchiveProductImage(ctx, db.CatalogWriteArchiveProductImageParams{
		ID:   productID,
		ID_2: imgID,
	}); err != nil {
		return nil, err
	}
	prod, err := qtx.CatalogAdminGetProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.bumpCache(ctx, companyID)
	pid := productID.String()
	mid := mediaID.String()
	s.auditRecord(ctx, companyID, compliance.ActionMediaUnboundFromProduct, "catalog.product", &pid, map[string]any{"product_id": pid, "media_id": mid})
	s.auditRecord(ctx, companyID, compliance.ActionMediaUnbound, "catalog.product", &pid, map[string]any{"product_id": pid, "media_id": mid})
	return &prod, nil
}

// ListAssetsByIDs returns media_assets rows for the given ids (empty ids yields empty slice).
func (s *Service) ListAssetsByIDs(ctx context.Context, ids []uuid.UUID) ([]db.MediaAsset, error) {
	if s == nil || s.pool == nil {
		return nil, ErrNotConfigured
	}
	if len(ids) == 0 {
		return nil, nil
	}
	q := db.New(s.pool)
	return q.MediaAdminListAssetsByIDs(ctx, ids)
}

// ListVariantsForAssets returns media_variants rows for assets (empty ids yields empty slice).
func (s *Service) ListVariantsForAssets(ctx context.Context, assetIDs []uuid.UUID) ([]db.MediaVariant, error) {
	if s == nil || s.pool == nil {
		return nil, ErrNotConfigured
	}
	if len(assetIDs) == 0 {
		return nil, nil
	}
	q := db.New(s.pool)
	return q.MediaAdminListVariantsForAssets(ctx, assetIDs)
}

// PresignedDownloadURL returns a short-lived GET URL for an object key.
func (s *Service) PresignedDownloadURL(ctx context.Context, objectKey string) (string, error) {
	if s == nil || s.store == nil {
		return "", ErrNotConfigured
	}
	key := strings.TrimSpace(objectKey)
	if key == "" {
		return "", fmt.Errorf("%w: empty object key", ErrInvalidArgument)
	}
	signed, err := s.store.PresignGet(ctx, key, s.putTTL)
	if err != nil {
		return "", err
	}
	return signed.URL, nil
}
