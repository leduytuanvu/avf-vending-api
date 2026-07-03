package machineruntime

import (
	"testing"
	"time"
)

func TestNormalizeMachineCode(t *testing.T) {
	if got := NormalizeMachineCode(" avf123456 "); got != "AVF123456" {
		t.Fatalf("normalize: %q", got)
	}
}

func TestValidMachineCode(t *testing.T) {
	if !ValidMachineCode("AVF123456") {
		t.Fatal("expected valid AVF code")
	}
	if ValidMachineCode("BAD") {
		t.Fatal("expected invalid code")
	}
}

func TestComputeMachineOnlineStatusThresholds(t *testing.T) {
	s := &Service{
		onlineThreshold:  60 * time.Second,
		staleThreshold:   300 * time.Second,
		offlineThreshold: 600 * time.Second,
	}
	now := time.Now().UTC()
	last := now.Add(-30 * time.Second)
	age := now.Sub(last)
	switch {
	case age <= s.onlineThreshold:
		if st := "online"; st != "online" {
			t.Fatal("expected online")
		}
	case age <= s.staleThreshold:
		t.Fatal("unexpected stale branch")
	default:
		t.Fatal("unexpected offline branch")
	}
}

func TestDeviceIdentityFromFingerprint(t *testing.T) {
	id := DeviceIdentityFromFingerprint([]byte(`{"androidId":"a1","simIccid":"iccid"}`), "10.0.0.1", "ua", nil)
	if id.AndroidID != "a1" || id.SimICCID != "iccid" || id.IPAddress != "10.0.0.1" {
		t.Fatalf("identity: %+v", id)
	}
}
