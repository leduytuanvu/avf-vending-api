package nats

import (
	"context"
	"fmt"
	"strings"

	natssrv "github.com/nats-io/nats.go"
)

// PublishDLQ copies a failed delivery into the DLQ stream with reason metadata (explicit replay path).
func PublishDLQ(ctx context.Context, js natssrv.JetStreamContext, reason string, parent natssrv.Header, body []byte) error {
	if js == nil {
		return fmt.Errorf("nats: nil jetstream for dlq")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "unknown"
	}
	subj := DLQSubject(reason)
	h := natssrv.Header{}
	if parent != nil {
		for k, vals := range parent {
			h[k] = append(h[k], vals...)
		}
	}
	h.Set("X-DLQ-Reason", reason)
	msg := &natssrv.Msg{Subject: subj, Data: body, Header: h}
	opts := []natssrv.PubOpt{}
	if ctx != nil {
		opts = append(opts, natssrv.Context(ctx))
	}
	_, err := js.PublishMsg(msg, opts...)
	if err != nil {
		return fmt.Errorf("nats: dlq publish %s: %w", subj, err)
	}
	return nil
}
