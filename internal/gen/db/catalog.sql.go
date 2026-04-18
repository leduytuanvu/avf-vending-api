package db

import (
	"context"

	"github.com/google/uuid"
)

const getProductByID = `-- name: GetProductByID :one
SELECT id, organization_id, sku, name, description, attrs, active, created_at, updated_at
FROM products
WHERE id = $1
`

func (q *Queries) GetProductByID(ctx context.Context, id uuid.UUID) (Product, error) {
	row := q.db.QueryRow(ctx, getProductByID, id)
	var p Product
	err := row.Scan(
		&p.ID,
		&p.OrganizationID,
		&p.Sku,
		&p.Name,
		&p.Description,
		&p.Attrs,
		&p.Active,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	return p, err
}

const listProductsByOrganization = `-- name: ListProductsByOrganization :many
SELECT id, organization_id, sku, name, description, attrs, active, created_at, updated_at
FROM products
WHERE organization_id = $1
ORDER BY sku
`

func (q *Queries) ListProductsByOrganization(ctx context.Context, organizationID uuid.UUID) ([]Product, error) {
	rows, err := q.db.Query(ctx, listProductsByOrganization, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(
			&p.ID,
			&p.OrganizationID,
			&p.Sku,
			&p.Name,
			&p.Description,
			&p.Attrs,
			&p.Active,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}
