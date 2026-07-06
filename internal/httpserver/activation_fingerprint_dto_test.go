package httpserver

import (
	"encoding/json"
	"testing"
)

func TestFingerprintDTO_UnmarshalJSON_camelCase(t *testing.T) {
	var dto fingerprintDTO
	err := json.Unmarshal([]byte(`{
		"androidId": "aid-1",
		"serialNumber": "sn-1",
		"boardSerial": "board-1",
		"simIccid": "iccid-1",
		"sdkInt": 34,
		"appBuildSha": "sha-1"
	}`), &dto)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	fp := dto.DeviceFingerprint
	if fp.AndroidID != "aid-1" || fp.SerialNumber != "sn-1" || fp.BoardSerial != "board-1" {
		t.Fatalf("unexpected camelCase mapping: %+v", fp)
	}
	if fp.SimIccid != "iccid-1" || fp.SdkInt != 34 || fp.AppBuildSha != "sha-1" {
		t.Fatalf("unexpected expanded camelCase mapping: %+v", fp)
	}
}

func TestFingerprintDTO_UnmarshalJSON_snakeCase(t *testing.T) {
	var dto fingerprintDTO
	err := json.Unmarshal([]byte(`{
		"android_id": "aid-2",
		"serial_number": "sn-2",
		"board_serial": "board-2",
		"device_serial": "dev-2",
		"sim_iccid": "iccid-2",
		"sim_country_iso": "VN",
		"device_model": "dm-2",
		"android_release": "14",
		"sdk_int": 33,
		"app_build_sha": "sha-2",
		"boot_id": "boot-2",
		"network_type": "wifi",
		"network_state": "connected"
	}`), &dto)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	fp := dto.DeviceFingerprint
	if fp.AndroidID != "aid-2" || fp.BoardSerial != "board-2" || fp.DeviceSerial != "dev-2" {
		t.Fatalf("unexpected snake_case mapping: %+v", fp)
	}
	if fp.SimIccid != "iccid-2" || fp.SimCountryIso != "VN" || fp.DeviceModel != "dm-2" {
		t.Fatalf("unexpected expanded snake_case mapping: %+v", fp)
	}
	if fp.AndroidRelease != "14" || fp.SdkInt != 33 || fp.AppBuildSha != "sha-2" {
		t.Fatalf("unexpected version snake_case mapping: %+v", fp)
	}
	if fp.BootID != "boot-2" || fp.NetworkType != "wifi" || fp.NetworkState != "connected" {
		t.Fatalf("unexpected network snake_case mapping: %+v", fp)
	}
}

func TestPublicClaimBody_acceptsSnakeCaseFingerprint(t *testing.T) {
	var body publicClaimBody
	err := json.Unmarshal([]byte(`{
		"activationCode": "342209",
		"deviceFingerprint": {
			"android_id": "aid-snake",
			"board_serial": "board-snake"
		}
	}`), &body)
	if err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body.DeviceFingerprint.AndroidID != "aid-snake" {
		t.Fatalf("android_id not mapped: %+v", body.DeviceFingerprint)
	}
	if body.DeviceFingerprint.BoardSerial != "board-snake" {
		t.Fatalf("board_serial not mapped: %+v", body.DeviceFingerprint)
	}
}
