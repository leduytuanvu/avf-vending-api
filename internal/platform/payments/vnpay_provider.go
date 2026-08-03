package payments

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/platform/payments/psp/httpx"
	"github.com/avf/avf-vending-api/internal/platform/payments/psp/vnpay"
)

// VNPayProvider is the live VNPay Merchant QR WiredLiveProvider adapter.
type VNPayProvider struct {
	creds  vnpay.Credentials
	wired  bool
	client *httpx.Client
}

// NewVNPayProvider builds a VNPay adapter from config credentials.
func NewVNPayProvider(cfg *config.Config) *VNPayProvider {
	p := &VNPayProvider{client: httpx.New(httpx.DefaultTimeout)}
	if cfg == nil {
		return p
	}
	p.creds = vnpay.Credentials{
		AppID:               cfg.PSP.VNPay.AppID,
		SecretKey:           cfg.PSP.VNPay.SecretKey,
		SecretKeyCheckTrans: cfg.PSP.VNPay.SecretKeyCheckTrans,
		SecretKeyRefund:     cfg.PSP.VNPay.SecretKeyRefund,
		MasterMerCode:       cfg.PSP.VNPay.MasterMerCode,
		MerchantCode:        cfg.PSP.VNPay.MerchantCode,
		TerminalID:          cfg.PSP.VNPay.TerminalID,
		EndpointURL:         cfg.PSP.VNPay.EndpointURL,
		EndpointQueryURL:    cfg.PSP.VNPay.EndpointQueryURL,
		ReturnURL:           cfg.PSP.VNPay.ReturnURL,
	}
	p.wired = cfg.PSP.VNPay.VNPayWired()
	return p
}

func (p *VNPayProvider) Key() string { return "vnpay" }

func (p *VNPayProvider) LivePaymentWired() bool {
	return p != nil && p.wired
}

func (p *VNPayProvider) VerifyWebhookSignature(secret string, tsHeader, sigHeader string, rawBody []byte, skew time.Duration) error {
	return VerifyCommerceWebhookHMAC(secret, tsHeader, sigHeader, rawBody, skew)
}

func (p *VNPayProvider) ParseWebhookEvent(rawBody []byte) (CommerceWebhookEventJSON, error) {
	return ParseCommerceWebhookEventJSON(rawBody)
}

func (p *VNPayProvider) SupportsQueryPaymentStatus() bool {
	return p.LivePaymentWired()
}

func (p *VNPayProvider) CreatePaymentSession(ctx context.Context, in CreatePaymentSessionInput) (CreatePaymentSessionResult, error) {
	if !p.LivePaymentWired() {
		return CreatePaymentSessionResult{}, fmt.Errorf("%w for provider %q", ErrLiveProviderNotWired, p.Key())
	}
	providerRef := resolveProviderRef(in)
	terminalID := strings.TrimSpace(in.StoreID)
	if terminalID == "" {
		terminalID = strings.TrimSpace(p.creds.TerminalID)
	}
	now := time.Now().UTC()
	// Preserve legacy Python strftime("%y%m%d%H%m") quirk (hour then month again).
	expDate := now.Format("06010215") + now.Format("01")
	amount := fmt.Sprintf("%d", in.AmountMinor)
	checksum := vnpay.CreateChecksum(vnpay.CreateChecksumFields{
		AppID:         p.creds.AppID,
		MasterMerCode: p.creds.MasterMerCode,
		MerchantCode:  p.creds.MerchantCode,
		TerminalID:    terminalID,
		ProductID:     "",
		TxnID:         providerRef,
		Amount:        amount,
		TipAndFee:     "",
		ExpDate:       expDate,
		Secret:        p.creds.SecretKey,
	})
	payload := map[string]any{
		"appId":         p.creds.AppID,
		"merchantName":  vnpay.MerchantName,
		"serviceCode":   vnpay.ServiceCode,
		"payType":       vnpay.PayType,
		"ccy":           vnpay.CCY,
		"merchantType":  vnpay.MerchantType,
		"masterMerCode": p.creds.MasterMerCode,
		"countryCode":   vnpay.CountryCode,
		"merchantCode":  p.creds.MerchantCode,
		"payloadFormat": "",
		"terminalId":    terminalID,
		"productId":     "",
		"productName":   "",
		"imageName":     "",
		"txnId":         providerRef,
		"amount":        amount,
		"tipAndFee":     "",
		"expDate":       expDate,
		"desc":          "",
		"merchantCity":  "",
		"merchantCC":    "",
		"fixedFee":      "",
		"percentageFee": "",
		"pinCode":       "",
		"billNumber":    providerRef,
		"creator":       "",
		"consumerID":    "",
		"purpose":       "",
		"checksum":      checksum,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return CreatePaymentSessionResult{}, err
	}
	endpoint := joinEndpoint(p.creds.EndpointURL, "/QRCreateAPIRestV2/rest/CreateQrcodeApi/createQrcode")
	respBody, status, err := p.client.PostRaw(ctx, endpoint, "text/plain", nil, body)
	if err != nil {
		return CreatePaymentSessionResult{}, fmt.Errorf("vnpay create: %w", err)
	}
	res, err := decodeJSONMap(respBody)
	if err != nil {
		return CreatePaymentSessionResult{}, fmt.Errorf("vnpay create decode: %w (http %d)", err, status)
	}
	if asString(res["code"]) != "00" {
		return CreatePaymentSessionResult{}, fmt.Errorf("vnpay create failed: code=%v message=%s", res["code"], asString(res["message"]))
	}
	qr := asString(res["data"])
	display, _ := json.Marshal(map[string]any{
		"provider":           p.Key(),
		"provider_reference": providerRef,
		"qr_payload":         qr,
		"code":               res["code"],
		"terminal_id":        terminalID,
	})
	return CreatePaymentSessionResult{
		ProviderReference:   providerRef,
		ProviderSessionID:   providerRef,
		QRPayloadOrURL:      qr,
		PaymentURL:          qr,
		CheckoutURL:         qr,
		ProviderDisplayJSON: display,
	}, nil
}

func (p *VNPayProvider) QueryPaymentStatus(ctx context.Context, lookup domaincommerce.PaymentProviderLookup) (domaincommerce.PaymentStatusSnapshot, error) {
	if !p.LivePaymentWired() {
		return domaincommerce.PaymentStatusSnapshot{}, ErrQueryPaymentStatusNotSupported
	}
	txnID := strings.TrimSpace(lookup.ProviderReference)
	if txnID == "" {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("vnpay query: provider_reference required")
	}
	terminalID := strings.TrimSpace(p.creds.TerminalID)
	payDate := time.Now().UTC().Format("02/01/2006")
	checksum := vnpay.QueryChecksum(payDate, txnID, p.creds.MerchantCode, terminalID, p.creds.SecretKeyCheckTrans)
	payload := map[string]any{
		"txnId":        txnID,
		"payDate":      payDate,
		"merchantCode": p.creds.MerchantCode,
		"terminalID":   terminalID,
		"checkSum":     checksum,
	}
	endpoint := joinEndpoint(p.creds.EndpointQueryURL, "/CheckTransaction/rest/api/CheckTrans")
	respBody, status, err := p.client.PostJSON(ctx, endpoint, nil, payload)
	if err != nil {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("vnpay query: %w", err)
	}
	res, err := decodeJSONMap(respBody)
	if err != nil {
		return domaincommerce.PaymentStatusSnapshot{}, fmt.Errorf("vnpay query decode: %w (http %d)", err, status)
	}
	code := asString(res["code"])
	norm, _ := vnpay.MapCode(code)
	// Legacy DB treated "08" as success; MapCode maps only "00" → captured (machine return_code).
	if code == "08" {
		norm = vnpay.StatusCaptured
	}
	hint, _ := json.Marshal(res)
	return domaincommerce.PaymentStatusSnapshot{
		NormalizedState: norm,
		ProviderHint:    hint,
	}, nil
}

func (p *VNPayProvider) CancelPayment(ctx context.Context, in CancelPaymentInput) error {
	_ = ctx
	_ = in
	return ErrNotImplemented
}

func (p *VNPayProvider) RefundPayment(ctx context.Context, in RefundPaymentInput) error {
	_ = ctx
	_ = in
	// Legacy refund used a hardcoded qrTrace; do not expose that as production behavior.
	return ErrNotImplemented
}

// ParseReturnQuery maps VNPay return-URL query params to a CommerceWebhookEventJSON (best-effort).
func (p *VNPayProvider) ParseReturnQuery(q url.Values) (CommerceWebhookEventJSON, error) {
	if q == nil {
		return CommerceWebhookEventJSON{}, fmt.Errorf("vnpay return: empty query")
	}
	txnRef := strings.TrimSpace(q.Get("vnp_TxnRef"))
	respCode := strings.TrimSpace(q.Get("vnp_ResponseCode"))
	transNo := strings.TrimSpace(q.Get("vnp_TransactionNo"))
	if txnRef == "" {
		return CommerceWebhookEventJSON{}, fmt.Errorf("vnpay return: missing vnp_TxnRef")
	}
	norm, _ := vnpay.MapCode(respCode)
	payload, _ := json.Marshal(map[string]string{
		"vnp_TxnRef":        txnRef,
		"vnp_ResponseCode":  respCode,
		"vnp_TransactionNo": transNo,
	})
	cur := "VND"
	return CommerceWebhookEventJSON{
		Provider:               p.Key(),
		ProviderReference:      txnRef,
		WebhookEventID:         transNo,
		EventType:              "vnpay.return",
		NormalizedPaymentState: norm,
		PayloadJSON:            payload,
		Currency:               &cur,
	}, nil
}
