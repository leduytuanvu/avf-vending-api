package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/avf/avf-vending-api/internal/platform/payments/psp/httpx"
	"github.com/avf/avf-vending-api/internal/platform/payments/psp/ref"
	"github.com/avf/avf-vending-api/internal/platform/payments/psp/shopeepay"
)

const (
	shopeeQRValidityPeriod = 1200
	shopeeTxnTypePayment   = 13
	shopeeCurrencyVND      = "VND"
)

// ShopeePayProvider is the live ShopeePay MPM WiredLiveProvider adapter.
type ShopeePayProvider struct {
	keys           shopeepay.TenantKeys
	wired          bool
	production     bool
	strict         bool
	trustProxy     bool
	whitelist      map[string]struct{}
	storeAllowlist map[string]struct{}
	client         *httpx.Client
}

// NewShopeePayProvider builds a ShopeePay adapter from config credentials.
func NewShopeePayProvider(cfg *config.Config) *ShopeePayProvider {
	p := &ShopeePayProvider{
		client:         httpx.New(httpx.DefaultTimeout),
		whitelist:      map[string]struct{}{},
		storeAllowlist: map[string]struct{}{},
	}
	if p.client != nil && p.client.HTTP != nil {
		p.client.HTTP.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	if cfg == nil {
		return p
	}
	p.keys = shopeepay.TenantKeys{
		AVF: shopeeCredentialsFromConfig(cfg.PSP.ShopeePay.AVF),
		TFO: shopeeCredentialsFromConfig(cfg.PSP.ShopeePay.TFO),
	}
	p.wired = cfg.PSP.ShopeePay.ShopeePayWired()
	p.production = cfg.AppEnv == config.AppEnvProduction
	p.strict = cfg.AppEnv == config.AppEnvProduction || cfg.AppEnv == config.AppEnvStaging
	p.trustProxy = cfg.PSP.ShopeePay.TrustProxy
	p.whitelist = shopeepay.ParseWhitelist(strings.Join(cfg.PSP.ShopeePay.CallbackIPWhitelist, ","))
	p.storeAllowlist = shopeepay.MergeAllowlist(
		strings.Join(cfg.PSP.ShopeePay.StoreExtIDAllowlist, ","),
		cfg.PSP.ShopeePay.StoreExtIDAllowlistText,
	)
	if len(p.storeAllowlist) == 0 && p.strict {
		p.storeAllowlist = shopeepay.ParseAllowlistLines(shopeepay.DefaultStoreAllowlistFile)
	}
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

func (p *ShopeePayProvider) TrustProxy() bool {
	return p != nil && p.trustProxy
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

func (p *ShopeePayProvider) resolveStoreExtID(machineCode string, creds shopeepay.Credentials) string {
	return shopeepay.ResolveStoreExtID(machineCode, creds.StoreExtID)
}

func (p *ShopeePayProvider) ensureStoreAllowed(storeExtID string) error {
	if shopeepay.StoreAllowed(storeExtID, p.storeAllowlist, p.strict) {
		return nil
	}
	return fmt.Errorf("shopeepay: %w", shopeepay.ErrStoreNotAllowlisted)
}

func (p *ShopeePayProvider) postShopee(ctx context.Context, creds shopeepay.Credentials, path string, payload map[string]any) (map[string]any, error) {
	if err := shopeepay.ValidateEndpoint(creds.Endpoint, p.production); err != nil {
		return nil, fmt.Errorf("shopeepay: %w", err)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	headers := shopeepay.BuildHeaders(creds.ClientID, creds.SecretKey, body)
	res, err := p.client.PostJSONBytesResult(ctx, joinEndpoint(creds.Endpoint, path), headers, body)
	if err != nil {
		return nil, err
	}
	if res.Status >= 300 && res.Status < 400 {
		return nil, fmt.Errorf("shopeepay: %w", shopeepay.ErrRedirectBlocked)
	}
	if res.Status >= 400 {
		return nil, fmt.Errorf("shopeepay: http %d", res.Status)
	}
	respSig := ""
	if res.Header != nil {
		respSig = strings.TrimSpace(res.Header.Get(shopeepay.HeaderRespH))
	}
	if !shopeepay.VerifySignature(creds.SecretKey, res.Body, respSig) {
		return nil, fmt.Errorf("shopeepay: %w", shopeepay.ErrInvalidResponseSignature)
	}
	decoded, err := decodeJSONMap(res.Body)
	if err != nil {
		return nil, fmt.Errorf("shopeepay decode: %w (http %d)", err, res.Status)
	}
	return decoded, nil
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
	storeExtID := p.resolveStoreExtID(in.MachineExternalCode, creds)
	if err := p.ensureStoreAllowed(storeExtID); err != nil {
		return CreatePaymentSessionResult{}, err
	}
	terminalID := strings.TrimSpace(in.StoreID)
	if terminalID == "" {
		terminalID = strings.TrimSpace(creds.TerminalID)
	}
	requestID := id.NewUUIDV7String()
	payload := map[string]any{
		"request_id":           requestID,
		"amount":               shopeepay.ToAPIAmount(in.AmountMinor),
		"currency":             shopeeCurrencyVND,
		"merchant_ext_id":      creds.MerchantExtID,
		"store_ext_id":         storeExtID,
		"payment_reference_id": providerRef,
		"qr_validity_period":   shopeeQRValidityPeriod,
	}
	if terminalID != "" {
		payload["terminal_id"] = terminalID
	}
	res, err := p.postShopee(ctx, creds, shopeepay.QRCreatePath, payload)
	if err != nil {
		return CreatePaymentSessionResult{}, fmt.Errorf("shopeepay create: %w", err)
	}
	if asInt(res["errcode"]) != 0 {
		return CreatePaymentSessionResult{}, fmt.Errorf("shopeepay create failed: errcode=%v debug_msg=%s", res["errcode"], asString(res["debug_msg"]))
	}
	qr := asString(res["qr_content"])
	if qr == "" {
		qr = asString(res["qr_url"])
	}
	if qr == "" {
		return CreatePaymentSessionResult{}, fmt.Errorf("shopeepay create failed: empty qr payload")
	}
	display, _ := json.Marshal(map[string]any{
		"provider":           p.Key(),
		"provider_reference": providerRef,
		"request_id":         requestID,
		"qr_content":         asString(res["qr_content"]),
		"qr_url":             asString(res["qr_url"]),
		"store_ext_id":       storeExtID,
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
	creds := p.credsForMachine(lookup.MachineExternalCode)
	storeExtID := p.resolveStoreExtID(lookup.MachineExternalCode, creds)
	amount := lookup.AmountMinor
	payload := map[string]any{
		"request_id":       id.NewUUIDV7String(),
		"reference_id":     orderCode,
		"transaction_type": shopeeTxnTypePayment,
		"merchant_ext_id":  creds.MerchantExtID,
		"store_ext_id":     storeExtID,
		"amount":           shopeepay.ToAPIAmount(amount),
	}
	res, err := p.postShopee(ctx, creds, shopeepay.TxnCheckPath, payload)
	if err != nil {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("shopeepay query: %w", err)
	}
	tx, _ := res["transaction"].(map[string]any)
	txStatus := 0
	if tx != nil {
		txStatus = asInt(tx["status"])
	}
	norm, _ := shopeepay.MapTxStatus(txStatus)
	if norm == shopeepay.StatusFailed {
		p.invalidateQR(ctx, creds, storeExtID, orderCode)
	}
	hint, _ := json.Marshal(res)
	return domaincommerce.PaymentStatusSnapshot{
		NormalizedState: norm,
		ProviderHint:    hint,
	}, nil
}

func (p *ShopeePayProvider) CancelPayment(ctx context.Context, in CancelPaymentInput) error {
	if !p.LivePaymentWired() {
		return fmt.Errorf("%w for provider %q", ErrLiveProviderNotWired, p.Key())
	}
	orderCode := strings.TrimSpace(in.ProviderReference)
	if orderCode == "" {
		return fmt.Errorf("shopeepay cancel: provider_reference required")
	}
	creds := p.credsForMachine(in.MachineExternalCode)
	storeExtID := p.resolveStoreExtID(in.MachineExternalCode, creds)
	return p.invalidateQR(ctx, creds, storeExtID, orderCode)
}

func (p *ShopeePayProvider) invalidateQR(ctx context.Context, creds shopeepay.Credentials, storeExtID, orderCode string) error {
	payload := map[string]any{
		"request_id":           id.NewUUIDV7String(),
		"payment_reference_id": orderCode,
		"merchant_ext_id":      creds.MerchantExtID,
		"store_ext_id":         storeExtID,
	}
	res, err := p.postShopee(ctx, creds, shopeepay.QRInvalidatePath, payload)
	if err != nil {
		return fmt.Errorf("shopeepay invalidate: %w", err)
	}
	if asInt(res["errcode"]) != 0 {
		return fmt.Errorf("shopeepay invalidate failed: errcode=%v debug_msg=%s", res["errcode"], asString(res["debug_msg"]))
	}
	return nil
}

func (p *ShopeePayProvider) RefundPayment(ctx context.Context, in RefundPaymentInput) error {
	if !p.LivePaymentWired() {
		return fmt.Errorf("%w for provider %q", ErrLiveProviderNotWired, p.Key())
	}
	orderCode := strings.TrimSpace(in.ProviderReference)
	if orderCode == "" {
		return fmt.Errorf("shopeepay refund: provider_reference required")
	}
	creds := p.credsForMachine(in.MachineExternalCode)
	storeExtID := p.resolveStoreExtID(in.MachineExternalCode, creds)
	refundRef := strings.TrimSpace(in.IdempotencyKey)
	if refundRef == "" {
		refundRef = id.NewUUIDV7String()
	}
	if len(refundRef) > shopeepay.PaymentReferenceIDMaxLength {
		refundRef = refundRef[:shopeepay.PaymentReferenceIDMaxLength]
	}
	payload := map[string]any{
		"request_id":          id.NewUUIDV7String(),
		"reference_id":        orderCode,
		"transaction_type":    shopeeTxnTypePayment,
		"refund_reference_id": refundRef,
		"merchant_ext_id":     creds.MerchantExtID,
		"store_ext_id":        storeExtID,
		"amount":              shopeepay.ToAPIAmount(in.AmountMinor),
	}
	res, err := p.postShopee(ctx, creds, shopeepay.RefundCreatePath, payload)
	if err != nil {
		return fmt.Errorf("shopeepay refund: %w", err)
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
	if !shopeepay.ClientIPPolicy(clientIP, p.whitelist, p.strict) {
		return "", "", "", CommerceWebhookEventJSON{}, shopeepay.ErrCallbackForbiddenIP
	}
	if strings.TrimSpace(clientID) == "" {
		return "", "", "", CommerceWebhookEventJSON{}, shopeepay.ErrCallbackMissingClientID
	}
	if strings.TrimSpace(signature) == "" {
		return "", "", "", CommerceWebhookEventJSON{}, shopeepay.ErrCallbackMissingSignature
	}
	creds, ok := p.credsByClientID(clientID)
	if !ok {
		return "", "", "", CommerceWebhookEventJSON{}, shopeepay.ErrCallbackUnknownClientID
	}
	if !shopeepay.VerifySignature(creds.SecretKey, raw, signature) {
		return "", "", "", CommerceWebhookEventJSON{}, shopeepay.ErrCallbackInvalidSignature
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

// ShopeeCallbackHTTPStatus maps native callback verify errors to HTTP status + debug_msg.
func ShopeeCallbackHTTPStatus(err error) (status int, message string) {
	switch {
	case err == nil:
		return http.StatusOK, "success"
	case errors.Is(err, shopeepay.ErrCallbackForbiddenIP):
		return http.StatusForbidden, err.Error()
	case errors.Is(err, shopeepay.ErrCallbackMissingClientID),
		errors.Is(err, shopeepay.ErrCallbackMissingSignature),
		errors.Is(err, shopeepay.ErrCallbackUnknownClientID),
		errors.Is(err, shopeepay.ErrCallbackInvalidSignature):
		return http.StatusUnauthorized, err.Error()
	default:
		return http.StatusBadRequest, err.Error()
	}
}
