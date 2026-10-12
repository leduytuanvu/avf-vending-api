package pricingengine

import (
	"context"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ProductPromoMeta is category + tag IDs used for promotion evaluation.
type ProductPromoMeta struct {
	Category pgtype.UUID
	Tags     []uuid.UUID
}

// LoadProductPromoMetaBatch loads category and tag IDs for many products in two queries.
func LoadProductPromoMetaBatch(ctx context.Context, q *db.Queries, productIDs []uuid.UUID) (map[uuid.UUID]ProductPromoMeta, error) {
	out := make(map[uuid.UUID]ProductPromoMeta)
	if q == nil || len(productIDs) == 0 {
		return out, nil
	}
	ids := dedupeUUIDs(productIDs)
	catRows, err := q.PromotionAdminGetProductCategories(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, row := range catRows {
		out[row.ID] = ProductPromoMeta{Category: row.CategoryID, Tags: nil}
	}
	tagRows, err := q.PromotionAdminListProductTagIDsForProducts(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, row := range tagRows {
		m, ok := out[row.ProductID]
		if !ok {
			m = ProductPromoMeta{}
		}
		m.Tags = append(m.Tags, row.TagID)
		out[row.ProductID] = m
	}
	return out, nil
}

func dedupeUUIDs(in []uuid.UUID) []uuid.UUID {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[uuid.UUID]struct{}, len(in))
	out := make([]uuid.UUID, 0, len(in))
	for _, id := range in {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
