package httpserver

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestPlanogramPublishPayload_roundtripJSON(t *testing.T) {
	id := uuid.New().String()
	p := planogramPublishPayload{
		PlanogramID:          id,
		PlanogramRevision:    4,
		DesiredConfigVersion: 12,
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var got planogramPublishPayload
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.PlanogramID != id || got.PlanogramRevision != 4 || got.DesiredConfigVersion != 12 {
		t.Fatalf("got %+v", got)
	}
}

func TestAdminMachinePlanogramPublishCommandType_constant(t *testing.T) {
	if adminMachinePlanogramPublishCommandType != "machine_planogram_publish" {
		t.Fatalf("got %q", adminMachinePlanogramPublishCommandType)
	}
	if adminMachineSetupSyncCommandType != "machine_setup_sync" {
		t.Fatalf("got %q", adminMachineSetupSyncCommandType)
	}
}
