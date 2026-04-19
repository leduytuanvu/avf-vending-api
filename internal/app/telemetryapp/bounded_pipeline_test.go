package telemetryapp

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	platformmqtt "github.com/avf/avf-vending-api/internal/platform/mqtt"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type stubIngest struct {
	telemetryCalls atomic.Int32
	block          chan struct{}
}

func (s *stubIngest) IngestTelemetry(ctx context.Context, in platformmqtt.TelemetryIngest) error {
	s.telemetryCalls.Add(1)
	if s.block != nil {
		<-s.block
	}
	return nil
}

func (s *stubIngest) IngestShadowReported(ctx context.Context, in platformmqtt.ShadowReportedIngest) error {
	return errors.New("unexpected shadow")
}

func (s *stubIngest) IngestCommandReceipt(ctx context.Context, in platformmqtt.CommandReceiptIngest) error {
	return errors.New("unexpected receipt")
}

func TestBoundedDeviceIngest_queueFullDrop(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	stub := &stubIngest{block: block}
	// Buffer must allow at least two queued jobs while the single worker is blocked on the first
	// (otherwise the second submit races with the worker dequeue and can be dropped as "queue_full").
	tel := config.MQTTDeviceTelemetryConfig{
		GlobalMaxInflight:     2,
		WorkerConcurrency:     1,
		DropOnBackpressure:    true,
		PerMachineMsgsPerSec:  1000,
		PerMachineBurst:       1000,
		SubmitWaitMs:          5000,
	}
	p := NewBoundedDeviceIngest(zap.NewNop(), stub, tel)
	defer p.Close()

	mid := uuid.New()
	ctx := context.Background()
	in := platformmqtt.TelemetryIngest{MachineID: mid, EventType: "ping", Payload: []byte("{}")}

	if err := p.IngestTelemetry(ctx, in); err != nil {
		t.Fatalf("first: %v", err)
	}
	// Let the worker dequeue the first job and block in the stub before filling the buffer,
	// otherwise two quick submits can fill the channel before the worker runs.
	time.Sleep(50 * time.Millisecond)
	for i := 0; i < 2; i++ {
		if err := p.IngestTelemetry(ctx, in); err != nil {
			t.Fatalf("submit %d: %v", i, err)
		}
	}
	if err := p.IngestTelemetry(ctx, in); err != nil {
		t.Fatalf("fourth (drop): %v", err)
	}

	close(block)
	deadline := time.Now().Add(2 * time.Second)
	for stub.telemetryCalls.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if c := stub.telemetryCalls.Load(); c != 3 {
		t.Fatalf("expected 3 processed, got %d", c)
	}
}

func TestBoundedDeviceIngest_perMachineRateLimit(t *testing.T) {
	t.Parallel()
	stub := &stubIngest{}
	tel := config.MQTTDeviceTelemetryConfig{
		GlobalMaxInflight:     32,
		WorkerConcurrency:     2,
		DropOnBackpressure:    true,
		PerMachineMsgsPerSec:  0.25,
		PerMachineBurst:       1,
		SubmitWaitMs:          1000,
	}
	p := NewBoundedDeviceIngest(zap.NewNop(), stub, tel)
	defer p.Close()

	mid := uuid.New()
	ctx := context.Background()
	in := platformmqtt.TelemetryIngest{MachineID: mid, EventType: "ping", Payload: []byte("{}")}

	if err := p.IngestTelemetry(ctx, in); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := p.IngestTelemetry(ctx, in); err == nil {
		t.Fatal("expected rate limit error")
	}
}
