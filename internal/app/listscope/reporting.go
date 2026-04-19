package listscope

import (
	"time"

	"github.com/google/uuid"
)

// ReportingQuery carries validated parameters for read-only GET /v1/reports/* analytics.
type ReportingQuery struct {
	IsPlatformAdmin bool
	OrganizationID  uuid.UUID
	From            time.Time
	To              time.Time
	GroupBy         string
	ExceptionKind   string
	Limit           int32
	Offset          int32
}
