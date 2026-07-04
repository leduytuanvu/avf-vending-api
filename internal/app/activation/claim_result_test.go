package activation

import (
	"testing"

	"github.com/google/uuid"
)

func TestClaimResult_includesDeviceAttachmentAndMqttFields(t *testing.T) {
	attachID := uuid.New()
	result := ClaimResult{
		MachineID:          uuid.New(),
		MQTTUsername:       "mqtt-user",
		MQTTPassword:       "mqtt-pass",
		DeviceAttachmentID: &attachID,
	}
	if result.MQTTUsername != "mqtt-user" {
		t.Fatalf("mqtt username missing")
	}
	if result.MQTTPassword != "mqtt-pass" {
		t.Fatalf("mqtt password missing")
	}
	if result.DeviceAttachmentID == nil || *result.DeviceAttachmentID != attachID {
		t.Fatalf("device attachment id missing")
	}
}
