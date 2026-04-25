package catalogadmin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
	OrganizationID  uuid.UUID
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
	if in.OrganizationID == uuid.Nil {
		return db.Product{}, ErrOrganizationRequired
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
		OrganizationID:  in.OrganizationID,
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
	return row, nil
}

// UpdateProductInput replaces mutable product fields.
type UpdateProductInput struct {
	OrganizationID  uuid.UUID
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
	if in.OrganizationID == uuid.Nil || in.ProductID == uuid.Nil {
		return db.Product{}, ErrOrganizationRequired
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
	row, err := s.q.CatalogWriteUpdateProduct(ctx, db.CatalogWriteUpdateProductParams{
		OrganizationID:  in.OrganizationID,
		ID:              in.ProductID,
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
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Product{}, ErrNotFound
		}
		return db.Product{}, err
	}
	return row, nil
}

// DeactivateProduct sets active=false (never hard-deletes; safe when referenced).
func (s *Service) DeactivateProduct(ctx context.Context, organizationID, productID uuid.UUID) (db.Product, error) {
	if s == nil {
		return db.Product{}, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || productID == uuid.Nil {
		return db.Product{}, ErrOrganizationRequired
	}
	row, err := s.q.CatalogWriteSetProductActive(ctx, db.CatalogWriteSetProductActiveParams{
		OrganizationID: organizationID,
		ID:             productID,
		Active:         false,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Product{}, ErrNotFound
		}
		return db.Product{}, err
	}
	return row, nil
}

// CreateBrandInput creates a brand.
type CreateBrandInput struct {
	OrganizationID uuid.UUID
	Slug           string
	Name           string
	Active         bool
}

// CreateBrand inserts a brand.
func (s *Service) CreateBrand(ctx context.Context, in CreateBrandInput) (db.Brand, error) {
	if s == nil {
		return db.Brand{}, errors.New("catalogadmin: nil service")
	}
	if in.OrganizationID == uuid.Nil {
		return db.Brand{}, ErrOrganizationRequired
	}
	slug := strings.TrimSpace(in.Slug)
	name := strings.TrimSpace(in.Name)
	if slug == "" || name == "" {
		return db.Brand{}, fmt.Errorf("%w: slug and name required", ErrInvalidArgument)
	}
	row, err := s.q.CatalogWriteInsertBrand(ctx, db.CatalogWriteInsertBrandParams{
		OrganizationID: in.OrganizationID,
		Slug:           slug,
		Name:           name,
		Active:         in.Active,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.Brand{}, ErrDuplicateSlug
		}
		return db.Brand{}, err
	}
	return row, nil
}

// UpdateBrandInput updates brand fields.
type UpdateBrandInput struct {
	OrganizationID uuid.UUID
	BrandID        uuid.UUID
	Slug           string
	Name           string
	Active         bool
}

// UpdateBrand updates a brand.
func (s *Service) UpdateBrand(ctx context.Context, in UpdateBrandInput) (db.Brand, error) {
	if s == nil {
		return db.Brand{}, errors.New("catalogadmin: nil service")
	}
	if in.OrganizationID == uuid.Nil || in.BrandID == uuid.Nil {
		return db.Brand{}, ErrOrganizationRequired
	}
	slug := strings.TrimSpace(in.Slug)
	name := strings.TrimSpace(in.Name)
	if slug == "" || name == "" {
		return db.Brand{}, fmt.Errorf("%w: slug and name required", ErrInvalidArgument)
	}
	row, err := s.q.CatalogWriteUpdateBrand(ctx, db.CatalogWriteUpdateBrandParams{
		OrganizationID: in.OrganizationID,
		ID:             in.BrandID,
		Slug:           slug,
		Name:           name,
		Active:         in.Active,
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
	return row, nil
}

// DeactivateBrand sets active=false.
func (s *Service) DeactivateBrand(ctx context.Context, organizationID, brandID uuid.UUID) (db.Brand, error) {
	if s == nil {
		return db.Brand{}, errors.New("catalogadmin: nil service")
	}
	b, err := s.q.CatalogAdminGetBrand(ctx, db.CatalogAdminGetBrandParams{
		OrganizationID: organizationID,
		ID:             brandID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Brand{}, ErrNotFound
		}
		return db.Brand{}, err
	}
	return s.UpdateBrand(ctx, UpdateBrandInput{
		OrganizationID: organizationID,
		BrandID:        brandID,
		Slug:           b.Slug,
		Name:           b.Name,
		Active:         false,
	})
}

// CreateCategoryInput creates a category.
type CreateCategoryInput struct {
	OrganizationID uuid.UUID
	Slug           string
	Name           string
	ParentID       *uuid.UUID
	Active         bool
}

// CreateCategory inserts a category.
func (s *Service) CreateCategory(ctx context.Context, in CreateCategoryInput) (db.Category, error) {
	if s == nil {
		return db.Category{}, errors.New("catalogadmin: nil service")
	}
	if in.OrganizationID == uuid.Nil {
		return db.Category{}, ErrOrganizationRequired
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
		OrganizationID: in.OrganizationID,
		Slug:           slug,
		Name:           name,
		ParentID:       parent,
		Active:         in.Active,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.Category{}, ErrDuplicateSlug
		}
		return db.Category{}, err
	}
	return row, nil
}

// UpdateCategoryInput updates category fields.
type UpdateCategoryInput struct {
	OrganizationID uuid.UUID
	CategoryID     uuid.UUID
	Slug           string
	Name           string
	ParentID       *uuid.UUID
	Active         bool
}

// UpdateCategory updates a category.
func (s *Service) UpdateCategory(ctx context.Context, in UpdateCategoryInput) (db.Category, error) {
	if s == nil {
		return db.Category{}, errors.New("catalogadmin: nil service")
	}
	if in.OrganizationID == uuid.Nil || in.CategoryID == uuid.Nil {
		return db.Category{}, ErrOrganizationRequired
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
	row, err := s.q.CatalogWriteUpdateCategory(ctx, db.CatalogWriteUpdateCategoryParams{
		OrganizationID: in.OrganizationID,
		ID:             in.CategoryID,
		Slug:           slug,
		Name:           name,
		ParentID:       parent,
		Active:         in.Active,
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
	return row, nil
}

// DeactivateCategory sets active=false.
func (s *Service) DeactivateCategory(ctx context.Context, organizationID, categoryID uuid.UUID) (db.Category, error) {
	if s == nil {
		return db.Category{}, errors.New("catalogadmin: nil service")
	}
	c, err := s.q.CatalogAdminGetCategory(ctx, db.CatalogAdminGetCategoryParams{
		OrganizationID: organizationID,
		ID:             categoryID,
	})
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
		OrganizationID: organizationID,
		CategoryID:     categoryID,
		Slug:           c.Slug,
		Name:           c.Name,
		ParentID:       parent,
		Active:         false,
	})
}

// CreateTagInput creates a tag.
type CreateTagInput struct {
	OrganizationID uuid.UUID
	Slug           string
	Name           string
	Active         bool
}

// CreateTag inserts a tag.
func (s *Service) CreateTag(ctx context.Context, in CreateTagInput) (db.Tag, error) {
	if s == nil {
		return db.Tag{}, errors.New("catalogadmin: nil service")
	}
	if in.OrganizationID == uuid.Nil {
		return db.Tag{}, ErrOrganizationRequired
	}
	slug := strings.TrimSpace(in.Slug)
	name := strings.TrimSpace(in.Name)
	if slug == "" || name == "" {
		return db.Tag{}, fmt.Errorf("%w: slug and name required", ErrInvalidArgument)
	}
	row, err := s.q.CatalogWriteInsertTag(ctx, db.CatalogWriteInsertTagParams{
		OrganizationID: in.OrganizationID,
		Slug:           slug,
		Name:           name,
		Active:         in.Active,
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
	OrganizationID uuid.UUID
	TagID          uuid.UUID
	Slug           string
	Name           string
	Active         bool
}

// UpdateTag updates a tag.
func (s *Service) UpdateTag(ctx context.Context, in UpdateTagInput) (db.Tag, error) {
	if s == nil {
		return db.Tag{}, errors.New("catalogadmin: nil service")
	}
	if in.OrganizationID == uuid.Nil || in.TagID == uuid.Nil {
		return db.Tag{}, ErrOrganizationRequired
	}
	slug := strings.TrimSpace(in.Slug)
	name := strings.TrimSpace(in.Name)
	if slug == "" || name == "" {
		return db.Tag{}, fmt.Errorf("%w: slug and name required", ErrInvalidArgument)
	}
	row, err := s.q.CatalogWriteUpdateTag(ctx, db.CatalogWriteUpdateTagParams{
		OrganizationID: in.OrganizationID,
		ID:             in.TagID,
		Slug:           slug,
		Name:           name,
		Active:         in.Active,
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
	return row, nil
}

// DeactivateTag sets active=false.
func (s *Service) DeactivateTag(ctx context.Context, organizationID, tagID uuid.UUID) (db.Tag, error) {
	if s == nil {
		return db.Tag{}, errors.New("catalogadmin: nil service")
	}
	tg, err := s.q.CatalogAdminGetTag(ctx, db.CatalogAdminGetTagParams{
		OrganizationID: organizationID,
		ID:             tagID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Tag{}, ErrNotFound
		}
		return db.Tag{}, err
	}
	return s.UpdateTag(ctx, UpdateTagInput{
		OrganizationID: organizationID,
		TagID:          tagID,
		Slug:           tg.Slug,
		Name:           tg.Name,
		Active:         false,
	})
}

// BindProductImageInput sets the primary product image (storage key derived from artifact id).
type BindProductImageInput struct {
	OrganizationID uuid.UUID
	ProductID      uuid.UUID
	ArtifactID     uuid.UUID
	ThumbURL       string
	DisplayURL     string
	ContentHash    string
	Width          int32
	Height         int32
	MimeType       string
}

// BindProductPrimaryImage replaces the primary image for a product.
func (s *Service) BindProductPrimaryImage(ctx context.Context, in BindProductImageInput) (db.Product, error) {
	if s == nil {
		return db.Product{}, errors.New("catalogadmin: nil service")
	}
	if in.OrganizationID == uuid.Nil || in.ProductID == uuid.Nil || in.ArtifactID == uuid.Nil {
		return db.Product{}, ErrOrganizationRequired
	}
	display := strings.TrimSpace(in.DisplayURL)
	thumb := strings.TrimSpace(in.ThumbURL)
	if display == "" {
		return db.Product{}, fmt.Errorf("%w: displayUrl required", ErrInvalidArgument)
	}
	if thumb == "" {
		thumb = display
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.Product{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)

	if _, err := qtx.CatalogAdminGetProduct(ctx, db.CatalogAdminGetProductParams{
		OrganizationID: in.OrganizationID,
		ID:             in.ProductID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Product{}, ErrNotFound
		}
		return db.Product{}, err
	}

	if err := qtx.CatalogWriteUnsetPrimaryImagesForProduct(ctx, db.CatalogWriteUnsetPrimaryImagesForProductParams{
		OrganizationID: in.OrganizationID,
		ID:             in.ProductID,
	}); err != nil {
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
	var mime pgtype.Text
	if mt := strings.TrimSpace(in.MimeType); mt != "" {
		mime = pgtype.Text{String: mt, Valid: true}
	}
	var thumbPg pgtype.Text
	if thumb != "" {
		thumbPg = pgtype.Text{String: thumb, Valid: true}
	}
	displayPg := pgtype.Text{String: display, Valid: true}

	img, err := qtx.CatalogWriteInsertProductImage(ctx, db.CatalogWriteInsertProductImageParams{
		ProductID:   in.ProductID,
		StorageKey:  "artifact:" + in.ArtifactID.String(),
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
		OrganizationID: in.OrganizationID,
		ID:             in.ProductID,
		PrimaryImageID: pgtype.UUID{Bytes: img.ID, Valid: true},
	})
	if err != nil {
		return db.Product{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.Product{}, err
	}
	return prod, nil
}

// ClearProductPrimaryImage removes the bound primary image row and clears the FK.
func (s *Service) ClearProductPrimaryImage(ctx context.Context, organizationID, productID uuid.UUID) (db.Product, error) {
	if s == nil {
		return db.Product{}, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || productID == uuid.Nil {
		return db.Product{}, ErrOrganizationRequired
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return db.Product{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := db.New(tx)

	p, err := qtx.CatalogAdminGetProduct(ctx, db.CatalogAdminGetProductParams{
		OrganizationID: organizationID,
		ID:             productID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return db.Product{}, ErrNotFound
		}
		return db.Product{}, err
	}
	if !p.PrimaryImageID.Valid {
		if err := tx.Commit(ctx); err != nil {
			return db.Product{}, err
		}
		row, gerr := s.q.CatalogAdminGetProduct(ctx, db.CatalogAdminGetProductParams{
			OrganizationID: organizationID,
			ID:             productID,
		})
		if gerr != nil {
			return db.Product{}, gerr
		}
		return row, nil
	}
	imgID := uuid.UUID(p.PrimaryImageID.Bytes)
	if err := qtx.CatalogWriteDeleteProductImage(ctx, db.CatalogWriteDeleteProductImageParams{
		OrganizationID: organizationID,
		ID:             productID,
		ID_2:           imgID,
	}); err != nil {
		return db.Product{}, err
	}
	prod, err := qtx.CatalogWriteClearProductPrimaryImage(ctx, db.CatalogWriteClearProductPrimaryImageParams{
		OrganizationID: organizationID,
		ID:             productID,
	})
	if err != nil {
		return db.Product{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.Product{}, err
	}
	return prod, nil
}
