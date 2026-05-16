package catalogadmin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/compliance"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Service provides catalog admin queries and writes for HTTP APIs.
type Service struct {
	q              *db.Queries
	pool           *pgxpool.Pool
	audit          compliance.EnterpriseRecorder
	promotionAudit func(context.Context, PromotionAuditEvent)
	media          ProductMediaDeps
	cache          CatalogCacheInvalidator
}

type CatalogCacheInvalidator interface {
	BumpCompanyMedia(ctx context.Context, companyID uuid.UUID)
}

// NewService constructs a catalog admin service (reads and writes).
// audit may be nil (no enterprise audit rows). Optional ProductMediaDeps enables deterministic object keys (org/.../products/.../display.webp) when Store is non-nil.
func NewService(q *db.Queries, pool *pgxpool.Pool, audit compliance.EnterpriseRecorder, media ...ProductMediaDeps) (*Service, error) {
	if q == nil {
		return nil, fmt.Errorf("catalogadmin: nil Queries")
	}
	if pool == nil {
		return nil, fmt.Errorf("catalogadmin: nil pool")
	}
	var m ProductMediaDeps
	if len(media) > 0 {
		m = media[0]
	}
	if m.MaxUploadBytes <= 0 {
		m.MaxUploadBytes = 10 << 20
	}
	if m.Store != nil && m.PresignTTL <= 0 {
		m.PresignTTL = 15 * time.Minute
	}
	return &Service{q: q, pool: pool, audit: audit, media: m}, nil
}

func (s *Service) SetCatalogCacheInvalidator(cache CatalogCacheInvalidator) {
	if s != nil {
		s.cache = cache
	}
}

func (s *Service) bumpCatalogCache(ctx context.Context, companyID uuid.UUID) {
	if s == nil || s.cache == nil || companyID == uuid.Nil {
		return
	}
	s.cache.BumpCompanyMedia(ctx, companyID)
}

func (s *Service) mediaMaxBytes() int64 {
	if s == nil {
		return 10 << 20
	}
	if s.media.MaxUploadBytes > 0 {
		return s.media.MaxUploadBytes
	}
	return 10 << 20
}

// UsesDeterministicProductMedia is true when object storage copies artifact payloads into org/.../products/... keys (artifacts subsystem configured).
func (s *Service) UsesDeterministicProductMedia() bool {
	return s != nil && s.media.Store != nil
}

// GetPrimaryProductImageForOrg returns the primary image row for a product within an company (ErrNoRows if none).
func (s *Service) GetPrimaryProductImageForOrg(ctx context.Context, companyID, productID uuid.UUID) (db.ProductImage, error) {
	if s == nil {
		return db.ProductImage{}, errors.New("catalogadmin: nil service")
	}
	if productID == uuid.Nil {
		return db.ProductImage{}, ErrCompanyRequired
	}
	return s.q.CatalogAdminGetPrimaryProductImageForOrg(ctx, productID)
}

// PrimaryProductImageOrNil loads the primary image when present; returns (nil, nil) when no primary image exists.
func (s *Service) PrimaryProductImageOrNil(ctx context.Context, companyID, productID uuid.UUID) (*db.ProductImage, error) {
	img, err := s.GetPrimaryProductImageForOrg(ctx, companyID, productID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &img, nil
}

// ListProductsParams filters and pages products for an company.
type ListProductsParams struct {
	Limit      int32
	Offset     int32
	Search     string
	ActiveOnly bool
}

// ListProductsResult is a paged product list.
type ListProductsResult struct {
	Items      []db.CatalogAdminListProductsRow
	TotalCount int64
}

// ListProducts returns products visible within the company.
func (s *Service) ListProducts(ctx context.Context, p ListProductsParams) (*ListProductsResult, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	search := p.Search
	cnt, err := s.q.CatalogAdminCountProducts(ctx, db.CatalogAdminCountProductsParams{
		Column1: search,
		Column2: p.ActiveOnly,
	})
	if err != nil {
		return nil, err
	}
	rows, err := s.q.CatalogAdminListProducts(ctx, db.CatalogAdminListProductsParams{Column1: search,
		Column2: p.ActiveOnly,

		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, err
	}
	return &ListProductsResult{Items: rows, TotalCount: cnt}, nil
}

// GetProduct returns a single product within the company.
func (s *Service) GetProduct(ctx context.Context, companyID, productID uuid.UUID) (db.Product, error) {
	if s == nil {
		return db.Product{}, errors.New("catalogadmin: nil service")
	}
	if productID == uuid.Nil {
		return db.Product{}, ErrCompanyRequired
	}
	row, err := s.q.CatalogAdminGetProduct(ctx, productID)
	if err != nil {
		return db.Product{}, err
	}
	return row, nil
}

// ListPriceBooksParams filters price books for pagination.
type ListPriceBooksParams struct {
	Limit           int32
	Offset          int32
	IncludeInactive bool
}

// ListPriceBooks returns price books for the company (active only unless IncludeInactive).
func (s *Service) ListPriceBooks(ctx context.Context, p ListPriceBooksParams) ([]db.PriceBook, int64, error) {
	if s == nil {
		return nil, 0, errors.New("catalogadmin: nil service")
	}
	cnt, err := s.q.CatalogAdminCountPriceBooks(ctx, p.IncludeInactive)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.CatalogAdminListPriceBooks(ctx, db.CatalogAdminListPriceBooksParams{Column1: p.IncludeInactive,

		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, cnt, nil
}

// ListPlanograms returns planograms for the company.
func (s *Service) ListPlanograms(ctx context.Context, companyID uuid.UUID, limit, offset int32) ([]db.Planogram, int64, error) {
	if s == nil {
		return nil, 0, errors.New("catalogadmin: nil service")
	}
	cnt, err := s.q.CatalogAdminCountPlanograms(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.CatalogAdminListPlanograms(ctx, db.CatalogAdminListPlanogramsParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, cnt, nil
}

// GetPlanogram returns a planogram in the company.
func (s *Service) GetPlanogram(ctx context.Context, companyID, planogramID uuid.UUID) (db.Planogram, error) {
	if s == nil {
		return db.Planogram{}, errors.New("catalogadmin: nil service")
	}
	if planogramID == uuid.Nil {
		return db.Planogram{}, ErrCompanyRequired
	}
	row, err := s.q.CatalogAdminGetPlanogram(ctx, planogramID)
	if err != nil {
		return db.Planogram{}, err
	}
	return row, nil
}

// ListPlanogramSlots returns slot rows for a planogram (must belong to company — enforced by query).
func (s *Service) ListPlanogramSlots(ctx context.Context, companyID, planogramID uuid.UUID) ([]db.CatalogAdminListSlotsByPlanogramRow, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	if planogramID == uuid.Nil {
		return nil, ErrCompanyRequired
	}
	if _, err := s.q.CatalogAdminGetPlanogram(ctx, planogramID); err != nil {
		return nil, err
	}
	return s.q.CatalogAdminListSlotsByPlanogram(ctx, planogramID)
}

// ListBrandsParams pages brands for an company.
type ListBrandsParams struct {
	Limit  int32
	Offset int32
}

// ListBrands returns brands for the company.
func (s *Service) ListBrands(ctx context.Context, p ListBrandsParams) ([]db.Brand, int64, error) {
	if s == nil {
		return nil, 0, errors.New("catalogadmin: nil service")
	}
	cnt, err := s.q.CatalogAdminCountBrands(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.CatalogAdminListBrands(ctx, db.CatalogAdminListBrandsParams{
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, cnt, nil
}

// GetBrand returns a brand in the company.
func (s *Service) GetBrand(ctx context.Context, companyID, brandID uuid.UUID) (db.Brand, error) {
	if s == nil {
		return db.Brand{}, errors.New("catalogadmin: nil service")
	}
	if brandID == uuid.Nil {
		return db.Brand{}, ErrCompanyRequired
	}
	return s.q.CatalogAdminGetBrand(ctx, brandID)
}

// ListCategoriesParams pages categories.
type ListCategoriesParams struct {
	Limit  int32
	Offset int32
}

// ListCategories returns categories for the company.
func (s *Service) ListCategories(ctx context.Context, p ListCategoriesParams) ([]db.Category, int64, error) {
	if s == nil {
		return nil, 0, errors.New("catalogadmin: nil service")
	}
	cnt, err := s.q.CatalogAdminCountCategories(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.CatalogAdminListCategories(ctx, db.CatalogAdminListCategoriesParams{
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, cnt, nil
}

// GetCategory returns a category in the company.
func (s *Service) GetCategory(ctx context.Context, companyID, categoryID uuid.UUID) (db.Category, error) {
	if s == nil {
		return db.Category{}, errors.New("catalogadmin: nil service")
	}
	if categoryID == uuid.Nil {
		return db.Category{}, ErrCompanyRequired
	}
	return s.q.CatalogAdminGetCategory(ctx, categoryID)
}

// ListTagsParams pages tags.
type ListTagsParams struct {
	Limit  int32
	Offset int32
}

// ListTags returns tags for the company.
func (s *Service) ListTags(ctx context.Context, p ListTagsParams) ([]db.Tag, int64, error) {
	if s == nil {
		return nil, 0, errors.New("catalogadmin: nil service")
	}
	cnt, err := s.q.CatalogAdminCountTags(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.CatalogAdminListTags(ctx, db.CatalogAdminListTagsParams{
		Limit:  p.Limit,
		Offset: p.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, cnt, nil
}

// GetTag returns a tag in the company.
func (s *Service) GetTag(ctx context.Context, companyID, tagID uuid.UUID) (db.Tag, error) {
	if s == nil {
		return db.Tag{}, errors.New("catalogadmin: nil service")
	}
	if tagID == uuid.Nil {
		return db.Tag{}, ErrCompanyRequired
	}
	return s.q.CatalogAdminGetTag(ctx, tagID)
}
