package setupapp

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

func TestPlanogramFingerprint_includesMergePairs(t *testing.T) {
	left := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	b := MachineBootstrap{
		CurrentCabinetSlots: []CabinetSlotConfigView{
			{
				ConfigID:    left,
				CabinetCode: "A",
				SlotCode:    "A1",
				MaxQuantity: 10,
			},
		},
	}
	without := PlanogramFingerprint(b)
	b.MergePairs = []LaneMergePair{{LeftSlotCode: "A1", RightSlotCode: "A2"}}
	with := PlanogramFingerprint(b)
	if without == with {
		t.Fatalf("expected fingerprint to change when merge pairs added")
	}
}

func TestEnsureNoMergeOverlap_rejectsConflict(t *testing.T) {
	active := map[string]db.MachineLaneMergePair{
		"A1": {LeftSlotCode: "A1", RightSlotCode: "A2"},
	}
	if err := ensureNoMergeOverlap(active, "A3", "A4", "A9"); err != nil {
		t.Fatalf("unexpected overlap for disjoint pair: %v", err)
	}
	if err := ensureNoMergeOverlap(active, "A2", "A3", "A9"); err == nil {
		t.Fatalf("expected overlap error")
	}
}

func TestListActiveMergePairs_nilPool(t *testing.T) {
	pairs, err := ListActiveMergePairs(context.Background(), nil, uuid.New())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if pairs != nil {
		t.Fatalf("expected nil pairs")
	}
}
