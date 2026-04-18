package auth

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

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

func writeAuthJSON(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"message": message},
	})
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
				writeAuthJSON(w, http.StatusUnauthorized, ErrUnauthenticated.Error())
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
					writeAuthJSON(w, http.StatusServiceUnavailable, err.Error())
					return
				}
				logBearerAuthReject(log, r, err)
				writeAuthJSON(w, http.StatusUnauthorized, ErrUnauthenticated.Error())
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
				writeAuthJSON(w, http.StatusUnauthorized, ErrUnauthenticated.Error())
				return
			}
			if !p.HasAnyRole(roles...) {
				writeAuthJSON(w, http.StatusForbidden, ErrForbidden.Error())
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
			writeAuthJSON(w, http.StatusUnauthorized, ErrUnauthenticated.Error())
			return
		}
		if p.HasRole(RolePlatformAdmin) {
			next.ServeHTTP(w, r)
			return
		}
		if !p.HasOrganization() {
			writeAuthJSON(w, http.StatusForbidden, "organization scope required")
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
				writeAuthJSON(w, http.StatusUnauthorized, ErrUnauthenticated.Error())
				return
			}
			raw := chi.URLParam(r, machineParam)
			machineID, err := uuid.Parse(strings.TrimSpace(raw))
			if err != nil {
				writeAuthJSON(w, http.StatusBadRequest, "invalid machine id")
				return
			}
			if !p.CanAccessMachineRead(machineID) {
				writeAuthJSON(w, http.StatusForbidden, ErrForbidden.Error())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
