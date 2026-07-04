package httpserver

import (
	"encoding/json"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/activation"
)

// fingerprintDTO accepts both camelCase and snake_case JSON keys for device fingerprint fields.
type fingerprintDTO struct {
	activation.DeviceFingerprint
}

func (f *fingerprintDTO) UnmarshalJSON(data []byte) error {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	getStr := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := m[k]; ok {
				if s, ok := v.(string); ok {
					return strings.TrimSpace(s)
				}
			}
		}
		return ""
	}
	getInt := func(keys ...string) int {
		for _, k := range keys {
			if v, ok := m[k]; ok {
				switch n := v.(type) {
				case float64:
					return int(n)
				case json.Number:
					i, _ := n.Int64()
					return int(i)
				}
			}
		}
		return 0
	}
	f.DeviceFingerprint = activation.DeviceFingerprint{
		AndroidID:      getStr("androidId", "android_id"),
		SerialNumber:   getStr("serialNumber", "serial_number"),
		Manufacturer:   getStr("manufacturer"),
		Model:          getStr("model"),
		PackageName:    getStr("packageName", "package_name"),
		VersionName:    getStr("versionName", "version_name"),
		VersionCode:    getInt("versionCode", "version_code"),
		AndroidSerial:  getStr("androidSerial", "android_serial"),
		BoardSerial:    getStr("boardSerial", "board_serial"),
		DeviceSerial:   getStr("deviceSerial", "device_serial"),
		SimSerial:      getStr("simSerial", "sim_serial"),
		SimIccid:       getStr("simIccid", "sim_iccid"),
		SimOperator:    getStr("simOperator", "sim_operator"),
		SimCountryIso:  getStr("simCountryIso", "sim_country_iso"),
		Brand:          getStr("brand"),
		DeviceModel:    getStr("deviceModel", "device_model"),
		Hardware:       getStr("hardware"),
		Product:        getStr("product"),
		AndroidRelease: getStr("androidRelease", "android_release"),
		SdkInt:         getInt("sdkInt", "sdk_int"),
		AppBuildSha:    getStr("appBuildSha", "app_build_sha"),
		BootID:         getStr("bootId", "boot_id"),
		NetworkType:    getStr("networkType", "network_type"),
		NetworkState:   getStr("networkState", "network_state"),
	}
	return nil
}
