package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appartifacts "github.com/avf/avf-vending-api/internal/app/artifacts"
	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	appfleet "github.com/avf/avf-vending-api/internal/app/fleet"
	appmediaadmin "github.com/avf/avf-vending-api/internal/app/mediaadmin"
	appsalecatalog "github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/grpcserver"
	"github.com/avf/avf-vending-api/internal/httpserver"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/observability"
	"github.com/avf/avf-vending-api/internal/observability/dbpoolmetrics"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/auth/revocation"
	platformmqtt "github.com/avf/avf-vending-api/internal/platform/mqtt"
	platformnats "github.com/avf/avf-vending-api/internal/platform/nats"
	"github.com/avf/avf-vending-api/internal/platform/objectstore"
	platformredis "github.com/avf/avf-vending-api/internal/platform/redis"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// RunAPI boots the HTTP API (and optional internal gRPC) until ctx is cancelled.
func RunAPI(ctx context.Context, cfg *config.Config, log *zap.Logger) error {
	if cfg == nil || log == nil {
		return fmt.Errorf("bootstrap: nil dependency")
	}
	if cfg.NATS.Required && strings.TrimSpace(cfg.NATS.URL) == "" {
		return fmt.Errorf("bootstrap: NATS_REQUIRED=true requires non-empty %s", platformnats.EnvNATSURL)
	}

	shutdownTracer, err := observability.InitTracer(ctx, cfg)
	if err != nil {
		return fmt.Errorf("bootstrap: init tracer: %w", err)
	}
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), cfg.Ops.TracerShutdownTimeout)
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

	mqttLayout := platformmqtt.TopicLayoutLegacy
	if strings.EqualFold(strings.TrimSpace(cfg.MQTT.TopicLayout), "enterprise") {
		mqttLayout = platformmqtt.TopicLayoutEnterprise
	}
	mqttBroker := platformmqtt.BrokerConfig{
		BrokerURL:          cfg.MQTT.BrokerURL,
		ClientID:           cfg.MQTT.ClientIDForProcess(cfg.ProcessName),
		Username:           cfg.MQTT.Username,
		Password:           cfg.MQTT.Password,
		TopicPrefix:        cfg.MQTT.TopicPrefix,
		TopicLayout:        mqttLayout,
		AppEnv:             string(cfg.AppEnv),
		TLSEnabled:         cfg.MQTT.TLSEnabled,
		CAFile:             cfg.MQTT.CAFile,
		CertFile:           cfg.MQTT.CertFile,
		KeyFile:            cfg.MQTT.KeyFile,
		InsecureSkipVerify: cfg.MQTT.InsecureSkipVerify,
	}
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

	if cfg.MetricsEnabled {
		dbpoolmetrics.Register(rt.Pool())
	}

	auditSvc := appaudit.NewService(rt.Pool(), appaudit.ServiceOpts{CriticalFailOpen: cfg.AuditCriticalFailOpen})
	store := postgres.NewStore(rt.Pool(), postgres.WithEnterpriseAudit(auditSvc))
	fleetRepo := postgres.NewFleetRepository(rt.Pool())
	fleetSvc := appfleet.NewService(fleetRepo)
	commerceSvc := appcommerce.NewService(appcommerce.Deps{
		OrderVend:                   store,
		PaymentOutbox:               store,
		Lifecycle:                   store,
		WebhookPersist:              store,
		SaleLines:                   store,
		WorkflowOrchestration:       rt.Deps.WorkflowOrchestration,
		ScheduleVendFailureFollowUp: cfg.Temporal.ScheduleVendFailureFollowUp,
		EnterpriseAudit:             auditSvc,
		PaymentSessionRegistry:      rt.Deps.PaymentProviders,
	})
	var artifactSvc *appartifacts.Service
	var machineMediaStore objectstore.Store
	var machineMediaPresignTTL time.Duration
	if cfg.Artifacts.Enabled {
		obj, oerr := objectstore.NewFromEnv(ctx)
		if oerr != nil {
			return fmt.Errorf("bootstrap: object store for artifacts (set S3 env or disable API_ARTIFACTS_ENABLED): %w", oerr)
		}
		rt.SetObjectStoreReadiness(obj.PingBucket)
		artifactSvc = appartifacts.NewService(appartifacts.Deps{
			Store:              obj,
			MaxUploadBytes:     cfg.Artifacts.MaxUploadBytes,
			DownloadPresignTTL: cfg.Artifacts.DownloadPresignTTL,
			ListMaxKeys:        cfg.Artifacts.ListMaxKeys,
		})
		machineMediaStore = obj
		machineMediaPresignTTL = cfg.Artifacts.DownloadPresignTTL
	}
	var catalogMediaBump appmediaadmin.CatalogMediaCacheBumper
	var accessRevocation revocation.Store
	var sessionCache auth.RefreshSessionCache
	var loginFailures auth.LoginFailureCounter
	if rdb := rt.Redis(); rdb != nil {
		catalogMediaBump = appmediaadmin.NewRedisCatalogMediaBumper(rdb)
		if cfg.RedisRuntime.AuthAccessJTIRevocationEnabled {
			rev, rerr := revocation.NewRedisStore(rdb)
			if rerr != nil {
				return fmt.Errorf("bootstrap: auth revocation redis: %w", rerr)
			}
			accessRevocation = rev
		}
		if cfg.RedisRuntime.SessionCacheEnabled {
			sessionCache = platformredis.NewRefreshSessionCache(rdb, cfg.Redis.KeyPrefix)
		}
		if cfg.RedisRuntime.LoginLockoutEnabled {
			loginFailures = platformredis.NewLoginFailureCounter(rdb, cfg.Redis.KeyPrefix)
		}
	}
	saleCatalogInner := appsalecatalog.NewService(rt.Pool())
	var saleCatalog appsalecatalog.SnapshotBuilder = saleCatalogInner
	if cfg.RedisRuntime.CacheEnabled && rt.Redis() != nil && cfg.RedisRuntime.SaleCatalogCacheTTL > 0 {
		saleCatalog = &appsalecatalog.RedisCachedSnapshotBuilder{
			Inner: saleCatalogInner,
			Pool:  rt.Pool(),
			RDB:   rt.Redis(),
			TTL:   cfg.RedisRuntime.SaleCatalogCacheTTL,
		}
	}
	httpApp := api.NewHTTPApplication(api.HTTPApplicationDeps{
		Store:                   store,
		Fleet:                   fleetSvc,
		Commerce:                commerceSvc,
		MQTTTopicPrefix:         cfg.MQTT.TopicPrefix,
		MQTTTopicLayout:         cfg.MQTT.TopicLayout,
		MQTTCommandPublish:      rt.Deps.MQTTPublisher,
		Artifacts:               artifactSvc,
		HTTPAuth:                cfg.HTTPAuth,
		AdminAuthSecurity:       cfg.AdminAuthSecurity,
		AppEnv:                  cfg.AppEnv,
		MachineJWT:              cfg.MachineJWT,
		CatalogMediaCacheBumper: catalogMediaBump,
		AccessRevocation:        accessRevocation,
		SessionCache:            sessionCache,
		LoginFailures:           loginFailures,
		EnterpriseAudit:         auditSvc,
		CashSettlementVarianceReviewThresholdMinor: cfg.CashSettlement.VarianceReviewThresholdMinor,
		AuditCriticalFailOpen:                      cfg.AuditCriticalFailOpen,
		ReportingSyncMaxSpan:                       cfg.Capacity.EffectiveReportingSyncMaxSpan(),
		ReportingExportMaxSpan:                     cfg.Capacity.EffectiveReportingExportMaxSpan(),
		ProductMediaThumbSize:                      cfg.Artifacts.ThumbSize,
		ProductMediaDisplaySize:                    cfg.Artifacts.DisplaySize,
	})
	if rt.Deps.PaymentProviders != nil {
		httpApp.ListPaymentProviders = func() []api.PaymentProviderRegistryInfo {
			rows := rt.Deps.PaymentProviders.ProviderSummaries()
			out := make([]api.PaymentProviderRegistryInfo, len(rows))
			for i, row := range rows {
				out[i] = api.PaymentProviderRegistryInfo{
					Key:              row.Key,
					QuerySupported:   row.QuerySupported,
					WebhookProfile:   row.WebhookProfile,
					ConfigSource:     row.ConfigSource,
					DefaultForEnv:    row.DefaultForEnv,
					ActiveSessionKey: row.ActiveSessionKey,
				}
			}
			return out
		}
	}

	if err := httpserver.ValidateP0HTTPApplication(cfg, httpApp); err != nil {
		return fmt.Errorf("bootstrap: P0 HTTP wiring: %w", err)
	}

	httpSrv, err := httpserver.NewHTTPServer(cfg, log, rt, httpApp, rt.Redis(), accessRevocation)
	if err != nil {
		return fmt.Errorf("bootstrap: http server: %w", err)
	}

	machineQueries := api.NewInternalMachineQueryService(store, httpApp.MachineShadow)
	telemetryQueries := api.NewInternalTelemetryQueryService(store)
	commerceQueries := api.NewInternalCommerceQueryService(commerceSvc)

	internalGRPCSrv, err := grpcserver.NewInternalGRPCServer(cfg, log, rt,
		grpcserver.RegisterInternalQueryServices(grpcserver.InternalQueryServices{
			Machine:   machineQueries,
			Telemetry: telemetryQueries,
			Commerce:  commerceQueries,
			Payment:   api.NewInternalPaymentQueryService(store),
			Catalog:   saleCatalog,
			Inventory: api.NewInternalInventoryQueryService(machineQueries),
			Reporting: httpApp.Reporting,
		}),
	)
	if err != nil {
		return fmt.Errorf("bootstrap: internal grpc server: %w", err)
	}

	var machineCertChecker auth.MachineGRPCClientCertChecker
	if cfg.GRPC.Enabled && rt.Pool() != nil {
		machineCertChecker = postgres.NewMachineGRPCClientCertAuth(db.New(rt.Pool()), cfg.GRPC.TLS.MachineIDFromCertURIPrefix)
	}
	machineTokenChecker := grpcserver.NewSQLMachineTokenCredentialChecker(rt.Pool(), auditSvc)
	replayLedger := grpcserver.NewMachineReplayLedger(rt.Pool(), auditSvc)

	grpcSrv, err := grpcserver.NewServer(cfg, log, rt.Redis(), accessRevocation, rt, replayLedger, machineTokenChecker, machineCertChecker,
		grpcserver.RegisterMachineGRPCServices(grpcserver.MachineGRPCServicesDeps{
			Activation:      httpApp.Activation,
			MachineQueries:  machineQueries,
			FeatureFlags:    httpApp.FeatureFlags,
			SaleCatalog:     saleCatalog,
			Pool:            rt.Pool(),
			MQTTBrokerURL:   cfg.MQTT.BrokerURL,
			MQTTTopicPrefix: cfg.MQTT.TopicPrefix,
			Config:          cfg,
			InventoryLedger: postgres.NewInventoryRepository(rt.Pool()),
			EnterpriseAudit: httpApp.EnterpriseAudit,
			Operator:        httpApp.MachineOperator,
			Commerce:        httpApp.Commerce,
			TelemetryStore:  httpApp.TelemetryStore,
			MediaStore:      machineMediaStore,
			MediaPresignTTL: machineMediaPresignTTL,
		}),
	)
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

	if internalGRPCSrv != nil {
		igs := internalGRPCSrv
		g.Go(func() error {
			return igs.ListenAndServe(gctx)
		})
	}

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
