package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	"github.com/avf/avf-vending-api/internal/config"
	platformdb "github.com/avf/avf-vending-api/internal/platform/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

// AuthRuntime is the authentication boundary for inbound HTTP (extend with real verification later).
type AuthRuntime interface {
	Health(ctx context.Context) error
}

// OutboxPublisher delivers durable outbox rows to async transports (JetStream, etc.).
type OutboxPublisher interface {
	Health(ctx context.Context) error
}

// MQTTPublisher publishes outbound MQTT messages to the device edge.
type MQTTPublisher interface {
	Health(ctx context.Context) error
	PublishDeviceDispatch(ctx context.Context, machineID uuid.UUID, payload []byte) error
}

// NATSRuntime is publish/consume access to NATS JetStream (or core) when wired.
type NATSRuntime interface {
	Health(ctx context.Context) error
}

// PaymentProviderRegistry resolves outbound PSP clients (implementations added later).
type PaymentProviderRegistry interface {
	Health(ctx context.Context) error
}

// WorkflowOrchestration schedules durable long-running work (Temporal when enabled).
// Always non-nil: use [workfloworch.Boundary.Enabled] before [workfloworch.Boundary.Start].
type WorkflowOrchestration = workfloworch.Boundary

// RuntimeDeps holds optional subsystem integrations. Nil fields are skipped for readiness
// and are not treated as healthy no-ops.
type RuntimeDeps struct {
	Auth                  AuthRuntime
	OutboxPublisher       OutboxPublisher
	MQTTPublisher         MQTTPublisher
	NATS                  NATSRuntime
	PaymentProviders      PaymentProviderRegistry
	WorkflowOrchestration WorkflowOrchestration
}

// Runtime holds cross-cutting infrastructure clients for the API process.
type Runtime struct {
	cfg *config.Config

	pool *pgxpool.Pool
	rdb  *goredis.Client

	Deps RuntimeDeps

	mqttClose func()

	close func()
}

// BuildRuntime constructs optional infrastructure clients from configuration.
func BuildRuntime(ctx context.Context, cfg *config.Config) (*Runtime, error) {
	if cfg == nil {
		return nil, fmt.Errorf("bootstrap: nil dependency")
	}

	pool, err := platformdb.NewPool(ctx, &cfg.Postgres)
	if err != nil {
		return nil, err
	}

	rdb, err := newRedisClient(&cfg.Redis)
	if err != nil {
		if pool != nil {
			pool.Close()
		}
		return nil, err
	}

	closeFn := func() {
		if rdb != nil {
			_ = rdb.Close()
		}
		if pool != nil {
			pool.Close()
		}
	}

	orch, temporalCleanup, err := BuildWorkflowOrchestration(ctx, cfg)
	if err != nil {
		closeFn()
		return nil, err
	}

	prevClose := closeFn
	closeFn = func() {
		if temporalCleanup != nil {
			temporalCleanup()
		}
		prevClose()
	}

	rt := &Runtime{
		cfg:   cfg,
		pool:  pool,
		rdb:   rdb,
		Deps:  RuntimeDeps{WorkflowOrchestration: orch},
		close: closeFn,
	}
	return rt, nil
}

// Pool exposes the Postgres pool when configured (nil if DATABASE_URL is unset).
func (r *Runtime) Pool() *pgxpool.Pool {
	if r == nil {
		return nil
	}
	return r.pool
}

// SetMQTTDisconnect registers an optional teardown hook for the MQTT publisher client.
func (r *Runtime) SetMQTTDisconnect(fn func()) {
	if r == nil {
		return
	}
	r.mqttClose = fn
}

// Close releases infrastructure clients.
func (r *Runtime) Close() {
	if r == nil || r.close == nil {
		return
	}
	if r.mqttClose != nil {
		r.mqttClose()
	}
	r.close()
}

type subsystemHealth interface {
	Health(context.Context) error
}

func probeHealth(ctx context.Context, timeout time.Duration, name string, sub subsystemHealth) error {
	if sub == nil {
		return nil
	}
	pctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if err := sub.Health(pctx); err != nil {
		return fmt.Errorf("readiness: %s: %w", name, err)
	}
	return nil
}

// Ready implements httpserver.ReadinessProbe.
func (r *Runtime) Ready(ctx context.Context) error {
	if r == nil || r.cfg == nil {
		return errors.New("bootstrap: nil runtime")
	}
	readinessTimeout := r.cfg.Ops.ReadinessTimeout

	if r.cfg.ReadinessStrict && r.pool == nil && r.rdb == nil {
		return fmt.Errorf("readiness: strict mode requires DATABASE_URL and/or REDIS_ADDR")
	}

	if r.pool != nil {
		pctx, cancel := context.WithTimeout(ctx, readinessTimeout)
		err := r.pool.Ping(pctx)
		cancel()
		if err != nil {
			return fmt.Errorf("readiness: postgres ping: %w", err)
		}
	}

	if r.rdb != nil {
		rctx, cancel := context.WithTimeout(ctx, readinessTimeout)
		err := r.rdb.Ping(rctx).Err()
		cancel()
		if err != nil {
			return fmt.Errorf("readiness: redis ping: %w", err)
		}
	}

	if err := probeHealth(ctx, readinessTimeout, "auth", r.Deps.Auth); err != nil {
		return err
	}
	if err := probeHealth(ctx, readinessTimeout, "outbox", r.Deps.OutboxPublisher); err != nil {
		return err
	}
	if err := probeHealth(ctx, readinessTimeout, "mqtt", r.Deps.MQTTPublisher); err != nil {
		return err
	}
	if err := probeHealth(ctx, readinessTimeout, "nats", r.Deps.NATS); err != nil {
		return err
	}
	if err := probeHealth(ctx, readinessTimeout, "payments", r.Deps.PaymentProviders); err != nil {
		return err
	}

	return nil
}

func newRedisClient(cfg *config.RedisConfig) (*goredis.Client, error) {
	if cfg == nil || cfg.Addr == "" {
		return nil, nil
	}
	opts := &goredis.Options{
		Addr:      cfg.Addr,
		Username:  cfg.Username,
		Password:  cfg.Password,
		DB:        cfg.DB,
		TLSConfig: cfg.TLSConfig(),
	}
	return goredis.NewClient(opts), nil
}

// ValidateRuntimeWiring fails startup when an enabled capability flag requires a missing adapter.
func ValidateRuntimeWiring(cfg *config.Config, rt *Runtime) error {
	if cfg == nil || rt == nil {
		return fmt.Errorf("bootstrap: nil dependency")
	}
	w := cfg.APIWiring
	if w.RequireAuthAdapter && rt.Deps.Auth == nil {
		return fmt.Errorf("bootstrap: API_REQUIRE_AUTH_ADAPTER=true but RuntimeDeps.Auth is nil")
	}
	if w.RequireOutboxPublisher && rt.Deps.OutboxPublisher == nil {
		return fmt.Errorf("bootstrap: API_REQUIRE_OUTBOX_PUBLISHER=true but RuntimeDeps.OutboxPublisher is nil")
	}
	if w.RequireMQTTPublisher && rt.Deps.MQTTPublisher == nil {
		return fmt.Errorf("bootstrap: API_REQUIRE_MQTT_PUBLISHER=true but RuntimeDeps.MQTTPublisher is nil")
	}
	if w.RequireNATSRuntime && rt.Deps.NATS == nil {
		return fmt.Errorf("bootstrap: API_REQUIRE_NATS_RUNTIME=true but RuntimeDeps.NATS is nil")
	}
	if w.RequirePaymentProviderRegistry && rt.Deps.PaymentProviders == nil {
		return fmt.Errorf("bootstrap: API_REQUIRE_PAYMENT_PROVIDER_REGISTRY=true but RuntimeDeps.PaymentProviders is nil")
	}
	return nil
}
