package finance

import "github.com/google/uuid"

// DailyCloseView is the JSON shape for finance daily close resources.
type DailyCloseView struct {
	ID               string `json:"id"`
	OrganizationID   string `json:"organizationId"`
	CloseDate        string `json:"closeDate"`
	Timezone         string `json:"timezone"`
	SiteID           string `json:"siteId,omitempty"`
	MachineID        string `json:"machineId,omitempty"`
	IdempotencyKey   string `json:"idempotencyKey"`
	GrossSalesMinor  int64  `json:"grossSalesMinor"`
	DiscountMinor    int64  `json:"discountMinor"`
	RefundMinor      int64  `json:"refundMinor"`
	NetMinor         int64  `json:"netMinor"`
	CashMinor        int64  `json:"cashMinor"`
	QRWalletMinor    int64  `json:"qrWalletMinor"`
	FailedMinor      int64  `json:"failedMinor"`
	PendingMinor     int64  `json:"pendingMinor"`
	CreatedAtRFC3339 string `json:"createdAt"`
}

// DailyCloseListResponse paginates GET /v1/admin/finance/daily-close.
type DailyCloseListResponse struct {
	Items []DailyCloseView `json:"items"`
	Meta  DailyCloseMeta   `json:"meta"`
}

// DailyCloseMeta follows admin collection list pagination style.
type DailyCloseMeta struct {
	Limit    int32 `json:"limit"`
	Offset   int32 `json:"offset"`
	Returned int   `json:"returned"`
	Total    int64 `json:"total"`
}

// CreateDailyCloseInput is validated application input for POST /v1/admin/finance/daily-close.
type CreateDailyCloseInput struct {
	OrganizationID uuid.UUID
	CloseDate      string
	Timezone       string
	SiteID         uuid.UUID
	MachineID      uuid.UUID
	IdempotencyKey string
	ActorType      string
	ActorID        *string
}
