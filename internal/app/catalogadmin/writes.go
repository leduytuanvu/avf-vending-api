package catalogadmin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func productUniqueViolationKind(err error, hasBarcode bool) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.ConstraintName {
		case "ux_products_org_barcode_lower":
			return ErrDuplicateBarcode
		case "ux_products_org_sku":
			return ErrDuplicateSKU
		}
	}
	if hasBarcode {
		return ErrDuplicateBarcode
	}
	return ErrDuplicateSKU
}

// CreateProductInput inserts a new product row.
type CreateProductInput struct {
	Sku             string
	Barcode         *string
	Name            string
	Description     string
	Attrs           json.RawMessage
	Active          bool
	CategoryID      *uuid.UUID
	BrandID         *uuid.UUID
	CountryOfOrigin *string
	AgeRestricted   bool
	AllergenCodes   []string
	NutritionalNote *string
}

// CreateProduct creates a product.
func (s *Service) CreateProduct(ctx context.Context, in CreateProductInput) (db.Product, error) {
	if s == nil {
		return db.Product{}, errors.New("catalogadmin: nil service")
	}
	sku := strings.TrimSpace(in.Sku)
	name := strings.TrimSpace(in.Name)
	if sku == "" || name == "" {
		return db.Product{}, fmt.Errorf("%w: sku and name required", ErrInvalidArgument)
	}
	var barcode pgtype.Text
	if in.Barcode != nil {
		b := strings.TrimSpace(*in.Barcode)
		if b != "" {
			barcode = pgtype.Text{String: b, Valid: true}
		}
	}
	attrs := []byte(`{}`)
	if len(in.Attrs) > 0 && json.Valid(in.Attrs) {
		attrs = in.Attrs
	}
	var cat pgtype.UUID
	if in.CategoryID != nil && *in.CategoryID != uuid.Nil {
		cat = pgtype.UUID{Bytes: *in.CategoryID, Valid: true}
	}
	var brand pgtype.UUID
	if in.BrandID != nil && *in.BrandID != uuid.Nil {
		brand = pgtype.UUID{Bytes: *in.BrandID, Valid: true}
	}
	var coo pgtype.Text
	if in.CountryOfOrigin != nil {
		c := strings.TrimSpace(*in.CountryOfOrigin)
		if c != "" {
			coo = pgtype.Text{String: c, Valid: true}
		}
	}
	var nut pgtype.Text
	if in.NutritionalNote != nil {
		n := strings.TrimSpace(*in.NutritionalNote)
		if n != "" {
			nut = pgtype.Text{String: n, Valid: true}
		}
	}
	allergen := in.AllergenCodes
	if allergen == nil {
		allergen = []string{}
	}
	row, err := s.q.CatalogWriteInsertProduct(ctx, db.CatalogWriteInsertProductParams{
		Sku:             sku,
		Barcode:         barcode,
		Name:            name,
		Description:     strings.TrimSpace(in.Description),
		Attrs:           attrs,
		Active:          in.Active,
		CategoryID:      cat,
		BrandID:         brand,
		CountryOfOrigin: coo,
		AgeRestricted:   in.AgeRestricted,
		AllergenCodes:   allergen,
		NutritionalNote: nut,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.Product{}, productUniqueViolationKind(err, barcode.Valid)
		}
		return db.Product{}, err
	}
	s.recordCatalogWriteAudit(ctx, uuid.Nil, compliance.ActionProductCreated, "catalog.product", row.ID, productAuditSnapshot(row))
	s.bumpCatalogCache(ctx, uuid.Nil)
	return row, nil
}

// UpdateProductInput replaces mutable product fields.
type UpdateProductInput struct {
	ProductID       uuid.UUID
	Sku             string
	Barcode         *string
	Name            string
	Description     string
	Attrs           json.RawMessage
	Active          bool
	CategoryID      *uuid.UUID
	BrandID         *uuid.UUID
	CountryOfOrigin *string
	AgeRestricted   bool
	AllergenCodes   []string
	NutritionalNote *string
}

// UpdateProduct updates a product.
func (s *Service) UpdateProduct(ctx context.Context, in UpdateProductInput) (db.Product, error) {
	if s == nil {
		return db.Product{}, errors.New("catalogadmin: nil service")
	}
	if in.ProductID == uuid.Nil {
		return db.Product{}, ErrCompanyRequired
	}
	sku := strings.TrimSpace(in.Sku)
	name := strings.TrimSpace(in.Name)
	if sku == "" || name == "" {
		return db.Product{}, fmt.Errorf("%w: sku and name required", ErrInvalidArgument)
	}
	var barcode pgtype.Text
	if in.Barcode != nil {
		b := strings.TrimSpace(*in.Barcode)
		if b != "" {
			barcode = pgtype.Text{String: b, Valid: true}
		}
	}
	attrs := []byte(`{}`)
	if len(in.Attrs) > 0 && json.Valid(in.Attrs) {
		attrs = in.Attrs
	}
	var cat pgtype.UUID
	if in.CategoryID != nil && *in.CategoryID != uuid.Nil {
		cat = pgtype.UUID{Bytes: *in.CategoryID, Valid: true}
	}
	var brand pgtype.UUID
	if in.BrandID != nil && *in.BrandID != uuid.Nil {
		brand = pgtype.UUID{Bytes: *in.BrandID, Valid: true}
	}
	var coo pgtype.Text
	if in.CountryOfOrigin != nil {
		c := strings.TrimSpace(*in.CountryOfOrigin)
		if c != "" {
			coo = pgtype.Text{String: c, Valid: true}
		}
	}
	var nut pgtype.Text
	if in.NutritionalNote != nil {
		n := strings.TrimSpace(*in.NutritionalNote)
		if n != "" {
			nut = pgtype.Text{String: n, Valid: true}
		}
	}
	allergen := in.AllergenCodes
	if allergen == nil {
		allergen = []string{}
	}
	row, err := s.q.CatalogWriteUpdateProduct(ctx, db.CatalogWriteUpdateProductParams{Sku: sku,
		Barcode:         barcode,
		Name:            name,
		Description:     strings.TrimSpace(in.Description),
		Attrs:           attrs,
		Active:          in.Active,
		CategoryID:      cat,
		BrandID:         brand,
		CountryOfOrigin: coo,
		AgeRestricted:   in.AgeRestricted,
		AllergenCodes:   allergen,
		NutritionalNote: nut,
		ID:              in.ProductID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.Product{}, productUniqueViolationKind(err, barcode.Valid)
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Product{}, ErrNotFound
		}
		return db.Product{}, err
	}
	s.recordCatalogWriteAudit(ctx, uuid.Nil, compliance.ActionProductUpdated, "catalog.product", row.ID, productAuditSnapshot(row))
	s.bumpCatalogCache(ctx, uuid.Nil)
	return row, nil
}

// DeactivateProduct sets active=false (never hard-deletes; safe when referenced).
func (s *Service) DeactivateProduct(ctx context.Context, companyID, productID uuid.UUID) (db.Product, error) {
	if s == nil {
		return db.Product{}, errors.New("catalogadmin: nil service")
	}
	if productID == uuid.Nil {
		return db.Product{}, ErrCompanyRequired
	}
	row, err := s.q.CatalogWriteSetProductActive(ctx, db.CatalogWriteSetProductActiveParams{Active: false, ID: productID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Product{}, ErrNotFound
		}
		return db.Product{}, err
	}
	s.recordCatalogWriteAudit(ctx, companyID, compliance.ActionProductDeactivated, "catalog.product", row.ID, productAuditSnapshot(row))
	s.bumpCatalogCache(ctx, companyID)
	return row, nil
}

// CreateBrandInput creates a brand.
type CreateBrandInput struct {
	Slug   string
	Name   string
	Active bool
}

// CreateBrand inserts a brand.
func (s *Service) CreateBrand(ctx context.Context, in CreateBrandInput) (db.Brand, error) {
	if s == nil {
		return db.Brand{}, errors.New("catalogadmin: nil service")
	}
	slug := strings.TrimSpace(in.Slug)
	name := strings.TrimSpace(in.Name)
	if slug == "" || name == "" {
		return db.Brand{}, fmt.Errorf("%w: slug and name required", ErrInvalidArgument)
	}
	row, err := s.q.CatalogWriteInsertBrand(ctx, db.CatalogWriteInsertBrandParams{
		Slug:   slug,
		Name:   name,
		Active: in.Active,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.Brand{}, ErrDuplicateSlug
		}
		return db.Brand{}, err
	}
	s.recordCatalogWriteAudit(ctx, uuid.Nil, compliance.ActionBrandCreated, "catalog.brand", row.ID, brandAuditSnapshot(row))
	return row, nil
}

// UpdateBrandInput updates brand fields.
type UpdateBrandInput struct {
	BrandID uuid.UUID
	Slug    string
	Name    string
	Active  bool
}

// UpdateBrand updates a brand.
func (s *Service) UpdateBrand(ctx context.Context, in UpdateBrandInput) (db.Brand, error) {
	if s == nil {
		return db.Brand{}, errors.New("catalogadmin: nil service")
	}
	if in.BrandID == uuid.Nil {
		return db.Brand{}, ErrCompanyRequired
	}
	slug := strings.TrimSpace(in.Slug)
	name := strings.TrimSpace(in.Name)
	if slug == "" || name == "" {
		return db.Brand{}, fmt.Errorf("%w: slug and name required", ErrInvalidArgument)
	}
	row, err := s.q.CatalogWriteUpdateBrand(ctx, db.CatalogWriteUpdateBrandParams{Slug: slug,
		Name:   name,
		Active: in.Active,
		ID:     in.BrandID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.Brand{}, ErrDuplicateSlug
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Brand{}, ErrNotFound
		}
		return db.Brand{}, err
	}
	s.recordCatalogWriteAudit(ctx, uuid.Nil, compliance.ActionBrandUpdated, "catalog.brand", row.ID, brandAuditSnapshot(row))
	return row, nil
}

// DeactivateBrand sets active=false.
func (s *Service) DeactivateBrand(ctx context.Context, companyID, brandID uuid.UUID) (db.Brand, error) {
	if s == nil {
		return db.Brand{}, errors.New("catalogadmin: nil service")
	}
	b, err := s.q.CatalogAdminGetBrand(ctx, brandID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Brand{}, ErrNotFound
		}
		return db.Brand{}, err
	}
	return s.UpdateBrand(ctx, UpdateBrandInput{
		BrandID: brandID,
		Slug:    b.Slug,
		Name:    b.Name,
		Active:  false,
	})
}

// CreateCategoryInput creates a category.
type CreateCategoryInput struct {
	Slug     string
	Name     string
	ParentID *uuid.UUID
	Active   bool
}

// CreateCategory inserts a category.
func (s *Service) CreateCategory(ctx context.Context, in CreateCategoryInput) (db.Category, error) {
	if s == nil {
		return db.Category{}, errors.New("catalogadmin: nil service")
	}
	slug := strings.TrimSpace(in.Slug)
	name := strings.TrimSpace(in.Name)
	if slug == "" || name == "" {
		return db.Category{}, fmt.Errorf("%w: slug and name required", ErrInvalidArgument)
	}
	var parent pgtype.UUID
	if in.ParentID != nil && *in.ParentID != uuid.Nil {
		parent = pgtype.UUID{Bytes: *in.ParentID, Valid: true}
	}
	row, err := s.q.CatalogWriteInsertCategory(ctx, db.CatalogWriteInsertCategoryParams{
		Slug:     slug,
		Name:     name,
		ParentID: parent,
		Active:   in.Active,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.Category{}, ErrDuplicateSlug
		}
		return db.Category{}, err
	}
	s.recordCatalogWriteAudit(ctx, uuid.Nil, compliance.ActionCategoryCreated, "catalog.category", row.ID, categoryAuditSnapshot(row))
	return row, nil
}

// UpdateCategoryInput updates category fields.
type UpdateCategoryInput struct {
	CategoryID uuid.UUID
	Slug       string
	Name       string
	ParentID   *uuid.UUID
	Active     bool
}

// UpdateCategory updates a category.
func (s *Service) UpdateCategory(ctx context.Context, in UpdateCategoryInput) (db.Category, error) {
	if s == nil {
		return db.Category{}, errors.New("catalogadmin: nil service")
	}
	if in.CategoryID == uuid.Nil {
		return db.Category{}, ErrCompanyRequired
	}
	slug := strings.TrimSpace(in.Slug)
	name := strings.TrimSpace(in.Name)
	if slug == "" || name == "" {
		return db.Category{}, fmt.Errorf("%w: slug and name required", ErrInvalidArgument)
	}
	var parent pgtype.UUID
	if in.ParentID != nil && *in.ParentID != uuid.Nil {
		parent = pgtype.UUID{Bytes: *in.ParentID, Valid: true}
	}
	row, err := s.q.CatalogWriteUpdateCategory(ctx, db.CatalogWriteUpdateCategoryParams{Slug: slug,
		Name:     name,
		ParentID: parent,
		Active:   in.Active,
		ID:       in.CategoryID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.Category{}, ErrDuplicateSlug
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Category{}, ErrNotFound
		}
		return db.Category{}, err
	}
	s.recordCatalogWriteAudit(ctx, uuid.Nil, compliance.ActionCategoryUpdated, "catalog.category", row.ID, categoryAuditSnapshot(row))
	return row, nil
}

// DeactivateCategory sets active=false.
func (s *Service) DeactivateCategory(ctx context.Context, companyID, categoryID uuid.UUID) (db.Category, error) {
	if s == nil {
		return db.Category{}, errors.New("catalogadmin: nil service")
	}
	c, err := s.q.CatalogAdminGetCategory(ctx, categoryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Category{}, ErrNotFound
		}
		return db.Category{}, err
	}
	var parent *uuid.UUID
	if c.ParentID.Valid {
		pid := uuid.UUID(c.ParentID.Bytes)
		parent = &pid
	}
	return s.UpdateCategory(ctx, UpdateCategoryInput{
		CategoryID: categoryID,
		Slug:       c.Slug,
		Name:       c.Name,
		ParentID:   parent,
		Active:     false,
	})
}

// CreateTagInput creates a tag.
type CreateTagInput struct {
	Slug   string
	Name   string
	Active bool
}

// CreateTag inserts a tag.
func (s *Service) CreateTag(ctx context.Context, in CreateTagInput) (db.Tag, error) {
	if s == nil {
		return db.Tag{}, errors.New("catalogadmin: nil service")
	}
	slug := strings.TrimSpace(in.Slug)
	name := strings.TrimSpace(in.Name)
	if slug == "" || name == "" {
		return db.Tag{}, fmt.Errorf("%w: slug and name required", ErrInvalidArgument)
	}
	row, err := s.q.CatalogWriteInsertTag(ctx, db.CatalogWriteInsertTagParams{
		Slug:   slug,
		Name:   name,
		Active: in.Active,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.Tag{}, ErrDuplicateSlug
		}
		return db.Tag{}, err
	}
	return row, nil
}

// UpdateTagInput updates tag fields.
type UpdateTagInput struct {
	TagID  uuid.UUID
	Slug   string
	Name   string
	Active bool
}

// UpdateTag updates a tag.
func (s *Service) UpdateTag(ctx context.Context, in UpdateTagInput) (db.Tag, error) {
	if s == nil {
		return db.Tag{}, errors.New("catalogadmin: nil service")
	}
	if in.TagID == uuid.Nil {
		return db.Tag{}, ErrCompanyRequired
	}
	slug := strings.TrimSpace(in.Slug)
	name := strings.TrimSpace(in.Name)
	if slug == "" || name == "" {
		return db.Tag{}, fmt.Errorf("%w: slug and name required", ErrInvalidArgument)
	}
	row, err := s.q.CatalogWriteUpdateTag(ctx, db.CatalogWriteUpdateTagParams{Slug: slug,
		Name:   name,
		Active: in.Active,
		ID:     in.TagID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.Tag{}, ErrDuplicateSlug
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Tag{}, ErrNotFound
		}
		return db.Tag{}, err
	}
	s.recordCatalogWriteAudit(ctx, uuid.Nil, compliance.ActionTagUpdated, "catalog.tag", row.ID, tagAuditSnapshot(row))
	return row, nil
}

// DeactivateTag sets active=false.
func (s *Service) DeactivateTag(ctx context.Context, companyID, tagID uuid.UUID) (db.Tag, error) {
	if s == nil {
		return db.Tag{}, errors.New("catalogadmin: nil service")
	}
	tg, err := s.q.CatalogAdminGetTag(ctx, tagID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Tag{}, ErrNotFound
		}
		return db.Tag{}, err
	}
	return s.UpdateTag(ctx, UpdateTagInput{
		TagID:  tagID,
		Slug:   tg.Slug,
		Name:   tg.Name,
		Active: false,
	})
}

// BindProductImageInput sets the primary product image (storage key derived from artifact id).
type BindProductImageInput struct {
	ProductID   uuid.UUID
	ArtifactID  uuid.UUID
	ThumbURL    string
	DisplayURL  string
	ContentHash string
	Width       int32
	Height      int32
	MimeType    string
}

// UpdateProductImageInput patches product-image presentation metadata.
type UpdateProductImageInput struct {
	ProductID uuid.UUID
	ImageID   uuid.UUID
	SortOrder *int32
	IsPrimary *bool
	AltText   *string
}

// ListProductImages returns active product images unless includeArchived is true.
func (s *Service) ListProductImages(ctx context.Context, companyID, productID uuid.UUID, includeArchived bool) ([]db.ProductImage, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	if productID == uuid.Nil {
		return nil, ErrCompanyRequired
	}
	return s.q.CatalogAdminListProductImagesForOrg(ctx, db.CatalogAdminListProductImagesForOrgParams{Column2: includeArchived,

		ID: productID,
	})
}

// ListProductMediumRowsForProduct returns projection rows for object-storage media (parallel to product_images ids).
func (s *Service) ListProductMediumRowsForProduct(ctx context.Context, companyID, productID uuid.UUID) ([]db.ProductMedium, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	if productID == uuid.Nil {
		return nil, ErrCompanyRequired
	}
	return s.q.CatalogAdminListProductMediumRowsForProduct(ctx, productID)
}

// GetProductMediumForOrgProductImage returns the product_media row for an image id when present.
func (s *Service) GetProductMediumForOrgProductImage(ctx context.Context, companyID, productID, imageID uuid.UUID) (db.ProductMedium, error) {
	if s == nil {
		return db.ProductMedium{}, errors.New("catalogadmin: nil service")
	}
	if productID == uuid.Nil || imageID == uuid.Nil {
		return db.ProductMedium{}, ErrCompanyRequired
	}
	return s.q.CatalogAdminGetProductMediumForOrgProductImage(ctx, db.CatalogAdminGetProductMediumForOrgProductImageParams{
		ProductID: productID,
		ID:        imageID,
	})
}

// UpdateProductImage patches sort/primary/alt metadata. Setting primary clears other active primaries first.
func (s *Service) UpdateProductImage(ctx context.Context, in UpdateProductImageInput) (db.ProductImage, error) {
	if s == nil {
		return db.ProductImage{}, errors.New("catalogadmin: nil service")
	}
	if in.ProductID == uuid.Nil || in.ImageID == uuid.Nil {
		return db.ProductImage{}, ErrCompanyRequired
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.ProductImage{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)
	if in.IsPrimary != nil && *in.IsPrimary {
		if _, err := qtx.CatalogWriteClearProductPrimaryImage(ctx, in.ProductID); err != nil {
			return db.ProductImage{}, err
		}
	}
	var sort pgtype.Int4
	if in.SortOrder != nil {
		sort = pgtype.Int4{Int32: *in.SortOrder, Valid: true}
	}
	var primary pgtype.Bool
	if in.IsPrimary != nil {
		primary = pgtype.Bool{Bool: *in.IsPrimary, Valid: true}
	}
	var alt pgtype.Text
	if in.AltText != nil {
		alt = pgtype.Text{String: strings.TrimSpace(*in.AltText), Valid: true}
	}
	img, err := qtx.CatalogWriteUpdateProductImageMetadata(ctx, db.CatalogWriteUpdateProductImageMetadataParams{
		ID:        in.ProductID,
		ID_2:      in.ImageID,
		SortOrder: sort,
		IsPrimary: primary,
		AltText:   alt,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ProductImage{}, ErrNotFound
		}
		return db.ProductImage{}, err
	}
	if in.IsPrimary != nil && *in.IsPrimary {
		if _, err := qtx.CatalogWriteSetProductPrimaryImage(ctx, db.CatalogWriteSetProductPrimaryImageParams{
			PrimaryImageID: pgtype.UUID{Bytes: img.ID, Valid: true},
			ID:             in.ProductID,
		}); err != nil {
			return db.ProductImage{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return db.ProductImage{}, err
	}
	s.recordCatalogWriteAudit(ctx, uuid.Nil, compliance.ActionProductUpdated, "catalog.product", in.ProductID, map[string]any{
		"productImageId": img.ID.String(),
		"mediaVersion":   img.MediaVersion,
	})
	s.bumpCatalogCache(ctx, uuid.Nil)
	return img, nil
}

// ArchiveProductImage hides an image from admin active lists and runtime catalogs.
func (s *Service) ArchiveProductImage(ctx context.Context, companyID, productID, imageID uuid.UUID) (db.ProductImage, error) {
	if s == nil {
		return db.ProductImage{}, errors.New("catalogadmin: nil service")
	}
	if productID == uuid.Nil || imageID == uuid.Nil {
		return db.ProductImage{}, ErrCompanyRequired
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.ProductImage{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)
	cur, err := qtx.CatalogAdminGetProductImageForOrg(ctx, db.CatalogAdminGetProductImageForOrgParams{
		ID:   productID,
		ID_2: imageID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ProductImage{}, ErrNotFound
		}
		return db.ProductImage{}, err
	}
	if cur.IsPrimary {
		if _, err := qtx.CatalogWriteClearProductPrimaryImage(ctx, productID); err != nil {
			return db.ProductImage{}, err
		}
	}
	img, err := qtx.CatalogWriteArchiveProductImage(ctx, db.CatalogWriteArchiveProductImageParams{
		ID:   productID,
		ID_2: imageID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.ProductImage{}, ErrNotFound
		}
		return db.ProductImage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.ProductImage{}, err
	}
	s.recordCatalogWriteAudit(ctx, companyID, compliance.ActionProductUpdated, "catalog.product", productID, map[string]any{
		"productImageId": img.ID.String(),
		"imageArchived":  true,
		"mediaVersion":   img.MediaVersion,
	})
	s.bumpCatalogCache(ctx, companyID)
	return img, nil
}

// BindProductPrimaryImage replaces the primary image for a product.
func (s *Service) BindProductPrimaryImage(ctx context.Context, in BindProductImageInput) (db.Product, error) {
	if s == nil {
		return db.Product{}, errors.New("catalogadmin: nil service")
	}
	if in.ProductID == uuid.Nil || in.ArtifactID == uuid.Nil {
		return db.Product{}, ErrCompanyRequired
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.Product{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)

	if _, err := qtx.CatalogAdminGetProduct(ctx, in.ProductID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Product{}, ErrNotFound
		}
		return db.Product{}, err
	}

	var storageKey string
	var displayPg, thumbPg pgtype.Text
	var mime pgtype.Text

	if s.media.Store != nil {
		displayURL, thumbURL, dKey, artifactMIME, err := copyArtifactIntoProductDeterministicKeys(ctx, s.media.Store, uuid.Nil, in.ArtifactID, in.ProductID, s.mediaMaxBytes(), s.media.PresignTTL)
		if err != nil {
			return db.Product{}, err
		}
		storageKey = dKey
		displayPg = pgtype.Text{String: displayURL, Valid: true}
		thumbPg = pgtype.Text{String: thumbURL, Valid: true}
		mime = pgtype.Text{String: artifactMIME, Valid: true}
	} else {
		display := strings.TrimSpace(in.DisplayURL)
		thumb := strings.TrimSpace(in.ThumbURL)
		if display == "" {
			return db.Product{}, fmt.Errorf("displayUrl required: %w", ErrInvalidArgument)
		}
		if thumb == "" {
			thumb = display
		}
		if mt := strings.TrimSpace(in.MimeType); mt != "" {
			if err := ValidateProductImageMIME(mt); err != nil {
				return db.Product{}, err
			}
			mime = pgtype.Text{String: normalizeProductImageMIME(mt), Valid: true}
		}
		storageKey = "artifact:" + in.ArtifactID.String()
		displayPg = pgtype.Text{String: display, Valid: true}
		thumbPg = pgtype.Text{String: thumb, Valid: true}
	}

	if _, err := qtx.CatalogWriteClearProductPrimaryImage(ctx, in.ProductID); err != nil {
		return db.Product{}, err
	}
	if err := qtx.CatalogWriteArchiveAllProductImagesForProduct(ctx, in.ProductID); err != nil {
		return db.Product{}, err
	}

	ch := strings.TrimSpace(in.ContentHash)
	var chText pgtype.Text
	if ch != "" {
		chText = pgtype.Text{String: ch, Valid: true}
	}
	var w, h pgtype.Int4
	if in.Width > 0 {
		w = pgtype.Int4{Int32: in.Width, Valid: true}
	}
	if in.Height > 0 {
		h = pgtype.Int4{Int32: in.Height, Valid: true}
	}

	img, err := qtx.CatalogWriteInsertProductImage(ctx, db.CatalogWriteInsertProductImageParams{
		ProductID:   in.ProductID,
		StorageKey:  storageKey,
		CdnUrl:      displayPg,
		ThumbCdnUrl: thumbPg,
		ContentHash: chText,
		Width:       w,
		Height:      h,
		MimeType:    mime,
		AltText:     "",
		SortOrder:   0,
		IsPrimary:   true,
	})
	if err != nil {
		return db.Product{}, err
	}

	prod, err := qtx.CatalogWriteSetProductPrimaryImage(ctx, db.CatalogWriteSetProductPrimaryImageParams{
		PrimaryImageID: pgtype.UUID{Bytes: img.ID, Valid: true},
		ID:             in.ProductID,
	})
	if err != nil {
		return db.Product{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.Product{}, err
	}
	after := productAuditSnapshot(prod)
	after["artifactId"] = in.ArtifactID.String()
	after["primaryImageBound"] = true
	s.recordCatalogWriteAudit(ctx, uuid.Nil, compliance.ActionProductUpdated, "catalog.product", prod.ID, after)
	s.bumpCatalogCache(ctx, uuid.Nil)
	return prod, nil
}

// ClearProductPrimaryImage removes the bound primary image row and clears the FK.
func (s *Service) ClearProductPrimaryImage(ctx context.Context, companyID, productID uuid.UUID) (db.Product, error) {
	if s == nil {
		return db.Product{}, errors.New("catalogadmin: nil service")
	}
	if productID == uuid.Nil {
		return db.Product{}, ErrCompanyRequired
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.Product{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)

	if _, err := qtx.CatalogAdminGetProduct(ctx, productID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Product{}, ErrNotFound
		}
		return db.Product{}, err
	}

	prev, ierr := qtx.CatalogAdminGetPrimaryProductImageForOrg(ctx, productID)
	prevKey := ""
	if ierr == nil {
		prevKey = strings.TrimSpace(prev.StorageKey)
	} else if !errors.Is(ierr, pgx.ErrNoRows) {
		return db.Product{}, ierr
	}

	if _, err := qtx.CatalogWriteClearProductPrimaryImage(ctx, productID); err != nil {
		return db.Product{}, err
	}
	if err := qtx.CatalogWriteArchiveAllProductImagesForProduct(ctx, productID); err != nil {
		return db.Product{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.Product{}, err
	}

	bestEffortDeleteDeterministicProductMedia(ctx, s.media.Store, prevKey, companyID, productID)

	row, gerr := s.q.CatalogAdminGetProduct(ctx, productID)
	if gerr != nil {
		return db.Product{}, gerr
	}
	s.recordCatalogWriteAudit(ctx, companyID, compliance.ActionProductUpdated, "catalog.product", productID,
		map[string]any{"productId": productID.String(), "primaryImageCleared": true})
	s.bumpCatalogCache(ctx, companyID)
	return row, nil
}
