package api

import "github.com/avf/avf-vending-api/internal/app/listscope"

// ErrAdminTenantScopeRequired is returned when a tenant-scoped admin list is called without an organization on the principal.
var ErrAdminTenantScopeRequired = listscope.ErrAdminOrganizationRequired

// ErrCommerceOrganizationQueryRequired is returned when a platform administrator omits organization_id on commerce list routes.
var ErrCommerceOrganizationQueryRequired = listscope.ErrCommerceOrganizationQueryRequired

// ErrInvalidListQuery is returned when list query parameters (pagination, UUIDs, times) fail validation.
var ErrInvalidListQuery = listscope.ErrInvalidListQuery
