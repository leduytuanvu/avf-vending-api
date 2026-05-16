package org

import (
	"context"

	"github.com/google/uuid"
)

// CompanyRepository reads company rows from the system of record.
type CompanyRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (Company, error)
}

// SiteRepository reads site rows from the system of record.
type SiteRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (Site, error)
}
