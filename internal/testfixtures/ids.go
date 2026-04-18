package testfixtures

import "github.com/google/uuid"

// Deterministic UUIDs from migrations/00003_seed_dev.sql
var (
	DevOrganizationID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	DevSiteID         = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	DevMachineID      = uuid.MustParse("55555555-5555-5555-5555-555555555555")
	DevTechnicianID   = uuid.MustParse("66666666-6666-6666-6666-666666666666")
	DevProductCola    = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-000000000001")
	DevProductWater   = uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-000000000002")
)
