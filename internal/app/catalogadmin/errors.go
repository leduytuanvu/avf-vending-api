package catalogadmin

import "errors"

// ErrOrganizationRequired is returned when organization scope is missing.
var ErrOrganizationRequired = errors.New("organization required")
