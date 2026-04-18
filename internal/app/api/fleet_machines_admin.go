package api

import (
	"context"
	"encoding/json"

	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	"github.com/google/uuid"
)

// FleetMachinesAdmin lists machines using the fleet application service (real Postgres-backed repository).
type FleetMachinesAdmin struct {
	Fleet *appfleet.Service
}

// NewFleetMachinesAdmin wires fleet list workflows for HTTP.
func NewFleetMachinesAdmin(s *appfleet.Service) *FleetMachinesAdmin {
	return &FleetMachinesAdmin{Fleet: s}
}

func (a *FleetMachinesAdmin) ListMachines(ctx context.Context, scope AdminListScope) (*ListView, error) {
	if a == nil {
		return nil, &CapabilityError{
			Capability: "v1.admin.machines.list",
			Message:    "fleet service is not configured",
		}
	}
	if scope.IsPlatformAdmin && scope.OrganizationID == uuid.Nil {
		return nil, ErrAdminTenantScopeRequired
	}
	if !scope.IsPlatformAdmin && scope.OrganizationID == uuid.Nil {
		return nil, ErrAdminTenantScopeRequired
	}
	if a.Fleet == nil {
		return nil, &CapabilityError{
			Capability: "v1.admin.machines.list",
			Message:    "fleet service is not configured",
		}
	}

	machines, err := a.Fleet.ListMachinesInScope(ctx, appfleet.ListMachinesScope{
		OrganizationID: scope.OrganizationID,
		SiteID:         scope.SiteID,
	})
	if err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(machines))
	for _, m := range machines {
		var hw any
		if m.HardwareProfileID != nil {
			hw = *m.HardwareProfileID
		} else {
			hw = nil
		}
		items = append(items, map[string]any{
			"id":                  m.ID.String(),
			"organization_id":     m.OrganizationID.String(),
			"site_id":             m.SiteID.String(),
			"hardware_profile_id": hw,
			"serial_number":       m.SerialNumber,
			"name":                m.Name,
			"status":              m.Status,
			"command_sequence":    m.CommandSequence,
			"created_at":          m.CreatedAt,
			"updated_at":          m.UpdatedAt,
		})
	}
	return &ListView{Items: items}, nil
}

func marshalJSONRawObject(b []byte) (map[string]any, error) {
	if len(b) == 0 {
		return map[string]any{}, nil
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}
