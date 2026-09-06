package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/platform/payments/psp/httpx"
	"github.com/avf/avf-vending-api/internal/platform/payments/psp/zalopay"
)

// ZaloPayProvider is the live ZaloPay (or VietQR-via-ZaloPay) WiredLiveProvider adapter.
type ZaloPayProvider struct {
	creds  zalopay.Credentials
	vietQR bool
	wired  bool
	client *httpx.Client
}

// NewZaloPayProvider builds a ZaloPay adapter. When vietQR is true, Key() is "vietqr".
func NewZaloPayProvider(cfg *config.Config, vietQR bool) *ZaloPayProvider {
	p := &ZaloPayProvider{vietQR: vietQR, client: httpx.New(httpx.DefaultTimeout)}
	if cfg == nil {
		return p
	}
	p.creds = zalopay.Credentials{
		AppID:       cfg.PSP.ZaloPay.AppID,
		Key1:        cfg.PSP.ZaloPay.Key1,
		Key2:        cfg.PSP.ZaloPay.Key2,
		Endpoint:    cfg.PSP.ZaloPay.Endpoint,
		CallbackURL: cfg.PSP.ZaloPay.CallbackURL,
	}
	p.wired = cfg.PSP.ZaloPay.ZaloPayWired()
	return p
}

func (p *ZaloPayProvider) Key() string {
	if p != nil && p.vietQR {
		return "vietqr"
	}
	return "zalopay"
}

func (p *ZaloPayProvider) LivePaymentWired() bool {
	return p != nil && p.wired
}

func (p *ZaloPayProvider) VerifyWebhookSignature(secret string, tsHeader, sigHeader string, rawBody []byte, skew time.Duration) error {
	return VerifyCommerceWebhookHMAC(secret, tsHeader, sigHeader, rawBody, skew)
}

func (p *ZaloPayProvider) ParseWebhookEvent(rawBody []byte) (CommerceWebhookEventJSON, error) {
	return ParseCommerceWebhookEventJSON(rawBody)
}

func (p *ZaloPayProvider) SupportsQueryPaymentStatus() bool {
	return p.LivePaymentWired()
}

func (p *ZaloPayProvider) CreatePaymentSession(ctx context.Context, in CreatePaymentSessionInput) (CreatePaymentSessionResult, error) {
	if !p.LivePaymentWired() {
		return CreatePaymentSessionResult{}, fmt.Errorf("%w for provider %q", ErrLiveProviderNotWired, p.Key())
	}
	providerRef := resolveProviderRef(in)
	now := time.Now()
	appTransID := zalopay.AppTransID(providerRef, now)
	appTime := now.UnixMilli()
	embed := zalopay.EmbedData(p.vietQR)
	if pm := strings.ToLower(strings.TrimSpace(in.PreferredMethod)); pm == "vietqr" {
		embed = zalopay.EmbedData(true)
	} else if pm == "zalopay" || pm == "zalopay_wallet" {
		embed = zalopay.EmbedData(false)
	}
	item := "[]"
	amount := zalopay.FormatAmount(in.AmountMinor)
	appUser := strings.TrimSpace(in.StoreID)
	if appUser == "" {
		appUser = "avf"
	}
	mac := zalopay.CreateOrderMAC(p.creds.Key1, p.creds.AppID, appTransID, appUser, amount, appTime, embed, item)
	callbackURL := strings.TrimSpace(p.creds.CallbackURL)
	if callbackURL == "" {
		// Logged at session create; startup validation should reject live deployments without ZALOPAY_CALLBACK_URL.
	}
	fields := map[string]string{
		"app_id":       p.creds.AppID,
		"app_trans_id": appTransID,
		"app_user":     appUser,
		"app_time":     strconv.FormatInt(appTime, 10),
		"embed_data":   embed,
		"item":         item,
		"amount":       amount,
		"callback_url": callbackURL,
		"description":  "AVF - Thanh toan cho don hang #" + providerRef,
		"bank_code":    "",
		"mac":          mac,
	}
	endpoint := joinEndpoint(p.creds.Endpoint, "/v2/create")
	respBody, status, err := p.client.PostForm(ctx, endpoint, nil, fields)
	if err != nil {
		return CreatePaymentSessionResult{}, fmt.Errorf("zalopay create: %w", err)
	}
	res, err := decodeJSONMap(respBody)
	if err != nil {
		return CreatePaymentSessionResult{}, fmt.Errorf("zalopay create decode: %w (http %d)", err, status)
	}
	if asInt(res["return_code"]) != 1 {
		msg := asString(res["sub_return_message"])
		if msg == "" {
			msg = asString(res["return_message"])
		}
		return CreatePaymentSessionResult{}, fmt.Errorf("zalopay create failed: return_code=%v message=%s", res["return_code"], msg)
	}
	qr := asString(res["qr_code"])
	display, _ := json.Marshal(map[string]any{
		"provider":           p.Key(),
		"provider_reference": providerRef,
		"app_trans_id":       appTransID,
		"qr_code":            qr,
		"return_code":        res["return_code"],
	})
	expiresAt := now.Add(15 * time.Minute)
	return CreatePaymentSessionResult{
		ProviderReference:   providerRef,
		ProviderSessionID:   appTransID,
		QRPayloadOrURL:      qr,
		PaymentURL:          qr,
		CheckoutURL:         qr,
		ExpiresAt:           &expiresAt,
		ProviderDisplayJSON: display,
	}, nil
}

func (p *ZaloPayProvider) QueryPaymentStatus(ctx context.Context, lookup domaincommerce.PaymentProviderLookup) (domaincommerce.PaymentStatusSnapshot, error) {
	if !p.LivePaymentWired() {
		return domaincommerce.PaymentStatusSnapshot{}, ErrQueryPaymentStatusNotSupported
	}
	appTransID := strings.TrimSpace(lookup.ProviderReference)
	if appTransID == "" {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("zalopay query: provider_reference required")
	}
	if !strings.Contains(appTransID, "_") {
		if stored := zalopayAppTransIDFromPayload(lookup.AttemptPayloadJSON); stored != "" {
			appTransID = stored
		} else {
			appTransID = zalopay.AppTransID(appTransID, time.Now())
		}
	}
	mac := zalopay.QueryMAC(p.creds.Key1, p.creds.AppID, appTransID)
	fields := map[string]string{
		"app_id":       p.creds.AppID,
		"app_trans_id": appTransID,
		"mac":          mac,
	}
	endpoint := joinEndpoint(p.creds.Endpoint, "/v2/query")
	respBody, status, err := p.client.PostForm(ctx, endpoint, nil, fields)
	if err != nil {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("zalopay query: %w", err)
	}
	res, err := decodeJSONMap(respBody)
	if err != nil {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("zalopay query decode: %w (http %d)", err, status)
	}
	norm, _ := zalopay.MapReturnCode(asInt(res["return_code"]))
	hint, _ := json.Marshal(res)
	return domaincommerce.PaymentStatusSnapshot{
		NormalizedState: norm,
		ProviderHint:    hint,
	}, nil
}

func (p *ZaloPayProvider) CancelPayment(ctx context.Context, in CancelPaymentInput) error {
	_ = ctx
	_ = in
	return ErrNotImplemented
}

func (p *ZaloPayProvider) RefundPayment(ctx context.Context, in RefundPaymentInput) error {
	_ = ctx
	_ = in
	return ErrNotImplemented
}

// VerifyAndParseCallback verifies a native ZaloPay callback (KEY2 MAC) and maps to CommerceWebhookEventJSON.
func (p *ZaloPayProvider) VerifyAndParseCallback(raw []byte) (orderID, status, zpTransID string, event CommerceWebhookEventJSON, err error) {
	var cb struct {
		Data string `json:"data"`
		Mac  string `json:"mac"`
		Type string `json:"type"`
	}
	if err = json.Unmarshal(raw, &cb); err != nil {
		return "", "", "", CommerceWebhookEventJSON{}, err
	}
	if !zalopay.VerifyCallbackMAC(p.creds.Key2, cb.Data, cb.Mac) {
		return "", "", "", CommerceWebhookEventJSON{}, fmt.Errorf("zalopay callback: invalid mac")
	}
	var data map[string]any
	if err = json.Unmarshal([]byte(cb.Data), &data); err != nil {
		return "", "", "", CommerceWebhookEventJSON{}, fmt.Errorf("zalopay callback data: %w", err)
	}
	appTransID := asString(data["app_trans_id"])
	orderID = appTransID
	if i := strings.Index(appTransID, "_"); i >= 0 && i+1 < len(appTransID) {
		orderID = appTransID[i+1:]
	}
	zpTransID = asString(data["zp_trans_id"])
	// ZaloPay callback embed often uses return_code 1 for success in query; callback data uses "status" or amount presence.
	// Prefer explicit return_code when present; otherwise treat as captured when zp_trans_id is set.
	if rc, ok := data["return_code"]; ok {
		status, _ = zalopay.MapReturnCode(asInt(rc))
	} else if zpTransID != "" {
		status = zalopay.StatusCaptured
	} else {
		status = zalopay.StatusPending
	}
	amount := asInt64(data["amount"])
	cur := "VND"
	event = CommerceWebhookEventJSON{
		Provider:               p.Key(),
		ProviderReference:      orderID,
		WebhookEventID:         zpTransID,
		EventType:              "zalopay.callback",
		NormalizedPaymentState: status,
		PayloadJSON:            json.RawMessage(raw),
		ProviderAmountMinor:    &amount,
		Currency:               &cur,
	}
	return orderID, status, zpTransID, event, nil
}

func zalopayAppTransIDFromPayload(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	var display map[string]any
	if err := json.Unmarshal(payload, &display); err != nil {
		return ""
	}
	return strings.TrimSpace(asString(display["app_trans_id"]))
}
