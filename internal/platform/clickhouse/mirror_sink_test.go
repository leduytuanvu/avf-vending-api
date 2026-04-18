package clickhouse

import (
	"context"
	"testing"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type blockingClient struct {
	wait chan struct{}
}

func (b *blockingClient) Ping(context.Context) error { return nil }
func (b *blockingClient) Close() error               { return nil }

func (b *blockingClient) InsertJSONEachRow(ctx context.Context, _ string, _ []byte) error {
	select {
	case <-b.wait:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestAsyncOutboxMirrorSink_DropsWhenSaturated(t *testing.T) {
	t.Parallel()
	wait := make(chan struct{})
	bc := &blockingClient{wait: wait}
	sink, err := NewAsyncOutboxMirrorSink(zap.NewNop(), bc, "t", 1, 30*time.Second, 1)
	if err != nil {
		t.Fatal(err)
	}
	ev := domaincommerce.OutboxEvent{ID: 1, AggregateID: uuid.New()}
	sink.EnqueuePublished(ev)
	sink.EnqueuePublished(ev)
	time.Sleep(20 * time.Millisecond)
	close(wait)
	sink.Shutdown()
}
