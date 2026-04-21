package fleetadmin

import (
	"time"

	"github.com/avf/avf-vending-api/internal/app/listscope"
)

// AdminMachineInventorySummary is derived from machine_slot_state + slots (same rules as InventoryAdminListMachineSlots).
type AdminMachineInventorySummary struct {
	TotalSlots      int64 `json:"totalSlots"`
	OccupiedSlots   int64 `json:"occupiedSlots"`
	LowStockSlots   int64 `json:"lowStockSlots"`
	OutOfStockSlots int64 `json:"outOfStockSlots"`
}

// AdminAssignedTechnician is an active technician_machine_assignments row (valid_to null or future).
type AdminAssignedTechnician struct {
	TechnicianID string  `json:"technicianId"`
	DisplayName  string  `json:"displayName"`
	Role         string  `json:"role"`
	ValidFrom    string  `json:"validFrom"`
	ValidTo      *string `json:"validTo,omitempty"`
}

// AdminCurrentOperator reflects v_machine_current_operator / active operator session (nil when nobody logged in).
type AdminCurrentOperator struct {
	SessionID             string  `json:"sessionId"`
	ActorType             string  `json:"actorType"`
	TechnicianID          *string `json:"technicianId,omitempty"`
	TechnicianDisplayName *string `json:"technicianDisplayName,omitempty"`
	UserPrincipal         *string `json:"userPrincipal,omitempty"`
	SessionStartedAt      string  `json:"sessionStartedAt"`
	SessionStatus         string  `json:"sessionStatus"`
	SessionExpiresAt      *string `json:"sessionExpiresAt,omitempty"`
}

// AdminMachineListItem is a normalized machine row for fleet admin lists and GET /v1/admin/machines/{machineId}.
type AdminMachineListItem struct {
	MachineID            string                       `json:"machineId"`
	MachineName          string                       `json:"machineName"`
	OrganizationID       string                       `json:"organizationId"`
	SiteID               string                       `json:"siteId"`
	SiteName             string                       `json:"siteName"`
	HardwareProfileID    *string                      `json:"hardwareProfileId,omitempty"`
	SerialNumber         string                       `json:"serialNumber"`
	Name                 string                       `json:"name"`
	Status               string                       `json:"status"`
	CommandSequence      int64                        `json:"commandSequence"`
	CreatedAt            string                       `json:"createdAt"`
	UpdatedAt            string                       `json:"updatedAt"`
	AndroidID            *string                      `json:"androidId,omitempty"`
	SimSerial            *string                      `json:"simSerial,omitempty"`
	SimIccid             *string                      `json:"simIccid,omitempty"`
	AppVersion           *string                      `json:"appVersion,omitempty"`
	FirmwareVersion      *string                      `json:"firmwareVersion,omitempty"`
	LastHeartbeatAt      *string                      `json:"lastHeartbeatAt,omitempty"`
	EffectiveTimezone    string                       `json:"effectiveTimezone"`
	AssignedTechnicians  []AdminAssignedTechnician    `json:"assignedTechnicians"`
	CurrentOperator      *AdminCurrentOperator        `json:"currentOperator"`
	InventorySummary     AdminMachineInventorySummary `json:"inventorySummary"`
}

// MachinesListResponse is returned by GET /v1/admin/machines.
type MachinesListResponse struct {
	Items []AdminMachineListItem   `json:"items"`
	Meta  listscope.CollectionMeta `json:"meta"`
}

// AdminTechnicianListItem is a normalized technician directory row.
type AdminTechnicianListItem struct {
	TechnicianID    string    `json:"technicianId"`
	OrganizationID  string    `json:"organizationId"`
	DisplayName     string    `json:"displayName"`
	Email           *string   `json:"email,omitempty"`
	Phone           *string   `json:"phone,omitempty"`
	ExternalSubject *string   `json:"externalSubject,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
}

// TechniciansListResponse is returned by GET /v1/admin/technicians.
type TechniciansListResponse struct {
	Items []AdminTechnicianListItem `json:"items"`
	Meta  listscope.CollectionMeta  `json:"meta"`
}

// AdminAssignmentListItem is a technician–machine assignment with joined labels.
type AdminAssignmentListItem struct {
	AssignmentID          string     `json:"assignmentId"`
	TechnicianID          string     `json:"technicianId"`
	TechnicianDisplayName string     `json:"technicianDisplayName"`
	MachineID             string     `json:"machineId"`
	MachineName           string     `json:"machineName"`
	MachineSerialNumber   string     `json:"machineSerialNumber"`
	Role                  string     `json:"role"`
	ValidFrom             time.Time  `json:"validFrom"`
	ValidTo               *time.Time `json:"validTo,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
}

// AssignmentsListResponse is returned by GET /v1/admin/assignments.
type AssignmentsListResponse struct {
	Items []AdminAssignmentListItem `json:"items"`
	Meta  listscope.CollectionMeta  `json:"meta"`
}

// AdminCommandListItem summarizes command_ledger with latest transport attempt state.
type AdminCommandListItem struct {
	CommandID           string    `json:"commandId"`
	MachineID           string    `json:"machineId"`
	OrganizationID      string    `json:"organizationId"`
	MachineName         string    `json:"machineName"`
	MachineSerialNumber string    `json:"machineSerialNumber"`
	Sequence            int64     `json:"sequence"`
	CommandType         string    `json:"commandType"`
	CreatedAt           time.Time `json:"createdAt"`
	AttemptCount        int32     `json:"attemptCount"`
	LatestAttemptStatus string    `json:"latestAttemptStatus"`
	CorrelationID       *string   `json:"correlationId,omitempty"`
}

// CommandsListResponse is returned by GET /v1/admin/commands.
type CommandsListResponse struct {
	Items []AdminCommandListItem   `json:"items"`
	Meta  listscope.CollectionMeta `json:"meta"`
}

// AdminOTAListItem summarizes an OTA campaign with linked artifact metadata.
type AdminOTAListItem struct {
	CampaignID         string    `json:"campaignId"`
	OrganizationID     string    `json:"organizationId"`
	CampaignName       string    `json:"campaignName"`
	Strategy           string    `json:"strategy"`
	CampaignStatus     string    `json:"campaignStatus"`
	CreatedAt          time.Time `json:"createdAt"`
	ArtifactID         string    `json:"artifactId"`
	ArtifactSemver     *string   `json:"artifactSemver,omitempty"`
	ArtifactStorageKey string    `json:"artifactStorageKey"`
}

// OTAListResponse is returned by GET /v1/admin/ota.
type OTAListResponse struct {
	Items []AdminOTAListItem       `json:"items"`
	Meta  listscope.CollectionMeta `json:"meta"`
}
