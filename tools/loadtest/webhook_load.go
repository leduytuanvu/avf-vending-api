package loadtest

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SignWebhookAVFHMAC builds X-AVF-Webhook-Timestamp / X-AVF-Webhook-Signature for COMMERCE_PAYMENT_WEBHOOK_VERIFICATION=avf_hmac.
func SignWebhookAVFHMAC(secret string, tsUnix int64, rawBody []byte) (tsHeader, sigHex string) {
	tsHeader = strconv.FormatInt(tsUnix, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(tsHeader))
	mac.Write([]byte{'.'})
	mac.Write(rawBody)
	sigHex = hex.EncodeToString(mac.Sum(nil))
	return tsHeader, sigHex
}

// WebhookBurst posts signed provider webhooks for load testing (sandbox/staging only).
func WebhookBurst(ctx context.Context, baseURL string, secret string, orderID, paymentID uuid.UUID, burst int, duplicateEvery int, recorder *LatencyRecorder) error {
	baseURL = strings.TrimRight(baseURL, "/")
	if secret == "" {
		return fmt.Errorf("webhook secret required for signed burst")
	}
	for i := 0; i < burst; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		evt := fmt.Sprintf("loadtest-%s-%d", orderID.String(), i)
		if duplicateEvery > 0 && i%duplicateEvery == 0 && i > 0 {
			evt = fmt.Sprintf("loadtest-%s-%d", orderID.String(), i-1)
		}
		body := map[string]any{
			"provider":           "loadtest_psp",
			"provider_reference": fmt.Sprintf("pref-%d", i),
			"webhook_event_id":   evt,
			"payment_state":      "captured",
			"event_type":         "payment.captured",
		}
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		ts := time.Now().Unix()
		tsH, sig := SignWebhookAVFHMAC(secret, ts, raw)
		u := fmt.Sprintf("%s/v1/commerce/orders/%s/payments/%s/webhooks", baseURL, orderID.String(), paymentID.String())
		start := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(string(raw)))
		if err != nil {
			recorder.Add(time.Since(start), true)
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-AVF-Webhook-Timestamp", tsH)
		req.Header.Set("X-AVF-Webhook-Signature", sig)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			recorder.Add(time.Since(start), true)
			return err
		}
		ok := resp.StatusCode < 300
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		recorder.Add(time.Since(start), !ok)
		if !ok {
			return fmt.Errorf("webhook burst %d: status=%d", i, resp.StatusCode)
		}
	}
	return nil
}
