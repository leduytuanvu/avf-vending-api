package nats

import (
	"context"
	"fmt"
	"time"

	natssrv "github.com/nats-io/nats.go"
)

// Client bundles a core NATS connection with a JetStream context.
type Client struct {
	Conn *natssrv.Conn
	JS   natssrv.JetStreamContext
}

// ConnectJetStream opens NATS and prepares JetStream. Pass url from NATS_URL (e.g. nats://127.0.0.1:4222).
func ConnectJetStream(ctx context.Context, url string, name string) (*Client, error) {
	if url == "" {
		return nil, fmt.Errorf("nats: empty url")
	}
	opts := []natssrv.Option{
		natssrv.Name(name),
		natssrv.MaxReconnects(-1),
		natssrv.ReconnectWait(time.Second),
		natssrv.Timeout(10 * time.Second),
	}
	// nats.go v1.38+: Connect no longer accepts ContextOpt; use request-scoped
	// nats.Context(ctx) on JetStream / subscribe calls instead.
	_ = ctx
	nc, err := natssrv.Connect(url, opts...)
	if err != nil {
		return nil, fmt.Errorf("nats: connect: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		_ = nc.Drain()
		return nil, fmt.Errorf("nats: jetstream: %w", err)
	}
	return &Client{Conn: nc, JS: js}, nil
}

// JetStreamFromConn returns JetStream context for an existing connection.
func JetStreamFromConn(nc *natssrv.Conn) (natssrv.JetStreamContext, error) {
	if nc == nil {
		return nil, fmt.Errorf("nats: nil connection")
	}
	return nc.JetStream()
}
