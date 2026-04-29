package httpserver

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	appmw "github.com/avf/avf-vending-api/internal/middleware"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/ratelimit"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

const maxRateLimitVisitors = 50_000

type visitorLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// SensitiveWriteRateLimitIfEnabled returns middleware that applies a token bucket per client IP
// for mutating methods (POST, PUT, PATCH, DELETE). Safe methods pass through unchanged.
// When disabled in cfg, returns a no-op middleware.
func SensitiveWriteRateLimitIfEnabled(cfg config.HTTPRateLimitConfig, log *zap.Logger) func(http.Handler) http.Handler {
	return SensitiveWriteRateLimitWithBackendIfEnabled(cfg, nil, log)
}

func SensitiveWriteRateLimitWithBackendIfEnabled(cfg config.HTTPRateLimitConfig, backend ratelimit.Backend, log *zap.Logger) func(http.Handler) http.Handler {
	if !cfg.SensitiveWritesEnabled {
		return func(next http.Handler) http.Handler { return next }
	}
	if cfg.SensitiveWritesRPS <= 0 || cfg.SensitiveWritesBurst <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	if log == nil {
		log = zap.NewNop()
	}
	lim := rate.Limit(cfg.SensitiveWritesRPS)
	burst := cfg.SensitiveWritesBurst
	window := time.Second
	if cfg.SensitiveWritesRPS > 0 {
		window = time.Duration(float64(time.Second) * float64(burst) / cfg.SensitiveWritesRPS)
		if window < time.Second {
			window = time.Second
		}
		if window > time.Minute {
			window = time.Minute
		}
	}

	var mu sync.Mutex
	visitors := make(map[string]*visitorLimiter)

	// Best-effort sweep to cap memory under abuse.
	go func() {
		t := time.NewTicker(2 * time.Minute)
		defer t.Stop()
		for range t.C {
			mu.Lock()
			now := time.Now()
			for k, v := range visitors {
				if now.Sub(v.lastSeen) > 10*time.Minute {
					delete(visitors, k)
				}
			}
			if len(visitors) > maxRateLimitVisitors {
				visitors = make(map[string]*visitorLimiter)
				log.Warn("rate_limit visitor map reset after max capacity")
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}

			ip := clientIP(r)
			if backend != nil {
				key := sensitiveWriteRateLimitKey(r, ip)
				allow, retryAfter := backend.Allow(r.Context(), key, int64(burst), window)
				if !allow {
					recordRedisRateLimitHit(sensitiveWriteRateLimitClass(r))
					w.Header().Set("Retry-After", strconv.Itoa(maxInt(1, int(retryAfter.Seconds()))))
					logRateLimitExceeded(log, r, ip)
					writeAPIError(w, r.Context(), http.StatusTooManyRequests, "rate_limited", "too many requests")
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			mu.Lock()
			v, ok := visitors[ip]
			if !ok {
				v = &visitorLimiter{limiter: rate.NewLimiter(lim, burst)}
				visitors[ip] = v
			}
			v.lastSeen = time.Now()
			allow := v.limiter.Allow()
			mu.Unlock()

			if !allow {
				logRateLimitExceeded(log, r, ip)
				writeAPIError(w, r.Context(), http.StatusTooManyRequests, "rate_limited", "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func sensitiveWriteRateLimitKey(r *http.Request, ip string) string {
	routeClass := sensitiveWriteRateLimitClass(r)
	if p, ok := auth.PrincipalFromContext(r.Context()); ok {
		return ratelimit.StableKey(routeClass, p.Subject, p.OrganizationID.String(), ip)
	}
	return ratelimit.StableKey(routeClass, ip)
}

func sensitiveWriteRateLimitClass(r *http.Request) string {
	routeClass := "write"
	path := strings.ToLower(strings.TrimSpace(r.URL.Path))
	switch {
	case strings.Contains(path, "/auth/login"):
		routeClass = "auth_login"
	case strings.Contains(path, "/password/reset"):
		routeClass = "password_reset"
	case strings.Contains(path, "/activation"):
		routeClass = "activation"
	case strings.Contains(path, "/webhooks"):
		routeClass = "webhook"
	case strings.Contains(path, "/admin/"):
		routeClass = "admin"
	}
	return routeClass
}

func logRateLimitExceeded(log *zap.Logger, r *http.Request, ip string) {
	reqID := appmw.RequestIDFromContext(r.Context())
	corr := appmw.CorrelationIDFromContext(r.Context())
	log.Warn("rate limit exceeded",
		zap.String("client_ip", ip),
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
		zap.String("request_id", reqID),
		zap.String("correlation_id", corr),
	)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}
