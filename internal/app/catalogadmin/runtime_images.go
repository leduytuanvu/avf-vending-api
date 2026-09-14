package catalogadmin

import (
	"context"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

// RuntimePrimaryImagesByProductIDs returns the primary active product image row per product id.
func (s *Service) RuntimePrimaryImagesByProductIDs(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]db.RuntimeListProductImagesForProductsRow, error) {
	if s == nil || s.q == nil {
		return nil, nil
	}
	if len(productIDs) == 0 {
		return map[uuid.UUID]db.RuntimeListProductImagesForProductsRow{}, nil
	}
	rows, err := s.q.RuntimeListProductImagesForProducts(ctx, productIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]db.RuntimeListProductImagesForProductsRow, len(productIDs))
	for _, row := range rows {
		existing, ok := out[row.ProductID]
		if !ok {
			out[row.ProductID] = row
			continue
		}
		if row.IsPrimary && !existing.IsPrimary {
			out[row.ProductID] = row
		}
	}
	return out, nil
}
