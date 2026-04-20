package catalogadmin

import (
	"context"
	"errors"
	"fmt"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

// Service provides read-only catalog queries for admin APIs.
type Service struct {
	q *db.Queries
}

// NewService constructs a catalog admin reader.
func NewService(q *db.Queries) (*Service, error) {
	if q == nil {
		return nil, fmt.Errorf("catalogadmin: nil Queries")
	}
	return &Service{q: q}, nil
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

// ListPriceBooks returns price books for the organization.
func (s *Service) ListPriceBooks(ctx context.Context, organizationID uuid.UUID, limit, offset int32) ([]db.PriceBook, int64, error) {
	if s == nil {
		return nil, 0, errors.New("catalogadmin: nil service")
	}
	if organizationID == uuid.Nil {
		return nil, 0, ErrOrganizationRequired
	}
	cnt, err := s.q.CatalogAdminCountPriceBooks(ctx, organizationID)
	if err != nil {
		return nil, 0, err
	}
	rows, err := s.q.CatalogAdminListPriceBooks(ctx, db.CatalogAdminListPriceBooksParams{
		OrganizationID: organizationID,
		Limit:          limit,
		Offset:         offset,
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
