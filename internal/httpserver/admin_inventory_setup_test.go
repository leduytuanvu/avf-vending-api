package httpserver

import (
	"encoding/json"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"testing"
)

func TestPlanogramPublishPayload_roundtripJSON(t *testing.T) {
	id := id.NewUUIDV7().String()
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

func TestTopologyLayoutStatusForWrite(t *testing.T) {
	t.Parallel()
	st, ok := topologyLayoutStatusForWrite("")
	if !ok || st != "published" {
		t.Fatalf("empty status: got %q ok=%v", st, ok)
	}
	st, ok = topologyLayoutStatusForWrite("published")
	if !ok || st != "published" {
		t.Fatalf("published: got %q ok=%v", st, ok)
	}
	if _, ok = topologyLayoutStatusForWrite("active"); ok {
		t.Fatal("active must be rejected")
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
