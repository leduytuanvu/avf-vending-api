package mediaadmin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/objectstore"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const s3UserMetaSHA256 = metaSHA256Hex

// Service manages media_assets rows and S3 objects for the enterprise media pipeline.
type Service struct {
	pool     *pgxpool.Pool
	store    objectstore.Store
	audit    compliance.EnterpriseRecorder
	variants VariantGenerator
	putTTL   time.Duration
	maxBytes int64
	cache    CatalogMediaCacheBumper
}

// NewService returns a media pipeline service. Store must be non-nil.
func NewService(d Deps) (*Service, error) {
	if d.Store == nil || d.Pool == nil {
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
		pool:     d.Pool,
		store:    d.Store,
		audit:    d.Audit,
		variants: v,
		putTTL:   ttl,
		maxBytes: maxB,
		cache:    d.Cache,
	}, nil
}

func (s *Service) bumpCache(ctx context.Context, org uuid.UUID) {
	if s == nil || s.cache == nil || org == uuid.Nil {
		return
	}
	s.cache.BumpOrganizationMedia(ctx, org)
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
		OrganizationID: org,
		ActorType:      actorType,
		ActorID:        aidPtr,
		Action:         action,
		ResourceType:   resType,
		ResourceID:     resID,
		RequestID:      strPtrTrim(meta.RequestID),
		TraceID:        strPtrTrim(meta.TraceID),
		IPAddress:      strPtrTrim(meta.IP),
		UserAgent:      strPtrTrim(meta.UserAgent),
		Metadata:       mdBytes,
		Outcome:        compliance.OutcomeSuccess,
	})
}

// InitUploadResult is returned from POST /v1/admin/media/uploads.
type InitUploadResult struct {
	MediaID        uuid.UUID
	UploadURL      string
	UploadMethod   string
	UploadHeaders  map[string][]string
	ExpiresAt      time.Time
	CompletePath   string
	OriginalKey    string
	ThumbKey       string
	DisplayKey     string
	OrganizationID uuid.UUID
}

// InitUpload creates a pending media_assets row and a presigned PUT for the original object.
func (s *Service) InitUpload(ctx context.Context, organizationID uuid.UUID, contentType string) (*InitUploadResult, error) {
	if s == nil {
		return nil, ErrNotConfigured
	}
	organizationID, ct, err := validateImageContentType(organizationID, contentType)
	if err != nil {
		return nil, err
	}
	mediaID := uuid.New()
	ok := objectstore.MediaAssetOriginalKey(organizationID, mediaID)
	tk := objectstore.MediaAssetThumbWebpKey(organizationID, mediaID)
	dk := objectstore.MediaAssetDisplayWebpKey(organizationID, mediaID)
	q := db.New(s.pool)
	createdBy := pgtype.UUID{}
	if p, ok := plauth.PrincipalFromContext(ctx); ok {
		if uid, err := uuid.Parse(strings.TrimSpace(p.Subject)); err == nil && uid != uuid.Nil {
			createdBy = pgtype.UUID{Bytes: uid, Valid: true}
		}
	}
	row, err := q.MediaAdminInsertAsset(ctx, db.MediaAdminInsertAssetParams{
		OrganizationID:    organizationID,
		Kind:              "product_image",
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
		if _, derr := q.MediaAdminDeletePendingAsset(ctx, db.MediaAdminDeletePendingAssetParams{
			ID:             row.ID,
			OrganizationID: organizationID,
		}); derr != nil {
			return nil, fmt.Errorf("presign put failed: %w (cleanup pending asset failed: %v)", err, derr)
		}
		return nil, fmt.Errorf("presign put: %w", err)
	}
	mid := row.ID.String()
	s.auditRecord(ctx, organizationID, compliance.ActionMediaCreated, "media.asset", &mid, map[string]any{
		"phase":     "init_upload",
		"kind":      "product_image",
		"status":    "pending",
		"mime_type": ct,
	})
	exp := time.Now().UTC().Add(s.putTTL)
	return &InitUploadResult{
		MediaID:        row.ID,
		UploadURL:      signed.URL,
		UploadMethod:   signed.Method,
		UploadHeaders:  signed.Headers,
		ExpiresAt:      exp,
		CompletePath:   "/v1/admin/media/" + row.ID.String() + "/complete",
		OriginalKey:    ok,
		ThumbKey:       tk,
		DisplayKey:     dk,
		OrganizationID: organizationID,
	}, nil
}

func validateImageContentType(organizationID uuid.UUID, contentType string) (uuid.UUID, string, error) {
	if organizationID == uuid.Nil {
		return uuid.Nil, "", fmt.Errorf("%w: organization_id", ErrInvalidArgument)
	}
	ct := normalizeMIMEHeader(contentType)
	if ct == "" {
		return uuid.Nil, "", fmt.Errorf("%w: content_type required", ErrInvalidArgument)
	}
	if err := validateRasterUploadMIME(ct); err != nil {
		return uuid.Nil, "", err
	}
	return organizationID, ct, nil
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

// CompleteUpload finalizes a pending asset: generates variants, records SHA256/size, marks ready.
func (s *Service) CompleteUpload(ctx context.Context, organizationID, mediaID uuid.UUID) (*db.MediaAsset, error) {
	if s == nil {
		return nil, ErrNotConfigured
	}
	if organizationID == uuid.Nil || mediaID == uuid.Nil {
		return nil, ErrInvalidArgument
	}
	q := db.New(s.pool)
	asset, err := q.MediaAdminGetAssetForOrg(ctx, db.MediaAdminGetAssetForOrgParams{
		ID:             mediaID,
		OrganizationID: organizationID,
	})
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
	if strings.TrimSpace(head.ContentType) != "" {
		if err := validateRasterUploadMIME(normalizeMIMEHeader(head.ContentType)); err != nil {
			return nil, err
		}
	}
	va, err := s.variants.GenerateWebPVariants(ctx, s.store, organizationID, mediaID, asset.OriginalObjectKey, s.maxBytes)
	if err != nil {
		_, _ = q.MediaAdminMarkAssetFailed(ctx, db.MediaAdminMarkAssetFailedParams{
			ID:             mediaID,
			OrganizationID: organizationID,
			FailedReason:   pgtype.Text{String: err.Error(), Valid: true},
		})
		mid := mediaID.String()
		s.auditRecord(ctx, organizationID, compliance.ActionMediaProcessingFailed, "media.asset", &mid, map[string]any{
			"phase": "complete_upload",
			"error": err.Error(),
		})
		return nil, err
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
	updated, err := q.MediaAdminUpdateAssetReady(ctx, db.MediaAdminUpdateAssetReadyParams{
		ID:             mediaID,
		OrganizationID: organizationID,
		MimeType:       pgtype.Text{String: webpContentType, Valid: true},
		SizeBytes:      pgtype.Int8{Int64: va.DisplayBytes, Valid: true},
		Sha256:         pgtype.Text{String: sha, Valid: sha != ""},
		Width:          pgtype.Int4{Int32: int32(va.DisplayWidth), Valid: va.DisplayWidth > 0},
		Height:         pgtype.Int4{Int32: int32(va.DisplayHeight), Valid: va.DisplayHeight > 0},
		Etag:           pgtype.Text{String: etag, Valid: etag != ""},
	})
	if err != nil {
		return nil, err
	}
	s.bumpCache(ctx, organizationID)
	mid := mediaID.String()
	s.auditRecord(ctx, organizationID, compliance.ActionMediaVariantGenerated, "media.asset", &mid, map[string]any{
		"phase":          "complete_upload",
		"variants":       []string{"original", "thumb", "display"},
		"thumb_mime":     webpContentType,
		"display_mime":   webpContentType,
		"display_bytes":  va.DisplayBytes,
		"thumb_bytes":    va.ThumbBytes,
		"thumb_sha256":   va.ThumbSHA256Hex,
		"display_sha256": va.DisplaySHA256Hex,
		"display_width":  va.DisplayWidth,
		"display_height": va.DisplayHeight,
	})
	s.auditRecord(ctx, organizationID, compliance.ActionMediaUploaded, "media.asset", &mid, map[string]any{"phase": "complete", "size_bytes": va.DisplayBytes})
	return &updated, nil
}

// GetAsset returns one media asset for the tenant.
func (s *Service) GetAsset(ctx context.Context, organizationID, mediaID uuid.UUID) (db.MediaAsset, error) {
	if s == nil {
		return db.MediaAsset{}, ErrNotConfigured
	}
	q := db.New(s.pool)
	a, err := q.MediaAdminGetAssetForOrg(ctx, db.MediaAdminGetAssetForOrgParams{
		ID:             mediaID,
		OrganizationID: organizationID,
	})
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
func (s *Service) ListAssetsPage(ctx context.Context, organizationID uuid.UUID, limit, offset int32) ([]db.MediaAsset, int64, error) {
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
	total, err := q.MediaAdminCountAssetsForOrg(ctx, organizationID)
	if err != nil {
		return nil, 0, err
	}
	rows, err := q.MediaAdminListAssetsForOrg(ctx, db.MediaAdminListAssetsForOrgParams{
		OrganizationID: organizationID,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// DeleteAsset soft-deletes the asset, removes product_image bindings, and deletes objects best-effort.
func (s *Service) DeleteAsset(ctx context.Context, organizationID, mediaID uuid.UUID) error {
	if s == nil {
		return ErrNotConfigured
	}
	q := db.New(s.pool)
	asset, err := q.MediaAdminGetAssetForOrg(ctx, db.MediaAdminGetAssetForOrgParams{
		ID:             mediaID,
		OrganizationID: organizationID,
	})
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
			if _, err := qtx.CatalogWriteClearProductPrimaryImage(ctx, db.CatalogWriteClearProductPrimaryImageParams{
				OrganizationID: b.OrganizationID,
				ID:             b.ProductID,
			}); err != nil {
				return err
			}
		}
	}
	if err := qtx.MediaAdminArchiveProductImagesForMediaAsset(ctx, pgtype.UUID{Bytes: mediaID, Valid: true}); err != nil {
		return err
	}
	if _, err := qtx.MediaAdminSoftDeleteAsset(ctx, db.MediaAdminSoftDeleteAssetParams{
		ID:             mediaID,
		OrganizationID: organizationID,
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	_ = s.store.Delete(ctx, asset.OriginalObjectKey)
	_ = s.store.Delete(ctx, asset.ThumbObjectKey)
	_ = s.store.Delete(ctx, asset.DisplayObjectKey)
	s.bumpCache(ctx, organizationID)
	mid := mediaID.String()
	s.auditRecord(ctx, organizationID, compliance.ActionMediaDeleted, "media.asset", &mid, map[string]any{"media_id": mid})
	return nil
}

// BindProductPrimaryMedia binds a ready asset as the sole primary image for a product.
func (s *Service) BindProductPrimaryMedia(ctx context.Context, organizationID, productID, mediaID uuid.UUID) (*db.Product, error) {
	if s == nil {
		return nil, ErrNotConfigured
	}
	asset, err := s.GetAsset(ctx, organizationID, mediaID)
	if err != nil {
		return nil, err
	}
	if asset.Status != "ready" {
		return nil, fmt.Errorf("%w: media not ready", ErrConflict)
	}
	q := db.New(s.pool)
	if _, err := q.CatalogAdminGetProduct(ctx, db.CatalogAdminGetProductParams{
		OrganizationID: organizationID,
		ID:             productID,
	}); err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNotFound
		}
		return nil, err
	}
	thumbSigned, err := s.store.PresignGet(ctx, asset.ThumbObjectKey, s.putTTL)
	if err != nil {
		return nil, err
	}
	dispSigned, err := s.store.PresignGet(ctx, asset.DisplayObjectKey, s.putTTL)
	if err != nil {
		return nil, err
	}
	ch := ""
	if asset.Sha256.Valid {
		ch = strings.TrimSpace(asset.Sha256.String)
	}
	if ch != "" && !strings.HasPrefix(ch, "sha256:") {
		ch = "sha256:" + ch
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)
	if _, err := qtx.CatalogWriteClearProductPrimaryImage(ctx, db.CatalogWriteClearProductPrimaryImageParams{
		OrganizationID: organizationID,
		ID:             productID,
	}); err != nil {
		return nil, err
	}
	if err := qtx.CatalogWriteArchiveAllProductImagesForProduct(ctx, db.CatalogWriteArchiveAllProductImagesForProductParams{
		OrganizationID: organizationID,
		ID:             productID,
	}); err != nil {
		return nil, err
	}
	img, err := qtx.CatalogWriteInsertProductImageWithMedia(ctx, db.CatalogWriteInsertProductImageWithMediaParams{
		ProductID:    productID,
		StorageKey:   asset.DisplayObjectKey,
		CdnUrl:       pgtype.Text{String: dispSigned.URL, Valid: true},
		ThumbCdnUrl:  pgtype.Text{String: thumbSigned.URL, Valid: true},
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
		OrganizationID: organizationID,
		ID:             productID,
		PrimaryImageID: pgtype.UUID{Bytes: img.ID, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.bumpCache(ctx, organizationID)
	pid := productID.String()
	mid := mediaID.String()
	s.auditRecord(ctx, organizationID, compliance.ActionMediaBoundToProduct, "catalog.product", &pid, map[string]any{"product_id": pid, "media_id": mid})
	s.auditRecord(ctx, organizationID, compliance.ActionMediaBound, "catalog.product", &pid, map[string]any{"product_id": pid, "media_id": mid})
	return &prod, nil
}

// UnbindProductMedia removes a product_image row bound to mediaID for the product.
func (s *Service) UnbindProductMedia(ctx context.Context, organizationID, productID, mediaID uuid.UUID) (*db.Product, error) {
	if s == nil {
		return nil, ErrNotConfigured
	}
	q := db.New(s.pool)
	imgID, err := q.MediaAdminFindProductImageBinding(ctx, db.MediaAdminFindProductImageBindingParams{
		OrganizationID: organizationID,
		ProductID:      productID,
		MediaAssetID:   pgtype.UUID{Bytes: mediaID, Valid: true},
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
	pimg, err := qtx.CatalogAdminGetPrimaryProductImageForOrg(ctx, db.CatalogAdminGetPrimaryProductImageForOrgParams{
		OrganizationID: organizationID,
		ID:             productID,
	})
	clearPrimary := err == nil && pimg.ID == imgID
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	if clearPrimary {
		if _, err := qtx.CatalogWriteClearProductPrimaryImage(ctx, db.CatalogWriteClearProductPrimaryImageParams{
			OrganizationID: organizationID,
			ID:             productID,
		}); err != nil {
			return nil, err
		}
	}
	if _, err := qtx.CatalogWriteArchiveProductImage(ctx, db.CatalogWriteArchiveProductImageParams{
		OrganizationID: organizationID,
		ID:             productID,
		ID_2:           imgID,
	}); err != nil {
		return nil, err
	}
	prod, err := qtx.CatalogAdminGetProduct(ctx, db.CatalogAdminGetProductParams{
		OrganizationID: organizationID,
		ID:             productID,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	s.bumpCache(ctx, organizationID)
	pid := productID.String()
	mid := mediaID.String()
	s.auditRecord(ctx, organizationID, compliance.ActionMediaUnboundFromProduct, "catalog.product", &pid, map[string]any{"product_id": pid, "media_id": mid})
	s.auditRecord(ctx, organizationID, compliance.ActionMediaUnbound, "catalog.product", &pid, map[string]any{"product_id": pid, "media_id": mid})
	return &prod, nil
}
