package mqtt

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestPublisherPublishDeviceDispatch_nilClient(t *testing.T) {
	var p *Publisher
	err := p.PublishDeviceDispatch(context.Background(), uuid.Nil, []byte("{}"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPublisherHealth_nil(t *testing.T) {
	var p *Publisher
	err := p.Health(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}
