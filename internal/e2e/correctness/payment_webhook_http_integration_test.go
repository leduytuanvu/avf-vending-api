package correctness

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/workfloworch"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/domain/commerce"
	domainreliability "github.com/avf/avf-vending-api/internal/domain/reliability"
	"github.com/avf/avf-vending-api/internal/httpserver"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func p06SignCommerceWebhook(secret string, body []byte) (ts string, sig string) {
	tunix := time.Now().UTC().Unix()
	ts = strconv.FormatInt(tunix, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte{'.'})
	mac.Write(body)
	sig = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return ts, sig
}

func p06CommerceWebhookReq(t *testing.T, orderID, paymentID uuid.UUID, body []byte, ts, sig string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost,
		"/v1/commerce/orders/"+orderID.String()+"/payments/"+paymentID.String()+"/webhooks",
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if ts != "" {
		req.Header.Set("X-AVF-Webhook-Timestamp", ts)
	}
	if sig != "" {
		req.Header.Set("X-AVF-Webhook-Signature", sig)
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("orderId", orderID.String())
	rctx.URLParams.Add("paymentId", paymentID.String())
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func p06WebhookTestDeps(t *testing.T, webhookSecret string, appEnv config.AppEnvironment) (*api.HTTPApplication, *config.Config) {
	t.Helper()
	pool := testPool(t)
	store := postgres.NewStore(pool)
	audit := appaudit.NewService(pool)
	commerceSvc := appcommerce.NewService(appcommerce.Deps{
		OrderVend:             store,
		PaymentOutbox:         store,
		Lifecycle:             store,
		WebhookPersist:        store,
		SaleLines:             store,
		WorkflowOrchestration: workfloworch.NewDisabled(),
		EnterpriseAudit:       audit,
	})
	app := &api.HTTPApplication{
		Commerce:        commerceSvc,
		TelemetryStore:  store,
		EnterpriseAudit: audit,
	}
	cfg := &config.Config{
		AppEnv: appEnv,
		Commerce: config.CommerceHTTPConfig{
			PaymentOutboxTopic:          "commerce.payments",
			PaymentOutboxEventType:      "payment.session_started",
			PaymentOutboxAggregateType:  "payment",
			PaymentWebhookHMACSecret:    webhookSecret,
			PaymentWebhookTimestampSkew: 300 * time.Second,
		},
	}
	return app, cfg
}

func TestP06_E2E_PaymentWebhookHTTP_validHMACAccepted(t *testing.T) {
	secret := "p06-hmac-" + uuid.NewString()[:8]
	app, cfg := p06WebhookTestDeps(t, secret, config.AppEnvDevelopment)
	pool := app.TelemetryStore.Pool()
	ctx := context.Background()

	orderIDem := "p06-http-wh-" + uuid.NewString()
	orderRes, err := postgres.NewStore(pool).CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      1,
		Currency:       "USD",
		SubtotalMinor:  200,
		TaxMinor:       0,
		TotalMinor:     200,
		IdempotencyKey: orderIDem,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	payIDem := orderIDem + ":pay"
	outIDem := orderIDem + ":out:" + orderRes.Order.ID.String()
	payRes, err := postgres.NewStore(pool).CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "psp_fixture",
		PaymentState:         "created",
		AmountMinor:          200,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          "commerce.payments",
		OutboxEventType:      "payment.session_started",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "payment",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	ref := "ref-p06-http-" + uuid.NewString()
	evt := "evt-p06-http-1-" + uuid.NewString()
	bodyObj := map[string]any{
		"provider":                 "psp_fixture",
		"provider_reference":       ref,
		"webhook_event_id":         evt,
		"event_type":               "payment.captured",
		"normalized_payment_state": "captured",
		"payload_json":             map[string]any{"ok": true},
		"provider_amount_minor":    200,
		"currency":                 "USD",
	}
	body, err := json.Marshal(bodyObj)
	require.NoError(t, err)
	ts, sig := p06SignCommerceWebhook(secret, body)

	rec := httptest.NewRecorder()
	httpserver.IntegrationTestCommercePublicPaymentWebhook(app, cfg).ServeHTTP(rec,
		p06CommerceWebhookReq(t, orderRes.Order.ID, payRes.Payment.ID, body, ts, sig))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestP06_E2E_PaymentWebhookHTTP_duplicateSignedDeliveryIsIdempotentHTTP(t *testing.T) {
	secret := "p06-hmac-dup-" + uuid.NewString()[:8]
	app, cfg := p06WebhookTestDeps(t, secret, config.AppEnvDevelopment)
	pool := app.TelemetryStore.Pool()
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "p06-http-dup-" + uuid.NewString()
	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      1,
		Currency:       "USD",
		SubtotalMinor:  200,
		TaxMinor:       0,
		TotalMinor:     200,
		IdempotencyKey: orderIDem,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	payIDem := orderIDem + ":pay"
	outIDem := orderIDem + ":out:" + orderRes.Order.ID.String()
	payRes, err := store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "psp_fixture",
		PaymentState:         "created",
		AmountMinor:          200,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          "commerce.payments",
		OutboxEventType:      "payment.session_started",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "payment",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	ref := "ref-dup-" + uuid.NewString()
	evt := "evt-p06-dup-" + uuid.NewString()
	bodyObj := map[string]any{
		"provider":                 "psp_fixture",
		"provider_reference":       ref,
		"webhook_event_id":         evt,
		"event_type":               "payment.captured",
		"normalized_payment_state": "captured",
		"payload_json":             map[string]any{"ok": true},
		"provider_amount_minor":    200,
		"currency":                 "USD",
	}
	body, err := json.Marshal(bodyObj)
	require.NoError(t, err)

	h := httpserver.IntegrationTestCommercePublicPaymentWebhook(app, cfg)

	ts1, sig1 := p06SignCommerceWebhook(secret, body)
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, p06CommerceWebhookReq(t, orderRes.Order.ID, payRes.Payment.ID, body, ts1, sig1))
	require.Equal(t, http.StatusOK, rec1.Code, rec1.Body.String())

	ts2, sig2 := p06SignCommerceWebhook(secret, body)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, p06CommerceWebhookReq(t, orderRes.Order.ID, payRes.Payment.ID, body, ts2, sig2))
	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())

	var resp struct {
		Replay bool `json:"replay"`
	}
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &resp))
	require.True(t, resp.Replay)
}

func TestP06_E2E_PaymentWebhookHTTP_invalidHMACRejected(t *testing.T) {
	secret := "p06-hmac-bad-" + uuid.NewString()[:8]
	app, cfg := p06WebhookTestDeps(t, secret, config.AppEnvDevelopment)
	pool := app.TelemetryStore.Pool()
	ctx := context.Background()

	orderIDem := "p06-http-bad-" + uuid.NewString()
	orderRes, err := postgres.NewStore(pool).CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      1,
		Currency:       "USD",
		SubtotalMinor:  200,
		TaxMinor:       0,
		TotalMinor:     200,
		IdempotencyKey: orderIDem,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	payIDem := orderIDem + ":pay"
	outIDem := orderIDem + ":out:" + orderRes.Order.ID.String()
	payRes, err := postgres.NewStore(pool).CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "psp_fixture",
		PaymentState:         "created",
		AmountMinor:          200,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          "commerce.payments",
		OutboxEventType:      "payment.session_started",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "payment",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	body, err := json.Marshal(map[string]any{
		"provider":                 "psp_fixture",
		"provider_reference":       "r-" + uuid.NewString(),
		"webhook_event_id":         "e1-" + uuid.NewString(),
		"event_type":               "payment.captured",
		"normalized_payment_state": "captured",
		"payload_json":             map[string]any{},
		"provider_amount_minor":    200,
		"currency":                 "USD",
	})
	require.NoError(t, err)
	ts := strconv.FormatInt(time.Now().Unix(), 10)

	rec := httptest.NewRecorder()
	httpserver.IntegrationTestCommercePublicPaymentWebhook(app, cfg).ServeHTTP(rec,
		p06CommerceWebhookReq(t, orderRes.Order.ID, payRes.Payment.ID, body, ts, "sha256=deadbeef"))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestP06_E2E_PaymentWebhookHTTP_oldTimestampRejected(t *testing.T) {
	secret := "p06-hmac-oldts-" + uuid.NewString()[:8]
	app, cfg := p06WebhookTestDeps(t, secret, config.AppEnvDevelopment)
	pool := app.TelemetryStore.Pool()
	ctx := context.Background()

	orderIDem := "p06-http-oldts-" + uuid.NewString()
	orderRes, err := postgres.NewStore(pool).CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      1,
		Currency:       "USD",
		SubtotalMinor:  200,
		TaxMinor:       0,
		TotalMinor:     200,
		IdempotencyKey: orderIDem,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	payIDem := orderIDem + ":pay"
	outIDem := orderIDem + ":out:" + orderRes.Order.ID.String()
	payRes, err := postgres.NewStore(pool).CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "psp_fixture",
		PaymentState:         "created",
		AmountMinor:          200,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          "commerce.payments",
		OutboxEventType:      "payment.session_started",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "payment",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	body, err := json.Marshal(map[string]any{
		"provider":                 "psp_fixture",
		"provider_reference":       "r-old-" + uuid.NewString(),
		"webhook_event_id":         "e-old-ts-" + uuid.NewString(),
		"event_type":               "payment.captured",
		"normalized_payment_state": "captured",
		"payload_json":             map[string]any{},
		"provider_amount_minor":    200,
		"currency":                 "USD",
	})
	require.NoError(t, err)
	oldUnix := time.Now().Add(-400 * time.Second).Unix()
	ts := strconv.FormatInt(oldUnix, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write([]byte{'.'})
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	rec := httptest.NewRecorder()
	httpserver.IntegrationTestCommercePublicPaymentWebhook(app, cfg).ServeHTTP(rec,
		p06CommerceWebhookReq(t, orderRes.Order.ID, payRes.Payment.ID, body, ts, sig))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestP06_E2E_PaymentWebhookHTTP_captureDoesNotAutoCompleteVend(t *testing.T) {
	secret := "p06-hmac-vend-" + uuid.NewString()[:8]
	app, cfg := p06WebhookTestDeps(t, secret, config.AppEnvDevelopment)
	pool := app.TelemetryStore.Pool()
	ctx := context.Background()

	orderIDem := "p06-http-vend-" + uuid.NewString()
	orderRes, err := postgres.NewStore(pool).CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      1,
		Currency:       "USD",
		SubtotalMinor:  200,
		TaxMinor:       0,
		TotalMinor:     200,
		IdempotencyKey: orderIDem,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	payIDem := orderIDem + ":pay"
	outIDem := orderIDem + ":out:" + orderRes.Order.ID.String()
	payRes, err := postgres.NewStore(pool).CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "psp_fixture",
		PaymentState:         "created",
		AmountMinor:          200,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          "commerce.payments",
		OutboxEventType:      "payment.session_started",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "payment",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	refVNC := "ref-vend-nc-" + uuid.NewString()
	evtVNC := "evt-vend-nc-" + uuid.NewString()
	bodyObj := map[string]any{
		"provider":                 "psp_fixture",
		"provider_reference":       refVNC,
		"webhook_event_id":         evtVNC,
		"event_type":               "payment.captured",
		"normalized_payment_state": "captured",
		"payload_json":             map[string]any{},
		"provider_amount_minor":    200,
		"currency":                 "USD",
	}
	body, err := json.Marshal(bodyObj)
	require.NoError(t, err)
	ts, sig := p06SignCommerceWebhook(secret, body)
	rec := httptest.NewRecorder()
	httpserver.IntegrationTestCommercePublicPaymentWebhook(app, cfg).ServeHTTP(rec,
		p06CommerceWebhookReq(t, orderRes.Order.ID, payRes.Payment.ID, body, ts, sig))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var vendState string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM vend_sessions WHERE order_id = $1 LIMIT 1`,
		orderRes.Order.ID).Scan(&vendState))
	require.Equal(t, "pending", vendState)
}

func TestP06_E2E_PaymentWebhookHTTP_productionUnsignedRejected(t *testing.T) {
	app, cfg := p06WebhookTestDeps(t, "", config.AppEnvProduction)

	body := []byte(`{"provider":"psp_fixture"}`)
	rec := httptest.NewRecorder()
	httpserver.IntegrationTestCommercePublicPaymentWebhook(app, cfg).ServeHTTP(rec,
		p06CommerceWebhookReq(t, uuid.New(), uuid.New(), body, "", ""))
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestP06_E2E_PaymentWebhookHTTP_amountMismatchCreatesReconciliationCase(t *testing.T) {
	secret := "p06-hmac-mm-" + uuid.NewString()[:8]
	app, cfg := p06WebhookTestDeps(t, secret, config.AppEnvDevelopment)
	pool := app.TelemetryStore.Pool()
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "p06-http-mm-" + uuid.NewString()
	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      1,
		Currency:       "USD",
		SubtotalMinor:  200,
		TaxMinor:       0,
		TotalMinor:     200,
		IdempotencyKey: orderIDem,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	payIDem := orderIDem + ":pay"
	outIDem := orderIDem + ":out:" + orderRes.Order.ID.String()
	payRes, err := store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "psp_fixture",
		PaymentState:         "created",
		AmountMinor:          200,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          "commerce.payments",
		OutboxEventType:      "payment.session_started",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "payment",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	refMM := "ref-mm-" + uuid.NewString()
	evtMM := "evt-mm-1-" + uuid.NewString()
	bodyObj := map[string]any{
		"provider":                 "psp_fixture",
		"provider_reference":       refMM,
		"webhook_event_id":         evtMM,
		"event_type":               "payment.captured",
		"normalized_payment_state": "captured",
		"payload_json":             map[string]any{},
		"provider_amount_minor":    999,
		"currency":                 "USD",
	}
	body, err := json.Marshal(bodyObj)
	require.NoError(t, err)
	ts, sig := p06SignCommerceWebhook(secret, body)

	rec := httptest.NewRecorder()
	httpserver.IntegrationTestCommercePublicPaymentWebhook(app, cfg).ServeHTTP(rec,
		p06CommerceWebhookReq(t, orderRes.Order.ID, payRes.Payment.ID, body, ts, sig))
	require.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())

	var n int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM commerce_reconciliation_cases
WHERE organization_id = $1 AND payment_id = $2 AND case_type = 'webhook_amount_currency_mismatch'`,
		testfixtures.DevOrganizationID, payRes.Payment.ID).Scan(&n))
	require.Equal(t, 1, n)

	var auditN int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM audit_events
WHERE organization_id = $1 AND action = 'payment.webhook.rejected'`,
		testfixtures.DevOrganizationID).Scan(&auditN))
	require.GreaterOrEqual(t, auditN, 1)
}

func TestP06_E2E_PaymentWebhookHTTP_terminalWebhookDoesNotCorruptWhenOutOfOrder(t *testing.T) {
	secret := "p06-hmac-term-" + uuid.NewString()[:8]
	app, cfg := p06WebhookTestDeps(t, secret, config.AppEnvDevelopment)
	pool := app.TelemetryStore.Pool()
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "p06-http-term-" + uuid.NewString()
	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      1,
		Currency:       "USD",
		SubtotalMinor:  200,
		TaxMinor:       0,
		TotalMinor:     200,
		IdempotencyKey: orderIDem,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	payIDem := orderIDem + ":pay"
	outIDem := orderIDem + ":out:" + orderRes.Order.ID.String()
	payRes, err := store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "psp_fixture",
		PaymentState:         "created",
		AmountMinor:          200,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          "commerce.payments",
		OutboxEventType:      "payment.session_started",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "payment",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	refTerm := "ref-term-" + uuid.NewString()
	evtFirst := "evt-term-first-" + uuid.NewString()
	evtSecond := "evt-term-second-" + uuid.NewString()

	mkJSON := func(evt string) []byte {
		b, mErr := json.Marshal(map[string]any{
			"provider": "psp_fixture", "provider_reference": refTerm, "webhook_event_id": evt,
			"event_type": "payment.captured", "normalized_payment_state": "captured",
			"payload_json": map[string]any{}, "provider_amount_minor": 200, "currency": "USD",
		})
		require.NoError(t, mErr)
		return b
	}

	in := appcommerce.ApplyPaymentProviderWebhookInput{
		OrganizationID:          testfixtures.DevOrganizationID,
		OrderID:                 orderRes.Order.ID,
		PaymentID:               payRes.Payment.ID,
		Provider:                "psp_fixture",
		ProviderReference:       refTerm,
		WebhookEventID:          evtFirst,
		EventType:               "payment.captured",
		NormalizedPaymentState:  "captured",
		Payload:                 []byte(`{}`),
		OutboxTopic:             "commerce.payments",
		OutboxEventType:         domainreliability.OutboxEventPaymentConfirmed,
		OutboxPayload:           []byte(`{}`),
		OutboxAggregateType:     "payment",
		OutboxAggregateID:       payRes.Payment.ID,
		OutboxIdempotencyKey:    orderIDem + ":wh:first",
		WebhookValidationStatus: "unsigned_development",
	}
	_, err = store.ApplyPaymentProviderWebhook(ctx, in)
	require.NoError(t, err)

	body2 := mkJSON(evtSecond)
	ts2, sig2 := p06SignCommerceWebhook(secret, body2)
	rec := httptest.NewRecorder()
	httpserver.IntegrationTestCommercePublicPaymentWebhook(app, cfg).ServeHTTP(rec,
		p06CommerceWebhookReq(t, orderRes.Order.ID, payRes.Payment.ID, body2, ts2, sig2))
	require.NotEqual(t, http.StatusOK, rec.Code, rec.Body.String())

	var payState string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM payments WHERE id = $1`, payRes.Payment.ID).Scan(&payState))
	require.Equal(t, "captured", payState)
}

func TestP06_E2E_PaymentWebhookHTTP_expiredTransitionFromCreatedAccepted(t *testing.T) {
	secret := "p06-hmac-exp-" + uuid.NewString()[:8]
	app, cfg := p06WebhookTestDeps(t, secret, config.AppEnvDevelopment)
	pool := app.TelemetryStore.Pool()
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orderIDem := "p06-http-exp-" + uuid.NewString()
	orderRes, err := store.CreateOrderWithVendSession(ctx, commerce.CreateOrderVendInput{
		OrganizationID: testfixtures.DevOrganizationID,
		MachineID:      testfixtures.DevMachineID,
		ProductID:      testfixtures.DevProductWater,
		SlotIndex:      1,
		Currency:       "USD",
		SubtotalMinor:  200,
		TaxMinor:       0,
		TotalMinor:     200,
		IdempotencyKey: orderIDem,
		OrderStatus:    "created",
		VendState:      "pending",
	})
	require.NoError(t, err)

	payIDem := orderIDem + ":pay"
	outIDem := orderIDem + ":out:" + orderRes.Order.ID.String()
	payRes, err := store.CreatePaymentWithOutbox(ctx, commerce.PaymentOutboxInput{
		OrganizationID:       testfixtures.DevOrganizationID,
		OrderID:              orderRes.Order.ID,
		Provider:             "psp_fixture",
		PaymentState:         "created",
		AmountMinor:          200,
		Currency:             "USD",
		IdempotencyKey:       payIDem,
		OutboxTopic:          "commerce.payments",
		OutboxEventType:      "payment.session_started",
		OutboxPayload:        []byte(`{}`),
		OutboxAggregateType:  "payment",
		OutboxAggregateID:    orderRes.Order.ID,
		OutboxIdempotencyKey: outIDem,
	})
	require.NoError(t, err)

	refExp := "ref-exp-1-" + uuid.NewString()
	evtExp := "evt-exp-1-" + uuid.NewString()
	bodyObj := map[string]any{
		"provider":                 "psp_fixture",
		"provider_reference":       refExp,
		"webhook_event_id":         evtExp,
		"event_type":               "payment.expired",
		"normalized_payment_state": "expired",
		"payload_json":             map[string]any{},
		"provider_amount_minor":    200,
		"currency":                 "USD",
	}
	body, err := json.Marshal(bodyObj)
	require.NoError(t, err)
	ts, sig := p06SignCommerceWebhook(secret, body)
	rec := httptest.NewRecorder()
	httpserver.IntegrationTestCommercePublicPaymentWebhook(app, cfg).ServeHTTP(rec,
		p06CommerceWebhookReq(t, orderRes.Order.ID, payRes.Payment.ID, body, ts, sig))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payState, ordStatus string
	require.NoError(t, pool.QueryRow(ctx, `SELECT state FROM payments WHERE id = $1`, payRes.Payment.ID).Scan(&payState))
	require.Equal(t, "expired", payState)
	require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderRes.Order.ID).Scan(&ordStatus))
	require.Equal(t, "created", ordStatus)
}
