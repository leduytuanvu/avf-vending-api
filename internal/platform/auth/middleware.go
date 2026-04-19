package auth

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/avf/avf-vending-api/internal/apierr"
	appmw "github.com/avf/avf-vending-api/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// EnvJWTSecret names the HS256 signing secret for bearer access tokens.
	EnvJWTSecret = "HTTP_AUTH_JWT_SECRET"
	// DefaultClockLeeway tolerates minor clock skew between issuer and this API.
	DefaultClockLeeway = 45 * time.Second
)

func writeAuthError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	rid := appmw.RequestIDFromContext(r.Context())
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apierr.V1(rid, code, message, nil))
}

func logBearerAuthReject(log *zap.Logger, r *http.Request, err error) {
	if log == nil {
		return
	}
	log.Warn("bearer auth rejected",
		zap.Error(err),
		zap.String("request_id", appmw.RequestIDFromContext(r.Context())),
		zap.String("correlation_id", appmw.CorrelationIDFromContext(r.Context())),
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
	)
}

// BearerAccessTokenMiddlewareWithValidator validates Authorization: Bearer <JWT> using the supplied validator.
func BearerAccessTokenMiddlewareWithValidator(v AccessTokenValidator, log *zap.Logger) func(http.Handler) http.Handler {
	if v == nil {
		panic("auth.BearerAccessTokenMiddlewareWithValidator: nil validator")
	}
	if log == nil {
		log = zap.NewNop()
	}
	var misconfigLog sync.Once
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			raw := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
			if raw == "" {
				logBearerAuthReject(log, r, ErrUnauthenticated)
				writeAuthError(w, r, http.StatusUnauthorized, "unauthenticated", ErrUnauthenticated.Error())
				return
			}
			p, err := v.ValidateAccessToken(r.Context(), raw)
			if err != nil {
				if err == ErrMisconfigured {
					misconfigLog.Do(func() {
						log.Error("bearer auth misconfigured (check HTTP_AUTH_* settings)",
							zap.Error(err),
							zap.String("request_id", appmw.RequestIDFromContext(r.Context())),
							zap.String("correlation_id", appmw.CorrelationIDFromContext(r.Context())),
						)
					})
					writeAuthError(w, r, http.StatusServiceUnavailable, "auth_misconfigured", err.Error())
					return
				}
				logBearerAuthReject(log, r, err)
				writeAuthError(w, r, http.StatusUnauthorized, "unauthenticated", ErrUnauthenticated.Error())
				return
			}
			next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
		})
	}
}

// BearerAccessTokenMiddleware validates HS256 JWTs using a shared secret (dev-friendly).
// Prefer config-driven BearerAccessTokenMiddlewareWithValidator(NewAccessTokenValidator(cfg)) for production.
func BearerAccessTokenMiddleware(secret []byte, leeway time.Duration, log *zap.Logger) func(http.Handler) http.Handler {
	if leeway <= 0 {
		leeway = DefaultClockLeeway
	}
	v := newHS256Validator(secret, nil, leeway)
	return BearerAccessTokenMiddlewareWithValidator(v, log)
}

// JWTSecretFromEnv returns the configured bearer JWT secret (may be empty).
func JWTSecretFromEnv() []byte {
	return []byte(strings.TrimSpace(os.Getenv(EnvJWTSecret)))
}

// RequireAnyRole returns middleware that enforces at least one role on the principal.
func RequireAnyRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := PrincipalFromContext(r.Context())
			if !ok {
				writeAuthError(w, r, http.StatusUnauthorized, "unauthenticated", ErrUnauthenticated.Error())
				return
			}
			if !p.HasAnyRole(roles...) {
				writeAuthError(w, r, http.StatusForbidden, "forbidden", ErrForbidden.Error())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireOrganizationScope requires a non-nil organization on the principal (tenant-scoped APIs).
// platform_admin may omit org_id; handlers and stores must still apply tenant filters when persisting.
func RequireOrganizationScope(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := PrincipalFromContext(r.Context())
		if !ok {
			writeAuthError(w, r, http.StatusUnauthorized, "unauthenticated", ErrUnauthenticated.Error())
			return
		}
		if p.HasRole(RolePlatformAdmin) {
			next.ServeHTTP(w, r)
			return
		}
		if !p.HasOrganization() {
			writeAuthError(w, r, http.StatusForbidden, "organization_scope_required", "organization scope required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireMachineURLAccess enforces machine-level read using URL param (chi) and principal claims.
func RequireMachineURLAccess(machineParam string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := PrincipalFromContext(r.Context())
			if !ok {
				writeAuthError(w, r, http.StatusUnauthorized, "unauthenticated", ErrUnauthenticated.Error())
				return
			}
			raw := chi.URLParam(r, machineParam)
			machineID, err := uuid.Parse(strings.TrimSpace(raw))
			if err != nil {
				writeAuthError(w, r, http.StatusBadRequest, "invalid_machine_id", "invalid machine id")
				return
			}
			if !p.CanAccessMachineRead(machineID) {
				writeAuthError(w, r, http.StatusForbidden, "forbidden", ErrForbidden.Error())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
