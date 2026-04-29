package loadtest

import (
	"strings"
	"testing"
)

func TestParsePrometheusText_outbox(t *testing.T) {
	txt := `# help
outbox_pending_total 3.5
outbox_lag_seconds_sum 2.5
outbox_lag_seconds_count 10
avf_commerce_payment_webhook_requests_total{result="accepted"} 5
avf_commerce_payment_webhook_requests_total{result="replayed"} 2
avf_db_pool_acquired_conns 4
avf_db_pool_max_conns 25
`
	s := ParsePrometheusText(strings.NewReader(txt))
	if s.OutboxPendingTotal == nil || *s.OutboxPendingTotal != 3.5 {
		t.Fatalf("outbox: %#v", s.OutboxPendingTotal)
	}
	if s.OutboxLagSum == nil || *s.OutboxLagSum != 2.5 || s.OutboxLagCount == nil || *s.OutboxLagCount != 10 {
		t.Fatalf("outbox lag: sum=%v count=%v", s.OutboxLagSum, s.OutboxLagCount)
	}
	if s.PaymentWebhookReq == nil || *s.PaymentWebhookReq != 7 {
		t.Fatalf("webhook sum: %#v", s.PaymentWebhookReq)
	}
	if s.DBPoolAcquired == nil || *s.DBPoolAcquired != 4 || s.DBPoolMax == nil || *s.DBPoolMax != 25 {
		t.Fatalf("db pool: acq=%v max=%v", s.DBPoolAcquired, s.DBPoolMax)
	}
}
