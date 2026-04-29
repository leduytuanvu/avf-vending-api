package bootstrap

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	goredis "github.com/redis/go-redis/v9"
)

func TestRuntimeReady_RedisFailureAllowedWhenNotStrict(t *testing.T) {
	t.Parallel()
	rt := &Runtime{
		cfg: &config.Config{
			ReadinessStrict: false,
			Ops:             config.OperationsConfig{ReadinessTimeout: time.Millisecond},
		},
		rdb: goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"}),
	}
	defer rt.rdb.Close()
	if err := rt.Ready(context.Background()); err != nil {
		t.Fatalf("non-strict readiness should tolerate redis outage: %v", err)
	}
}

func TestRuntimeReady_RedisFailureStrict(t *testing.T) {
	t.Parallel()
	rt := &Runtime{
		cfg: &config.Config{
			ReadinessStrict: true,
			Ops:             config.OperationsConfig{ReadinessTimeout: time.Millisecond},
		},
		rdb: goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"}),
	}
	defer rt.rdb.Close()
	if err := rt.Ready(context.Background()); err == nil {
		t.Fatal("strict readiness should fail on redis outage")
	}
}
