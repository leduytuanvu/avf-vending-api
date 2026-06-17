package deviceconfig

import (
	"encoding/json"
	"fmt"
)

// ValidationError is a typed reject for invalid desired device_config in cabinet metadata.
type ValidationError struct {
	Code    string
	Message string
}

func (e ValidationError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code
}

var standardDenominationsVND = map[int]struct{}{
	1000: {}, 2000: {}, 5000: {}, 10000: {}, 20000: {}, 50000: {},
	100000: {}, 200000: {}, 500000: {},
}

type billDeviceConfig struct {
	EnabledDenominationsVND []int  `json:"enabledDenominationsVnd"`
	ChangeDenominationVND   int    `json:"changeDenominationVnd"`
	EscrowEnabled           bool   `json:"escrowEnabled"`
	RecyclingCapacity       *int   `json:"recyclingCapacity"`
}

type tcnDeviceConfig struct {
	LaneMode               int    `json:"laneMode"`
	ShakeCount             int    `json:"shakeCount"`
	DropWaitSeconds        int    `json:"dropWaitSeconds"`
	LightsOn               bool   `json:"lightsOn"`
	CoolingCapable         bool   `json:"coolingCapable"`
	TempControlEnabled     bool   `json:"tempControlEnabled"`
	CoolingMode            string `json:"coolingMode"`
	TargetTempC            int    `json:"targetTempC"`
	TempHysteresisC        int    `json:"tempHysteresisC"`
	TempCompensationC      int    `json:"tempCompensationC"`
	DefrostMinutes         int    `json:"defrostMinutes"`
	CompressorWorkMinutes  int    `json:"compressorWorkMinutes"`
	FanStopMinutes         int    `json:"fanStopMinutes"`
	GlassHeaterOn          bool   `json:"glassHeaterOn"`
}

type deviceConfig struct {
	SchemaVersion int              `json:"schemaVersion"`
	Origin        string           `json:"origin"`
	Bill          billDeviceConfig `json:"bill"`
	Tcn           tcnDeviceConfig  `json:"tcn"`
}

// ValidateMetadataDeviceConfig validates metadata.device_config when present.
// Absent device_config is allowed (backward compatible). Empty {} is rejected.
func ValidateMetadataDeviceConfig(metadata json.RawMessage) error {
	if len(metadata) == 0 {
		return nil
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(metadata, &root); err != nil {
		return ValidationError{Code: "invalid_metadata", Message: "cabinet metadata must be JSON object"}
	}
	raw, ok := root["device_config"]
	if !ok {
		return nil
	}
	if len(raw) == 0 || string(raw) == "null" {
		return reject("device_config_empty", "device_config must not be empty")
	}
	var probe map[string]any
	if err := json.Unmarshal(raw, &probe); err != nil {
		return reject("device_config_invalid", "device_config must be a JSON object")
	}
	if len(probe) == 0 {
		return reject("device_config_empty", "device_config must not be empty object")
	}

	var cfg deviceConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return reject("device_config_invalid", "device_config must be a JSON object")
	}
	if cfg.SchemaVersion <= 0 {
		return reject("device_config_schema_version_invalid", "device_config.schemaVersion must be positive")
	}
	if err := validateBill(cfg.Bill); err != nil {
		return err
	}
	if err := validateTcn(cfg.Tcn); err != nil {
		return err
	}
	return nil
}

func validateBill(b billDeviceConfig) error {
	if len(b.EnabledDenominationsVND) == 0 {
		return reject("device_config_bill_enabled_denominations_empty", "device_config.bill.enabledDenominationsVnd must not be empty")
	}
	enabled := make(map[int]struct{}, len(b.EnabledDenominationsVND))
	for _, vnd := range b.EnabledDenominationsVND {
		if _, ok := standardDenominationsVND[vnd]; !ok {
			return reject("device_config_bill_invalid_denomination", fmt.Sprintf("device_config.bill.enabledDenominationsVnd invalid denomination=%d", vnd))
		}
		enabled[vnd] = struct{}{}
	}
	if _, ok := enabled[b.ChangeDenominationVND]; !ok {
		return reject("device_config_bill_change_denomination_invalid", "device_config.bill.changeDenominationVnd must be in enabledDenominationsVnd")
	}
	if b.RecyclingCapacity != nil {
		cap := *b.RecyclingCapacity
		if cap < 0 {
			return reject("device_config_bill_recycling_capacity_invalid", "device_config.bill.recyclingCapacity must be >= 0")
		}
		if cap > 35 {
			return reject("device_config_bill_recycling_capacity_invalid", "device_config.bill.recyclingCapacity must be <= 35")
		}
	}
	return nil
}

func validateTcn(t tcnDeviceConfig) error {
	if t.LaneMode != 10 && t.LaneMode != 12 {
		return reject("device_config_tcn_lane_mode_invalid", "device_config.tcn.laneMode must be 10 or 12")
	}
	if t.ShakeCount < 0 || t.ShakeCount > 5 {
		return reject("device_config_tcn_shake_count_invalid", "device_config.tcn.shakeCount must be 0..5")
	}
	if t.DropWaitSeconds < 2 || t.DropWaitSeconds > 6 {
		return reject("device_config_tcn_drop_wait_invalid", "device_config.tcn.dropWaitSeconds must be 2..6")
	}
	if !t.CoolingCapable && t.TempControlEnabled {
		return reject("device_config_tcn_temp_control_requires_cooling", "device_config.tcn.tempControlEnabled requires coolingCapable=true")
	}
	if t.CoolingCapable && t.TempControlEnabled {
		if t.TargetTempC < 1 || t.TargetTempC > 12 {
			return reject("device_config_tcn_target_temp_invalid", "device_config.tcn.targetTempC must be 1..12 for refrigerated units")
		}
		if t.TargetTempC < -20 || t.TargetTempC > 100 {
			return reject("device_config_tcn_target_temp_out_of_range", "device_config.tcn.targetTempC out of hardware range")
		}
		if t.TempHysteresisC < 0 || t.TempHysteresisC > 10 {
			return reject("device_config_tcn_temp_hysteresis_invalid", "device_config.tcn.tempHysteresisC invalid")
		}
		if t.TempCompensationC < 0 || t.TempCompensationC > 10 {
			return reject("device_config_tcn_temp_compensation_invalid", "device_config.tcn.tempCompensationC invalid")
		}
		if t.DefrostMinutes < 10 || t.DefrostMinutes > 60 {
			return reject("device_config_tcn_defrost_minutes_invalid", "device_config.tcn.defrostMinutes must be 10..60")
		}
		if t.CompressorWorkMinutes < 30 || t.CompressorWorkMinutes > 240 {
			return reject("device_config_tcn_compressor_work_invalid", "device_config.tcn.compressorWorkMinutes must be 30..240")
		}
		if t.FanStopMinutes < 60 || t.FanStopMinutes > 180 {
			return reject("device_config_tcn_fan_stop_invalid", "device_config.tcn.fanStopMinutes must be 60..180")
		}
	}
	return nil
}

func reject(code, message string) error {
	return ValidationError{Code: code, Message: message}
}
