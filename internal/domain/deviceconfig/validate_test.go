package deviceconfig

import (
	"encoding/json"
	"errors"
	"testing"
)

func validDeviceConfigJSON() string {
	return `{
		"schemaVersion": 1,
		"origin": "default_seed",
		"bill": {
			"enabledDenominationsVnd": [1000, 2000, 5000, 10000, 20000, 50000, 100000, 200000, 500000],
			"changeDenominationVnd": 10000,
			"escrowEnabled": true
		},
		"tcn": {
			"laneMode": 10,
			"shakeCount": 2,
			"dropWaitSeconds": 3,
			"lightsOn": true,
			"coolingCapable": true,
			"tempControlEnabled": true,
			"targetTempC": 5,
			"tempHysteresisC": 2,
			"tempCompensationC": 1,
			"defrostMinutes": 20,
			"compressorWorkMinutes": 120,
			"fanStopMinutes": 120
		}
	}`
}

func TestValidateMetadataDeviceConfig_absentAllowed(t *testing.T) {
	t.Parallel()
	meta := json.RawMessage(`{"board_protocol":"tcn","bill_protocol":"ict_bc_v1"}`)
	if err := ValidateMetadataDeviceConfig(meta); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateMetadataDeviceConfig_rejectsEmptyObject(t *testing.T) {
	t.Parallel()
	meta := json.RawMessage(`{"device_config":{}}`)
	err := ValidateMetadataDeviceConfig(meta)
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if ve.Code != "device_config_empty" {
		t.Fatalf("code=%q", ve.Code)
	}
}

func TestValidateMetadataDeviceConfig_acceptsValidBlock(t *testing.T) {
	t.Parallel()
	meta := json.RawMessage(`{"device_config":` + validDeviceConfigJSON() + `}`)
	if err := ValidateMetadataDeviceConfig(meta); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateMetadataDeviceConfig_rejectsInvalidDenomination(t *testing.T) {
	t.Parallel()
	meta := json.RawMessage(`{"device_config":{"schemaVersion":1,"bill":{"enabledDenominationsVnd":[777],"changeDenominationVnd":777},"tcn":{"laneMode":10,"shakeCount":2,"dropWaitSeconds":3}}}`)
	err := ValidateMetadataDeviceConfig(meta)
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if ve.Code != "device_config_bill_invalid_denomination" {
		t.Fatalf("code=%q", ve.Code)
	}
}

func TestValidateMetadataDeviceConfig_rejectsTempControlWithoutCooling(t *testing.T) {
	t.Parallel()
	meta := json.RawMessage(`{"device_config":{"schemaVersion":1,"bill":{"enabledDenominationsVnd":[10000],"changeDenominationVnd":10000},"tcn":{"laneMode":10,"shakeCount":2,"dropWaitSeconds":3,"tempControlEnabled":true}}}`)
	err := ValidateMetadataDeviceConfig(meta)
	var ve ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if ve.Code != "device_config_tcn_temp_control_requires_cooling" {
		t.Fatalf("code=%q", ve.Code)
	}
}
