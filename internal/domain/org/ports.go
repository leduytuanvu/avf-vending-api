package org

import (
	"context"

	"github.com/google/uuid"
)

// OrganizationRepository reads organization rows from the system of record.
type OrganizationRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (Organization, error)
}

// SiteRepository reads site rows from the system of record.
type SiteRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (Site, error)
}
