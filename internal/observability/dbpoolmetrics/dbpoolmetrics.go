package dbpoolmetrics

import (
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var register sync.Once

// Register exposes pgxpool stats as Prometheus gauges (safe to call once per process).
func Register(pool *pgxpool.Pool) {
	if pool == nil {
		return
	}
	register.Do(func() {
		promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "avf",
			Subsystem: "db_pool",
			Name:      "acquired_conns",
			Help:      "Postgres pool connections currently acquired (in use).",
		}, func() float64 { return float64(pool.Stat().AcquiredConns()) })
		promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "avf",
			Subsystem: "db_pool",
			Name:      "idle_conns",
			Help:      "Postgres pool idle connections.",
		}, func() float64 { return float64(pool.Stat().IdleConns()) })
		promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "avf",
			Subsystem: "db_pool",
			Name:      "total_conns",
			Help:      "Postgres pool total connections (acquired + idle).",
		}, func() float64 { return float64(pool.Stat().TotalConns()) })
		promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "avf",
			Subsystem: "db_pool",
			Name:      "max_conns",
			Help:      "Postgres pool max size from configuration.",
		}, func() float64 { return float64(pool.Stat().MaxConns()) })
		promauto.NewGaugeFunc(prometheus.GaugeOpts{
			Namespace: "avf",
			Subsystem: "db_pool",
			Name:      "constructing_conns",
			Help:      "Postgres pool connections being established.",
		}, func() float64 { return float64(pool.Stat().ConstructingConns()) })
		promauto.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "avf",
			Subsystem: "db_pool",
			Name:      "acquire_count_total",
			Help:      "Total successful connection acquires from the pool.",
		}, func() float64 { return float64(pool.Stat().AcquireCount()) })
		promauto.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "avf",
			Subsystem: "db_pool",
			Name:      "acquire_duration_seconds_total",
			Help:      "Total time spent acquiring connections from the pool (seconds).",
		}, func() float64 { return pool.Stat().AcquireDuration().Seconds() })
		promauto.NewCounterFunc(prometheus.CounterOpts{
			Namespace: "avf",
			Subsystem: "db_pool",
			Name:      "canceled_acquire_count_total",
			Help:      "Total acquires canceled while waiting for a connection.",
		}, func() float64 { return float64(pool.Stat().CanceledAcquireCount()) })
	})
}
