package httpserver

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/platform/ratelimit"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

const planogramWriteRateBurst = 6

// MachinePlanogramWriteRateLimitIfEnabled limits PUT merge-pairs and POST planograms/publish per machineId.
func MachinePlanogramWriteRateLimitIfEnabled(cfg config.HTTPRateLimitConfig, log *zap.Logger) func(http.Handler) http.Handler {
	if !cfg.SensitiveWritesEnabled {
		return func(next http.Handler) http.Handler { return next }
	}
	rps := cfg.SensitiveWritesRPS
	if rps <= 0 {
		rps = 2
	}
	lim := rate.Limit(rps)
	burst := planogramWriteRateBurst
	if cfg.SensitiveWritesBurst > 0 && cfg.SensitiveWritesBurst < burst {
		burst = cfg.SensitiveWritesBurst
	}
	if log == nil {
		log = zap.NewNop()
	}
	var mu sync.Mutex
	visitors := make(map[string]*visitorLimiter)
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
			mu.Unlock()
		}
	}()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			machineID := strings.TrimSpace(chi.URLParam(r, "machineId"))
			if machineID == "" || uuid.Validate(machineID) != nil {
				next.ServeHTTP(w, r)
				return
			}
			key := ratelimit.StableKey("planogram_write", machineID, clientIP(r))
			mu.Lock()
			v, ok := visitors[key]
			if !ok {
				v = &visitorLimiter{limiter: rate.NewLimiter(lim, burst)}
				visitors[key] = v
			}
			v.lastSeen = time.Now()
			allow := v.limiter.Allow()
			retryAfterSec := 1
			if !allow {
				retryAfterSec = maxInt(1, int(float64(time.Second)/float64(lim)))
			}
			mu.Unlock()
			if !allow {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfterSec))
				logRateLimitExceeded(log, r, clientIP(r))
				writeAPIError(w, r.Context(), http.StatusTooManyRequests, "rate_limited", "too many planogram writes for this machine")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
