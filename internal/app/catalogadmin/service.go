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
	BumpOrganizationMedia(ctx context.Context, organizationID uuid.UUID)
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

func (s *Service) bumpCatalogCache(ctx context.Context, organizationID uuid.UUID) {
	if s == nil || s.cache == nil || organizationID == uuid.Nil {
		return
	}
	s.cache.BumpOrganizationMedia(ctx, organizationID)
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

// GetPrimaryProductImageForOrg returns the primary image row for a product within an organization (ErrNoRows if none).
func (s *Service) GetPrimaryProductImageForOrg(ctx context.Context, organizationID, productID uuid.UUID) (db.ProductImage, error) {
	if s == nil {
		return db.ProductImage{}, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || productID == uuid.Nil {
		return db.ProductImage{}, ErrOrganizationRequired
	}
	return s.q.CatalogAdminGetPrimaryProductImageForOrg(ctx, db.CatalogAdminGetPrimaryProductImageForOrgParams{
		OrganizationID: organizationID,
		ID:             productID,
	})
}

// PrimaryProductImageOrNil loads the primary image when present; returns (nil, nil) when no primary image exists.
func (s *Service) PrimaryProductImageOrNil(ctx context.Context, organizationID, productID uuid.UUID) (*db.ProductImage, error) {
	img, err := s.GetPrimaryProductImageForOrg(ctx, organizationID, productID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &img, nil
}

// ListProductsParams filters and pages products for an organization.
type ListProductsParams struct {
	OrganizationID uuid.UUID
	Limit          int32
	Offset         int32
	Search         string
	ActiveOnly     bool
}

// ListProductsResult is a paged product list.
type ListProductsResult struct {
	Items      []db.CatalogAdminListProductsRow
	TotalCount int64
}

// ListProducts returns products visible within the organization.
func (s *Service) ListProducts(ctx context.Context, p ListProductsParams) (*ListProductsResult, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	if p.OrganizationID == uuid.Nil {
		return nil, ErrOrganizationRequired
	}
	search := p.Search
	cnt, err := s.q.CatalogAdminCountProducts(ctx, db.CatalogAdminCountProductsParams{
		OrganizationID: p.OrganizationID,
		Column2:        search,
		Column3:        p.ActiveOnly,
	})
	if err != nil {
		return nil, err
	}
	rows, err := s.q.CatalogAdminListProducts(ctx, db.CatalogAdminListProductsParams{
		OrganizationID: p.OrganizationID,
		Limit:          p.Limit,
		Offset:         p.Offset,
		Column4:        search,
		Column5:        p.ActiveOnly,
	})
	if err != nil {
		return nil, err
	}
	return &ListProductsResult{Items: rows, TotalCount: cnt}, nil
}

// GetProduct returns a single product within the organization.
func (s *Service) GetProduct(ctx context.Context, organizationID, productID uuid.UUID) (db.Product, error) {
	if s == nil {
		return db.Product{}, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || productID == uuid.Nil {
		return db.Product{}, ErrOrganizationRequired
	}
	row, err := s.q.CatalogAdminGetProduct(ctx, db.CatalogAdminGetProductParams{
		OrganizationID: organizationID,
		ID:             productID,
	})
	if err != nil {
		return db.Product{}, err
	}
	return row, nil
}

// ListPriceBooksParams filters price books for pagination.
type ListPriceBooksParams struct {
	OrganizationID  uuid.UUID
	Limit           int32
	Offset          int32
	IncludeInactive bool
}

// ListPriceBooks returns price books for the organization (active only unless IncludeInactive).
func (s *Service) ListPriceBooks(ctx context.Context, p ListPriceBooksParams) ([]db.PriceBook, int64, error) {
	if s == nil {
		return nil, 0, errors.New("catalogadmin: nil service")
	}
	if p.OrganizationID == uuid.Nil {
		return nil, 0, ErrOrganizationRequired
	}
	cnt, err := s.q.CatalogAdminCountPriceBooks(ctx, db.CatalogAdminCountPriceBooksParams{
		OrganizationID: p.OrganizationID,
		Column2:        p.IncludeInactive,
	})
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.CatalogAdminListPriceBooks(ctx, db.CatalogAdminListPriceBooksParams{
		OrganizationID: p.OrganizationID,
		Limit:          p.Limit,
		Offset:         p.Offset,
		Column4:        p.IncludeInactive,
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, cnt, nil
}

// ListPlanograms returns planograms for the organization.
func (s *Service) ListPlanograms(ctx context.Context, organizationID uuid.UUID, limit, offset int32) ([]db.Planogram, int64, error) {
	if s == nil {
		return nil, 0, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil {
		return nil, 0, ErrOrganizationRequired
	}
	cnt, err := s.q.CatalogAdminCountPlanograms(ctx, organizationID)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.CatalogAdminListPlanograms(ctx, db.CatalogAdminListPlanogramsParams{
		OrganizationID: organizationID,
		Limit:          limit,
		Offset:         offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, cnt, nil
}

// GetPlanogram returns a planogram in the organization.
func (s *Service) GetPlanogram(ctx context.Context, organizationID, planogramID uuid.UUID) (db.Planogram, error) {
	if s == nil {
		return db.Planogram{}, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || planogramID == uuid.Nil {
		return db.Planogram{}, ErrOrganizationRequired
	}
	row, err := s.q.CatalogAdminGetPlanogram(ctx, db.CatalogAdminGetPlanogramParams{
		OrganizationID: organizationID,
		ID:             planogramID,
	})
	if err != nil {
		return db.Planogram{}, err
	}
	return row, nil
}

// ListPlanogramSlots returns slot rows for a planogram (must belong to organization — enforced by query).
func (s *Service) ListPlanogramSlots(ctx context.Context, organizationID, planogramID uuid.UUID) ([]db.CatalogAdminListSlotsByPlanogramRow, error) {
	if s == nil {
		return nil, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || planogramID == uuid.Nil {
		return nil, ErrOrganizationRequired
	}
	if _, err := s.q.CatalogAdminGetPlanogram(ctx, db.CatalogAdminGetPlanogramParams{
		OrganizationID: organizationID,
		ID:             planogramID,
	}); err != nil {
		return nil, err
	}
	return s.q.CatalogAdminListSlotsByPlanogram(ctx, planogramID)
}

// ListBrandsParams pages brands for an organization.
type ListBrandsParams struct {
	OrganizationID uuid.UUID
	Limit          int32
	Offset         int32
}

// ListBrands returns brands for the organization.
func (s *Service) ListBrands(ctx context.Context, p ListBrandsParams) ([]db.Brand, int64, error) {
	if s == nil {
		return nil, 0, errors.New("catalogadmin: nil service")
	}
	if p.OrganizationID == uuid.Nil {
		return nil, 0, ErrOrganizationRequired
	}
	cnt, err := s.q.CatalogAdminCountBrands(ctx, p.OrganizationID)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.CatalogAdminListBrands(ctx, db.CatalogAdminListBrandsParams{
		OrganizationID: p.OrganizationID,
		Limit:          p.Limit,
		Offset:         p.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, cnt, nil
}

// GetBrand returns a brand in the organization.
func (s *Service) GetBrand(ctx context.Context, organizationID, brandID uuid.UUID) (db.Brand, error) {
	if s == nil {
		return db.Brand{}, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || brandID == uuid.Nil {
		return db.Brand{}, ErrOrganizationRequired
	}
	return s.q.CatalogAdminGetBrand(ctx, db.CatalogAdminGetBrandParams{
		OrganizationID: organizationID,
		ID:             brandID,
	})
}

// ListCategoriesParams pages categories.
type ListCategoriesParams struct {
	OrganizationID uuid.UUID
	Limit          int32
	Offset         int32
}

// ListCategories returns categories for the organization.
func (s *Service) ListCategories(ctx context.Context, p ListCategoriesParams) ([]db.Category, int64, error) {
	if s == nil {
		return nil, 0, errors.New("catalogadmin: nil service")
	}
	if p.OrganizationID == uuid.Nil {
		return nil, 0, ErrOrganizationRequired
	}
	cnt, err := s.q.CatalogAdminCountCategories(ctx, p.OrganizationID)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.CatalogAdminListCategories(ctx, db.CatalogAdminListCategoriesParams{
		OrganizationID: p.OrganizationID,
		Limit:          p.Limit,
		Offset:         p.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, cnt, nil
}

// GetCategory returns a category in the organization.
func (s *Service) GetCategory(ctx context.Context, organizationID, categoryID uuid.UUID) (db.Category, error) {
	if s == nil {
		return db.Category{}, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || categoryID == uuid.Nil {
		return db.Category{}, ErrOrganizationRequired
	}
	return s.q.CatalogAdminGetCategory(ctx, db.CatalogAdminGetCategoryParams{
		OrganizationID: organizationID,
		ID:             categoryID,
	})
}

// ListTagsParams pages tags.
type ListTagsParams struct {
	OrganizationID uuid.UUID
	Limit          int32
	Offset         int32
}

// ListTags returns tags for the organization.
func (s *Service) ListTags(ctx context.Context, p ListTagsParams) ([]db.Tag, int64, error) {
	if s == nil {
		return nil, 0, errors.New("catalogadmin: nil service")
	}
	if p.OrganizationID == uuid.Nil {
		return nil, 0, ErrOrganizationRequired
	}
	cnt, err := s.q.CatalogAdminCountTags(ctx, p.OrganizationID)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.CatalogAdminListTags(ctx, db.CatalogAdminListTagsParams{
		OrganizationID: p.OrganizationID,
		Limit:          p.Limit,
		Offset:         p.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return rows, cnt, nil
}

// GetTag returns a tag in the organization.
func (s *Service) GetTag(ctx context.Context, organizationID, tagID uuid.UUID) (db.Tag, error) {
	if s == nil {
		return db.Tag{}, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil || tagID == uuid.Nil {
		return db.Tag{}, ErrOrganizationRequired
	}
	return s.q.CatalogAdminGetTag(ctx, db.CatalogAdminGetTagParams{
		OrganizationID: organizationID,
		ID:             tagID,
	})
}
