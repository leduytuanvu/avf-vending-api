package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appartifacts "github.com/avf/avf-vending-api/internal/app/artifacts"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/grpcserver"
	"github.com/avf/avf-vending-api/internal/httpserver"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/observability"
	platformmqtt "github.com/avf/avf-vending-api/internal/platform/mqtt"
	platformnats "github.com/avf/avf-vending-api/internal/platform/nats"
	"github.com/avf/avf-vending-api/internal/platform/objectstore"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// RunAPI boots the HTTP API (and optional internal gRPC) until ctx is cancelled.
func RunAPI(ctx context.Context, cfg *config.Config, log *zap.Logger) error {
	if cfg == nil || log == nil {
		return fmt.Errorf("bootstrap: nil dependency")
	}
	if cfg.AppEnv == config.AppEnvProduction && strings.TrimSpace(os.Getenv(platformnats.EnvNATSURL)) == "" {
		return fmt.Errorf("bootstrap: APP_ENV=production requires non-empty %s (NATS/JetStream is mandatory for telemetry and related async paths)", platformnats.EnvNATSURL)
	}

	shutdownTracer, err := observability.InitTracer(ctx, cfg)
	if err != nil {
		return fmt.Errorf("bootstrap: init tracer: %w", err)
	}
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := shutdownTracer(sctx); err != nil {
			log.Warn("tracer shutdown error", zap.Error(err))
		}
	}()

	rt, err := BuildRuntime(ctx, cfg)
	if err != nil {
		return fmt.Errorf("bootstrap: build runtime: %w", err)
	}
	defer rt.Close()

	mqttBroker := platformmqtt.LoadBrokerFromEnv()
	log.Info("mqtt bootstrap config",
		zap.Bool("require_mqtt_publisher", cfg.APIWiring.RequireMQTTPublisher),
		zap.String("mqtt_broker_url", mqttBroker.BrokerURL),
		zap.String("mqtt_client_id", mqttBroker.ClientID),
		zap.String("mqtt_username", mqttBroker.Username),
		zap.String("mqtt_topic_prefix", mqttBroker.TopicPrefix),
	)
	switch {
	case cfg.APIWiring.RequireMQTTPublisher:
		if err := mqttBroker.Validate(); err != nil {
			return fmt.Errorf("bootstrap mqtt validate failed: %w", err)
		}
		pub, perr := platformmqtt.NewPublisher(mqttBroker, log, "-api-publish")
		if perr != nil {
			return fmt.Errorf("bootstrap mqtt publisher init failed: %w", perr)
		}
		rt.Deps.MQTTPublisher = pub
		rt.SetMQTTDisconnect(pub.Close)
	default:
		if strings.TrimSpace(mqttBroker.BrokerURL) != "" {
			pub, perr := platformmqtt.NewPublisher(mqttBroker, log, "-api-publish")
			if perr != nil {
				log.Warn("mqtt publisher disabled", zap.Error(perr))
			} else {
				rt.Deps.MQTTPublisher = pub
				rt.SetMQTTDisconnect(pub.Close)
			}
		}
	}

	if err := ValidateRuntimeWiring(cfg, rt); err != nil {
		return err
	}
	if rt.Pool() == nil {
		return fmt.Errorf("bootstrap: DATABASE_URL is required for the HTTP API process")
	}

	store := postgres.NewStore(rt.Pool())
	fleetRepo := postgres.NewFleetRepository(rt.Pool())
	fleetSvc := appfleet.NewService(fleetRepo)
	commerceSvc := appcommerce.NewService(appcommerce.Deps{
		OrderVend:      store,
		PaymentOutbox:  store,
		Lifecycle:      store,
		WebhookPersist: store,
	})
	var artifactSvc *appartifacts.Service
	if cfg.Artifacts.Enabled {
		obj, oerr := objectstore.NewFromEnv(ctx)
		if oerr != nil {
			return fmt.Errorf("bootstrap: object store for artifacts (set S3 env or disable API_ARTIFACTS_ENABLED): %w", oerr)
		}
		artifactSvc = appartifacts.NewService(appartifacts.Deps{
			Store:              obj,
			MaxUploadBytes:     cfg.Artifacts.MaxUploadBytes,
			DownloadPresignTTL: cfg.Artifacts.DownloadPresignTTL,
			ListMaxKeys:        cfg.Artifacts.ListMaxKeys,
		})
	}
	httpApp := api.NewHTTPApplication(api.HTTPApplicationDeps{
		Store:              store,
		Fleet:              fleetSvc,
		Commerce:           commerceSvc,
		MQTTCommandPublish: rt.Deps.MQTTPublisher,
		Artifacts:          artifactSvc,
		HTTPAuth:           cfg.HTTPAuth,
	})

	httpSrv, err := httpserver.NewHTTPServer(cfg, log, rt, httpApp)
	if err != nil {
		return fmt.Errorf("bootstrap: http server: %w", err)
	}

	// gRPC: health check only; no domain ServiceRegistrars wired (see internal/grpcserver).
	grpcSrv, err := grpcserver.NewServer(cfg, log)
	if err != nil {
		return fmt.Errorf("bootstrap: grpc server: %w", err)
	}

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		return httpSrv.ListenAndServe(gctx)
	})

	if grpcSrv != nil {
		gs := grpcSrv
		g.Go(func() error {
			return gs.ListenAndServe(gctx)
		})
	}

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
