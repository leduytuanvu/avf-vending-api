package httpserver

import (
	"net/http"
	"strconv"
	"time"

	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var redisRateLimitHitsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "avf",
	Subsystem: "redis",
	Name:      "rate_limit_hits_total",
	Help:      "Redis-backed HTTP rate limit denials by route class.",
}, []string{"route_class"})

func recordRedisRateLimitHit(routeClass string) {
	if routeClass == "" {
		routeClass = "unknown"
	}
	redisRateLimitHitsTotal.WithLabelValues(routeClass).Inc()
}

func routeLabel(r *http.Request) string {
	rc := chi.RouteContext(r.Context())
	if rc == nil {
		return "unknown"
	}
	p := rc.RoutePattern()
	if p == "" {
		return "unmatched"
	}
	return p
}

func requestMetricsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			start := time.Now()
			next.ServeHTTP(ww, r)
			status := strconv.Itoa(ww.Status())
			rl := routeLabel(r)
			productionmetrics.RecordHTTPRequest(r.Method, rl, status, time.Since(start))
		})
	}
}
