package listscope

import "errors"

var (
	// ErrAdminCompanyRequired is returned when an admin list is called without a resolved company id.
	ErrAdminCompanyRequired = errors.New("listscope: company scope required for this admin list")

	// ErrCommerceCompanyQueryRequired is returned when a platform administrator omits scope_id on commerce list routes.
	ErrCommerceCompanyQueryRequired = errors.New("listscope: scope_id query parameter is required for platform administrators")

	// ErrInvalidListQuery is returned when list query parameters fail validation (UUIDs, site ownership, etc.).
	ErrInvalidListQuery = errors.New("listscope: invalid list query parameters")
)
