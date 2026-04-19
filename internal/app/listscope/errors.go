package listscope

import "errors"

var (
	// ErrAdminOrganizationRequired is returned when an admin list is called without a resolved organization id.
	ErrAdminOrganizationRequired = errors.New("listscope: organization scope required for this admin list")

	// ErrCommerceOrganizationQueryRequired is returned when a platform administrator omits organization_id on commerce list routes.
	ErrCommerceOrganizationQueryRequired = errors.New("listscope: organization_id query parameter is required for platform administrators")

	// ErrInvalidListQuery is returned when list query parameters fail validation (UUIDs, site ownership, etc.).
	ErrInvalidListQuery = errors.New("listscope: invalid list query parameters")
)
