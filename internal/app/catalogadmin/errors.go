package catalogadmin

import "errors"

var (
	// ErrOrganizationRequired is returned when a tenant scope is missing.
	ErrOrganizationRequired = errors.New("catalogadmin: organization id required")
	// ErrDuplicateSKU is returned on unique (organization_id, sku) violations.
	ErrDuplicateSKU = errors.New("catalogadmin: duplicate sku in organization")
	// ErrDuplicateBarcode is returned on unique barcode violations within the org.
	ErrDuplicateBarcode = errors.New("catalogadmin: duplicate barcode in organization")
	// ErrDuplicateSlug is returned for brands/categories/tags slug collisions.
	ErrDuplicateSlug = errors.New("catalogadmin: duplicate slug in organization")
	// ErrNotFound is returned when a catalog row is missing for the org.
	ErrNotFound = errors.New("catalogadmin: not found")
	// ErrInvalidArgument is returned for bad input.
	ErrInvalidArgument = errors.New("catalogadmin: invalid argument")
)
