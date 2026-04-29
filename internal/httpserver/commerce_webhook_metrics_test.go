package httpserver

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestRecordCommercePaymentWebhookResultRejectedIncrementsCanonicalCounter(t *testing.T) {
	before := gatherCounterValue(t, "payment_webhook_rejections_total")
	recordCommercePaymentWebhookResult("401_hmac_invalid")
	after := gatherCounterValue(t, "payment_webhook_rejections_total")
	if after <= before {
		t.Fatalf("expected payment_webhook_rejections_total to increase: before=%g after=%g", before, after)
	}
}

func gatherCounterValue(t *testing.T, metricFamily string) float64 {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() != metricFamily {
			continue
		}
		var sum float64
		for _, m := range mf.Metric {
			if m.Counter == nil {
				continue
			}
			sum += m.Counter.GetValue()
		}
		return sum
	}
	return 0
}
