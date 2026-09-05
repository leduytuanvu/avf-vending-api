package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/avf/avf-vending-api/internal/platform/payments/psp/httpx"
	"github.com/avf/avf-vending-api/internal/platform/payments/psp/momo"
)

// MoMoProvider is the live MoMo Wallet WiredLiveProvider adapter.
type MoMoProvider struct {
	keys   momo.TenantKeys
	wired  bool
	client *httpx.Client
}

// NewMoMoProvider builds a MoMo adapter from config credentials.
func NewMoMoProvider(cfg *config.Config) *MoMoProvider {
	p := &MoMoProvider{client: httpx.New(httpx.DefaultTimeout)}
	if cfg == nil {
		return p
	}
	p.keys = momo.TenantKeys{
		AVF: momoCredentialsFromConfig(cfg.PSP.MoMo.AVF),
		TFO: momoCredentialsFromConfig(cfg.PSP.MoMo.TFO),
	}
	p.wired = cfg.PSP.MoMo.MoMoWired()
	return p
}

func momoCredentialsFromConfig(t config.MoMoTenantCredentials) momo.Credentials {
	return momo.Credentials{
		PartnerCode: t.PartnerCode,
		AccessKey:   t.AccessKey,
		SecretKey:   t.SecretKey,
		Endpoint:    t.Endpoint,
		RedirectURL: t.RedirectURL,
		IPNURL:      t.IPNURL,
		TerminalID:  t.TerminalID,
	}
}

func (p *MoMoProvider) Key() string { return "momo" }

func (p *MoMoProvider) LivePaymentWired() bool {
	return p != nil && p.wired
}

func (p *MoMoProvider) VerifyWebhookSignature(secret string, tsHeader, sigHeader string, rawBody []byte, skew time.Duration) error {
	return fmt.Errorf("momo: use VerifyAndParseIPN for native IPN callbacks, not header HMAC")
}

func (p *MoMoProvider) ParseWebhookEvent(rawBody []byte) (CommerceWebhookEventJSON, error) {
	return ParseCommerceWebhookEventJSON(rawBody)
}

func (p *MoMoProvider) SupportsQueryPaymentStatus() bool {
	return p.LivePaymentWired()
}

func (p *MoMoProvider) credsForMachine(machineCode string) momo.Credentials {
	return p.keys.ForTenant(momo.ResolveTenant(machineCode))
}

func (p *MoMoProvider) CreatePaymentSession(ctx context.Context, in CreatePaymentSessionInput) (CreatePaymentSessionResult, error) {
	if !p.LivePaymentWired() {
		return CreatePaymentSessionResult{}, fmt.Errorf("%w for provider %q", ErrLiveProviderNotWired, p.Key())
	}
	creds := p.credsForMachine(in.MachineExternalCode)
	if strings.TrimSpace(creds.Endpoint) == "" || strings.TrimSpace(creds.PartnerCode) == "" {
		return CreatePaymentSessionResult{}, fmt.Errorf("%w for provider %q", ErrLiveProviderNotWired, p.Key())
	}

	providerRef := resolveProviderRef(in)
	requestID := id.NewUUIDV7String()
	storeID := strings.TrimSpace(in.StoreID)
	if storeID == "" {
		storeID = strings.TrimSpace(creds.TerminalID)
	}
	orderInfo := "Thanh toan don hang MoMo " + providerRef
	amount := momo.FormatAmount(in.AmountMinor)

	req := momo.CreateRequest{
		AccessKey:   creds.AccessKey,
		Amount:      amount,
		ExtraData:   "",
		IPNURL:      creds.IPNURL,
		OrderID:     providerRef,
		OrderInfo:   orderInfo,
		PartnerCode: creds.PartnerCode,
		RedirectURL: creds.RedirectURL,
		RequestID:   requestID,
		RequestType: momo.RequestTypeCreate,
		StoreID:     storeID,
		Lang:        "vi",
	}
	body := momo.BuildCreateBody(creds.SecretKey, req)
	endpoint := joinEndpoint(creds.Endpoint, "/v2/gateway/api/create")
	respBody, status, err := p.client.PostJSON(ctx, endpoint, nil, body)
	if err != nil {
		return CreatePaymentSessionResult{}, fmt.Errorf("momo create: %w", err)
	}
	res, err := decodeJSONMap(respBody)
	if err != nil {
		return CreatePaymentSessionResult{}, fmt.Errorf("momo create decode: %w (http %d)", err, status)
	}
	if asInt(res["resultCode"]) != 0 {
		return CreatePaymentSessionResult{}, fmt.Errorf("momo create failed: resultCode=%v message=%s", res["resultCode"], asString(res["message"]))
	}
	qr := asString(res["qrCodeUrl"])
	display, _ := json.Marshal(map[string]any{
		"provider":           p.Key(),
		"provider_reference": providerRef,
		"request_id":         requestID,
		"qr_code_url":        qr,
		"result_code":        res["resultCode"],
	})
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	return CreatePaymentSessionResult{
		ProviderReference:   providerRef,
		ProviderSessionID:   requestID,
		QRPayloadOrURL:      qr,
		PaymentURL:          qr,
		CheckoutURL:         qr,
		ExpiresAt:           &expiresAt,
		ProviderDisplayJSON: display,
	}, nil
}

func (p *MoMoProvider) QueryPaymentStatus(ctx context.Context, lookup domaincommerce.PaymentProviderLookup) (domaincommerce.PaymentStatusSnapshot, error) {
	if !p.LivePaymentWired() {
		return domaincommerce.PaymentStatusSnapshot{}, ErrQueryPaymentStatusNotSupported
	}
	orderID := strings.TrimSpace(lookup.ProviderReference)
	if orderID == "" {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("momo query: provider_reference required")
	}
	creds := p.credsForMachine(lookup.MachineExternalCode)
	requestID := id.NewUUIDV7String()
	sig := momo.SignQuery(creds.SecretKey, creds.AccessKey, orderID, creds.PartnerCode, requestID)
	body := map[string]any{
		"partnerCode": creds.PartnerCode,
		"requestId":   requestID,
		"orderId":     orderID,
		"signature":   sig,
		"lang":        "vi",
	}
	endpoint := joinEndpoint(creds.Endpoint, "/v2/gateway/api/query")
	respBody, status, err := p.client.PostJSON(ctx, endpoint, nil, body)
	if err != nil {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("momo query: %w", err)
	}
	res, err := decodeJSONMap(respBody)
	if err != nil {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("momo query decode: %w (http %d)", err, status)
	}
	resultCode := asInt(res["resultCode"])
	norm := momo.MapResultCode(resultCode)
	hint, _ := json.Marshal(res)
	return domaincommerce.PaymentStatusSnapshot{
		NormalizedState: norm,
		ProviderHint:    hint,
	}, nil
}

func (p *MoMoProvider) CancelPayment(ctx context.Context, in CancelPaymentInput) error {
	_ = ctx
	_ = in
	return ErrNotImplemented
}

func (p *MoMoProvider) RefundPayment(ctx context.Context, in RefundPaymentInput) error {
	if !p.LivePaymentWired() {
		return fmt.Errorf("%w for provider %q", ErrLiveProviderNotWired, p.Key())
	}
	// MoMo refund requires the original capture transId; without it we cannot call the API.
	transID := ""
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(in.IdempotencyKey)), "trans:") {
		transID = strings.TrimSpace(in.IdempotencyKey[len("trans:"):])
	}
	if transID == "" {
		return ErrNotImplemented
	}
	orderID := strings.TrimSpace(in.ProviderReference)
	if orderID == "" {
		return fmt.Errorf("momo refund: provider_reference required")
	}
	creds := p.keys.AVF
	if strings.TrimSpace(creds.PartnerCode) == "" {
		creds = p.keys.TFO
	}
	requestID := id.NewUUIDV7String()
	amount := momo.FormatAmount(in.AmountMinor)
	description := "Refund don hang " + orderID
	raw := fmt.Sprintf(
		"accessKey=%s&amount=%s&description=%s&orderId=%s&partnerCode=%s&requestId=%s&transId=%s",
		creds.AccessKey, amount, description, orderID, creds.PartnerCode, requestID, transID,
	)
	mac := hmac.New(sha256.New, []byte(creds.SecretKey))
	_, _ = mac.Write([]byte(raw))
	sig := hex.EncodeToString(mac.Sum(nil))
	body := map[string]any{
		"partnerCode": creds.PartnerCode,
		"orderId":     orderID,
		"requestId":   requestID,
		"amount":      amount,
		"transId":     transID,
		"description": description,
		"ipnUrl":      creds.IPNURL,
		"signature":   sig,
	}
	endpoint := joinEndpoint(creds.Endpoint, "/v2/gateway/api/refund")
	respBody, status, err := p.client.PostJSON(ctx, endpoint, nil, body)
	if err != nil {
		return fmt.Errorf("momo refund: %w", err)
	}
	res, err := decodeJSONMap(respBody)
	if err != nil {
		return fmt.Errorf("momo refund decode: %w (http %d)", err, status)
	}
	if asInt(res["resultCode"]) != 0 {
		return fmt.Errorf("momo refund failed: resultCode=%v message=%s", res["resultCode"], asString(res["message"]))
	}
	return nil
}

// VerifyAndParseIPN verifies a native MoMo IPN body and maps it to CommerceWebhookEventJSON.
func (p *MoMoProvider) VerifyAndParseIPN(raw []byte) (orderID, status, transID string, event CommerceWebhookEventJSON, err error) {
	var data map[string]any
	if err = json.Unmarshal(raw, &data); err != nil {
		return "", "", "", CommerceWebhookEventJSON{}, err
	}
	partnerCode := asString(data["partnerCode"])
	if partnerCode == "" {
		return "", "", "", CommerceWebhookEventJSON{}, fmt.Errorf("momo ipn: missing partnerCode")
	}
	creds := p.keys.AVF
	switch {
	case strings.TrimSpace(p.keys.TFO.PartnerCode) != "" && partnerCode == p.keys.TFO.PartnerCode:
		creds = p.keys.TFO
	case strings.TrimSpace(p.keys.AVF.PartnerCode) != "" && partnerCode == p.keys.AVF.PartnerCode:
		creds = p.keys.AVF
	default:
		return "", "", "", CommerceWebhookEventJSON{}, fmt.Errorf("momo ipn: unknown partnerCode")
	}
	fields := momo.IPNFields{
		AccessKey:    creds.AccessKey,
		Amount:       asString(data["amount"]),
		ExtraData:    asString(data["extraData"]),
		Message:      asString(data["message"]),
		OrderID:      asString(data["orderId"]),
		OrderInfo:    asString(data["orderInfo"]),
		OrderType:    asString(data["orderType"]),
		PartnerCode:  partnerCode,
		PayType:      asString(data["payType"]),
		RequestID:    asString(data["requestId"]),
		ResponseTime: asString(data["responseTime"]),
		ResultCode:   asString(data["resultCode"]),
		TransID:      asString(data["transId"]),
	}
	sig := asString(data["signature"])
	if !momo.VerifyIPNSignature(creds.SecretKey, fields, sig) {
		return "", "", "", CommerceWebhookEventJSON{}, fmt.Errorf("momo ipn: invalid signature")
	}
	orderID = fields.OrderID
	transID = fields.TransID
	status = momo.MapResultCode(asInt(data["resultCode"]))
	amount := asInt64(data["amount"])
	cur := "VND"
	event = CommerceWebhookEventJSON{
		Provider:               p.Key(),
		ProviderReference:      orderID,
		WebhookEventID:         fields.RequestID,
		EventType:              "momo.ipn",
		NormalizedPaymentState: status,
		PayloadJSON:            json.RawMessage(raw),
		ProviderAmountMinor:    &amount,
		Currency:               &cur,
	}
	return orderID, status, transID, event, nil
}

// ParseMoMoIPN is an exported helper alias for native IPN handlers.
func (p *MoMoProvider) ParseMoMoIPN(raw []byte) (CommerceWebhookEventJSON, error) {
	_, _, _, event, err := p.VerifyAndParseIPN(raw)
	return event, err
}
