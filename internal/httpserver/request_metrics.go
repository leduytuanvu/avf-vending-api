package httpserver

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Namespace: "avf",
	Subsystem: "http",
	Name:      "request_duration_seconds",
	Help:      "HTTP request duration for this process.",
	Buckets:   prometheus.ExponentialBuckets(0.001, 2, 16),
}, []string{"method", "route", "status"})

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
			httpRequestDuration.WithLabelValues(
				r.Method,
				routeLabel(r),
				strconv.Itoa(ww.Status()),
			).Observe(time.Since(start).Seconds())
		})
	}
}
