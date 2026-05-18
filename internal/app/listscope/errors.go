package listscope

import "errors"

var (
	// ErrAdminCompanyRequired is returned when an admin list is called without a resolved company id.
	ErrAdminCompanyRequired = errors.New("listscope: company scope required for this admin list")

	// ErrInvalidListQuery is returned when list query parameters fail validation (UUIDs, site ownership, etc.).
	ErrInvalidListQuery = errors.New("listscope: invalid list query parameters")
)
