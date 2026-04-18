package api

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestFleetMachinesAdmin_ListMachines_requiresTenantWhenMissingOrg(t *testing.T) {
	adm := &FleetMachinesAdmin{Fleet: nil}
	_, err := adm.ListMachines(context.Background(), AdminListScope{
		IsPlatformAdmin: true,
		OrganizationID:  uuid.Nil,
	})
	if !errors.Is(err, ErrAdminTenantScopeRequired) {
		t.Fatalf("got %v want ErrAdminTenantScopeRequired", err)
	}
}

func TestFleetMachinesAdmin_ListMachines_nilFleet(t *testing.T) {
	adm := &FleetMachinesAdmin{Fleet: nil}
	_, err := adm.ListMachines(context.Background(), AdminListScope{
		IsPlatformAdmin: false,
		OrganizationID:  uuid.New(),
	})
	var cap *CapabilityError
	if !errors.As(err, &cap) {
		t.Fatalf("want CapabilityError, got %v", err)
	}
	if cap.Capability != "v1.admin.machines.list" {
		t.Fatalf("capability: %q", cap.Capability)
	}
}
