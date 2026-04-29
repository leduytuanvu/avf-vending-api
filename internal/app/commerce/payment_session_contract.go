package commerce

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
)

// PaymentSessionKioskView is the stable vending-app contract after StartPaymentWithOutbox (HTTP and docs).
type PaymentSessionKioskView struct {
	SaleID         string
	SessionID      string // vend_session_id
	PaymentID      string
	AmountMinor    int64
	Currency       string
	Provider       string
	Status         string // client lifecycle: pending|authorized|paid|failed|expired|refunded|canceled|dispute
	PaymentState   string // raw DB payment.state
	QrURL          *string
	PaymentURL     *string
	CheckoutURL    *string
	ExpiresAt      *time.Time
	IdempotencyKey string
	RequestID      string
	OutboxEventID  int64
	Replay         bool
}

// BuildPaymentSessionKioskView maps domain payment/outbox plus checkout row and outbox JSON envelope.
func BuildPaymentSessionKioskView(
	st CheckoutStatusView,
	res domaincommerce.PaymentOutboxResult,
	outboxPayloadJSON []byte,
	idempotencyKey string,
) PaymentSessionKioskView {
	pay := res.Payment
	idem := strings.TrimSpace(idempotencyKey)

	qr, pURL, chk := extractHostedCheckoutURLs(outboxPayloadJSON)
	exp := parseExpiresFromOutboxJSON(outboxPayloadJSON, pay.CreatedAt)

	return PaymentSessionKioskView{
		SaleID:         st.Order.ID.String(),
		SessionID:      st.Vend.ID.String(),
		PaymentID:      pay.ID.String(),
		AmountMinor:    pay.AmountMinor,
		Currency:       strings.ToUpper(strings.TrimSpace(pay.Currency)),
		Provider:       strings.TrimSpace(pay.Provider),
		Status:         paymentLifecycleClientStatus(pay.State),
		PaymentState:   pay.State,
		QrURL:          qr,
		PaymentURL:     pURL,
		CheckoutURL:    chk,
		ExpiresAt:      exp,
		IdempotencyKey: idem,
		RequestID:      idem,
		OutboxEventID:  res.Outbox.ID,
		Replay:         res.Replay,
	}
}

func paymentLifecycleClientStatus(dbPaymentState string) string {
	switch strings.TrimSpace(strings.ToLower(dbPaymentState)) {
	case "":
		return "pending"
	case "created":
		return "pending"
	case "authorized":
		return "authorized"
	case "captured":
		return "paid"
	case "failed":
		return "failed"
	case "expired":
		return "expired"
	case "canceled":
		return "canceled"
	case "refunded", "partially_refunded":
		return "refunded"
	default:
		return strings.TrimSpace(dbPaymentState)
	}
}

func extractHostedCheckoutURLs(payload []byte) (qrURL, paymentURL, checkoutURL *string) {
	payload = compactJSONEnvelope(payload)
	if len(payload) == 0 || !json.Valid(payload) {
		return nil, nil, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, nil, nil
	}
	qrURL = decodeOptionalURL(m, "qr_url", "qrUrl")
	paymentURL = decodeOptionalURL(m, "payment_url", "paymentUrl")
	checkoutURL = decodeOptionalURL(m, "checkout_url", "checkoutUrl")
	return qrURL, paymentURL, checkoutURL
}

func decodeOptionalURL(fields map[string]json.RawMessage, keys ...string) *string {
	for _, k := range keys {
		raw := fields[k]
		if len(raw) == 0 {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			continue
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
		return &s
	}
	return nil
}

func compactJSONEnvelope(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func parseExpiresFromOutboxJSON(payload []byte, paymentCreatedAt time.Time) *time.Time {
	payload = compactJSONEnvelope(payload)
	if len(payload) == 0 || !json.Valid(payload) {
		return nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil
	}
	for _, k := range []string{
		"expires_at", "expiresAt", "expiration_time", "expirationTime",
		"session_expires_at", "sessionExpiresAt", "expire_at", "expireAt",
	} {
		if raw, ok := m[k]; ok && len(raw) > 0 {
			var s string
			if err := json.Unmarshal(raw, &s); err == nil {
				s = strings.TrimSpace(s)
				if t, err := time.Parse(time.RFC3339, s); err == nil {
					return ptrTime(t.UTC())
				}
				if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
					return ptrTime(t.UTC())
				}
			}
			var f float64
			if err := json.Unmarshal(raw, &f); err == nil && f > 0 {
				// Unix seconds (float or int JSON)
				return ptrTime(time.Unix(int64(f), 0).UTC())
			}
		}
	}
	if raw, ok := m["expires_in"]; ok && len(raw) > 0 {
		var sec float64
		if err := json.Unmarshal(raw, &sec); err == nil && sec > 0 {
			t := paymentCreatedAt.UTC().Add(time.Duration(sec * float64(time.Second)))
			return ptrTime(t)
		}
		if s, err := strconv.Unquote(string(raw)); err == nil {
			if d, err := time.ParseDuration(s); err == nil {
				return ptrTime(paymentCreatedAt.UTC().Add(d))
			}
		}
	}
	return nil
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
