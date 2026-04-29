package telemetryapp

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	platformmqtt "github.com/avf/avf-vending-api/internal/platform/mqtt"
)

type mqttDispatchHookProbe struct{}

func (mqttDispatchHookProbe) IngestTelemetry(ctx context.Context, in platformmqtt.TelemetryIngest) error {
	_, _ = ctx, in
	return nil
}

func (mqttDispatchHookProbe) IngestShadowReported(ctx context.Context, in platformmqtt.ShadowReportedIngest) error {
	_, _ = ctx, in
	return nil
}

func (mqttDispatchHookProbe) IngestShadowDesired(ctx context.Context, in platformmqtt.ShadowDesiredIngest) error {
	_, _ = ctx, in
	return nil
}

func (mqttDispatchHookProbe) IngestCommandReceipt(ctx context.Context, in platformmqtt.CommandReceiptIngest) error {
	_, _ = ctx, in
	return nil
}

func testdataTelemetryPath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "telemetry", name))
}

func TestTelemetryIngest_EnterpriseTopicTelemetryMachineBodyMismatchRejected(t *testing.T) {
	t.Parallel()
	payload, err := os.ReadFile(testdataTelemetryPath(t, "invalid_critical_wrong_machine_id.json"))
	if err != nil {
		t.Fatal(err)
	}
	topic := "avf/devices/machines/55555555-5555-5555-5555-555555555555/telemetry"
	ing := mqttDispatchHookProbe{}
	h := NewIngestHooks()
	var reasons []string
	innerRejected := h.OnIngressRejected
	h.OnIngressRejected = func(topic string, reason string, payloadBytes int) {
		reasons = append(reasons, reason)
		innerRejected(topic, reason, payloadBytes)
	}
	err = platformmqtt.Dispatch(context.Background(), platformmqtt.TopicLayoutEnterprise, "avf/devices", topic, payload, ing, nil, h)
	if err == nil {
		t.Fatal("expected error")
	}
	found := false
	for _, r := range reasons {
		if r == "machine_id_mismatch" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected machine_id_mismatch in reject reasons, got %v", reasons)
	}
}
