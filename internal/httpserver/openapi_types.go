package httpserver

import "encoding/json"

// OpenAPI / Swagger documentation types (runtime responses use compatible JSON shapes).
// Handlers may return additional fields; these structs capture stable fields for spec generation.

// V1ErrorBody is the inner object for all JSON API errors under /v1 (handlers and auth middleware).
type V1ErrorBody struct {
	Code      string         `json:"code" example:"invalid_json"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details"`
	RequestID string         `json:"requestId" example:"550e8400-e29b-41d4-a716-446655440000"`
}

// V1StandardError is the usual handler JSON error (see writeAPIError).
type V1StandardError struct {
	Error V1ErrorBody `json:"error"`
}

// V1NotImplementedError is HTTP **501**; error.details carries capability + implemented.
type V1NotImplementedError struct {
	Error V1ErrorBody `json:"error"`
}

// V1CapabilityNotConfiguredError is HTTP **503** when optional wiring is missing; details carry capability + implemented.
type V1CapabilityNotConfiguredError struct {
	Error V1ErrorBody `json:"error"`
}

// V1BearerAuthError is returned by Bearer and RBAC middleware (HTTP 401/403/400/503 from auth layer).
type V1BearerAuthError struct {
	Error V1ErrorBody `json:"error"`
}

// V1OperatorListEnvelope matches writeOperatorListEnvelope (items + meta.limit + meta.returned).
type V1OperatorListEnvelope struct {
	Items []any      `json:"items"`
	Meta  V1ListMeta `json:"meta"`
}

// V1OperatorSessionEnvelope wraps a session object for login/logout/heartbeat success bodies.
type V1OperatorSessionEnvelope struct {
	Session map[string]any `json:"session"`
}

// V1OperatorCurrentEnvelope is the /operator-sessions/current success shape.
type V1OperatorCurrentEnvelope struct {
	ActiveSession         any     `json:"active_session"`
	TechnicianDisplayName *string `json:"technician_display_name,omitempty"`
}

// V1CommerceCreateOrderResponse matches commerceCreateOrderResponse JSON.
type V1CommerceCreateOrderResponse struct {
	OrderID       string `json:"order_id"`
	VendSessionID string `json:"vend_session_id"`
	Replay        bool   `json:"replay"`
	OrderStatus   string `json:"order_status" enums:"created,quoted,paid,vending,completed,failed,cancelled"`
	VendState     string `json:"vend_state" enums:"pending,in_progress,success,failed"`
	SlotID        string `json:"slot_id"`
	CabinetCode   string `json:"cabinet_code"`
	SlotCode      string `json:"slot_code"`
	SlotIndex     int32  `json:"slot_index"`
	SubtotalMinor int64  `json:"subtotal_minor"`
	TaxMinor      int64  `json:"tax_minor"`
	TotalMinor    int64  `json:"total_minor"`
	PriceMinor    int64  `json:"price_minor"`
}

// V1CommerceCashCheckoutResponse matches commerceCashCheckoutResponse JSON (POST /v1/commerce/cash-checkout).
// See docs/api/setup-machine.md (commerce) and OpenAPI example on that path.
type V1CommerceCashCheckoutResponse struct {
	OrderID       string `json:"order_id"`
	VendSessionID string `json:"vend_session_id"`
	PaymentID     string `json:"payment_id"`
	OrderStatus   string `json:"order_status" enums:"created,quoted,paid,vending,completed,failed,cancelled"`
	PaymentState  string `json:"payment_state" enums:"created,authorized,captured,failed,refunded"`
	Replay        bool   `json:"replay"`
}

// V1ListViewEnvelope is the success shape for admin and tenant list endpoints.
type V1ListViewEnvelope struct {
	Items []any `json:"items"`
	Meta  any   `json:"meta,omitempty"`
}

// V1ListMeta is common pagination metadata for list responses.
type V1ListMeta struct {
	Limit    int32 `json:"limit" example:"50"`
	Returned int   `json:"returned" example:"12"`
}

// --- Auth session (POST /v1/auth/login, /v1/auth/refresh; GET /v1/auth/me; POST /v1/auth/logout) ---

// V1AuthLoginRequest is documented in tools/build_openapi.py (example organizationId + email + password).
type V1AuthLoginRequest struct {
	OrganizationID string `json:"organizationId" example:"11111111-1111-1111-1111-111111111111"`
	Email          string `json:"email" example:"admin@example.com"`
	Password       string `json:"password" example:"••••••••"`
}

// V1AuthTokenPair is nested under login/refresh responses.
type V1AuthTokenPair struct {
	AccessToken      string `json:"accessToken"`
	AccessExpiresAt  string `json:"accessExpiresAt"`
	RefreshToken     string `json:"refreshToken"`
	RefreshExpiresAt string `json:"refreshExpiresAt"`
	TokenType        string `json:"tokenType" example:"Bearer"`
}

// V1AuthLoginResponse documents POST /v1/auth/login success.
type V1AuthLoginResponse struct {
	AccountID      string          `json:"accountId"`
	OrganizationID string          `json:"organizationId"`
	Email          string          `json:"email"`
	Roles          []string        `json:"roles"`
	Tokens         V1AuthTokenPair `json:"tokens"`
}

// V1AuthMeResponse documents GET /v1/auth/me success.
type V1AuthMeResponse struct {
	AccountID      string   `json:"accountId"`
	OrganizationID string   `json:"organizationId"`
	Email          string   `json:"email"`
	Roles          []string `json:"roles"`
}

// V1AuthRefreshRequest is the refresh body.
type V1AuthRefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// V1AuthRefreshResponse is POST /v1/auth/refresh success.
type V1AuthRefreshResponse struct {
	Tokens V1AuthTokenPair `json:"tokens"`
}

// V1AuthLogoutRequest optionally revokes a single refresh token or all sessions.
type V1AuthLogoutRequest struct {
	RefreshToken string `json:"refreshToken,omitempty"`
	RevokeAll    bool   `json:"revokeAll,omitempty"`
}

// --- Admin catalog (read-only) ---

// V1AdminPageMeta is pagination metadata for admin catalog lists.
type V1AdminPageMeta struct {
	Limit      int32 `json:"limit"`
	Offset     int32 `json:"offset"`
	Returned   int   `json:"returned"`
	TotalCount int64 `json:"totalCount"`
}

// V1AdminProductListItem is a row in GET /v1/admin/products.
type V1AdminProductListItem struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organizationId"`
	Sku            string  `json:"sku"`
	Barcode        *string `json:"barcode,omitempty"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	Active         bool    `json:"active"`
	CategoryID     *string `json:"categoryId,omitempty"`
	BrandID        *string `json:"brandId,omitempty"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

// V1AdminProductListEnvelope matches listAdminProducts success JSON.
type V1AdminProductListEnvelope struct {
	Items []V1AdminProductListItem `json:"items"`
	Meta  V1AdminPageMeta          `json:"meta"`
}

// V1AdminProduct is GET /v1/admin/products/{productId} success.
type V1AdminProduct struct {
	ID              string          `json:"id"`
	OrganizationID  string          `json:"organizationId"`
	Sku             string          `json:"sku"`
	Barcode         *string         `json:"barcode,omitempty"`
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Attrs           json.RawMessage `json:"attrs,omitempty"`
	Active          bool            `json:"active"`
	CategoryID      *string         `json:"categoryId,omitempty"`
	BrandID         *string         `json:"brandId,omitempty"`
	PrimaryImageID  *string         `json:"primaryImageId,omitempty"`
	CountryOfOrigin *string         `json:"countryOfOrigin,omitempty"`
	AgeRestricted   bool            `json:"ageRestricted"`
	AllergenCodes   []string        `json:"allergenCodes"`
	NutritionalNote *string         `json:"nutritionalNote,omitempty"`
	CreatedAt       string          `json:"createdAt"`
	UpdatedAt       string          `json:"updatedAt"`
}

// V1AdminProductMutationRequest is the body for POST/PUT/PATCH /v1/admin/products.
type V1AdminProductMutationRequest struct {
	Sku             string          `json:"sku"`
	Name            string          `json:"name"`
	Description     string          `json:"description,omitempty"`
	Attrs           json.RawMessage `json:"attrs,omitempty"`
	Active          bool            `json:"active"`
	CategoryID      *string         `json:"categoryId,omitempty"`
	BrandID         *string         `json:"brandId,omitempty"`
	Barcode         *string         `json:"barcode,omitempty"`
	CountryOfOrigin *string         `json:"countryOfOrigin,omitempty"`
	AgeRestricted   bool            `json:"ageRestricted"`
	AllergenCodes   []string        `json:"allergenCodes,omitempty"`
	NutritionalNote *string         `json:"nutritionalNote,omitempty"`
}

// V1AdminBrand is a brand row.
type V1AdminBrand struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	Active         bool   `json:"active"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// V1AdminBrandListEnvelope is GET /v1/admin/brands.
type V1AdminBrandListEnvelope struct {
	Items []V1AdminBrand  `json:"items"`
	Meta  V1AdminPageMeta `json:"meta"`
}

// V1AdminBrandMutationRequest is POST/PUT/PATCH /v1/admin/brands.
type V1AdminBrandMutationRequest struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// V1AdminCategory is a category row.
type V1AdminCategory struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organizationId"`
	Slug           string  `json:"slug"`
	Name           string  `json:"name"`
	ParentID       *string `json:"parentId,omitempty"`
	Active         bool    `json:"active"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

// V1AdminCategoryListEnvelope is GET /v1/admin/categories.
type V1AdminCategoryListEnvelope struct {
	Items []V1AdminCategory `json:"items"`
	Meta  V1AdminPageMeta   `json:"meta"`
}

// V1AdminCategoryMutationRequest is POST/PUT/PATCH /v1/admin/categories.
type V1AdminCategoryMutationRequest struct {
	Slug     string  `json:"slug"`
	Name     string  `json:"name"`
	ParentID *string `json:"parentId,omitempty"`
	Active   bool    `json:"active"`
}

// V1AdminTag is a tag row.
type V1AdminTag struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	Active         bool   `json:"active"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

// V1AdminTagListEnvelope is GET /v1/admin/tags.
type V1AdminTagListEnvelope struct {
	Items []V1AdminTag    `json:"items"`
	Meta  V1AdminPageMeta `json:"meta"`
}

// V1AdminTagMutationRequest is POST/PUT/PATCH /v1/admin/tags.
type V1AdminTagMutationRequest struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// V1AdminProductImageBindRequest binds CDN image URLs to a product (primary image).
type V1AdminProductImageBindRequest struct {
	ArtifactID  string `json:"artifactId"`
	ThumbURL    string `json:"thumbUrl"`
	DisplayURL  string `json:"displayUrl"`
	ContentHash string `json:"contentHash,omitempty"`
	Width       int32  `json:"width,omitempty"`
	Height      int32  `json:"height,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// V1AdminPriceBook is a row in GET /v1/admin/price-books.
type V1AdminPriceBook struct {
	ID             string  `json:"id"`
	OrganizationID string  `json:"organizationId"`
	Name           string  `json:"name"`
	Currency       string  `json:"currency"`
	EffectiveFrom  string  `json:"effectiveFrom"`
	EffectiveTo    *string `json:"effectiveTo,omitempty"`
	IsDefault      bool    `json:"isDefault"`
	ScopeType      string  `json:"scopeType"`
	SiteID         *string `json:"siteId,omitempty"`
	MachineID      *string `json:"machineId,omitempty"`
	Priority       int32   `json:"priority"`
	CreatedAt      string  `json:"createdAt"`
}

// V1AdminPriceBookListEnvelope matches listAdminPriceBooks success JSON.
type V1AdminPriceBookListEnvelope struct {
	Items []V1AdminPriceBook `json:"items"`
	Meta  V1AdminPageMeta    `json:"meta"`
}

// V1AdminPlanogram is a planogram summary row.
type V1AdminPlanogram struct {
	ID             string          `json:"id"`
	OrganizationID string          `json:"organizationId"`
	Name           string          `json:"name"`
	Revision       int32           `json:"revision"`
	Status         string          `json:"status"`
	Meta           json.RawMessage `json:"meta,omitempty"`
	CreatedAt      string          `json:"createdAt"`
}

// V1AdminPlanogramListEnvelope matches GET /v1/admin/planograms.
type V1AdminPlanogramListEnvelope struct {
	Items []V1AdminPlanogram `json:"items"`
	Meta  V1AdminPageMeta    `json:"meta"`
}

// V1AdminPlanogramSlot is a slot assignment on a planogram.
type V1AdminPlanogramSlot struct {
	ID          string  `json:"id"`
	PlanogramID string  `json:"planogramId"`
	SlotIndex   int32   `json:"slotIndex"`
	ProductID   *string `json:"productId,omitempty"`
	MaxQuantity int32   `json:"maxQuantity"`
	ProductSku  *string `json:"productSku,omitempty"`
	ProductName *string `json:"productName,omitempty"`
	CreatedAt   string  `json:"createdAt"`
}

// V1AdminPlanogramDetail is GET /v1/admin/planograms/{planogramId} (includes slot layout).
type V1AdminPlanogramDetail struct {
	Planogram V1AdminPlanogram       `json:"planogram"`
	Slots     []V1AdminPlanogramSlot `json:"slots"`
}

// --- Admin inventory (read-only) ---

// V1AdminMachineSlot is a machine slot projection with catalog joins.
type V1AdminMachineSlot struct {
	MachineID                string  `json:"machineId"`
	MachineName              string  `json:"machineName"`
	MachineStatus            string  `json:"machineStatus"`
	PlanogramID              string  `json:"planogramId"`
	PlanogramName            string  `json:"planogramName"`
	SlotIndex                int32   `json:"slotIndex"`
	CabinetCode              string  `json:"cabinetCode"`
	CabinetIndex             int32   `json:"cabinetIndex"`
	SlotCode                 string  `json:"slotCode"`
	CurrentQuantity          int32   `json:"currentQuantity"`
	CurrentStock             int32   `json:"currentStock"`
	MaxQuantity              int32   `json:"maxQuantity"`
	Capacity                 int32   `json:"capacity"`
	ParLevel                 int32   `json:"parLevel"`
	LowStockThreshold        int32   `json:"lowStockThreshold"`
	PriceMinor               int64   `json:"priceMinor"`
	Currency                 string  `json:"currency"`
	Status                   string  `json:"status"`
	PlanogramRevisionApplied int32   `json:"planogramRevisionApplied"`
	UpdatedAt                string  `json:"updatedAt"`
	ProductID                *string `json:"productId,omitempty"`
	ProductSku               *string `json:"productSku,omitempty"`
	ProductName              *string `json:"productName,omitempty"`
	IsEmpty                  bool    `json:"isEmpty"`
	LowStock                 bool    `json:"lowStock"`
}

// V1AdminStockAdjustmentItem is one slot adjustment in POST /v1/admin/machines/{machineId}/stock-adjustments.
type V1AdminStockAdjustmentItem struct {
	PlanogramID    string  `json:"planogramId"`
	SlotIndex      int32   `json:"slotIndex"`
	QuantityBefore int32   `json:"quantityBefore"`
	QuantityAfter  int32   `json:"quantityAfter"`
	CabinetCode    string  `json:"cabinetCode,omitempty"`
	SlotCode       string  `json:"slotCode,omitempty"`
	ProductID      *string `json:"productId,omitempty"`
}

// V1AdminStockAdjustmentsRequest is POST /v1/admin/machines/{machineId}/stock-adjustments.
// See docs/api/inventory-adjustments.md and OpenAPI examples on that path.
type V1AdminStockAdjustmentsRequest struct {
	OperatorSessionID string                       `json:"operator_session_id"`
	Reason            string                       `json:"reason"`
	OccurredAt        *string                      `json:"occurredAt,omitempty"`
	Items             []V1AdminStockAdjustmentItem `json:"items"`
}

// V1AdminStockAdjustmentsResponse is the success body for stock adjustments.
type V1AdminStockAdjustmentsResponse struct {
	Replay   bool    `json:"replay"`
	EventIds []int64 `json:"eventIds,omitempty"`
}

// V1AdminInventoryEvent is one append-only inventory_events row (audit / refill / future vend).
type V1AdminInventoryEvent struct {
	ID                      int64   `json:"id"`
	OrganizationID          string  `json:"organizationId"`
	MachineID               string  `json:"machineId"`
	CabinetCode             *string `json:"cabinetCode,omitempty"`
	SlotCode                *string `json:"slotCode,omitempty"`
	ProductID               *string `json:"productId,omitempty"`
	EventType               string  `json:"eventType"`
	ReasonCode              *string `json:"reasonCode,omitempty"`
	QuantityBefore          *int32  `json:"quantityBefore,omitempty"`
	QuantityDelta           int32   `json:"quantityDelta"`
	QuantityAfter           *int32  `json:"quantityAfter,omitempty"`
	UnitPriceMinor          int64   `json:"unitPriceMinor"`
	Currency                string  `json:"currency"`
	CorrelationID           *string `json:"correlationId,omitempty"`
	OperatorSessionID       *string `json:"operatorSessionId,omitempty"`
	TechnicianID            *string `json:"technicianId,omitempty"`
	TechnicianDisplayName   *string `json:"technicianDisplayName,omitempty"`
	RefillSessionID         *string `json:"refillSessionId,omitempty"`
	InventoryCountSessionID *string `json:"inventoryCountSessionId,omitempty"`
	OccurredAt              string  `json:"occurredAt"`
	RecordedAt              string  `json:"recordedAt"`
}

// V1AdminInventoryEventListEnvelope is GET /v1/admin/machines/{machineId}/inventory-events.
type V1AdminInventoryEventListEnvelope struct {
	Items []V1AdminInventoryEvent `json:"items"`
}

// V1AdminMachineSlotListEnvelope is GET /v1/admin/machines/{machineId}/slots.
type V1AdminMachineSlotListEnvelope struct {
	Items []V1AdminMachineSlot `json:"items"`
}

// V1AdminMachineInventoryLine is a rolled-up inventory row per product.
type V1AdminMachineInventoryLine struct {
	MachineID          string  `json:"machineId"`
	MachineName        string  `json:"machineName"`
	MachineStatus      string  `json:"machineStatus"`
	ProductID          string  `json:"productId"`
	ProductName        string  `json:"productName"`
	ProductSku         string  `json:"productSku"`
	TotalQuantity      int64   `json:"totalQuantity"`
	SlotCount          int64   `json:"slotCount"`
	MaxCapacityAnySlot int32   `json:"maxCapacityAnySlot"`
	LowStock           bool    `json:"lowStock"`
	CabinetCode        *string `json:"cabinetCode,omitempty"`
	CabinetIndex       *int32  `json:"cabinetIndex,omitempty"`
}

// V1AdminMachineInventoryEnvelope is GET /v1/admin/machines/{machineId}/inventory.
type V1AdminMachineInventoryEnvelope struct {
	Items []V1AdminMachineInventoryLine `json:"items"`
}

// --- Machine setup (technician bootstrap + admin topology / planogram) ---

// V1SetupMachineBootstrapResponse is GET /v1/setup/machines/{machineId}/bootstrap.
// Integration notes: docs/api/setup-machine.md; copy-paste examples in docs/swagger/swagger.json.
type V1SetupMachineBootstrapResponse struct {
	Machine  V1SetupMachineSummary `json:"machine"`
	Topology V1SetupTopology       `json:"topology"`
	Catalog  V1SetupCatalog        `json:"catalog"`
}

// V1SetupMachineSummary is machine identity for setup clients.
type V1SetupMachineSummary struct {
	MachineID         string  `json:"machineId"`
	OrganizationID    string  `json:"organizationId"`
	SiteID            string  `json:"siteId"`
	HardwareProfileID *string `json:"hardwareProfileId,omitempty"`
	SerialNumber      string  `json:"serialNumber"`
	Name              string  `json:"name"`
	Status            string  `json:"status"`
	CommandSequence   int64   `json:"commandSequence"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
}

// V1SetupTopology is nested cabinets with current slot assignments.
type V1SetupTopology struct {
	Cabinets []V1SetupTopologyCabinet `json:"cabinets"`
}

// V1SetupTopologyCabinet is one cabinet and its current slots.
type V1SetupTopologyCabinet struct {
	ID        string                `json:"id"`
	Code      string                `json:"code"`
	Title     string                `json:"title"`
	SortOrder int32                 `json:"sortOrder"`
	Metadata  json.RawMessage       `json:"metadata,omitempty"`
	Slots     []V1SetupTopologySlot `json:"slots"`
}

// V1SetupTopologySlot is a current cabinet slot config row.
type V1SetupTopologySlot struct {
	ConfigID          string          `json:"configId"`
	SlotCode          string          `json:"slotCode"`
	SlotIndex         *int32          `json:"slotIndex,omitempty"`
	ProductID         *string         `json:"productId,omitempty"`
	ProductSku        string          `json:"productSku"`
	ProductName       string          `json:"productName"`
	MaxQuantity       int32           `json:"maxQuantity"`
	PriceMinor        int64           `json:"priceMinor"`
	EffectiveFrom     string          `json:"effectiveFrom"`
	IsCurrent         bool            `json:"isCurrent"`
	MachineSlotLayout string          `json:"machineSlotLayoutId"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
}

// V1SetupCatalog lists assortment products available for slot assignment on this machine.
type V1SetupCatalog struct {
	Products []V1SetupCatalogProduct `json:"products"`
}

// V1SetupCatalogProduct is one assortment line for the machine's primary binding.
type V1SetupCatalogProduct struct {
	ProductID      string `json:"productId"`
	Sku            string `json:"sku"`
	Name           string `json:"name"`
	SortOrder      int32  `json:"sortOrder"`
	AssortmentID   string `json:"assortmentId"`
	AssortmentName string `json:"assortmentName"`
}

// V1AdminPlanogramPublishResponse is POST /v1/admin/machines/{machineId}/planograms/publish.
type V1AdminPlanogramPublishResponse struct {
	DesiredConfigVersion int32                       `json:"desiredConfigVersion"`
	PlanogramID          string                      `json:"planogramId"`
	PlanogramRevision    int32                       `json:"planogramRevision"`
	Command              V1AdminPlanogramCommandInfo `json:"command"`
}

// V1AdminPlanogramCommandInfo summarizes the MQTT command ledger row after publish/sync dispatch.
type V1AdminPlanogramCommandInfo struct {
	CommandID     string `json:"commandId"`
	Sequence      int64  `json:"sequence"`
	DispatchState string `json:"dispatchState"`
	Replay        bool   `json:"replay"`
}

// V1AdminMachineSyncResponse is POST /v1/admin/machines/{machineId}/sync.
type V1AdminMachineSyncResponse struct {
	Command V1AdminPlanogramCommandInfo `json:"command"`
}

// --- Operational collection lists (GET /v1/orders, /v1/payments, /v1/admin/* lists) ---

// V1CollectionListMeta is shared pagination metadata (limit, offset, returned, total).
type V1CollectionListMeta struct {
	Limit    int32 `json:"limit"`
	Offset   int32 `json:"offset"`
	Returned int   `json:"returned"`
	Total    int64 `json:"total"`
}

// V1OrderListItem is one row in GET /v1/orders.
type V1OrderListItem struct {
	OrderID        string  `json:"orderId"`
	OrganizationID string  `json:"organizationId"`
	MachineID      string  `json:"machineId"`
	Status         string  `json:"status"`
	Currency       string  `json:"currency"`
	SubtotalMinor  int64   `json:"subtotalMinor"`
	TaxMinor       int64   `json:"taxMinor"`
	TotalMinor     int64   `json:"totalMinor"`
	IdempotencyKey *string `json:"idempotencyKey,omitempty"`
	CreatedAt      string  `json:"createdAt"`
	UpdatedAt      string  `json:"updatedAt"`
}

// V1OrdersListResponse is GET /v1/orders success body.
type V1OrdersListResponse struct {
	Items []V1OrderListItem    `json:"items"`
	Meta  V1CollectionListMeta `json:"meta"`
}

// V1PaymentListItem is one row in GET /v1/payments.
type V1PaymentListItem struct {
	PaymentID            string `json:"paymentId"`
	OrderID              string `json:"orderId"`
	OrganizationID       string `json:"organizationId"`
	MachineID            string `json:"machineId"`
	Provider             string `json:"provider"`
	PaymentState         string `json:"paymentState"`
	OrderStatus          string `json:"orderStatus"`
	AmountMinor          int64  `json:"amountMinor"`
	Currency             string `json:"currency"`
	ReconciliationStatus string `json:"reconciliationStatus"`
	SettlementStatus     string `json:"settlementStatus"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
}

// V1PaymentsListResponse is GET /v1/payments success body.
type V1PaymentsListResponse struct {
	Items []V1PaymentListItem  `json:"items"`
	Meta  V1CollectionListMeta `json:"meta"`
}

// V1AdminMachineInventorySummary is slot-derived counts for admin machine payloads.
type V1AdminMachineInventorySummary struct {
	TotalSlots      int64 `json:"totalSlots"`
	OccupiedSlots   int64 `json:"occupiedSlots"`
	LowStockSlots   int64 `json:"lowStockSlots"`
	OutOfStockSlots int64 `json:"outOfStockSlots"`
}

// V1AdminAssignedTechnician is an active technician–machine assignment.
type V1AdminAssignedTechnician struct {
	TechnicianID string  `json:"technicianId"`
	DisplayName  string  `json:"displayName"`
	Role         string  `json:"role"`
	ValidFrom    string  `json:"validFrom"`
	ValidTo      *string `json:"validTo,omitempty"`
}

// V1AdminCurrentOperator is the active operator session on a machine (if any).
type V1AdminCurrentOperator struct {
	SessionID             string  `json:"sessionId"`
	ActorType             string  `json:"actorType"`
	TechnicianID          *string `json:"technicianId,omitempty"`
	TechnicianDisplayName *string `json:"technicianDisplayName,omitempty"`
	UserPrincipal         *string `json:"userPrincipal,omitempty"`
	SessionStartedAt      string  `json:"sessionStartedAt"`
	SessionStatus         string  `json:"sessionStatus"`
	SessionExpiresAt      *string `json:"sessionExpiresAt,omitempty"`
}

// V1AdminMachineListItem is one machine in GET /v1/admin/machines and GET /v1/admin/machines/{machineId}.
type V1AdminMachineListItem struct {
	MachineID           string                         `json:"machineId"`
	MachineName         string                         `json:"machineName"`
	OrganizationID      string                         `json:"organizationId"`
	SiteID              string                         `json:"siteId"`
	SiteName            string                         `json:"siteName"`
	HardwareProfileID   *string                        `json:"hardwareProfileId,omitempty"`
	SerialNumber        string                         `json:"serialNumber"`
	Name                string                         `json:"name"`
	Status              string                         `json:"status"`
	CommandSequence     int64                          `json:"commandSequence"`
	CreatedAt           string                         `json:"createdAt"`
	UpdatedAt           string                         `json:"updatedAt"`
	AndroidID           *string                        `json:"androidId,omitempty"`
	SimSerial           *string                        `json:"simSerial,omitempty"`
	SimIccid            *string                        `json:"simIccid,omitempty"`
	AppVersion          *string                        `json:"appVersion,omitempty"`
	FirmwareVersion     *string                        `json:"firmwareVersion,omitempty"`
	LastHeartbeatAt     *string                        `json:"lastHeartbeatAt,omitempty"`
	EffectiveTimezone   string                         `json:"effectiveTimezone"`
	AssignedTechnicians []V1AdminAssignedTechnician    `json:"assignedTechnicians"`
	CurrentOperator     *V1AdminCurrentOperator        `json:"currentOperator"`
	InventorySummary    V1AdminMachineInventorySummary `json:"inventorySummary"`
}

// V1MachineTelemetrySnapshotResponse is GET /v1/machines/{machineId}/telemetry/snapshot.
// All timestamps are RFC3339Nano strings with explicit timezone offset (responses use UTC, "Z").
type V1MachineTelemetrySnapshotResponse struct {
	MachineID         string          `json:"machineId"`
	OrganizationID    string          `json:"organizationId"`
	SiteID            string          `json:"siteId"`
	ReportedState     json.RawMessage `json:"reportedState"`
	MetricsState      json.RawMessage `json:"metricsState"`
	LastHeartbeatAt   *string         `json:"lastHeartbeatAt,omitempty"`
	AppVersion        *string         `json:"appVersion,omitempty"`
	FirmwareVersion   *string         `json:"firmwareVersion,omitempty"`
	UpdatedAt         string          `json:"updatedAt"`
	AndroidID         *string         `json:"androidId,omitempty"`
	SimSerial         *string         `json:"simSerial,omitempty"`
	SimIccid          *string         `json:"simIccid,omitempty"`
	DeviceModel       *string         `json:"deviceModel,omitempty"`
	OSVersion         *string         `json:"osVersion,omitempty"`
	LastIdentityAt    *string         `json:"lastIdentityAt,omitempty"`
	EffectiveTimezone string          `json:"effectiveTimezone"`
}

// V1MachineTelemetryIncidentItem is one element of GET /v1/machines/{machineId}/telemetry/incidents items.
type V1MachineTelemetryIncidentItem struct {
	ID        string          `json:"id"`
	Severity  string          `json:"severity"`
	Code      string          `json:"code"`
	Title     *string         `json:"title,omitempty"`
	Detail    json.RawMessage `json:"detail"`
	DedupeKey *string         `json:"dedupeKey,omitempty"`
	OpenedAt  string          `json:"openedAt"`
	UpdatedAt string          `json:"updatedAt"`
}

// V1MachineTelemetryIncidentsMeta is the meta object for telemetry incidents.
type V1MachineTelemetryIncidentsMeta struct {
	Limit    int32 `json:"limit"`
	Returned int   `json:"returned"`
}

// V1MachineTelemetryIncidentsResponse is GET /v1/machines/{machineId}/telemetry/incidents.
type V1MachineTelemetryIncidentsResponse struct {
	Items []V1MachineTelemetryIncidentItem `json:"items"`
	Meta  V1MachineTelemetryIncidentsMeta  `json:"meta"`
}

// V1MachineTelemetryRollupItem is one telemetry rollup bucket row.
type V1MachineTelemetryRollupItem struct {
	BucketStart string          `json:"bucketStart"`
	Granularity string          `json:"granularity"`
	MetricKey   string          `json:"metricKey"`
	SampleCount int64           `json:"sampleCount"`
	Sum         *float64        `json:"sum,omitempty"`
	Min         *float64        `json:"min,omitempty"`
	Max         *float64        `json:"max,omitempty"`
	Last        *float64        `json:"last,omitempty"`
	Extra       json.RawMessage `json:"extra"`
}

// V1MachineTelemetryRollupsMeta documents the window and query echo for rollup listing.
type V1MachineTelemetryRollupsMeta struct {
	Granularity string `json:"granularity"`
	From        string `json:"from"`
	To          string `json:"to"`
	Returned    int    `json:"returned"`
	Note        string `json:"note"`
}

// V1MachineTelemetryRollupsResponse is GET /v1/machines/{machineId}/telemetry/rollups.
type V1MachineTelemetryRollupsResponse struct {
	Items []V1MachineTelemetryRollupItem `json:"items"`
	Meta  V1MachineTelemetryRollupsMeta  `json:"meta"`
}

// V1AdminMachinesListResponse is GET /v1/admin/machines success body.
type V1AdminMachinesListResponse struct {
	Items []V1AdminMachineListItem `json:"items"`
	Meta  V1CollectionListMeta     `json:"meta"`
}

// V1AdminTechnicianListItem is one technician in GET /v1/admin/technicians.
type V1AdminTechnicianListItem struct {
	TechnicianID    string  `json:"technicianId"`
	OrganizationID  string  `json:"organizationId"`
	DisplayName     string  `json:"displayName"`
	Email           *string `json:"email,omitempty"`
	Phone           *string `json:"phone,omitempty"`
	ExternalSubject *string `json:"externalSubject,omitempty"`
	CreatedAt       string  `json:"createdAt"`
}

// V1AdminTechniciansListResponse is GET /v1/admin/technicians success body.
type V1AdminTechniciansListResponse struct {
	Items []V1AdminTechnicianListItem `json:"items"`
	Meta  V1CollectionListMeta        `json:"meta"`
}

// V1AdminAssignmentListItem is one assignment in GET /v1/admin/assignments.
type V1AdminAssignmentListItem struct {
	AssignmentID          string  `json:"assignmentId"`
	TechnicianID          string  `json:"technicianId"`
	TechnicianDisplayName string  `json:"technicianDisplayName"`
	MachineID             string  `json:"machineId"`
	MachineName           string  `json:"machineName"`
	MachineSerialNumber   string  `json:"machineSerialNumber"`
	Role                  string  `json:"role"`
	ValidFrom             string  `json:"validFrom"`
	ValidTo               *string `json:"validTo,omitempty"`
	CreatedAt             string  `json:"createdAt"`
}

// V1AdminAssignmentsListResponse is GET /v1/admin/assignments success body.
type V1AdminAssignmentsListResponse struct {
	Items []V1AdminAssignmentListItem `json:"items"`
	Meta  V1CollectionListMeta        `json:"meta"`
}

// V1AdminCommandListItem is one command in GET /v1/admin/commands.
type V1AdminCommandListItem struct {
	CommandID           string  `json:"commandId"`
	MachineID           string  `json:"machineId"`
	OrganizationID      string  `json:"organizationId"`
	MachineName         string  `json:"machineName"`
	MachineSerialNumber string  `json:"machineSerialNumber"`
	Sequence            int64   `json:"sequence"`
	CommandType         string  `json:"commandType"`
	CreatedAt           string  `json:"createdAt"`
	AttemptCount        int32   `json:"attemptCount"`
	LatestAttemptStatus string  `json:"latestAttemptStatus"`
	CorrelationID       *string `json:"correlationId,omitempty"`
}

// V1AdminCommandsListResponse is GET /v1/admin/commands success body.
type V1AdminCommandsListResponse struct {
	Items []V1AdminCommandListItem `json:"items"`
	Meta  V1CollectionListMeta     `json:"meta"`
}

// V1AdminOTAListItem is one OTA campaign in GET /v1/admin/ota.
type V1AdminOTAListItem struct {
	CampaignID         string  `json:"campaignId"`
	OrganizationID     string  `json:"organizationId"`
	CampaignName       string  `json:"campaignName"`
	Strategy           string  `json:"strategy"`
	CampaignStatus     string  `json:"campaignStatus"`
	CreatedAt          string  `json:"createdAt"`
	ArtifactID         string  `json:"artifactId"`
	ArtifactSemver     *string `json:"artifactSemver,omitempty"`
	ArtifactStorageKey string  `json:"artifactStorageKey"`
}

// V1AdminOTAListResponse is GET /v1/admin/ota success body.
type V1AdminOTAListResponse struct {
	Items []V1AdminOTAListItem `json:"items"`
	Meta  V1CollectionListMeta `json:"meta"`
}

// V1CashDenominationExpectation is an optional breakdown hint (not hardware-sourced today).
type V1CashDenominationExpectation struct {
	DenominationMinor int64  `json:"denominationMinor"`
	ExpectedCount     int64  `json:"expectedCount"`
	Source            string `json:"source"` // e.g. bill_recycler, vault_model
}

// V1AdminMachineCashboxResponse is GET /v1/admin/machines/{machineId}/cashbox.
type V1AdminMachineCashboxResponse struct {
	MachineID                    string                          `json:"machineId"`
	Currency                     string                          `json:"currency"`
	ExpectedCashboxMinor         int64                           `json:"expectedCashboxMinor"` // legacy alias; same as ExpectedCloudCashMinor
	ExpectedCloudCashMinor       int64                           `json:"expectedCloudCashMinor"`
	ExpectedRecyclerMinor        int64                           `json:"expectedRecyclerMinor"`
	LastCollectionAt             *string                         `json:"lastCollectionAt,omitempty"`
	Denominations                []V1CashDenominationExpectation `json:"denominations"`
	OpenCollectionID             *string                         `json:"openCollectionId,omitempty"`
	VarianceReviewThresholdMinor int64                           `json:"varianceReviewThresholdMinor"`
	Disclosure                   string                          `json:"disclosure"`
}

// V1AdminCashCollection is one cash collection session row (open or closed).
type V1AdminCashCollection struct {
	ID                       string  `json:"id"`
	MachineID                string  `json:"machine_id"`
	OrganizationID           string  `json:"organization_id"`
	CollectedAt              string  `json:"collected_at"`
	OpenedAt                 string  `json:"opened_at"`
	ClosedAt                 *string `json:"closed_at,omitempty"`
	LifecycleStatus          string  `json:"lifecycle_status"`
	CountedAmountMinor       int64   `json:"counted_amount_minor"`
	ExpectedAmountMinor      int64   `json:"expected_amount_minor"`
	VarianceAmountMinor      int64   `json:"variance_amount_minor"`
	CountedPhysicalCashMinor int64   `json:"countedPhysicalCashMinor"`
	ExpectedCloudCashMinor   int64   `json:"expectedCloudCashMinor"`
	VarianceMinor            int64   `json:"varianceMinor"`
	ReviewState              string  `json:"reviewState"`
	RequiresReview           bool    `json:"requires_review"`
	CloseRequestHashHex      *string `json:"close_request_hash_hex,omitempty"`
	Currency                 string  `json:"currency"`
	ReconciliationStatus     string  `json:"reconciliation_status"`
	Disclosure               string  `json:"disclosure"`
}

// V1AdminCashCollectionListResponse is GET /v1/admin/machines/{machineId}/cash-collections.
type V1AdminCashCollectionListResponse struct {
	Items []V1AdminCashCollection `json:"items"`
	Meta  V1CollectionListMeta    `json:"meta"`
}
