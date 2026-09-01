package config

import (
	"os"
	"strconv"
	"time"
)

// DeviceClockSkewConfig bounds acceptance of device-supplied occurred_at timestamps.
type DeviceClockSkewConfig struct {
	CheckInFutureMax        time.Duration
	CheckInPastMax          time.Duration
	OfflineFutureMax        time.Duration
	OfflineMinYear          int
	AdminInventoryPastMax   time.Duration
	AdminInventoryFutureMax time.Duration
}

func loadDeviceClockSkewFromEnv() DeviceClockSkewConfig {
	return DeviceClockSkewConfig{
		CheckInFutureMax:        envDurationMinutes("DEVICE_CLOCK_CHECKIN_FUTURE_MAX_MIN", 5),
		CheckInPastMax:          envDurationMinutes("DEVICE_CLOCK_CHECKIN_PAST_MAX_MIN", 60),
		OfflineFutureMax:        envDurationMinutes("DEVICE_CLOCK_OFFLINE_FUTURE_MAX_MIN", 5),
		OfflineMinYear:          envIntDefault("DEVICE_CLOCK_OFFLINE_MIN_YEAR", 2020),
		AdminInventoryPastMax:   envDurationHours("DEVICE_CLOCK_ADMIN_INVENTORY_PAST_MAX_H", 24),
		AdminInventoryFutureMax: envDurationMinutes("DEVICE_CLOCK_ADMIN_INVENTORY_FUTURE_MAX_MIN", 5),
	}
}

func envDurationMinutes(key string, defaultMin int) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return time.Duration(defaultMin) * time.Minute
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return time.Duration(defaultMin) * time.Minute
	}
	return time.Duration(n) * time.Minute
}

func envDurationHours(key string, defaultH int) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return time.Duration(defaultH) * time.Hour
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return time.Duration(defaultH) * time.Hour
	}
	return time.Duration(n) * time.Hour
}

func envIntDefault(key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}
