package httpserver

import (
	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var commercePaymentWebhookTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "avf",
	Subsystem: "commerce",
	Name:      "payment_webhook_requests_total",
	Help:      "Payment provider webhook POST outcomes (HMAC public route).",
}, []string{"result"})

var commercePaymentWebhookClassTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Namespace: "avf",
	Subsystem: "commerce",
	Name:      "payment_webhook_outcomes_total",
	Help:      "Payment provider webhook outcomes grouped for alerting: accepted, rejected, or replayed.",
}, []string{"outcome"})

func recordCommercePaymentWebhookResult(result string) {
	r := result
	if r == "" {
		r = "unknown"
	}
	commercePaymentWebhookTotal.WithLabelValues(r).Inc()
	commercePaymentWebhookClassTotal.WithLabelValues(classifyCommercePaymentWebhookOutcome(r)).Inc()
	productionmetrics.RecordPaymentWebhook(r)
	if classifyCommercePaymentWebhookOutcome(r) == "rejected" {
		productionmetrics.RecordPaymentWebhookRejection(r)
	}
}

func classifyCommercePaymentWebhookOutcome(result string) string {
	switch result {
	case "accepted":
		return "accepted"
	case "replayed":
		return "replayed"
	default:
		return "rejected"
	}
}
