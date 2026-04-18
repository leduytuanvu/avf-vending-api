package refunds

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	natssrv "github.com/nats-io/nats.go"
)

// NATSCoreRefundReviewSink publishes refund review tickets on core NATS (fire-and-forget with broker ack on Publish).
type NATSCoreRefundReviewSink struct {
	nc      *natssrv.Conn
	subject string
}

// NewNATSCoreRefundReviewSink validates subject and returns a sink backed by an existing connection.
func NewNATSCoreRefundReviewSink(nc *natssrv.Conn, subject string) (*NATSCoreRefundReviewSink, error) {
	if nc == nil {
		return nil, fmt.Errorf("refunds: nil NATS connection")
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return nil, fmt.Errorf("refunds: refund review subject is required")
	}
	return &NATSCoreRefundReviewSink{nc: nc, subject: subject}, nil
}

var _ domaincommerce.RefundReviewSink = (*NATSCoreRefundReviewSink)(nil)

// EnqueueRefundReview marshals the ticket and publishes it to the configured subject.
func (s *NATSCoreRefundReviewSink) EnqueueRefundReview(ctx context.Context, ticket domaincommerce.RefundReviewTicket) error {
	_ = ctx
	if s == nil || s.nc == nil {
		return fmt.Errorf("refunds: nil NATSCoreRefundReviewSink")
	}
	b, err := json.Marshal(ticket)
	if err != nil {
		return fmt.Errorf("refunds: marshal ticket: %w", err)
	}
	if err := s.nc.Publish(s.subject, b); err != nil {
		return fmt.Errorf("refunds: nats publish: %w", err)
	}
	return nil
}
