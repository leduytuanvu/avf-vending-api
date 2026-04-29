package otaadmin

import (
	"time"

	"github.com/avf/avf-vending-api/internal/app/listscope"
	"github.com/google/uuid"
)

// CampaignListParams filters GET /v1/admin/ota/campaigns.
type CampaignListParams struct {
	OrganizationID uuid.UUID
	Limit          int32
	Offset         int32
	Status         string
	From           *time.Time
	To             *time.Time
}

// CampaignListItem is one campaign row for list APIs.
type CampaignListItem struct {
	CampaignID         string     `json:"campaignId"`
	OrganizationID     string     `json:"organizationId"`
	Name               string     `json:"name"`
	RolloutStrategy    string     `json:"rolloutStrategy"`
	Status             string     `json:"status"`
	CampaignType       string     `json:"campaignType"`
	CanaryPercent      int32      `json:"canaryPercent"`
	RolloutNextOffset  int32      `json:"rolloutNextOffset"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
	ApprovedAt         *time.Time `json:"approvedAt,omitempty"`
	ArtifactID         string     `json:"artifactId"`
	ArtifactSemver     *string    `json:"artifactSemver,omitempty"`
	ArtifactStorageKey string     `json:"artifactStorageKey"`
	ArtifactVersion    *string    `json:"artifactVersion,omitempty"`
	RollbackArtifactID *string    `json:"rollbackArtifactId,omitempty"`
}

// CampaignListResponse is GET /v1/admin/ota/campaigns.
type CampaignListResponse struct {
	Items []CampaignListItem       `json:"items"`
	Meta  listscope.CollectionMeta `json:"meta"`
}

// CreateCampaignInput binds POST /v1/admin/ota/campaigns.
type CreateCampaignInput struct {
	OrganizationID     uuid.UUID
	Name               string
	ArtifactID         uuid.UUID
	ArtifactVersion    *string
	CampaignType       string
	RolloutStrategy    string
	CanaryPercent      int32
	RollbackArtifactID *uuid.UUID
	CreatedBy          uuid.UUID
}

// PatchCampaignInput binds PATCH (draft/approved only for mutable fields).
type PatchCampaignInput struct {
	Name               *string
	ArtifactVersion    *string
	CampaignType       *string
	RolloutStrategy    *string
	CanaryPercent      *int32
	RollbackArtifactID *uuid.UUID
}

// PutTargetsInput replaces all targets (draft/approved only).
type PutTargetsInput struct {
	OrganizationID uuid.UUID
	CampaignID     uuid.UUID
	MachineIDs     []uuid.UUID
}

// CampaignDetail is the full campaign payload for GET / PATCH / lifecycle mutations.
type CampaignDetail struct {
	CampaignListItem
	CreatedBy  *string    `json:"createdBy,omitempty"`
	ApprovedBy *string    `json:"approvedBy,omitempty"`
	PausedAt   *time.Time `json:"pausedAt,omitempty"`
}

// CampaignTargetItem is one machine target row for GET/PUT targets responses.
type CampaignTargetItem struct {
	MachineID string    `json:"machineId"`
	State     string    `json:"state"`
	LastError *string   `json:"lastError,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// CampaignTargetsPutBody binds PUT /v1/admin/ota/campaigns/{campaignId}/targets.
type CampaignTargetsPutBody struct {
	MachineIDs []uuid.UUID `json:"machineIds"`
}

// MachineResultItem is one rollout/rollback command outcome row.
type MachineResultItem struct {
	MachineID string    `json:"machineId"`
	Wave      string    `json:"wave"`
	CommandID *string   `json:"commandId,omitempty"`
	Status    string    `json:"status"`
	LastError *string   `json:"lastError,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
	CreatedAt time.Time `json:"createdAt"`
}

// RollbackCampaignBody optional override for POST .../rollback.
type RollbackCampaignBody struct {
	RollbackArtifactID *uuid.UUID `json:"rollbackArtifactId"`
}
