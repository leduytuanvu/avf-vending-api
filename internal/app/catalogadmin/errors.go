package catalogadmin

import "errors"

var (
	// ErrCompanyRequired is returned when a company scope is missing.
	ErrCompanyRequired = errors.New("catalogadmin: company id required")
	// ErrDuplicateSKU is returned on unique SKU violations within the catalog.
	ErrDuplicateSKU = errors.New("catalogadmin: duplicate sku in company")
	// ErrDuplicateBarcode is returned on unique barcode violations within the org.
	ErrDuplicateBarcode = errors.New("catalogadmin: duplicate barcode in company")
	// ErrDuplicateSlug is returned for brands/categories/tags slug collisions.
	ErrDuplicateSlug = errors.New("catalogadmin: duplicate slug in company")
	// ErrDuplicateNameRevision is returned when planogram name+revision already exists.
	ErrDuplicateNameRevision = errors.New("catalogadmin: duplicate planogram name and revision")
	// ErrNotFound is returned when a catalog row is missing for the org.
	ErrNotFound = errors.New("catalogadmin: not found")
	// ErrInvalidArgument is returned for bad input.
	ErrInvalidArgument = errors.New("catalogadmin: invalid argument")
	// ErrConflict is returned when an operation violates business constraints (e.g. duplicate target assignment).
	ErrConflict = errors.New("catalogadmin: conflict")
)
