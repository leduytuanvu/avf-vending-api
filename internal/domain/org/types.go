package org

import (
	"time"

	"github.com/google/uuid"
)

// Organization is the top-level tenant boundary.
type Organization struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Region is a geographic grouping within an organization.
type Region struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Code           string
	CreatedAt      time.Time
}

// Site is a physical location that hosts machines.
type Site struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	RegionID       *uuid.UUID
	Name           string
	CreatedAt      time.Time
}
