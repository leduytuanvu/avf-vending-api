package db

import (
	"time"

	"github.com/google/uuid"
)

type Organization struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Region struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Name           string    `json:"name"`
	Code           string    `json:"code"`
	CreatedAt      time.Time `json:"created_at"`
}

type Site struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	RegionID       *uuid.UUID `json:"region_id"`
	Name           string     `json:"name"`
	Address        []byte     `json:"address"`
	CreatedAt      time.Time  `json:"created_at"`
}

type MachineHardwareProfile struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID *uuid.UUID `json:"organization_id"`
	Name           string     `json:"name"`
	Spec           []byte     `json:"spec"`
	CreatedAt      time.Time  `json:"created_at"`
}

type Machine struct {
	ID                uuid.UUID  `json:"id"`
	OrganizationID    uuid.UUID  `json:"organization_id"`
	SiteID            uuid.UUID  `json:"site_id"`
	HardwareProfileID *uuid.UUID `json:"hardware_profile_id"`
	SerialNumber      string     `json:"serial_number"`
	Name              string     `json:"name"`
	Status            string     `json:"status"`
	CommandSequence   int64      `json:"command_sequence"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type Technician struct {
	ID              uuid.UUID `json:"id"`
	OrganizationID  uuid.UUID `json:"organization_id"`
	DisplayName     string    `json:"display_name"`
	Email           *string   `json:"email"`
	Phone           *string   `json:"phone"`
	ExternalSubject *string   `json:"external_subject"`
	CreatedAt       time.Time `json:"created_at"`
}

type TechnicianMachineAssignment struct {
	ID           uuid.UUID  `json:"id"`
	TechnicianID uuid.UUID  `json:"technician_id"`
	MachineID    uuid.UUID  `json:"machine_id"`
	Role         string     `json:"role"`
	ValidFrom    time.Time  `json:"valid_from"`
	ValidTo      *time.Time `json:"valid_to"`
	CreatedAt    time.Time  `json:"created_at"`
}

type Product struct {
	ID              uuid.UUID  `json:"id"`
	OrganizationID  uuid.UUID  `json:"organization_id"`
	Sku             string     `json:"sku"`
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	Attrs           []byte     `json:"attrs"`
	Active          bool       `json:"active"`
	CategoryID      *uuid.UUID `json:"category_id"`
	BrandID         *uuid.UUID `json:"brand_id"`
	PrimaryImageID  *uuid.UUID `json:"primary_image_id"`
	CountryOfOrigin *string    `json:"country_of_origin"`
	AgeRestricted   bool       `json:"age_restricted"`
	AllergenCodes   *[]string  `json:"allergen_codes"`
	NutritionalNote *string    `json:"nutritional_note"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Category struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	Slug           string     `json:"slug"`
	Name           string     `json:"name"`
	ParentID       *uuid.UUID `json:"parent_id"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type Brand struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Slug           string    `json:"slug"`
	Name           string    `json:"name"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ProductImage struct {
	ID         uuid.UUID `json:"id"`
	ProductID  uuid.UUID `json:"product_id"`
	StorageKey string    `json:"storage_key"`
	CdnURL     *string   `json:"cdn_url"`
	AltText    string    `json:"alt_text"`
	SortOrder  int32     `json:"sort_order"`
	IsPrimary  bool      `json:"is_primary"`
	CreatedAt  time.Time `json:"created_at"`
}

type PriceBook struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	Name           string     `json:"name"`
	Currency       string     `json:"currency"`
	EffectiveFrom  time.Time  `json:"effective_from"`
	EffectiveTo    *time.Time `json:"effective_to"`
	IsDefault      bool       `json:"is_default"`
	ScopeType      string     `json:"scope_type"`
	SiteID         *uuid.UUID `json:"site_id"`
	MachineID      *uuid.UUID `json:"machine_id"`
	Priority       int32      `json:"priority"`
	CreatedAt      time.Time  `json:"created_at"`
}

type PriceBookItem struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	PriceBookID    uuid.UUID `json:"price_book_id"`
	ProductID      uuid.UUID `json:"product_id"`
	UnitPriceMinor int64     `json:"unit_price_minor"`
	CreatedAt      time.Time `json:"created_at"`
}

type MachinePriceOverride struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	MachineID      uuid.UUID  `json:"machine_id"`
	ProductID      uuid.UUID  `json:"product_id"`
	UnitPriceMinor int64      `json:"unit_price_minor"`
	Currency       string     `json:"currency"`
	ValidFrom      time.Time  `json:"valid_from"`
	ValidTo        *time.Time `json:"valid_to"`
	CreatedAt      time.Time  `json:"created_at"`
}

type Promotion struct {
	ID               uuid.UUID `json:"id"`
	OrganizationID   uuid.UUID `json:"organization_id"`
	Name             string    `json:"name"`
	ApprovalStatus   string    `json:"approval_status"`
	StartsAt         time.Time `json:"starts_at"`
	EndsAt           time.Time `json:"ends_at"`
	BudgetLimitMinor *int64    `json:"budget_limit_minor"`
	RedemptionLimit  *int32    `json:"redemption_limit"`
	ChannelScope     *string   `json:"channel_scope"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type PromotionRule struct {
	ID          uuid.UUID `json:"id"`
	PromotionID uuid.UUID `json:"promotion_id"`
	RuleType    string    `json:"rule_type"`
	Payload     []byte    `json:"payload"`
	Priority    int32     `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
}

type PromotionTarget struct {
	ID                   uuid.UUID  `json:"id"`
	PromotionID          uuid.UUID  `json:"promotion_id"`
	OrganizationID       uuid.UUID  `json:"organization_id"`
	TargetType           string     `json:"target_type"`
	ProductID            *uuid.UUID `json:"product_id"`
	CategoryID           *uuid.UUID `json:"category_id"`
	MachineID            *uuid.UUID `json:"machine_id"`
	SiteID               *uuid.UUID `json:"site_id"`
	OrganizationTargetID *uuid.UUID `json:"organization_target_id"`
	CreatedAt            time.Time  `json:"created_at"`
}

type Planogram struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Name           string    `json:"name"`
	Revision       int32     `json:"revision"`
	Status         string    `json:"status"`
	Meta           []byte    `json:"meta"`
	CreatedAt      time.Time `json:"created_at"`
}

type Slot struct {
	ID          uuid.UUID  `json:"id"`
	PlanogramID uuid.UUID  `json:"planogram_id"`
	SlotIndex   int32      `json:"slot_index"`
	ProductID   *uuid.UUID `json:"product_id"`
	MaxQuantity int32      `json:"max_quantity"`
	CreatedAt   time.Time  `json:"created_at"`
}

type MachineSlotState struct {
	ID                       uuid.UUID `json:"id"`
	MachineID                uuid.UUID `json:"machine_id"`
	PlanogramID              uuid.UUID `json:"planogram_id"`
	SlotIndex                int32     `json:"slot_index"`
	CurrentQuantity          int32     `json:"current_quantity"`
	PriceMinor               int64     `json:"price_minor"`
	PlanogramRevisionApplied int32     `json:"planogram_revision_applied"`
	UpdatedAt                time.Time `json:"updated_at"`
}

type Order struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	MachineID      uuid.UUID `json:"machine_id"`
	Status         string    `json:"status"`
	Currency       string    `json:"currency"`
	SubtotalMinor  int64     `json:"subtotal_minor"`
	TaxMinor       int64     `json:"tax_minor"`
	TotalMinor     int64     `json:"total_minor"`
	IdempotencyKey *string   `json:"idempotency_key"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type VendSession struct {
	ID                    uuid.UUID  `json:"id"`
	OrderID               uuid.UUID  `json:"order_id"`
	MachineID             uuid.UUID  `json:"machine_id"`
	SlotIndex             int32      `json:"slot_index"`
	ProductID             uuid.UUID  `json:"product_id"`
	State                 string     `json:"state"`
	FailureReason         *string    `json:"failure_reason"`
	CorrelationID         *uuid.UUID `json:"correlation_id"`
	StartedAt             *time.Time `json:"started_at"`
	CompletedAt           *time.Time `json:"completed_at"`
	FinalCommandAttemptID *uuid.UUID `json:"final_command_attempt_id"`
	CreatedAt             time.Time  `json:"created_at"`
}

type Payment struct {
	ID                   uuid.UUID  `json:"id"`
	OrderID              uuid.UUID  `json:"order_id"`
	Provider             string     `json:"provider"`
	State                string     `json:"state"`
	AmountMinor          int64      `json:"amount_minor"`
	Currency             string     `json:"currency"`
	IdempotencyKey       *string    `json:"idempotency_key"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	ReconciliationStatus string     `json:"reconciliation_status"`
	SettlementStatus     string     `json:"settlement_status"`
	SettlementBatchID    *uuid.UUID `json:"settlement_batch_id"`
}

type PaymentAttempt struct {
	ID                uuid.UUID `json:"id"`
	PaymentID         uuid.UUID `json:"payment_id"`
	ProviderReference *string   `json:"provider_reference"`
	State             string    `json:"state"`
	Payload           []byte    `json:"payload"`
	CreatedAt         time.Time `json:"created_at"`
}

type Refund struct {
	ID                   uuid.UUID  `json:"id"`
	PaymentID            uuid.UUID  `json:"payment_id"`
	OrderID              uuid.UUID  `json:"order_id"`
	AmountMinor          int64      `json:"amount_minor"`
	State                string     `json:"state"`
	Reason               *string    `json:"reason"`
	CreatedAt            time.Time  `json:"created_at"`
	ReconciliationStatus string     `json:"reconciliation_status"`
	SettlementStatus     string     `json:"settlement_status"`
	SettlementBatchID    *uuid.UUID `json:"settlement_batch_id"`
}

type SettlementBatch struct {
	ID          uuid.UUID `json:"id"`
	Provider    string    `json:"provider"`
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	Status      string    `json:"status"`
	Metadata    []byte    `json:"metadata"`
	CreatedAt   time.Time `json:"created_at"`
}

type MachineReconciliationSession struct {
	ID                         uuid.UUID  `json:"id"`
	MachineID                  uuid.UUID  `json:"machine_id"`
	BusinessDate               time.Time  `json:"business_date"`
	OpenedAt                   time.Time  `json:"opened_at"`
	ClosedAt                   *time.Time `json:"closed_at"`
	ExpectedCashAmountMinor    int64      `json:"expected_cash_amount_minor"`
	ActualCashAmountMinor      int64      `json:"actual_cash_amount_minor"`
	ExpectedDigitalAmountMinor int64      `json:"expected_digital_amount_minor"`
	ActualDigitalAmountMinor   int64      `json:"actual_digital_amount_minor"`
	VarianceAmountMinor        int64      `json:"variance_amount_minor"`
	Status                     string     `json:"status"`
}

type CashCollection struct {
	ID                   uuid.UUID  `json:"id"`
	OrganizationID       uuid.UUID  `json:"organization_id"`
	MachineID            uuid.UUID  `json:"machine_id"`
	CollectedAt          time.Time  `json:"collected_at"`
	AmountMinor          int64      `json:"amount_minor"`
	Currency             string     `json:"currency"`
	Metadata             []byte     `json:"metadata"`
	ReconciliationStatus string     `json:"reconciliation_status"`
	ReconciledBy         *string    `json:"reconciled_by"`
	ReconciledAt         *time.Time `json:"reconciled_at"`
	CreatedAt            time.Time  `json:"created_at"`
	OperatorSessionID    *uuid.UUID `json:"operator_session_id"`
}

type RefillSession struct {
	ID                uuid.UUID  `json:"id"`
	OrganizationID    uuid.UUID  `json:"organization_id"`
	MachineID         uuid.UUID  `json:"machine_id"`
	StartedAt         time.Time  `json:"started_at"`
	EndedAt           *time.Time `json:"ended_at"`
	OperatorSessionID *uuid.UUID `json:"operator_session_id"`
	Metadata          []byte     `json:"metadata"`
	CreatedAt         time.Time  `json:"created_at"`
}

type MachineConfig struct {
	ID                uuid.UUID  `json:"id"`
	OrganizationID    uuid.UUID  `json:"organization_id"`
	MachineID         uuid.UUID  `json:"machine_id"`
	AppliedAt         time.Time  `json:"applied_at"`
	ConfigRevision    int32      `json:"config_revision"`
	ConfigPayload     []byte     `json:"config_payload"`
	OperatorSessionID *uuid.UUID `json:"operator_session_id"`
	Metadata          []byte     `json:"metadata"`
	CreatedAt         time.Time  `json:"created_at"`
}

type Incident struct {
	ID                uuid.UUID  `json:"id"`
	OrganizationID    uuid.UUID  `json:"organization_id"`
	MachineID         uuid.UUID  `json:"machine_id"`
	Status            string     `json:"status"`
	Title             string     `json:"title"`
	OpenedAt          time.Time  `json:"opened_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	OperatorSessionID *uuid.UUID `json:"operator_session_id"`
	Metadata          []byte     `json:"metadata"`
}

type CashEvent struct {
	ID                      int64      `json:"id"`
	OrganizationID          uuid.UUID  `json:"organization_id"`
	MachineID               uuid.UUID  `json:"machine_id"`
	EventType               string     `json:"event_type"`
	AmountMinor             int64      `json:"amount_minor"`
	Currency                string     `json:"currency"`
	OccurredAt              time.Time  `json:"occurred_at"`
	CorrelationID           *uuid.UUID `json:"correlation_id"`
	Metadata                []byte     `json:"metadata"`
	ReconciliationSessionID *uuid.UUID `json:"reconciliation_session_id"`
}

type PaymentProviderEvent struct {
	ID                  int64      `json:"id"`
	PaymentID           *uuid.UUID `json:"payment_id"`
	Provider            string     `json:"provider"`
	ProviderRef         *string    `json:"provider_ref"`
	ProviderAmountMinor *int64     `json:"provider_amount_minor"`
	Currency            *string    `json:"currency"`
	EventType           string     `json:"event_type"`
	Payload             []byte     `json:"payload"`
	ReceivedAt          time.Time  `json:"received_at"`
}

type PaymentReconciliation struct {
	ID                  uuid.UUID `json:"id"`
	PaymentID           uuid.UUID `json:"payment_id"`
	Provider            string    `json:"provider"`
	ProviderRef         string    `json:"provider_ref"`
	ProviderAmountMinor int64     `json:"provider_amount_minor"`
	InternalAmountMinor int64     `json:"internal_amount_minor"`
	Currency            string    `json:"currency"`
	ReconciledAt        time.Time `json:"reconciled_at"`
	Status              string    `json:"status"`
	MismatchReason      *string   `json:"mismatch_reason"`
}

type CashReconciliation struct {
	ID                  uuid.UUID  `json:"id"`
	MachineID           uuid.UUID  `json:"machine_id"`
	CashSessionID       *uuid.UUID `json:"cash_session_id"`
	CashCollectionID    *uuid.UUID `json:"cash_collection_id"`
	ExpectedAmountMinor int64      `json:"expected_amount_minor"`
	CountedAmountMinor  int64      `json:"counted_amount_minor"`
	VarianceAmountMinor int64      `json:"variance_amount_minor"`
	ReconciledAt        time.Time  `json:"reconciled_at"`
	Status              string     `json:"status"`
	Metadata            []byte     `json:"metadata"`
}

type FinancialLedgerEntry struct {
	ID                int64      `json:"id"`
	OrganizationID    uuid.UUID  `json:"organization_id"`
	MachineID         *uuid.UUID `json:"machine_id"`
	SiteID            *uuid.UUID `json:"site_id"`
	OrderID           *uuid.UUID `json:"order_id"`
	PaymentID         *uuid.UUID `json:"payment_id"`
	RefundID          *uuid.UUID `json:"refund_id"`
	CashEventID       *int64     `json:"cash_event_id"`
	CashCollectionID  *uuid.UUID `json:"cash_collection_id"`
	EntryType         string     `json:"entry_type"`
	SignedAmountMinor int64      `json:"signed_amount_minor"`
	Currency          string     `json:"currency"`
	OccurredAt        time.Time  `json:"occurred_at"`
	ReferenceType     *string    `json:"reference_type"`
	ReferenceID       *uuid.UUID `json:"reference_id"`
	CorrelationID     *uuid.UUID `json:"correlation_id"`
	Metadata          []byte     `json:"metadata"`
}

type CommandLedger struct {
	ID                uuid.UUID  `json:"id"`
	MachineID         uuid.UUID  `json:"machine_id"`
	Sequence          int64      `json:"sequence"`
	CommandType       string     `json:"command_type"`
	Payload           []byte     `json:"payload"`
	CorrelationID     *uuid.UUID `json:"correlation_id"`
	IdempotencyKey    *string    `json:"idempotency_key"`
	CreatedAt         time.Time  `json:"created_at"`
	ProtocolType      *string    `json:"protocol_type"`
	DeadlineAt        *time.Time `json:"deadline_at"`
	TimeoutAt         *time.Time `json:"timeout_at"`
	AttemptCount      int32      `json:"attempt_count"`
	LastAttemptAt     *time.Time `json:"last_attempt_at"`
	RouteKey          *string    `json:"route_key"`
	SourceSystem      *string    `json:"source_system"`
	SourceEventID     *string    `json:"source_event_id"`
	OperatorSessionID *uuid.UUID `json:"operator_session_id"`
}

type MachineShadow struct {
	MachineID     uuid.UUID `json:"machine_id"`
	DesiredState  []byte    `json:"desired_state"`
	ReportedState []byte    `json:"reported_state"`
	Version       int64     `json:"version"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type OutboxEvent struct {
	ID                   int64      `json:"id"`
	OrganizationID       *uuid.UUID `json:"organization_id"`
	Topic                string     `json:"topic"`
	EventType            string     `json:"event_type"`
	Payload              []byte     `json:"payload"`
	AggregateType        string     `json:"aggregate_type"`
	AggregateID          uuid.UUID  `json:"aggregate_id"`
	IdempotencyKey       *string    `json:"idempotency_key"`
	CreatedAt            time.Time  `json:"created_at"`
	PublishedAt          *time.Time `json:"published_at"`
	PublishAttemptCount  int32      `json:"publish_attempt_count"`
	LastPublishError     *string    `json:"last_publish_error"`
	LastPublishAttemptAt *time.Time `json:"last_publish_attempt_at"`
	NextPublishAfter     *time.Time `json:"next_publish_after"`
	DeadLetteredAt       *time.Time `json:"dead_lettered_at"`
}

type AuditLog struct {
	ID             int64      `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	ActorType      string     `json:"actor_type"`
	ActorID        string     `json:"actor_id"`
	Action         string     `json:"action"`
	ResourceType   string     `json:"resource_type"`
	ResourceID     *uuid.UUID `json:"resource_id"`
	Payload        []byte     `json:"payload"`
	Ip             *string    `json:"ip"`
	CreatedAt      time.Time  `json:"created_at"`
}

type OtaArtifact struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	StorageKey     string    `json:"storage_key"`
	Sha256         *string   `json:"sha256"`
	SizeBytes      *int64    `json:"size_bytes"`
	Semver         *string   `json:"semver"`
	CreatedAt      time.Time `json:"created_at"`
}

type OtaCampaign struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Name           string    `json:"name"`
	ArtifactID     uuid.UUID `json:"artifact_id"`
	Strategy       string    `json:"strategy"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

type OtaTarget struct {
	ID         uuid.UUID `json:"id"`
	CampaignID uuid.UUID `json:"campaign_id"`
	MachineID  uuid.UUID `json:"machine_id"`
	State      string    `json:"state"`
	LastError  *string   `json:"last_error"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type DeviceTelemetryEvent struct {
	ID         int64     `json:"id"`
	MachineID  uuid.UUID `json:"machine_id"`
	EventType  string    `json:"event_type"`
	Payload    []byte    `json:"payload"`
	DedupeKey  *string   `json:"dedupe_key"`
	ReceivedAt time.Time `json:"received_at"`
}

type DeviceCommandReceipt struct {
	ID               int64      `json:"id"`
	MachineID        uuid.UUID  `json:"machine_id"`
	Sequence         int64      `json:"sequence"`
	Status           string     `json:"status"`
	CorrelationID    *uuid.UUID `json:"correlation_id"`
	Payload          []byte     `json:"payload"`
	DedupeKey        string     `json:"dedupe_key"`
	ReceivedAt       time.Time  `json:"received_at"`
	CommandAttemptID *uuid.UUID `json:"command_attempt_id"`
}

type MachineModule struct {
	ID         uuid.UUID `json:"id"`
	MachineID  uuid.UUID `json:"machine_id"`
	ModuleKind string    `json:"module_kind"`
	ModuleCode *string   `json:"module_code"`
	Metadata   []byte    `json:"metadata"`
	CreatedAt  time.Time `json:"created_at"`
}

type MachineTransportSession struct {
	ID               uuid.UUID  `json:"id"`
	MachineID        uuid.UUID  `json:"machine_id"`
	ProtocolType     string     `json:"protocol_type"`
	TransportType    string     `json:"transport_type"`
	ClientID         *string    `json:"client_id"`
	BridgeID         *string    `json:"bridge_id"`
	ConnectedAt      time.Time  `json:"connected_at"`
	DisconnectedAt   *time.Time `json:"disconnected_at"`
	DisconnectReason *string    `json:"disconnect_reason"`
	SessionMetadata  []byte     `json:"session_metadata"`
}

type MachineCommandAttempt struct {
	ID                 uuid.UUID  `json:"id"`
	CommandID          uuid.UUID  `json:"command_id"`
	MachineID          uuid.UUID  `json:"machine_id"`
	TransportSessionID *uuid.UUID `json:"transport_session_id"`
	AttemptNo          int32      `json:"attempt_no"`
	SentAt             time.Time  `json:"sent_at"`
	AckDeadlineAt      *time.Time `json:"ack_deadline_at"`
	AckedAt            *time.Time `json:"acked_at"`
	ResultReceivedAt   *time.Time `json:"result_received_at"`
	Status             string     `json:"status"`
	TimeoutReason      *string    `json:"timeout_reason"`
	ProtocolPackNo     *int64     `json:"protocol_pack_no"`
	SequenceNo         *int64     `json:"sequence_no"`
	CorrelationID      *uuid.UUID `json:"correlation_id"`
	RequestPayloadJSON []byte     `json:"request_payload_json"`
	RawRequest         []byte     `json:"raw_request"`
	RawResponse        []byte     `json:"raw_response"`
	LatencyMs          *int32     `json:"latency_ms"`
}

type MachineOperatorSession struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	MachineID      uuid.UUID  `json:"machine_id"`
	ActorType      string     `json:"actor_type"`
	TechnicianID   *uuid.UUID `json:"technician_id"`
	UserPrincipal  *string    `json:"user_principal"`
	Status         string     `json:"status"`
	StartedAt      time.Time  `json:"started_at"`
	EndedAt        *time.Time `json:"ended_at"`
	ExpiresAt      *time.Time `json:"expires_at"`
	ClientMetadata []byte     `json:"client_metadata"`
	LastActivityAt time.Time  `json:"last_activity_at"`
	EndedReason    *string    `json:"ended_reason"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type MachineOperatorAuthEvent struct {
	ID                int64      `json:"id"`
	OperatorSessionID *uuid.UUID `json:"operator_session_id"`
	MachineID         uuid.UUID  `json:"machine_id"`
	EventType         string     `json:"event_type"`
	AuthMethod        string     `json:"auth_method"`
	OccurredAt        time.Time  `json:"occurred_at"`
	CorrelationID     *uuid.UUID `json:"correlation_id"`
	Metadata          []byte     `json:"metadata"`
}

type MachineActionAttribution struct {
	ID                int64      `json:"id"`
	OperatorSessionID *uuid.UUID `json:"operator_session_id"`
	MachineID         uuid.UUID  `json:"machine_id"`
	ActionOriginType  string     `json:"action_origin_type"`
	ResourceType      string     `json:"resource_type"`
	ResourceID        string     `json:"resource_id"`
	OccurredAt        time.Time  `json:"occurred_at"`
	Metadata          []byte     `json:"metadata"`
	CorrelationID     *uuid.UUID `json:"correlation_id"`
}

type VMachineCurrentOperator struct {
	MachineID             uuid.UUID  `json:"machine_id"`
	OrganizationID        uuid.UUID  `json:"organization_id"`
	OperatorSessionID     *uuid.UUID `json:"operator_session_id"`
	ActorType             *string    `json:"actor_type"`
	TechnicianID          *uuid.UUID `json:"technician_id"`
	TechnicianDisplayName *string    `json:"technician_display_name"`
	UserPrincipal         *string    `json:"user_principal"`
	SessionStartedAt      *time.Time `json:"session_started_at"`
	SessionStatus         *string    `json:"session_status"`
	SessionExpiresAt      *time.Time `json:"session_expires_at"`
}

type DeviceMessagesRaw struct {
	ID                 int64      `json:"id"`
	MachineID          uuid.UUID  `json:"machine_id"`
	ModuleID           *uuid.UUID `json:"module_id"`
	TransportSessionID *uuid.UUID `json:"transport_session_id"`
	Direction          string     `json:"direction"`
	ProtocolType       string     `json:"protocol_type"`
	MessageType        string     `json:"message_type"`
	CorrelationID      *uuid.UUID `json:"correlation_id"`
	PackNo             *int64     `json:"pack_no"`
	SequenceNo         *int64     `json:"sequence_no"`
	PayloadJSON        []byte     `json:"payload_json"`
	RawPayload         []byte     `json:"raw_payload"`
	MessageHash        []byte     `json:"message_hash"`
	OccurredAt         time.Time  `json:"occurred_at"`
}

type ProtocolAckEvent struct {
	ID               int64      `json:"id"`
	MachineID        uuid.UUID  `json:"machine_id"`
	CommandAttemptID *uuid.UUID `json:"command_attempt_id"`
	RawMessageID     *int64     `json:"raw_message_id"`
	DeviceReceiptID  *int64     `json:"device_receipt_id"`
	EventType        string     `json:"event_type"`
	OccurredAt       time.Time  `json:"occurred_at"`
	LatencyMs        *int32     `json:"latency_ms"`
	Details          []byte     `json:"details"`
}

// --- Telemetry projection tables (see migrations/00013_telemetry_pipeline.sql) ---

type MachineCurrentSnapshot struct {
	MachineID           uuid.UUID  `json:"machine_id"`
	OrganizationID      uuid.UUID  `json:"organization_id"`
	SiteID              uuid.UUID  `json:"site_id"`
	ReportedFingerprint *string    `json:"reported_fingerprint"`
	MetricsFingerprint  *string    `json:"metrics_fingerprint"`
	ReportedState       []byte     `json:"reported_state"`
	MetricsState        []byte     `json:"metrics_state"`
	LastHeartbeatAt     *time.Time `json:"last_heartbeat_at"`
	AppVersion          *string    `json:"app_version"`
	FirmwareVersion     *string    `json:"firmware_version"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type MachineStateTransition struct {
	ID             int64      `json:"id"`
	MachineID      uuid.UUID  `json:"machine_id"`
	TransitionKey  string     `json:"transition_key"`
	FromValue      []byte     `json:"from_value"`
	ToValue        []byte     `json:"to_value"`
	Metadata       []byte     `json:"metadata"`
	OccurredAt     time.Time  `json:"occurred_at"`
}

type MachineIncident struct {
	ID         uuid.UUID  `json:"id"`
	MachineID  uuid.UUID  `json:"machine_id"`
	Severity   string     `json:"severity"`
	Code       string     `json:"code"`
	Title      *string    `json:"title"`
	Detail     []byte     `json:"detail"`
	DedupeKey  *string    `json:"dedupe_key"`
	OpenedAt   time.Time  `json:"opened_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type TelemetryRollup struct {
	MachineID    uuid.UUID  `json:"machine_id"`
	BucketStart  time.Time  `json:"bucket_start"`
	Granularity  string     `json:"granularity"`
	MetricKey    string     `json:"metric_key"`
	SampleCount  int64      `json:"sample_count"`
	SumVal       *float64   `json:"sum_val"`
	MinVal       *float64   `json:"min_val"`
	MaxVal       *float64   `json:"max_val"`
	LastVal      *float64   `json:"last_val"`
	Extra        []byte     `json:"extra"`
}

type DiagnosticBundleManifest struct {
	ID               uuid.UUID  `json:"id"`
	MachineID        uuid.UUID  `json:"machine_id"`
	StorageKey       string     `json:"storage_key"`
	StorageProvider  string     `json:"storage_provider"`
	ContentType      *string    `json:"content_type"`
	SizeBytes        *int64     `json:"size_bytes"`
	Sha256Hex        *string    `json:"sha256_hex"`
	Metadata         []byte     `json:"metadata"`
	CreatedAt        time.Time  `json:"created_at"`
	ExpiresAt        *time.Time `json:"expires_at"`
}
