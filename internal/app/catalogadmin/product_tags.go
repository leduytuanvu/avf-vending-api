package catalogadmin

import (
	"context"
	"fmt"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

// normalizeDedupeProductTagIDs removes duplicates (stable order), rejects uuid.Nil.
func normalizeDedupeProductTagIDs(ids []uuid.UUID) ([]uuid.UUID, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			return nil, fmt.Errorf("%w: tagIds must not contain null UUID", ErrInvalidArgument)
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func (s *Service) validateProductTagIDsExist(ctx context.Context, q *db.Queries, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	cnt, err := q.CatalogAdminCountTagsMatchingIDs(ctx, ids)
	if err != nil {
		return err
	}
	if cnt != int64(len(ids)) {
		return fmt.Errorf("%w: unknown tag id in tagIds", ErrInvalidArgument)
	}
	return nil
}

// ProductTagsByProductIDs loads linked tags for many products (batch, ordered by tag name per product).
func (s *Service) ProductTagsByProductIDs(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID][]db.Tag, error) {
	if s == nil {
		return nil, fmt.Errorf("catalogadmin: nil service")
	}
	out := make(map[uuid.UUID][]db.Tag)
	if len(productIDs) == 0 {
		return out, nil
	}
	rows, err := s.q.CatalogAdminListProductTagsForProducts(ctx, productIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		t := db.Tag{
			ID:        row.ID,
			Slug:      row.Slug,
			Name:      row.Name,
			Active:    row.Active,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		}
		out[row.ProductID] = append(out[row.ProductID], t)
	}
	return out, nil
}
