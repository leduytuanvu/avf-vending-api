package machineruntime

import "testing"

func TestComputeFinalSellReady(t *testing.T) {
	blockers := []byte(`[{"code":"x","severity":"critical","message":"blocked"}]`)
	if computeFinalSellReady("active", true, true, blockers) {
		t.Fatal("critical blockers should prevent final sell ready")
	}
	if !computeFinalSellReady("active", true, true, []byte(`[]`)) {
		t.Fatal("expected sell ready when lifecycle active and sale enabled")
	}
	if computeFinalSellReady("suspended", true, true, []byte(`[]`)) {
		t.Fatal("suspended lifecycle should not be sell ready")
	}
	if computeFinalSellReady("active", false, true, []byte(`[]`)) {
		t.Fatal("sale disabled should not be sell ready")
	}
}

func TestDeviceIdentityExtendedFingerprint(t *testing.T) {
	raw := []byte(`{"androidId":"a1","boardSerial":"BS1","simIccid":"8901","appBuildSha":"sha1","versionName":"2.0.0"}`)
	id := DeviceIdentityFromFingerprint(raw, "1.2.3.4", "ua", nil)
	if id.BoardSerial != "BS1" || id.SimICCID != "8901" || id.AppBuildSHA != "sha1" || id.VersionName != "2.0.0" {
		t.Fatalf("extended identity: %+v", id)
	}
}
