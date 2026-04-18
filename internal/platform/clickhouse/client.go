package clickhouse

import (
	"context"
	"fmt"
	"strings"
)

// Client is the ClickHouse HTTP client surface used for cold-path analytics inserts.
type Client interface {
	Ping(ctx context.Context) error
	Close() error
	// InsertJSONEachRow runs INSERT ... FORMAT JSONEachRow with a single line payload (including trailing newline).
	InsertJSONEachRow(ctx context.Context, table string, line []byte) error
}

// Config holds optional ClickHouse HTTP settings. Disabled remains the default.
type Config struct {
	Enabled bool
	// HTTPEndpoint is a ClickHouse HTTP URL including the database path segment, e.g.
	// http://avf:avf@localhost:8123/avf
	HTTPEndpoint string
}

type noopClient struct{}

// NewNoopClient returns a Client that performs no network I/O.
func NewNoopClient() Client {
	return noopClient{}
}

func (noopClient) Ping(ctx context.Context) error {
	_ = ctx
	return nil
}

func (noopClient) Close() error {
	return nil
}

func (noopClient) InsertJSONEachRow(context.Context, string, []byte) error {
	return nil
}

// Open returns NewNoopClient when cfg.Enabled is false. When Enabled is true, Open dials via HTTP
// (native TCP driver intentionally not used to keep dependencies minimal).
func Open(ctx context.Context, cfg Config) (Client, error) {
	if !cfg.Enabled {
		return NewNoopClient(), nil
	}
	pingCtx := ctx
	if pingCtx == nil {
		pingCtx = context.Background()
	}
	ep := strings.TrimSpace(cfg.HTTPEndpoint)
	if ep == "" {
		return nil, fmt.Errorf("clickhouse: enabled but HTTPEndpoint is empty")
	}
	hc, err := newHTTPClient(ep)
	if err != nil {
		return nil, err
	}
	if err := hc.Ping(pingCtx); err != nil {
		_ = hc.Close()
		return nil, fmt.Errorf("clickhouse: ping: %w", err)
	}
	return hc, nil
}
