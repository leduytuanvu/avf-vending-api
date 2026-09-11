package layoutassignment

import (
	"time"

	"github.com/google/uuid"
)

const (
	SourceServer = "SERVER"
	SourceLocal  = "LOCAL"

	SyncStatusInSync         = "IN_SYNC"
	SyncStatusPending        = "PENDING"
	SyncStatusDrift          = "DRIFT"
	SyncStatusApplyFailed    = "APPLY_FAILED"
	SyncStatusOfflineUnknown = "OFFLINE_OR_UNKNOWN"

	DefaultGridRows = 6
	DefaultGridCols = 10

	MaxGridRows = 26
	MaxGridCols = 12
	MaxTCNLanes = 80
)

// AssignServerLayoutInput is the atomic SERVER layout assignment command.
type AssignServerLayoutInput struct {
	MachineID               uuid.UUID
	LayoutVersionID         uuid.UUID
	OrgLayoutVersionID      *uuid.UUID
	ExpectedCurrentRevision *int32
	IdempotencyKey          string
	ActorAccountID          *uuid.UUID
	OperatorSessionID       *uuid.UUID
}

// AssignServerLayoutResult is returned after a successful assignment.
type AssignServerLayoutResult struct {
	AssignmentID  uuid.UUID
	Source        string
	Revision      int32
	Rows          int32
	Columns       int32
	Fingerprint   string
	DesiredSource *string
	SyncStatus    string
	RequestID     string
}

// LayoutStateView is the admin read model for desired vs reported layout.
type LayoutStateView struct {
	MachineID           uuid.UUID
	ServerAssignment    *AssignmentView
	LocalAssignment     *AssignmentView
	LocalMirror         *LocalMirrorView
	DesiredSource       *string
	DesiredRevision     *int32
	DesiredFingerprint  *string
	ReportedSource      *string
	ReportedRevision    *int32
	ReportedFingerprint *string
	ReportedAt          *time.Time
	SyncStatus          string
	ApplyFailureReason  *string
}

// AssignmentView is one current or historical assignment row.
type AssignmentView struct {
	AssignmentID    uuid.UUID
	Source          string
	LayoutVersionID *uuid.UUID
	Revision        int32
	Rows            int32
	Columns         int32
	Fingerprint     string
	EffectiveFrom   time.Time
}

// LocalMirrorView is the device-reported LOCAL snapshot mirror.
type LocalMirrorView struct {
	LocalLayoutID     uuid.UUID
	Revision          int32
	Rows              int32
	Columns           int32
	Fingerprint       string
	ReportedAt        time.Time
	DeviceInstanceID  string
	ReportedGeneration *int64
	MirrorSlots       []LocalMirrorSlotView
	MergeGroups       []LocalMirrorMergeGroupView
}

// LocalMirrorSlotView is one slot from the device mirror payload.
type LocalMirrorSlotView struct {
	SlotCode          string  `json:"slotCode"`
	SlotOrdinal       int32   `json:"slotOrdinal,omitempty"`
	ProductID         string  `json:"productId,omitempty"`
	CurrentInventory  *int32  `json:"currentInventory,omitempty"`
	MaxQuantity       int32   `json:"maxQuantity,omitempty"`
	Enabled           *bool   `json:"enabled,omitempty"`
	OperationalState  *string `json:"operationalState,omitempty"`
	PriceMinor        int64   `json:"priceMinor,omitempty"`
}

// LocalMirrorMergeGroupView groups merged lanes reported by the device.
type LocalMirrorMergeGroupView struct {
	LeftSlotCode  string `json:"leftSlotCode"`
	RightSlotCode string `json:"rightSlotCode"`
}

// BulkAssignResult is per-machine outcome for bulk assignment.
type BulkAssignResult struct {
	MachineID    uuid.UUID
	Status       string
	AssignmentID *uuid.UUID
	Revision     *int32
	ErrorCode    string
	ErrorMessage string
	RequestID    string
}

// MachineAuthContext is the authenticated machine identity propagated from gRPC.
type MachineAuthContext struct {
	MachineID uuid.UUID
}

// ReportLocalLayoutInput is device-authored LOCAL layout reporting.
type ReportLocalLayoutInput struct {
	MachineID        uuid.UUID
	LocalLayoutID    uuid.UUID
	Revision         int32
	Rows             int32
	Columns          int32
	SlotsJSON        []byte
	Fingerprint      string
	DeviceInstanceID string
	IdempotencyKey   string
	LocalGeneration  int64
}

// ReportLocalLayoutResult is returned after a device LOCAL layout report.
type ReportLocalLayoutResult struct {
	Accepted          bool
	StoredRevision    int32
	StoredGeneration  int64
	StoredFingerprint string
}

// SetDesiredSourceInput switches the desired active layout source for a machine.
type SetDesiredSourceInput struct {
	MachineID               uuid.UUID
	Source                  string
	ExpectedCurrentRevision *int32
}

// SetDesiredSourceResult is returned after a successful desired-source update.
type SetDesiredSourceResult struct {
	DesiredSource      string
	DesiredRevision    int32
	DesiredFingerprint string
	SyncStatus         string
}
