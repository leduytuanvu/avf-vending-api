package loadtest

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Snapshot holds scraped Prometheus exposition numbers referenced by load-test docs.
type Snapshot struct {
	OutboxPendingTotal *float64
	OutboxLagSum       *float64
	OutboxLagCount     *float64
	PaymentWebhookReq  *float64
	RedisRateLimitHits *float64
	MQTTAckTimeouts    *float64

	DBPoolAcquired *float64
	DBPoolIdle     *float64
	DBPoolTotal    *float64
	DBPoolMax      *float64

	Lines map[string]string
}

// ParsePrometheusText extracts known counters/gauges (single-line unlabeled or first labeled sample;
// avf_commerce_payment_webhook_requests_total is summed across label values).
func ParsePrometheusText(r io.Reader) Snapshot {
	snap := Snapshot{Lines: make(map[string]string)}
	sc := bufio.NewScanner(r)
	buf := make([]byte, 0, 1024*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		full := fields[0]
		base := full
		if i := strings.IndexByte(full, '{'); i >= 0 {
			base = full[:i]
		}
		valStr := fields[len(fields)-1]
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}

		switch base {
		case "avf_commerce_payment_webhook_requests_total", "payment_webhook_requests_total":
			if snap.PaymentWebhookReq == nil {
				z := 0.0
				snap.PaymentWebhookReq = &z
			}
			*snap.PaymentWebhookReq += val
			continue
		case "outbox_pending_total":
			if snap.OutboxPendingTotal == nil {
				v := val
				snap.OutboxPendingTotal = &v
			}
			continue
		case "outbox_lag_seconds_sum":
			v := val
			snap.OutboxLagSum = &v
			continue
		case "outbox_lag_seconds_count":
			v := val
			snap.OutboxLagCount = &v
			continue
		case "avf_redis_rate_limit_hits_total":
			if snap.RedisRateLimitHits == nil {
				v := val
				snap.RedisRateLimitHits = &v
			}
			continue
		case "avf_mqtt_command_ack_timeout_total":
			if snap.MQTTAckTimeouts == nil {
				v := val
				snap.MQTTAckTimeouts = &v
			}
			continue
		case "avf_db_pool_acquired_conns":
			if snap.DBPoolAcquired == nil {
				v := val
				snap.DBPoolAcquired = &v
			}
			continue
		case "avf_db_pool_idle_conns":
			if snap.DBPoolIdle == nil {
				v := val
				snap.DBPoolIdle = &v
			}
			continue
		case "avf_db_pool_total_conns":
			if snap.DBPoolTotal == nil {
				v := val
				snap.DBPoolTotal = &v
			}
			continue
		case "avf_db_pool_max_conns":
			if snap.DBPoolMax == nil {
				v := val
				snap.DBPoolMax = &v
			}
			continue
		default:
		}

		if _, ok := snap.Lines[base]; ok {
			continue // keep first for generic Lines map
		}
		snap.Lines[base] = valStr
	}
	return snap
}

// FetchPrometheus pulls /metrics text from an ops URL (optional Bearer scrape token).
func FetchPrometheus(ctx context.Context, metricsURL, bearerToken string) (Snapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(metricsURL), nil)
	if err != nil {
		return Snapshot{}, err
	}
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Snapshot{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return Snapshot{}, fmt.Errorf("metrics status=%d body=%s", resp.StatusCode, string(b))
	}
	return ParsePrometheusText(resp.Body), nil
}
