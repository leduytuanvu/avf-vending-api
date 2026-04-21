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
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// HTTPServer hosts the public HTTP API (health, optional metrics, /v1 routes — see mountV1).
type HTTPServer struct {
	cfg *config.Config
	log *zap.Logger
	srv *http.Server
}

// NewHTTPServer constructs an HTTP server with standard middleware and routing.
func NewHTTPServer(cfg *config.Config, log *zap.Logger, probe ReadinessProbe, httpApp *api.HTTPApplication) (*HTTPServer, error) {
	if cfg == nil || log == nil {
		return nil, fmt.Errorf("httpserver: nil dependency")
	}
	if probe == nil {
		return nil, fmt.Errorf("httpserver: nil readiness probe")
	}
	if httpApp == nil {
		return nil, fmt.Errorf("httpserver: nil http application")
	}

	validator, err := auth.NewAccessTokenValidator(cfg.HTTPAuth)
	if err != nil {
		return nil, fmt.Errorf("httpserver: auth validator: %w", err)
	}
	if sec := auth.TrimSecret(cfg.HTTPAuth.LoginJWTSecret); len(sec) > 0 {
		secondary := auth.NewHS256AccessTokenValidator(sec, cfg.HTTPAuth.JWTLeeway)
		validator = auth.ChainAccessTokenValidators(validator, secondary)
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.HTTPAuth.Mode))
	if mode == "rs256_jwks" && !cfg.HTTPAuth.JWKSSkipStartupWarm {
		if err := auth.MaybeWarmJWKS(context.Background(), validator); err != nil {
			return nil, fmt.Errorf("httpserver: JWKS warmup failed: %w", err)
		}
	}

	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(appmw.RequestID)
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
		if err := probe.Ready(r.Context()); err != nil {
			log.Warn("readiness failed", zap.Error(err))
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	if cfg.MetricsEnabled {
		r.Method(http.MethodGet, "/metrics", promhttp.Handler())
	}

	if cfg.SwaggerUIEnabled {
		MountSwaggerUI(r, log)
	}

	writeRL := SensitiveWriteRateLimitIfEnabled(cfg.HTTPRateLimit, log)
	mountV1(r, httpApp, log, cfg, validator, writeRL)

	s := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           r,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	return &HTTPServer{cfg: cfg, log: log, srv: s}, nil
}

func mountV1(r chi.Router, app *api.HTTPApplication, log *zap.Logger, cfg *config.Config, validator auth.AccessTokenValidator, writeRL func(http.Handler) http.Handler) {
	v1Auth := auth.BearerAccessTokenMiddlewareWithValidator(validator, log)

	r.Route("/v1", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			mountAuthRoutes(r, app)
			r.Group(func(r chi.Router) {
				r.Use(v1Auth)
				mountAuthBearerRoutes(r, app)
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(v1Auth)

			r.Route("/admin", func(r chi.Router) {
				r.Use(auth.RequireAnyRole(auth.RolePlatformAdmin, auth.RoleOrgAdmin))
				mountAdminCatalogRoutes(r, app)
				mountAdminInventoryRoutes(r, app, writeRL)
				r.Get("/machines", func(w http.ResponseWriter, r *http.Request) {
					scope, err := parseAdminFleetListScope(r)
					if err != nil {
						writeV1ListError(w, r.Context(), err)
						return
					}
					out, err := app.AdminMachines.ListMachines(r.Context(), scope)
					writeV1Collection(w, r.Context(), out, err)
				})
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
				r.Get("/ota", func(w http.ResponseWriter, r *http.Request) {
					scope, err := parseAdminFleetListScope(r)
					if err != nil {
						writeV1ListError(w, r.Context(), err)
						return
					}
					out, err := app.AdminOTA.ListOTA(r.Context(), scope)
					writeV1Collection(w, r.Context(), out, err)
				})
				mountArtifactAdminRoutes(r, app, writeRL)
			})

			r.Route("/reports", func(r chi.Router) {
				r.Use(auth.RequireAnyRole(auth.RolePlatformAdmin, auth.RoleOrgAdmin))
				mountReportingRoutes(r, app)
			})

			// Operator cross-machine read APIs for support (wider role gate than /admin).
			r.Route("/operator-insights", func(r chi.Router) {
				r.Use(auth.RequireAnyRole(auth.RolePlatformAdmin, auth.RoleOrgAdmin, auth.RoleOrgMember))
				mountOperatorAdminInsightRoutes(r, app)
			})

			r.Group(func(r chi.Router) {
				r.Use(auth.RequireOrganizationScope)
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

			mountSetupBootstrapRoutes(r, app)
			r.With(auth.RequireMachineURLAccess("machineId")).Get("/machines/{machineId}/shadow", machineShadowGet(app.MachineShadow))
			mountMachineTelemetryRoutes(r, app)

			// Sensitive writes: commerce, operator sessions, remote command dispatch (token bucket per IP when enabled).
			r.Group(func(r chi.Router) {
				r.Use(writeRL)
				mountDeviceCommandRoutes(r, app)
				mountDeviceBridgeRoutes(r, app)
				mountOperatorSessionRoutes(r, app)
				mountCommerceRoutes(r, app, cfg)
			})
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
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("http listening", zap.String("addr", s.srv.Addr))
		err := s.srv.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			errCh <- nil
			return
		}
		errCh <- err
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.HTTP.ShutdownTimeout)
		defer cancel()

		if err := s.srv.Shutdown(shutdownCtx); err != nil {
			s.log.Error("http shutdown error", zap.Error(err))
		}

		err := <-errCh
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return nil

	case err := <-errCh:
		return err
	}
}
