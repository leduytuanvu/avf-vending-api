package productionmetrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestGatherIncludesCanonicalMetricFamilies(t *testing.T) {
	warmupCanonicalMetricFamilies(t)
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]struct{}{}
	for _, mf := range mfs {
		if mf.GetName() != "" {
			names[mf.GetName()] = struct{}{}
		}
	}
	want := []string{
		"http_requests_total",
		"grpc_requests_total",
		"machine_checkins_total",
		"payment_webhooks_total",
		"payment_webhook_amount_currency_mismatch_total",
		"payment_provider_probe_stale_pending_queue",
		"commands_dispatched_total",
		"outbox_pending_total",
		"audit_events_total",
	}
	for _, w := range want {
		if _, ok := names[w]; !ok {
			t.Fatalf("missing canonical metric family %q (gather returned %d families)", w, len(names))
		}
	}
}

func TestRecordAuditWriteFailureIncrementsCounter(t *testing.T) {
	RecordAuditWriteFailure("baseline")
	before := gatherSumFamily(t, "audit_write_failures_total")
	RecordAuditWriteFailure("unit_test")
	after := gatherSumFamily(t, "audit_write_failures_total")
	if after <= before {
		t.Fatalf("audit_write_failures_total expected increase before=%g after=%g", before, after)
	}
}

func warmupCanonicalMetricFamilies(t *testing.T) {
	t.Helper()
	RecordHTTPRequest("GET", "/warmup", "200", time.Nanosecond)
	RecordGRPCUnary("WarmupService", "Warmup", "OK", time.Nanosecond)
	RecordMachineCheckIn("warmup")
	RecordPaymentWebhook("accepted")
	RecordCommandDispatched()
	SetOutboxPending(1)
	RecordAuditEvent("warmup")
	SetPaymentProviderProbeStalePendingQueue(0)
}

func gatherSumFamily(t *testing.T, family string) float64 {
	t.Helper()
	mfs, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() != family {
			continue
		}
		var sum float64
		for _, m := range mf.Metric {
			if m.Counter != nil {
				sum += m.Counter.GetValue()
			}
		}
		return sum
	}
	t.Fatalf("metric family %q not found", family)
	return 0
}

func TestRecordPaymentWebhookAmountCurrencyMismatchIncrements(t *testing.T) {
	before := gatherSumFamily(t, "payment_webhook_amount_currency_mismatch_total")
	RecordPaymentWebhookAmountCurrencyMismatch()
	after := gatherSumFamily(t, "payment_webhook_amount_currency_mismatch_total")
	if after <= before {
		t.Fatalf("payment_webhook_amount_currency_mismatch_total expected increase before=%g after=%g", before, after)
	}
}
