package planogram

import (
	"testing"

	"github.com/google/uuid"
)

func Test_snapshotBytesToSaveInput_roundTrip(t *testing.T) {
	pg := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	raw := []byte(`{"planogramId":"` + pg.String() + `","planogramRevision":2,"syncLegacyReadModel":false,"items":[{"cabinetCode":"A","layoutKey":"default","layoutRevision":1,"slotCode":"S1","maxQuantity":5,"priceMinor":100}]}`)
	save, err := snapshotBytesToSaveInput(raw, true)
	if err != nil {
		t.Fatal(err)
	}
	if save.PlanogramID != pg {
		t.Fatalf("planogram id")
	}
	if save.PlanogramRevision != 2 {
		t.Fatalf("revision")
	}
	if !save.PublishAsCurrent {
		t.Fatalf("publish flag")
	}
	if len(save.Items) != 1 || save.Items[0].SlotCode != "S1" {
		t.Fatalf("items")
	}
}
