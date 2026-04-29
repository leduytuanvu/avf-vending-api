package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/config"
	appmw "github.com/avf/avf-vending-api/internal/middleware"
	"github.com/avf/avf-vending-api/internal/observability"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/auth/revocation"
	"github.com/avf/avf-vending-api/internal/platform/ratelimit"
	platformredis "github.com/avf/avf-vending-api/internal/platform/redis"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// HTTPServer hosts the public HTTP API (health, optional metrics, /v1 routes — see mountV1).
type HTTPServer struct {
	cfg *config.Config
	log *zap.Logger
	srv *http.Server
	ops *http.Server

	redisCleanup func()
}

// NewHTTPServer constructs an HTTP server with standard middleware and routing.
// sharedRedis is optional: when set, abuse rate limiting reuses this client (no second pool).
// accessRevocation is optional; when AUTH_ACCESS_JTI_REVOCATION_ENABLED is true, it wraps the access validator (same store as app auth logout / admin deactivate).
func NewHTTPServer(cfg *config.Config, log *zap.Logger, probe ReadinessProbe, httpApp *api.HTTPApplication, sharedRedis *goredis.Client, accessRevocation revocation.Store) (*HTTPServer, error) {
	if cfg == nil || log == nil {
		return nil, fmt.Errorf("httpserver: nil dependency")
	}
	if probe == nil {
		return nil, fmt.Errorf("httpserver: nil readiness probe")
	}
	if httpApp == nil {
		return nil, fmt.Errorf("httpserver: nil http application")
	}

	if cfg.MetricsEnabled && cfg.AppEnv == config.AppEnvProduction && !cfg.MetricsExposeOnPublicHTTP {
		if strings.TrimSpace(cfg.Ops.HTTPAddr) == "" {
			return nil, fmt.Errorf("httpserver: APP_ENV=production with METRICS_ENABLED=true requires HTTP_OPS_ADDR for private /metrics scraping, or set METRICS_EXPOSE_ON_PUBLIC_HTTP=true with METRICS_SCRAPE_TOKEN")
		}
	}

	validator, err := auth.NewAccessTokenValidator(cfg.HTTPAuth)
	if err != nil {
		return nil, fmt.Errorf("httpserver: auth validator: %w", err)
	}
	if sec := auth.TrimSecret(cfg.HTTPAuth.LoginJWTSecret); len(sec) > 0 {
		secondary := auth.NewHS256AccessTokenValidator(sec, cfg.HTTPAuth.JWTLeeway)
		validator = auth.ChainAccessTokenValidators(validator, secondary)
	}
	if cfg.RedisRuntime.AuthAccessJTIRevocationEnabled && accessRevocation != nil {
		validator = auth.WrapWithRevocation(validator, accessRevocation, cfg.RedisRuntime.AuthRevocationRedisFailOpen)
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.HTTPAuth.Mode))
	if mode == "rs256_jwks" && !cfg.HTTPAuth.JWKSSkipStartupWarm {
		if err := auth.MaybeWarmJWKS(context.Background(), validator); err != nil {
			return nil, fmt.Errorf("httpserver: JWKS warmup failed: %w", err)
		}
	}

	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	if len(cfg.HTTP.CORSAllowedOrigins) > 0 {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   cfg.HTTP.CORSAllowedOrigins,
			AllowedMethods:   []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID", "X-Correlation-ID", "X-Correlation-Id", "Idempotency-Key", "X-Idempotency-Key", "X-AVF-Webhook-Timestamp", "X-AVF-Webhook-Signature"},
			ExposedHeaders:   []string{"X-Request-ID", "X-Correlation-ID"},
			AllowCredentials: cfg.HTTP.CORSAllowCredentials,
			MaxAge:           300,
		}))
	}
	r.Use(chimw.Recoverer)
	r.Use(appmw.RequestID)
	r.Use(traceMiddleware())
	r.Use(requestObservabilityMiddleware(log))
	r.Use(requestLoggingMiddleware(log))
	r.Use(chimw.Timeout(60 * time.Second))
	if cfg.MetricsEnabled {
		r.Use(requestMetricsMiddleware())
	}

	r.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		ctx, cancel := context.WithTimeout(r.Context(), cfg.Ops.ReadinessTimeout)
		defer cancel()
		if err := probe.Ready(ctx); err != nil {
			observability.EnrichLogger(log, r.Context()).Warn("readiness failed", zap.Error(err))
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Get("/version", func(w http.ResponseWriter, _ *http.Request) {
		observability.WriteVersionJSON(w, cfg)
	})

	if cfg.MetricsEnabled && (cfg.AppEnv != config.AppEnvProduction || cfg.MetricsExposeOnPublicHTTP) {
		h := promhttp.Handler()
		if tok := strings.TrimSpace(cfg.MetricsScrapeToken); tok != "" {
			h = metricsBearerGate(tok, h)
		}
		r.Method(http.MethodGet, "/metrics", h)
	}

	if cfg.AppEnv == config.AppEnvProduction && cfg.TransportBoundary.MachineRESTLegacyEnabled {
		log.Warn("transport boundary: legacy machine HTTP runtime is ENABLED in production (ENABLE_LEGACY_MACHINE_HTTP / MACHINE_REST_LEGACY_ENABLED). Production vending apps must use avf.machine.v1 gRPC; REST is compatibility-only.",
			zap.Bool("machine_rest_legacy_allow_in_production", cfg.TransportBoundary.MachineRESTLegacyAllowInProduction))
	}

	if cfg.SwaggerUIEnabled {
		MountSwaggerUI(r, log)
	} else if cfg.OpenAPIJSONEnabled {
		MountOpenAPIJSON(r, log)
	}

	var redisCleanup func()
	abuseCfg := cfg.HTTPRateLimit.Abuse
	var rlBackend ratelimit.Backend
	if abuseCfg.Enabled || (cfg.HTTPRateLimit.SensitiveWritesEnabled && cfg.RedisRuntime.RateLimitEnabled) {
		rc := sharedRedis
		owns := false
		if rc == nil && cfg.RedisRuntime.RateLimitEnabled {
			var err error
			rc, err = platformredis.NewClient(&cfg.Redis)
			if err != nil {
				return nil, fmt.Errorf("httpserver: redis client: %w", err)
			}
			owns = rc != nil
		}
		if rc != nil {
			rb, err := ratelimit.NewRedisBackend(rc)
			if err != nil {
				if owns {
					_ = rc.Close()
				}
				return nil, fmt.Errorf("httpserver: redis abuse limiter: %w", err)
			}
			rlBackend = rb
			if owns {
				redisCleanup = func() { _ = rc.Close() }
			}
			log.Info("HTTP rate limiting using Redis", zap.String("redis_addr", cfg.Redis.Addr), zap.Bool("shared_client", sharedRedis != nil))
		} else {
			rlBackend = ratelimit.NewMemoryBackend()
			if cfg.AppEnv == config.AppEnvProduction || cfg.AppEnv == config.AppEnvStaging {
				return nil, fmt.Errorf("httpserver: Redis rate limit enabled but Redis is not configured")
			}
			log.Warn("Redis rate limiting unavailable; using in-memory rate limiter for local development only")
		}
	}
	writeRL := SensitiveWriteRateLimitWithBackendIfEnabled(cfg.HTTPRateLimit, rlBackend, log)
	abuse := NewAbuseProtection(abuseCfg, rlBackend, log)

	mountV1(r, httpApp, log, cfg, validator, writeRL, abuse)

	s := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           r,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	var opsServer *http.Server
	if strings.TrimSpace(cfg.Ops.HTTPAddr) != "" {
		opsServer = &http.Server{
			Addr:    cfg.Ops.HTTPAddr,
			Handler: observability.NewOperationsMux(cfg, log, cfg.MetricsEnabled, probe.Ready),
		}
	}

	return &HTTPServer{cfg: cfg, log: log, srv: s, ops: opsServer, redisCleanup: redisCleanup}, nil
}

func mountV1(r chi.Router, app *api.HTTPApplication, log *zap.Logger, cfg *config.Config, validator auth.AccessTokenValidator, writeRL func(http.Handler) http.Handler, abuse *AbuseProtection) {
	if abuse == nil {
		abuse = &AbuseProtection{}
	}
	v1Auth := auth.BearerAccessTokenMiddlewareWithValidator(validator, log)

	r.Route("/v1", func(r chi.Router) {
		mountCommercePublicWebhookPost(r, app, cfg, abuse, writeRL)
		mountPublicActivationClaim(r, app, cfg, abuse, writeRL)

		r.Route("/auth", func(r chi.Router) {
			mountAuthRoutes(r, app, abuse, writeRL)
			r.Group(func(r chi.Router) {
				r.Use(v1Auth)
				r.Use(authObservabilityMiddleware(log))
				mountAuthBearerSessionRoutes(r, app, writeRL)
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(v1Auth)
			r.Use(authObservabilityMiddleware(log))
			r.Use(auth.RequireInteractiveAccountActive)

			r.Route("/admin", func(r chi.Router) {
				r.Use(auth.RequireDenyMachinePrincipal)
				r.Use(auditTransportMetaMiddleware)
				r.Use(abuse.AdminMutation())
				mountAdminAuthUserRoutes(r, app, writeRL)
				mountAdminAuditRoutes(r, app)
				MountAdminOutboxOpsRoutes(r, app, cfg, writeRL)
				MountAdminSystemOutboxRoutes(r, app, cfg, writeRL)
				MountAdminRetentionSystemRoutes(r, app, cfg, writeRL)
				mountAdminCatalogRoutes(r, app, writeRL)
				mountAdminMediaRoutes(r, app, writeRL)
				mountAdminInventoryRoutes(r, app, writeRL)
				mountAdminFeatureConfigRoutes(r, app, writeRL)
				mountAdminCashSettlementRoutes(r, app, writeRL)
				mountAdminCommerceRoutes(r, app, writeRL)
				mountAdminPaymentOpsRoutes(r, app, writeRL)
				r.Group(func(r chi.Router) {
					r.Use(auth.RequireAnyPermission(auth.PermReportsRead))
					r.Use(abuse.ReportsReadGET())
					mountAdminReportingCSVExports(r, app)
					mountAdminOrganizationReportingRoutes(r, app)
				})
				r.Group(func(r chi.Router) {
					r.Use(auth.RequireAnyPermission(auth.PermCashWrite))
					mountAdminFinanceDailyCloseRoutes(r, app)
				})
				mountAdminFleetWriteRoutes(r, app, writeRL)
				mountAdminMachineDiagnosticsRoutes(r, app, writeRL)
				r.Group(func(r chi.Router) {
					r.Use(auth.RequireAnyPermission(auth.PermFleetRead, auth.PermSiteRead, auth.PermTechnicianRead))
					r.Get("/machines", serveAdminMachinesList(app))
					r.Get("/machines/{machineId}", serveAdminMachineGet(app))
					r.Get("/technicians", func(w http.ResponseWriter, r *http.Request) {
						scope, err := parseAdminFleetListScope(r)
						if err != nil {
							writeV1ListError(w, r.Context(), err)
							return
						}
						out, err := app.AdminTechnicians.ListTechnicians(r.Context(), scope)
						writeV1Collection(w, r.Context(), out, err)
					})
					r.Get("/assignments", func(w http.ResponseWriter, r *http.Request) {
						scope, err := parseAdminFleetListScope(r)
						if err != nil {
							writeV1ListError(w, r.Context(), err)
							return
						}
						out, err := app.AdminAssignments.ListAssignments(r.Context(), scope)
						writeV1Collection(w, r.Context(), out, err)
					})
					r.Get("/commands", func(w http.ResponseWriter, r *http.Request) {
						scope, err := parseAdminFleetListScope(r)
						if err != nil {
							writeV1ListError(w, r.Context(), err)
							return
						}
						out, err := app.AdminCommands.ListCommands(r.Context(), scope)
						writeV1Collection(w, r.Context(), out, err)
					})
				})
				r.Group(func(r chi.Router) {
					r.Use(auth.RequireAnyPermission(auth.PermOTARead))
					r.Get("/ota", func(w http.ResponseWriter, r *http.Request) {
						scope, err := parseAdminFleetListScope(r)
						if err != nil {
							writeV1ListError(w, r.Context(), err)
							return
						}
						out, err := app.AdminOTA.ListOTA(r.Context(), scope)
						writeV1Collection(w, r.Context(), out, err)
					})
				})
				mountAdminOTACampaignRoutes(r, app, writeRL)
				mountArtifactAdminRoutes(r, app, writeRL)
				mountAdminActivationRoutes(r, app, writeRL)
				mountAdminPaymentProviderRoutes(r, app)
			})

			r.Route("/reports", func(r chi.Router) {
				r.Use(auth.RequireDenyMachinePrincipal)
				r.Use(auth.RequireAnyPermission(auth.PermReportsRead))
				r.Use(abuse.ReportsReadGET())
				mountReportingRoutes(r, app)
			})

			// Operator cross-machine read APIs for support (wider role gate than /admin).
			r.Route("/operator-insights", func(r chi.Router) {
				r.Use(auth.RequireDenyMachinePrincipal)
				r.Use(auth.RequireAnyRole(auth.RolePlatformAdmin, auth.RoleOrgAdmin, auth.RoleOrgMember))
				mountOperatorAdminInsightRoutes(r, app)
			})

			r.Group(func(r chi.Router) {
				r.Use(auth.RequireOrganizationScope)
				r.Use(auth.RequireInteractivePermissionOrMachinePrincipal(auth.PermCommerceRead))
				r.Get("/payments", func(w http.ResponseWriter, r *http.Request) {
					scope, err := parseTenantCommerceListScope(r)
					if err != nil {
						writeV1ListError(w, r.Context(), err)
						return
					}
					out, err := app.Payments.ListPayments(r.Context(), scope)
					writeV1Collection(w, r.Context(), out, err)
				})
				r.Get("/orders", func(w http.ResponseWriter, r *http.Request) {
					scope, err := parseTenantCommerceListScope(r)
					if err != nil {
						writeV1ListError(w, r.Context(), err)
						return
					}
					out, err := app.Orders.ListOrders(r.Context(), scope)
					writeV1Collection(w, r.Context(), out, err)
				})
			})

			if cfg.TransportBoundary.MachineRESTLegacyEnabled {
				// Legacy machine REST runtime (deprecated): setup, catalog HTTP, shadow, telemetry reads — superseded by gRPC.
				r.Group(func(r chi.Router) {
					r.Use(machineLegacyRESTGuard(cfg))
					mountSetupBootstrapRoutes(r, app)
					mountSaleCatalogRoute(r, app)
					r.With(RequireMachineTenantAccess(app, "machineId")).Get("/machines/{machineId}/shadow", machineShadowGet(app.MachineShadow))
					mountMachineTelemetryRoutes(r, app, abuse)
				})
			}

			// Sensitive writes: fleet command dispatch over REST (admin/operator); uses MQTT + command ledger — not gated as legacy machine runtime.
			r.Group(func(r chi.Router) {
				r.Use(writeRL)
				mountDeviceCommandRoutes(r, app, abuse)
			})

			if cfg.TransportBoundary.MachineRESTLegacyEnabled {
				r.Group(func(r chi.Router) {
					r.Use(writeRL)
					r.Use(machineLegacyRESTGuard(cfg))
					mountDeviceBridgeRoutes(r, app)
					mountMachineRuntimeRoutes(r, app, abuse)
					mountOperatorSessionRoutes(r, app)
					mountCommerceRoutes(r, app, cfg)
				})
			}
		})
	})
}

func machineShadowGet(svc api.MachineShadowService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw := chi.URLParam(r, "machineId")
		id, err := uuid.Parse(raw)
		if err != nil {
			writeAPIError(w, r.Context(), http.StatusBadRequest, "invalid_machine_id", "invalid machineId")
			return
		}
		out, err := svc.GetShadow(r.Context(), id)
		if err != nil {
			writeMachineShadowLoadError(w, r.Context(), err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ListenAndServe starts serving and blocks until ctx is cancelled, then shuts down gracefully.
func (s *HTTPServer) ListenAndServe(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		s.log.Info("http listening", zap.String("addr", s.srv.Addr))
		err := s.srv.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	})

	if s.ops != nil {
		g.Go(func() error {
			s.log.Info("http ops listening", zap.String("addr", s.ops.Addr))
			err := s.ops.ListenAndServe()
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		})
	}

	g.Go(func() error {
		<-gctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.HTTP.ShutdownTimeout)
		defer cancel()
		if err := s.srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Error("http shutdown error", zap.Error(err))
		}
		if s.redisCleanup != nil {
			s.redisCleanup()
			s.redisCleanup = nil
		}
		if s.ops != nil {
			opsCtx, opsCancel := context.WithTimeout(context.Background(), s.cfg.Ops.ShutdownTimeout)
			defer opsCancel()
			if err := s.ops.Shutdown(opsCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				s.log.Error("http ops shutdown error", zap.Error(err))
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}
