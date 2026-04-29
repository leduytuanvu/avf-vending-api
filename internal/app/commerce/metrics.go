package commerce

import (
	"strings"

	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var vendFinalizedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "avf",
	Subsystem: "commerce",
	Name:      "vend_finalized_total",
	Help:      "Terminal vend outcomes recorded by the commerce state machine.",
}, []string{"state"})

func recordVendFinalized(state string) {
	state = strings.TrimSpace(strings.ToLower(state))
	switch state {
	case "success", "failed":
	default:
		state = "unknown"
	}
	vendFinalizedTotal.WithLabelValues(state).Inc()
	switch state {
	case "success":
		productionmetrics.RecordVendSuccess()
	case "failed":
		productionmetrics.RecordVendFailure()
	}
}
