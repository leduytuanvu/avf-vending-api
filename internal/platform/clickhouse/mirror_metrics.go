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
	projectionEnqueueDropped = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "analytics_projection",
		Name:      "enqueue_dropped_total",
		Help:      "Typed analytics projection events dropped because max concurrent inserts was reached (cold path only).",
	})
	projectionInsertOK = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "analytics_projection",
		Name:      "insert_ok_total",
		Help:      "Successful ClickHouse inserts for typed analytics projection rows.",
	})
	projectionInsertFailed = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "analytics_projection",
		Name:      "insert_failed_total",
		Help:      "ClickHouse typed analytics projection inserts that exhausted retries (cold path only).",
	})
	projectionMarshalFailed = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "analytics_projection",
		Name:      "marshal_failed_total",
		Help:      "Failed to marshal typed analytics projection row before insert.",
	})
	projectionSkipped = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "avf",
		Subsystem: "analytics_projection",
		Name:      "skipped_total",
		Help:      "Published outbox events skipped by typed analytics projection by reason.",
	}, []string{"reason"})
	projectionLagSeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "avf",
		Subsystem: "analytics_projection",
		Name:      "lag_seconds",
		Help:      "Age from source outbox creation to successful typed analytics projection insert.",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 12),
	})
)
