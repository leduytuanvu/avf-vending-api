package retail

import (
	"context"

	"github.com/google/uuid"
)

// ProductRepository reads product rows from the system of record.
type ProductRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (Product, error)
}
