package payments

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

// VerifyCommerceWebhookHMAC validates AVF webhook authentication:
// HMAC-SHA256 over "{timestamp}.{rawBody}" compared to X-AVF-Webhook-Signature (hex, optional "sha256=" prefix).
// tsHeader is Unix seconds from X-AVF-Webhook-Timestamp; skew bounds replay acceptance.
func VerifyCommerceWebhookHMAC(secret, tsHeader, sigHeader string, body []byte, skew time.Duration) error {
	sigHeader = strings.TrimSpace(sigHeader)
	tsHeader = strings.TrimSpace(tsHeader)
	if sigHeader == "" || tsHeader == "" {
		return errors.New("missing X-AVF-Webhook-Timestamp or X-AVF-Webhook-Signature")
	}
	ts, err := strconv.ParseInt(tsHeader, 10, 64)
	if err != nil {
		return errors.New("invalid X-AVF-Webhook-Timestamp")
	}
	skewSec := int64(skew / time.Second)
	if skewSec < 1 {
		skewSec = 300
	}
	now := time.Now().Unix()
	if ts > now+skewSec || ts < now-skewSec {
		return errors.New("webhook timestamp outside allowed skew")
	}
	tsStr := strconv.FormatInt(ts, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(tsStr))
	mac.Write([]byte{'.'})
	mac.Write(body)
	sum := mac.Sum(nil)

	sigHex := strings.TrimPrefix(sigHeader, "sha256=")
	sigHex = strings.TrimSpace(sigHex)
	want, err := hex.DecodeString(sigHex)
	if err != nil || len(want) != len(sum) || !hmac.Equal(sum, want) {
		return errors.New("invalid webhook signature")
	}
	return nil
}

// PeekWebhookProvider returns the JSON "provider" field when present (best-effort; used before full parse to pick per-provider HMAC secrets).
func PeekWebhookProvider(body []byte) string {
	var v struct {
		Provider string `json:"provider"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return ""
	}
	return strings.TrimSpace(v.Provider)
}
