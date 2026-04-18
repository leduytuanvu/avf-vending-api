package nats

import (
	"errors"
	"fmt"
	"time"

	natssrv "github.com/nats-io/nats.go"
)

// HandlePullBatch fetches a batch, runs fn per message, Ack on success or Nak on failure (DLQ-friendly consumer loop).
func HandlePullBatch(sub *natssrv.Subscription, batch int, maxWait time.Duration, fn func(*natssrv.Msg) error) error {
	if sub == nil {
		return fmt.Errorf("nats: nil subscription")
	}
	if batch <= 0 {
		batch = 10
	}
	if maxWait <= 0 {
		maxWait = 5 * time.Second
	}
	msgs, err := sub.Fetch(batch, natssrv.MaxWait(maxWait))
	if err != nil {
		if errors.Is(err, natssrv.ErrTimeout) {
			return nil
		}
		return fmt.Errorf("nats: fetch: %w", err)
	}
	for _, m := range msgs {
		if err := fn(m); err != nil {
			_ = m.Nak()
			continue
		}
		if err := m.Ack(); err != nil {
			return fmt.Errorf("nats: ack: %w", err)
		}
	}
	return nil
}
