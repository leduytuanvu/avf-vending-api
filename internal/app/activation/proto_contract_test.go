package activation

import (
	"testing"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
)

func TestProtoContract_DeviceFingerprintFields8Through24(t *testing.T) {
	fp := &machinev1.DeviceFingerprint{
		AndroidSerial:  "as",
		BoardSerial:    "bs",
		DeviceSerial:   "ds",
		SimSerial:      "ss",
		SimIccid:       "iccid",
		SimOperator:    "op",
		SimCountryIso:  "US",
		Brand:          "brand",
		DeviceModel:    "dm",
		Hardware:       "hw",
		Product:        "prod",
		AndroidRelease: "14",
		SdkInt:         34,
		AppBuildSha:    "sha",
		BootId:         "boot",
		NetworkType:    "wifi",
		NetworkState:   "connected",
	}
	checks := map[string]string{
		"AndroidSerial":  fp.GetAndroidSerial(),
		"BoardSerial":    fp.GetBoardSerial(),
		"DeviceSerial":   fp.GetDeviceSerial(),
		"SimSerial":      fp.GetSimSerial(),
		"SimIccid":       fp.GetSimIccid(),
		"SimOperator":    fp.GetSimOperator(),
		"SimCountryIso":  fp.GetSimCountryIso(),
		"Brand":          fp.GetBrand(),
		"DeviceModel":    fp.GetDeviceModel(),
		"Hardware":       fp.GetHardware(),
		"Product":        fp.GetProduct(),
		"AndroidRelease": fp.GetAndroidRelease(),
		"AppBuildSha":    fp.GetAppBuildSha(),
		"BootId":         fp.GetBootId(),
		"NetworkType":    fp.GetNetworkType(),
		"NetworkState":   fp.GetNetworkState(),
	}
	for name, got := range checks {
		if got == "" && name != "SimOperator" {
			t.Fatalf("field %s getter returned empty", name)
		}
	}
	if fp.GetSdkInt() != 34 {
		t.Fatalf("SdkInt getter: got %d", fp.GetSdkInt())
	}
}

func TestProtoContract_ClaimActivationResponseDeviceAttachmentId(t *testing.T) {
	resp := &machinev1.ClaimActivationResponse{
		MqttUsername:       "user",
		MqttPassword:       "pass",
		DeviceAttachmentId: "11111111-2222-3333-4444-555555555555",
	}
	if resp.GetDeviceAttachmentId() == "" {
		t.Fatal("device_attachment_id getter missing")
	}
	if resp.GetMqttUsername() == "" || resp.GetMqttPassword() == "" {
		t.Fatal("mqtt fields must remain available")
	}
}
