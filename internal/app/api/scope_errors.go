package api

import "github.com/avf/avf-vending-api/internal/app/listscope"

// ErrAdminCompanyScopeRequired is returned when a single-company admin list is called without an company on the principal.
var ErrAdminCompanyScopeRequired = listscope.ErrAdminCompanyRequired

// ErrCommerceCompanyQueryRequired is returned when a platform administrator omits scope_id on commerce list routes.
var ErrCommerceCompanyQueryRequired = listscope.ErrCommerceCompanyQueryRequired

// ErrInvalidListQuery is returned when list query parameters (pagination, UUIDs, times) fail validation.
var ErrInvalidListQuery = listscope.ErrInvalidListQuery
