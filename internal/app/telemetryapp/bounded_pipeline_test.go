//go:build !windows

package telemetryapp

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	platformmqtt "github.com/avf/avf-vending-api/internal/platform/mqtt"
	tel "github.com/avf/avf-vending-api/internal/platform/telemetry"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type stubIngest struct {
	telemetryCalls atomic.Int32
	shadowCalls    atomic.Int32
	receiptCalls   atomic.Int32
	block          chan struct{}
	mu             sync.Mutex
	eventTypes     []string
}

// blockForTelemetry gates only bounded-queue telemetry; critical_no_drop runs inline and must not wait on block.
func (s *stubIngest) blockForTelemetry(eventType string) bool {
	if s.block == nil {
		return false
	}
	return tel.CriticalityForEventType(eventType) != tel.CriticalityCriticalNoDrop
}

func (s *stubIngest) IngestTelemetry(ctx context.Context, in platformmqtt.TelemetryIngest) error {
	s.telemetryCalls.Add(1)
	s.mu.Lock()
	s.eventTypes = append(s.eventTypes, in.EventType)
	s.mu.Unlock()
	if s.blockForTelemetry(in.EventType) {
		<-s.block
	}
	return nil
}

func (s *stubIngest) IngestShadowReported(ctx context.Context, in platformmqtt.ShadowReportedIngest) error {
	s.shadowCalls.Add(1)
	if s.block != nil {
		<-s.block
	}
	return nil
}

func (s *stubIngest) IngestShadowDesired(ctx context.Context, in platformmqtt.ShadowDesiredIngest) error {
	s.shadowCalls.Add(1)
	if s.block != nil {
		<-s.block
	}
	return nil
}

func (s *stubIngest) IngestCommandReceipt(ctx context.Context, in platformmqtt.CommandReceiptIngest) error {
	s.receiptCalls.Add(1)
	return nil
}

type stubIngestErr struct {
	telemetryCalls atomic.Int32
	receiptCalls   atomic.Int32
	errTelemetry   error
	errReceipt     error
}

func (s *stubIngestErr) IngestTelemetry(ctx context.Context, in platformmqtt.TelemetryIngest) error {
	s.telemetryCalls.Add(1)
	return s.errTelemetry
}

func (s *stubIngestErr) IngestShadowReported(ctx context.Context, in platformmqtt.ShadowReportedIngest) error {
	return errors.New("unexpected shadow")
}

func (s *stubIngestErr) IngestShadowDesired(ctx context.Context, in platformmqtt.ShadowDesiredIngest) error {
	return errors.New("unexpected shadow desired")
}

func (s *stubIngestErr) IngestCommandReceipt(ctx context.Context, in platformmqtt.CommandReceiptIngest) error {
	s.receiptCalls.Add(1)
	if s.errReceipt != nil {
		return s.errReceipt
	}
	return nil
}

func TestBoundedDeviceIngest_queueFullDrop(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	var unblockOnce sync.Once
	unblock := func() { unblockOnce.Do(func() { close(block) }) }
	stub := &stubIngest{block: block}
	// Buffer must allow at least two queued jobs while the single worker is blocked on the first
	// (otherwise the second submit races with the worker dequeue and can be dropped as "queue_full").
	tel := config.MQTTDeviceTelemetryConfig{
		GlobalMaxInflight:    2,
		WorkerConcurrency:    1,
		DropOnBackpressure:   true,
		PerMachineMsgsPerSec: 1000,
		PerMachineBurst:      1000,
		SubmitWaitMs:         5000,
	}
	p := NewBoundedDeviceIngest(zap.NewNop(), stub, tel)
	defer func() {
		unblock()
		p.Close()
	}()

	mid := uuid.New()
	ctx := context.Background()
	in := platformmqtt.TelemetryIngest{MachineID: mid, EventType: "heartbeat", Payload: []byte("{}")}

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

	unblock()
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
		GlobalMaxInflight:    32,
		WorkerConcurrency:    2,
		DropOnBackpressure:   true,
		PerMachineMsgsPerSec: 0.25,
		PerMachineBurst:      1,
		SubmitWaitMs:         1000,
	}
	p := NewBoundedDeviceIngest(zap.NewNop(), stub, tel)
	defer p.Close()

	mid := uuid.New()
	ctx := context.Background()
	in := platformmqtt.TelemetryIngest{MachineID: mid, EventType: "heartbeat", Payload: []byte("{}")}

	if err := p.IngestTelemetry(ctx, in); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := p.IngestTelemetry(ctx, in); err == nil {
		t.Fatal("expected rate limit error")
	}
}

func TestBoundedDeviceIngest_criticalTelemetryBypassesQueueUnderDroppablePressure(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	var unblockOnce sync.Once
	unblock := func() { unblockOnce.Do(func() { close(block) }) }
	stub := &stubIngest{block: block}
	telCfg := config.MQTTDeviceTelemetryConfig{
		GlobalMaxInflight:    1,
		WorkerConcurrency:    1,
		DropOnBackpressure:   true,
		PerMachineMsgsPerSec: 1000,
		PerMachineBurst:      1000,
		SubmitWaitMs:         1000,
	}
	p := NewBoundedDeviceIngest(zap.NewNop(), stub, telCfg)
	defer func() {
		unblock()
		p.Close()
	}()

	mid := uuid.New()
	ctx := context.Background()
	hb := platformmqtt.TelemetryIngest{MachineID: mid, EventType: "heartbeat", Payload: []byte("{}")}
	if err := p.IngestTelemetry(ctx, hb); err != nil {
		t.Fatalf("heartbeat first: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := p.IngestTelemetry(ctx, hb); err != nil {
		t.Fatalf("heartbeat second: %v", err)
	}
	if err := p.IngestTelemetry(ctx, hb); err != nil {
		t.Fatalf("heartbeat third should drop: %v", err)
	}

	criticalTypes := []string{
		"events.vend",
		"payment.capture",
		"events.cash",
		"events.inventory",
	}
	for _, et := range criticalTypes {
		in := platformmqtt.TelemetryIngest{MachineID: mid, EventType: et, Payload: []byte("{}")}
		if err := p.IngestTelemetry(ctx, in); err != nil {
			t.Fatalf("critical %q: %v", et, err)
		}
		if got := tel.CriticalityForEventType(et); got != tel.CriticalityCriticalNoDrop {
			t.Fatalf("criticality %q: want critical_no_drop got %s", et, got)
		}
	}

	unblock()
	deadline := time.Now().Add(3 * time.Second)
	for stub.telemetryCalls.Load() < 2+int32(len(criticalTypes)) && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	wantMin := int32(2 + len(criticalTypes))
	if c := stub.telemetryCalls.Load(); c < wantMin {
		t.Fatalf("expected at least %d telemetry ingests, got %d", wantMin, c)
	}
}

func TestBoundedDeviceIngest_criticalCommandReceiptBypassesQueue(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	var unblockOnce sync.Once
	unblock := func() { unblockOnce.Do(func() { close(block) }) }
	stub := &stubIngest{block: block}
	telCfg := config.MQTTDeviceTelemetryConfig{
		GlobalMaxInflight:    1,
		WorkerConcurrency:    1,
		DropOnBackpressure:   true,
		PerMachineMsgsPerSec: 1000,
		PerMachineBurst:      1000,
		SubmitWaitMs:         1000,
	}
	p := NewBoundedDeviceIngest(zap.NewNop(), stub, telCfg)
	defer func() {
		unblock()
		p.Close()
	}()

	mid := uuid.New()
	ctx := context.Background()
	hb := platformmqtt.TelemetryIngest{MachineID: mid, EventType: "heartbeat", Payload: []byte("{}")}
	if err := p.IngestTelemetry(ctx, hb); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := p.IngestTelemetry(ctx, hb); err != nil {
		t.Fatalf("heartbeat2: %v", err)
	}

	rec := platformmqtt.CommandReceiptIngest{
		MachineID: mid, Sequence: 1, Status: "acked", DedupeKey: "d1", Payload: []byte("{}"),
	}
	if err := p.IngestCommandReceipt(ctx, rec); err != nil {
		t.Fatalf("command receipt: %v", err)
	}
	if stub.receiptCalls.Load() != 1 {
		t.Fatalf("expected 1 receipt ingest, got %d", stub.receiptCalls.Load())
	}
}

func TestBoundedDeviceIngest_criticalReturnsInnerErrorWithoutSuccess(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("downstream failed")
	stub := &stubIngestErr{errTelemetry: wantErr}
	telCfg := config.MQTTDeviceTelemetryConfig{
		GlobalMaxInflight:    8,
		WorkerConcurrency:    2,
		DropOnBackpressure:   true,
		PerMachineMsgsPerSec: 1000,
		PerMachineBurst:      1000,
		SubmitWaitMs:         1000,
	}
	p := NewBoundedDeviceIngest(zap.NewNop(), stub, telCfg)
	defer p.Close()

	mid := uuid.New()
	ctx := context.Background()
	in := platformmqtt.TelemetryIngest{MachineID: mid, EventType: "events.vend", Payload: []byte("{}")}
	if err := p.IngestTelemetry(ctx, in); !errors.Is(err, wantErr) {
		t.Fatalf("want %v got %v", wantErr, err)
	}
	if stub.telemetryCalls.Load() != 1 {
		t.Fatalf("inner called once, got %d", stub.telemetryCalls.Load())
	}
}

func TestBoundedDeviceIngest_compactableShadowLatestCoalesces(t *testing.T) {
	t.Parallel()
	block := make(chan struct{})
	var unblockOnce sync.Once
	unblock := func() { unblockOnce.Do(func() { close(block) }) }
	stub := &stubIngest{block: block}
	telCfg := config.MQTTDeviceTelemetryConfig{
		GlobalMaxInflight:    1,
		WorkerConcurrency:    1,
		DropOnBackpressure:   true,
		PerMachineMsgsPerSec: 1000,
		PerMachineBurst:      1000,
		SubmitWaitMs:         1000,
	}
	p := NewBoundedDeviceIngest(zap.NewNop(), stub, telCfg)
	defer func() {
		unblock()
		p.Close()
	}()

	mid := uuid.New()
	ctx := context.Background()

	if err := p.IngestShadowReported(ctx, platformmqtt.ShadowReportedIngest{MachineID: mid, ReportedJSON: []byte(`{"v":1}`)}); err != nil {
		t.Fatalf("first: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := p.IngestShadowReported(ctx, platformmqtt.ShadowReportedIngest{MachineID: mid, ReportedJSON: []byte(`{"v":2}`)}); err != nil {
		t.Fatalf("second: %v", err)
	}
	if err := p.IngestShadowReported(ctx, platformmqtt.ShadowReportedIngest{MachineID: mid, ReportedJSON: []byte(`{"v":3}`)}); err != nil {
		t.Fatalf("third compacted: %v", err)
	}

	unblock()
	deadline := time.Now().Add(2 * time.Second)
	for stub.shadowCalls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if c := stub.shadowCalls.Load(); c != 2 {
		t.Fatalf("expected 2 shadow calls after compaction, got %d", c)
	}
}
