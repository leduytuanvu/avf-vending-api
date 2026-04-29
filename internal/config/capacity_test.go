package config

import "testing"

func TestCapacityLimitsConfig_validate_defaultsLoad(t *testing.T) {
	c := loadCapacityLimitsConfig()
	if err := c.validate(); err != nil {
		t.Fatal(err)
	}
	if c.MaxTelemetryGRPCBatchEvents != defaultMaxTelemetryGRPCBatchEvents {
		t.Fatalf("events default: %d", c.MaxTelemetryGRPCBatchEvents)
	}
}
