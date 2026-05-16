package listscope

import (
	"time"

	"github.com/google/uuid"
)

// AdminFleet carries resolved company and filters for GET /v1/admin/* collection routes.
type AdminFleet struct {
	IsPlatformAdmin bool
	SiteID          *uuid.UUID
	MachineID       *uuid.UUID
	TechnicianID    *uuid.UUID
	Status          string
	Search          string
	From            *time.Time
	To              *time.Time
	Limit           int32
	Offset          int32
}

// CompanyCommerce carries resolved company and filters for GET /v1/orders and /v1/payments.
type CompanyCommerce struct {
	IsPlatformAdmin bool
	Limit           int32
	Offset          int32
	Status          string
	MachineID       *uuid.UUID
	PaymentMethod   string
	CaseType        string
	Search          string
	From            *time.Time
	To              *time.Time
}
