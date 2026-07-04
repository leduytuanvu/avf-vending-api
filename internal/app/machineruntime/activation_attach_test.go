package machineruntime

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestDeviceIdentityMatchesAttachment_sameIdentity(t *testing.T) {
	att := db.MachineDeviceAttachment{
		AndroidID:     pgtype.Text{String: "aid-1", Valid: true},
		BoardSerial:   pgtype.Text{String: "board-1", Valid: true},
		SimIccid:      pgtype.Text{String: "iccid-1", Valid: true},
		AndroidSerial: pgtype.Text{String: "serial-1", Valid: true},
	}
	id := DeviceIdentity{
		AndroidID:     "aid-1",
		BoardSerial:   "board-1",
		SimICCID:      "iccid-1",
		AndroidSerial: "serial-1",
	}
	if !DeviceIdentityMatchesAttachment(att, id) {
		t.Fatal("expected match")
	}
}

func TestDeviceIdentityMatchesAttachment_differentBoardSerial(t *testing.T) {
	att := db.MachineDeviceAttachment{
		AndroidID:   pgtype.Text{String: "aid-1", Valid: true},
		BoardSerial: pgtype.Text{String: "board-1", Valid: true},
	}
	id := DeviceIdentity{
		AndroidID:   "aid-1",
		BoardSerial: "board-2",
	}
	if DeviceIdentityMatchesAttachment(att, id) {
		t.Fatal("expected mismatch")
	}
}

func TestDeviceIdentityMatchesAttachment_incompleteFingerprintReplay(t *testing.T) {
	att := db.MachineDeviceAttachment{
		AndroidID: pgtype.Text{String: "aid-1", Valid: true},
	}
	id := DeviceIdentity{AndroidID: "aid-1"}
	if !DeviceIdentityMatchesAttachment(att, id) {
		t.Fatal("expected match on android_id")
	}
}

func TestDeviceIdentityMatchesAttachment_bothEmptyDiscriminators(t *testing.T) {
	att := db.MachineDeviceAttachment{}
	id := DeviceIdentity{}
	if !DeviceIdentityMatchesAttachment(att, id) {
		t.Fatal("expected reuse on incomplete fingerprint replay")
	}
}
