package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/platform/payments/psp/httpx"
	"github.com/avf/avf-vending-api/internal/platform/payments/psp/ref"
	"github.com/avf/avf-vending-api/internal/platform/payments/psp/shopeepay"
	"github.com/google/uuid"
)

const (
	shopeeQRCreatePath     = "/v3/merchant-host/qr/create"
	shopeeTxnCheckPath     = "/v3/merchant-host/transaction/check"
	shopeeRefundCreatePath = "/v3/merchant-host/transaction/refund/create-new"
	shopeeQRValidityPeriod = 1200
	shopeeTxnTypePayment   = 13
	shopeeCurrencyVND      = "VND"
)

// ShopeePayProvider is the live ShopeePay MPM WiredLiveProvider adapter.
type ShopeePayProvider struct {
	keys      shopeepay.TenantKeys
	wired     bool
	whitelist map[string]struct{}
	client    *httpx.Client
}

// NewShopeePayProvider builds a ShopeePay adapter from config credentials.
func NewShopeePayProvider(cfg *config.Config) *ShopeePayProvider {
	p := &ShopeePayProvider{
		client:    httpx.New(httpx.DefaultTimeout),
		whitelist: map[string]struct{}{},
	}
	if cfg == nil {
		return p
	}
	p.keys = shopeepay.TenantKeys{
		AVF: shopeeCredentialsFromConfig(cfg.PSP.ShopeePay.AVF),
		TFO: shopeeCredentialsFromConfig(cfg.PSP.ShopeePay.TFO),
	}
	p.wired = cfg.PSP.ShopeePay.ShopeePayWired()
	p.whitelist = shopeepay.ParseWhitelist(strings.Join(cfg.PSP.ShopeePay.CallbackIPWhitelist, ","))
	return p
}

func shopeeCredentialsFromConfig(t config.ShopeePayTenantCredentials) shopeepay.Credentials {
	return shopeepay.Credentials{
		ClientID:      t.ClientID,
		SecretKey:     t.SecretKey,
		Endpoint:      t.Endpoint,
		MerchantExtID: t.MerchantExtID,
		StoreExtID:    t.StoreExtID,
		CallbackURL:   t.CallbackURL,
		TerminalID:    t.TerminalID,
	}
}

func (p *ShopeePayProvider) Key() string { return "shopeepay" }

func (p *ShopeePayProvider) LivePaymentWired() bool {
	return p != nil && p.wired
}

func (p *ShopeePayProvider) VerifyWebhookSignature(secret string, tsHeader, sigHeader string, rawBody []byte, skew time.Duration) error {
	return VerifyCommerceWebhookHMAC(secret, tsHeader, sigHeader, rawBody, skew)
}

func (p *ShopeePayProvider) ParseWebhookEvent(rawBody []byte) (CommerceWebhookEventJSON, error) {
	return ParseCommerceWebhookEventJSON(rawBody)
}

func (p *ShopeePayProvider) SupportsQueryPaymentStatus() bool {
	return p.LivePaymentWired()
}

func (p *ShopeePayProvider) credsForMachine(machineCode string) shopeepay.Credentials {
	return p.keys.ForTenant(shopeepay.ResolveTenant(machineCode))
}

func (p *ShopeePayProvider) CreatePaymentSession(ctx context.Context, in CreatePaymentSessionInput) (CreatePaymentSessionResult, error) {
	if !p.LivePaymentWired() {
		return CreatePaymentSessionResult{}, fmt.Errorf("%w for provider %q", ErrLiveProviderNotWired, p.Key())
	}
	creds := p.credsForMachine(in.MachineExternalCode)
	providerRef := resolveProviderRef(in)
	if err := ref.Validate(providerRef); err != nil {
		return CreatePaymentSessionResult{}, fmt.Errorf("shopeepay: %w", err)
	}
	if len(providerRef) > shopeepay.PaymentReferenceIDMaxLength {
		return CreatePaymentSessionResult{}, fmt.Errorf("shopeepay: provider_reference exceeds %d chars", shopeepay.PaymentReferenceIDMaxLength)
	}
	terminalID := strings.TrimSpace(in.StoreID)
	if terminalID == "" {
		terminalID = strings.TrimSpace(creds.TerminalID)
	}
	requestID := uuid.NewString()
	payload := map[string]any{
		"request_id":           requestID,
		"amount":               shopeepay.ToAPIAmount(in.AmountMinor),
		"currency":             shopeeCurrencyVND,
		"merchant_ext_id":      creds.MerchantExtID,
		"store_ext_id":         creds.StoreExtID,
		"payment_reference_id": providerRef,
		"qr_validity_period":   shopeeQRValidityPeriod,
	}
	if terminalID != "" {
		payload["terminal_id"] = terminalID
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return CreatePaymentSessionResult{}, err
	}
	headers := shopeepay.BuildHeaders(creds.ClientID, creds.SecretKey, body)
	endpoint := joinEndpoint(creds.Endpoint, shopeeQRCreatePath)
	respBody, status, err := p.client.PostJSONBytes(ctx, endpoint, headers, body)
	if err != nil {
		return CreatePaymentSessionResult{}, fmt.Errorf("shopeepay create: %w", err)
	}
	res, err := decodeJSONMap(respBody)
	if err != nil {
		return CreatePaymentSessionResult{}, fmt.Errorf("shopeepay create decode: %w (http %d)", err, status)
	}
	if asInt(res["errcode"]) != 0 {
		return CreatePaymentSessionResult{}, fmt.Errorf("shopeepay create failed: errcode=%v debug_msg=%s", res["errcode"], asString(res["debug_msg"]))
	}
	qr := asString(res["qr_url"])
	if qr == "" {
		qr = asString(res["qr_content"])
	}
	display, _ := json.Marshal(map[string]any{
		"provider":           p.Key(),
		"provider_reference": providerRef,
		"request_id":         requestID,
		"qr_url":             qr,
		"errcode":            res["errcode"],
	})
	exp := time.Now().UTC().Add(time.Duration(shopeeQRValidityPeriod) * time.Second)
	return CreatePaymentSessionResult{
		ProviderReference:   providerRef,
		ProviderSessionID:   requestID,
		QRPayloadOrURL:      qr,
		PaymentURL:          qr,
		CheckoutURL:         qr,
		ExpiresAt:           &exp,
		ProviderDisplayJSON: display,
	}, nil
}

func (p *ShopeePayProvider) QueryPaymentStatus(ctx context.Context, lookup domaincommerce.PaymentProviderLookup) (domaincommerce.PaymentStatusSnapshot, error) {
	if !p.LivePaymentWired() {
		return domaincommerce.PaymentStatusSnapshot{}, ErrQueryPaymentStatusNotSupported
	}
	orderCode := strings.TrimSpace(lookup.ProviderReference)
	if orderCode == "" {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("shopeepay query: provider_reference required")
	}
	creds := p.keys.AVF
	if strings.TrimSpace(creds.ClientID) == "" {
		creds = p.keys.TFO
	}
	amount := lookup.AmountMinor
	payload := map[string]any{
		"request_id":       uuid.NewString(),
		"reference_id":     orderCode,
		"transaction_type": shopeeTxnTypePayment,
		"merchant_ext_id":  creds.MerchantExtID,
		"store_ext_id":     creds.StoreExtID,
		"amount":           shopeepay.ToAPIAmount(amount),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return domaincommerce.PaymentStatusSnapshot{}, err
	}
	headers := shopeepay.BuildHeaders(creds.ClientID, creds.SecretKey, body)
	endpoint := joinEndpoint(creds.Endpoint, shopeeTxnCheckPath)
	respBody, status, err := p.client.PostJSONBytes(ctx, endpoint, headers, body)
	if err != nil {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("shopeepay query: %w", err)
	}
	res, err := decodeJSONMap(respBody)
	if err != nil {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("shopeepay query decode: %w (http %d)", err, status)
	}
	tx, _ := res["transaction"].(map[string]any)
	txStatus := 0
	if tx != nil {
		txStatus = asInt(tx["status"])
	}
	norm, _ := shopeepay.MapTxStatus(txStatus)
	hint, _ := json.Marshal(res)
	return domaincommerce.PaymentStatusSnapshot{
		NormalizedState: norm,
		ProviderHint:    hint,
	}, nil
}

func (p *ShopeePayProvider) CancelPayment(ctx context.Context, in CancelPaymentInput) error {
	_ = ctx
	_ = in
	return ErrNotImplemented
}

func (p *ShopeePayProvider) RefundPayment(ctx context.Context, in RefundPaymentInput) error {
	if !p.LivePaymentWired() {
		return fmt.Errorf("%w for provider %q", ErrLiveProviderNotWired, p.Key())
	}
	orderCode := strings.TrimSpace(in.ProviderReference)
	if orderCode == "" {
		return fmt.Errorf("shopeepay refund: provider_reference required")
	}
	creds := p.keys.AVF
	if strings.TrimSpace(creds.ClientID) == "" {
		creds = p.keys.TFO
	}
	refundRef := strings.TrimSpace(in.IdempotencyKey)
	if refundRef == "" {
		refundRef = uuid.NewString()
	}
	if len(refundRef) > shopeepay.PaymentReferenceIDMaxLength {
		refundRef = refundRef[:shopeepay.PaymentReferenceIDMaxLength]
	}
	payload := map[string]any{
		"request_id":           uuid.NewString(),
		"reference_id":         orderCode,
		"transaction_type":     shopeeTxnTypePayment,
		"refund_reference_id":  refundRef,
		"merchant_ext_id":      creds.MerchantExtID,
		"store_ext_id":         creds.StoreExtID,
		"amount":               shopeepay.ToAPIAmount(in.AmountMinor),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	headers := shopeepay.BuildHeaders(creds.ClientID, creds.SecretKey, body)
	endpoint := joinEndpoint(creds.Endpoint, shopeeRefundCreatePath)
	respBody, status, err := p.client.PostJSONBytes(ctx, endpoint, headers, body)
	if err != nil {
		return fmt.Errorf("shopeepay refund: %w", err)
	}
	res, err := decodeJSONMap(respBody)
	if err != nil {
		return fmt.Errorf("shopeepay refund decode: %w (http %d)", err, status)
	}
	if asInt(res["errcode"]) != 0 {
		return fmt.Errorf("shopeepay refund failed: errcode=%v debug_msg=%s", res["errcode"], asString(res["debug_msg"]))
	}
	return nil
}

// SoftACKJSON returns the soft-ACK body for ShopeePay HTTP handlers.
func (p *ShopeePayProvider) SoftACKJSON() []byte {
	return shopeepay.SoftACKJSON()
}

// SoftACKMap returns the soft-ACK payload as a map.
func (p *ShopeePayProvider) SoftACKMap() map[string]any {
	return shopeepay.SoftACKMap()
}

// VerifyAndParseCallback verifies IP whitelist + body signature and maps to CommerceWebhookEventJSON.
func (p *ShopeePayProvider) VerifyAndParseCallback(raw []byte, clientID, signature, clientIP string) (orderID, status, transSN string, event CommerceWebhookEventJSON, err error) {
	if !shopeepay.ClientIPAllowed(clientIP, p.whitelist) {
		return "", "", "", CommerceWebhookEventJSON{}, fmt.Errorf("shopeepay callback: forbidden ip")
	}
	if strings.TrimSpace(clientID) == "" {
		return "", "", "", CommerceWebhookEventJSON{}, fmt.Errorf("shopeepay callback: missing client id")
	}
	if strings.TrimSpace(signature) == "" {
		return "", "", "", CommerceWebhookEventJSON{}, fmt.Errorf("shopeepay callback: missing signature")
	}
	creds, ok := p.credsByClientID(clientID)
	if !ok {
		return "", "", "", CommerceWebhookEventJSON{}, fmt.Errorf("shopeepay callback: unknown client id")
	}
	if !shopeepay.VerifySignature(creds.SecretKey, raw, signature) {
		return "", "", "", CommerceWebhookEventJSON{}, fmt.Errorf("shopeepay callback: invalid signature")
	}
	var data map[string]any
	if err = json.Unmarshal(raw, &data); err != nil {
		return "", "", "", CommerceWebhookEventJSON{}, err
	}
	if mid := asString(data["merchant_ext_id"]); mid != "" {
		if c, ok := p.credsByMerchantExtID(mid); ok {
			creds = c
			_ = creds
		}
	}
	orderID = asString(data["reference_id"])
	if orderID == "" {
		orderID = asString(data["payment_reference_id"])
	}
	txStatus := asInt(data["transaction_status"])
	if _, has := data["transaction_status"]; !has {
		if asInt(data["payment_status"]) == 1 {
			txStatus = shopeepay.TXStatusSuccess
		}
	}
	status, _ = shopeepay.MapTxStatus(txStatus)
	transSN = asString(data["transaction_sn"])
	apiAmount := asInt64(data["amount"])
	vnd := shopeepay.FromAPIAmount(apiAmount)
	cur := shopeeCurrencyVND
	event = CommerceWebhookEventJSON{
		Provider:               p.Key(),
		ProviderReference:      orderID,
		WebhookEventID:         transSN,
		EventType:              "shopeepay.callback",
		NormalizedPaymentState: status,
		PayloadJSON:            json.RawMessage(raw),
		ProviderAmountMinor:    &vnd,
		Currency:               &cur,
	}
	return orderID, status, transSN, event, nil
}

func (p *ShopeePayProvider) credsByClientID(clientID string) (shopeepay.Credentials, bool) {
	clientID = strings.TrimSpace(clientID)
	if clientID != "" && clientID == p.keys.AVF.ClientID {
		return p.keys.AVF, true
	}
	if clientID != "" && clientID == p.keys.TFO.ClientID {
		return p.keys.TFO, true
	}
	return shopeepay.Credentials{}, false
}

func (p *ShopeePayProvider) credsByMerchantExtID(merchantExtID string) (shopeepay.Credentials, bool) {
	merchantExtID = strings.TrimSpace(merchantExtID)
	if merchantExtID != "" && merchantExtID == p.keys.AVF.MerchantExtID {
		return p.keys.AVF, true
	}
	if merchantExtID != "" && merchantExtID == p.keys.TFO.MerchantExtID {
		return p.keys.TFO, true
	}
	return shopeepay.Credentials{}, false
}
