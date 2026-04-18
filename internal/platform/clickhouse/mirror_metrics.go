package clickhouse

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	mirrorEnqueueDropped = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "analytics_mirror",
		Name:      "enqueue_dropped_total",
		Help:      "Outbox mirror events dropped because max concurrent inserts was reached (cold path only).",
	})
	mirrorInsertOK = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "analytics_mirror",
		Name:      "insert_ok_total",
		Help:      "Successful ClickHouse inserts for outbox mirror rows.",
	})
	mirrorInsertFailed = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "analytics_mirror",
		Name:      "insert_failed_total",
		Help:      "ClickHouse inserts that exhausted retries (cold path only).",
	})
	mirrorMarshalFailed = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "analytics_mirror",
		Name:      "marshal_failed_total",
		Help:      "Failed to marshal outbox mirror row before insert.",
	})
)
