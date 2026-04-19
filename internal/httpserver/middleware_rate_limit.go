package httpserver

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	appmw "github.com/avf/avf-vending-api/internal/middleware"
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
				reqID := appmw.RequestIDFromContext(r.Context())
				corr := appmw.CorrelationIDFromContext(r.Context())
				log.Warn("rate limit exceeded",
					zap.String("client_ip", ip),
					zap.String("path", r.URL.Path),
					zap.String("method", r.Method),
					zap.String("request_id", reqID),
					zap.String("correlation_id", corr),
				)
				writeAPIError(w, r.Context(), http.StatusTooManyRequests, "rate_limited", "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}
