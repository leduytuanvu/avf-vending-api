package fleetadmin

import (
	"time"

	"github.com/avf/avf-vending-api/internal/app/listscope"
)

// AdminMachineListItem is a normalized machine row for fleet admin lists.
type AdminMachineListItem struct {
	MachineID         string    `json:"machineId"`
	OrganizationID    string    `json:"organizationId"`
	SiteID            string    `json:"siteId"`
	HardwareProfileID *string   `json:"hardwareProfileId,omitempty"`
	SerialNumber      string    `json:"serialNumber"`
	Name              string    `json:"name"`
	Status            string    `json:"status"`
	CommandSequence   int64     `json:"commandSequence"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
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
