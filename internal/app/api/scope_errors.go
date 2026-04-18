package api

import "errors"

// ErrAdminTenantScopeRequired is returned when a tenant-scoped admin list is called without an organization on the principal.
var ErrAdminTenantScopeRequired = errors.New("api: organization scope required for this admin list")
