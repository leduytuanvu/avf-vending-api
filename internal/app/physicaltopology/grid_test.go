package physicaltopology

import "testing"

func TestAllSlotCodes_10x6(t *testing.T) {
	codes := AllSlotCodes(6, 10)
	if len(codes) != 60 {
		t.Fatalf("len=%d want 60", len(codes))
	}
	if codes[0] != "A1" || codes[1] != "A2" || codes[9] != "A10" || codes[10] != "B1" {
		t.Fatalf("unexpected codes: %v", codes[:12])
	}
}

func TestSlotIndexFromCode(t *testing.T) {
	if got := SlotIndexFromCode("A2", 10); got != 2 {
		t.Fatalf("A2=%d want 2", got)
	}
	if got := SlotIndexFromCode("B1", 10); got != 11 {
		t.Fatalf("B1=%d want 11", got)
	}
}

func TestPickTemplateRow(t *testing.T) {
	_, ok := PickTemplateRow(nil)
	if ok {
		t.Fatal("expected false for empty")
	}
}
