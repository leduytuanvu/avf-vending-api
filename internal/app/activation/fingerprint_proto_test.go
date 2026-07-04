package activation

import (
	"testing"

	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
)

func TestDeviceFingerprintFromProto_allFields(t *testing.T) {
	fp := DeviceFingerprintFromProto(&machinev1.DeviceFingerprint{
		AndroidId:      "aid",
		SerialNumber:   "sn",
		Manufacturer:   "mfg",
		Model:          "model",
		PackageName:    "pkg",
		VersionName:    "1.0",
		VersionCode:    42,
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
	})
	if fp.AndroidID != "aid" || fp.BoardSerial != "bs" || fp.SimIccid != "iccid" || fp.SdkInt != 34 {
		t.Fatalf("unexpected mapping: %+v", fp)
	}
}

func TestDeviceFingerprintFromProto_nil(t *testing.T) {
	fp := DeviceFingerprintFromProto(nil)
	if fp.AndroidID != "" {
		t.Fatal("expected empty fingerprint")
	}
}
