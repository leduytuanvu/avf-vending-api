package productmastercatalog

import (
	"context"
	"fmt"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/google/uuid"
)

// VisibleList is a named product list a machine may download.
type VisibleList struct {
	ListID      uuid.UUID
	Name        string
	Kind        string
	Status      string
	IsPrimary   bool
	Assigned    bool
	UpdatedAt   time.Time
	Description string
}

// ListSnapshot is a product-list download package mapped onto product master records.
type ListSnapshot struct {
	ListID      uuid.UUID
	ListVersion string
	Snapshot    Snapshot
}

// ListVisibleLists returns published common lists plus lists assigned to the machine.
func (s *Service) ListVisibleLists(ctx context.Context, machineID uuid.UUID) ([]VisibleList, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("productmastercatalog: nil service")
	}
	if machineID == uuid.Nil {
		return nil, fmt.Errorf("productmastercatalog: machine id required")
	}
	q := pgxutil.NewQueries(s.pool)
	rows, err := q.ProductListVisibleForMachine(ctx, machineID)
	if err != nil {
		return nil, err
	}
	out := make([]VisibleList, 0, len(rows))
	for _, row := range rows {
		out = append(out, VisibleList{
			ListID:      row.ID,
			Name:        row.Name,
			Kind:        row.Kind,
			Status:      row.Status,
			IsPrimary:   row.IsPrimary,
			Assigned:    row.Assigned,
			UpdatedAt:   row.UpdatedAt.UTC(),
			Description: row.Description,
		})
	}
	return out, nil
}

// BuildListSnapshot returns active products for a list visible to the machine.
// Empty productIDs means the full list. Unknown ids are ignored.
func (s *Service) BuildListSnapshot(ctx context.Context, machineID, listID uuid.UUID, productIDs []uuid.UUID) (ListSnapshot, error) {
	if s == nil || s.pool == nil {
		return ListSnapshot{}, fmt.Errorf("productmastercatalog: nil service")
	}
	if machineID == uuid.Nil || listID == uuid.Nil {
		return ListSnapshot{}, fmt.Errorf("productmastercatalog: machine id and list id required")
	}
	q := pgxutil.NewQueries(s.pool)
	visible, err := q.ProductListVisibleForMachine(ctx, machineID)
	if err != nil {
		return ListSnapshot{}, err
	}
	found := false
	for _, row := range visible {
		if row.ID == listID {
			found = true
			break
		}
	}
	if !found {
		return ListSnapshot{}, fmt.Errorf("productmastercatalog: product list not visible to machine")
	}
	ids := productIDs
	if ids == nil {
		ids = []uuid.UUID{}
	}
	rows, err := q.ProductListSnapshotRows(ctx, db.ProductListSnapshotRowsParams{
		ListID:     listID,
		ProductIds: ids,
	})
	if err != nil {
		return ListSnapshot{}, err
	}
	mapped := make([]db.ProductMasterListActiveProductsPageRow, 0, len(rows))
	for _, row := range rows {
		mapped = append(mapped, db.ProductMasterListActiveProductsPageRow{
			ID:          row.ID,
			Sku:         row.Sku,
			Barcode:     row.Barcode,
			Name:        row.Name,
			Description: row.Description,
			Active:      row.Active,
			CategoryID:  row.CategoryID,
			BrandID:     row.BrandID,
			UpdatedAt:   row.UpdatedAt,
		})
	}
	records, _, err := s.mapRows(ctx, q, mapped)
	if err != nil {
		return ListSnapshot{}, err
	}
	version := catalogVersion(records)
	now := time.Now().UTC()
	return ListSnapshot{
		ListID:      listID,
		ListVersion: version,
		Snapshot: Snapshot{
			CatalogVersion:   version,
			GeneratedAt:      now,
			TotalActiveCount: int32(len(records)),
			Products:         records,
		},
	}, nil
}
