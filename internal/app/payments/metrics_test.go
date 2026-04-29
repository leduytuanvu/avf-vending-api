package payments

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/platform/observability/productionmetrics"
	"github.com/prometheus/client_golang/prometheus"
)

// Ensures ops metrics used by the payment domain stay registered when this package is linked in tests.
func TestPaymentsRelatedProductionMetricsGather(t *testing.T) {
	t.Parallel()
	productionmetrics.RecordPaymentWebhook("warmup")
	productionmetrics.RecordPaymentWebhookAmountCurrencyMismatch()
	productionmetrics.SetPaymentProviderProbeStalePendingQueue(0)
	if _, err := prometheus.DefaultGatherer.Gather(); err != nil {
		t.Fatal(err)
	}
}
