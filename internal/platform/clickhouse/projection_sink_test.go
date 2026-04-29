package clickhouse

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type failingProjectionClient struct {
	calls atomic.Int32
}

func (f *failingProjectionClient) Ping(context.Context) error { return nil }
func (f *failingProjectionClient) Close() error               { return nil }
func (f *failingProjectionClient) InsertJSONEachRow(context.Context, string, []byte) error {
	f.calls.Add(1)
	return errors.New("clickhouse unavailable")
}

func TestAsyncProjectionSink_FailureDoesNotReturnToCaller(t *testing.T) {
	t.Parallel()
	client := &failingProjectionClient{}
	sink, err := NewAsyncProjectionSink(zap.NewNop(), client, "analytics_projection", 1, time.Millisecond, 2)
	if err != nil {
		t.Fatal(err)
	}
	sink.EnqueuePublished(domaincommerce.OutboxEvent{
		ID:            99,
		EventType:     "payment.captured",
		AggregateType: "payment",
		AggregateID:   uuid.New(),
		CreatedAt:     time.Now().UTC().Add(-time.Second),
	})
	sink.Shutdown()
	if got := client.calls.Load(); got != 2 {
		t.Fatalf("calls=%d want 2", got)
	}
}
