package activation

import machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"

// DeviceFingerprintFromProto maps proto DeviceFingerprint to the activation domain type.
func DeviceFingerprintFromProto(fp *machinev1.DeviceFingerprint) DeviceFingerprint {
	if fp == nil {
		return DeviceFingerprint{}
	}
	return DeviceFingerprint{
		AndroidID:      fp.GetAndroidId(),
		SerialNumber:   fp.GetSerialNumber(),
		Manufacturer:   fp.GetManufacturer(),
		Model:          fp.GetModel(),
		PackageName:    fp.GetPackageName(),
		VersionName:    fp.GetVersionName(),
		VersionCode:    int(fp.GetVersionCode()),
		AndroidSerial:  fp.GetAndroidSerial(),
		BoardSerial:    fp.GetBoardSerial(),
		DeviceSerial:   fp.GetDeviceSerial(),
		SimSerial:      fp.GetSimSerial(),
		SimIccid:       fp.GetSimIccid(),
		SimOperator:    fp.GetSimOperator(),
		SimCountryIso:  fp.GetSimCountryIso(),
		Brand:          fp.GetBrand(),
		DeviceModel:    fp.GetDeviceModel(),
		Hardware:       fp.GetHardware(),
		Product:        fp.GetProduct(),
		AndroidRelease: fp.GetAndroidRelease(),
		SdkInt:         int(fp.GetSdkInt()),
		AppBuildSha:    fp.GetAppBuildSha(),
		BootID:         fp.GetBootId(),
		NetworkType:    fp.GetNetworkType(),
		NetworkState:   fp.GetNetworkState(),
	}
}
