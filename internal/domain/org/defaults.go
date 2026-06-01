package org

import "github.com/google/uuid"

// DefaultCompanyID is the singleton company anchor for local dev and integration fixtures.
var DefaultCompanyID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
