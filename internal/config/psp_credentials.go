package config

import (
	"os"
	"strings"
)

// MoMoTenantCredentials is one MoMo partner credential set (AVF or TFO).
type MoMoTenantCredentials struct {
	PartnerCode string
	AccessKey   string
	SecretKey   string
	Endpoint    string
	RedirectURL string
	IPNURL      string
	TerminalID  string
}

// MoMoConfig holds AVF + TFO MoMo credentials.
type MoMoConfig struct {
	AVF MoMoTenantCredentials
	TFO MoMoTenantCredentials
}

// ZaloPayConfig holds global ZaloPay credentials (no AVF/TFO split).
type ZaloPayConfig struct {
	AppID       string
	Key1        string
	Key2        string
	Endpoint    string
	CallbackURL string
}

// VNPayConfig holds VNPay Merchant QR credentials.
type VNPayConfig struct {
	AppID               string
	SecretKey           string
	SecretKeyCheckTrans string
	SecretKeyRefund     string
	MasterMerCode       string
	MerchantCode        string
	TerminalID          string
	EndpointURL         string
	EndpointQueryURL    string
	ReturnURL           string
}

// ShopeePayTenantCredentials is one ShopeePay merchant set.
type ShopeePayTenantCredentials struct {
	ClientID      string
	SecretKey     string
	Endpoint      string
	MerchantExtID string
	StoreExtID    string
	CallbackURL   string
	TerminalID    string
}

// ShopeePayConfig holds AVF + TFO ShopeePay credentials and callback IP whitelist.
type ShopeePayConfig struct {
	AVF                ShopeePayTenantCredentials
	TFO                ShopeePayTenantCredentials
	CallbackIPWhitelist []string
}

// PSPCredentials aggregates all live payment gateway credentials.
type PSPCredentials struct {
	MoMo      MoMoConfig
	ZaloPay   ZaloPayConfig
	VNPay     VNPayConfig
	ShopeePay ShopeePayConfig
}

// MoMoWired reports whether at least AVF MoMo credentials are complete enough for outbound create.
func (c MoMoConfig) MoMoWired() bool {
	return momoTenantWired(c.AVF) || momoTenantWired(c.TFO)
}

func momoTenantWired(t MoMoTenantCredentials) bool {
	return strings.TrimSpace(t.PartnerCode) != "" &&
		strings.TrimSpace(t.AccessKey) != "" &&
		strings.TrimSpace(t.SecretKey) != "" &&
		strings.TrimSpace(t.Endpoint) != ""
}

// ZaloPayWired reports whether ZaloPay credentials are complete.
func (c ZaloPayConfig) ZaloPayWired() bool {
	return strings.TrimSpace(c.AppID) != "" &&
		strings.TrimSpace(c.Key1) != "" &&
		strings.TrimSpace(c.Key2) != "" &&
		strings.TrimSpace(c.Endpoint) != ""
}

// VNPayWired reports whether VNPay QR credentials are complete.
func (c VNPayConfig) VNPayWired() bool {
	return strings.TrimSpace(c.AppID) != "" &&
		strings.TrimSpace(c.SecretKey) != "" &&
		strings.TrimSpace(c.MerchantCode) != "" &&
		strings.TrimSpace(c.EndpointURL) != "" &&
		strings.TrimSpace(c.EndpointQueryURL) != ""
}

// ShopeePayWired reports whether at least one tenant has complete ShopeePay credentials.
func (c ShopeePayConfig) ShopeePayWired() bool {
	return shopeeTenantWired(c.AVF) || shopeeTenantWired(c.TFO)
}

func shopeeTenantWired(t ShopeePayTenantCredentials) bool {
	return strings.TrimSpace(t.ClientID) != "" &&
		strings.TrimSpace(t.SecretKey) != "" &&
		strings.TrimSpace(t.Endpoint) != "" &&
		strings.TrimSpace(t.MerchantExtID) != "" &&
		strings.TrimSpace(t.StoreExtID) != ""
}

func loadPSPCredentials() PSPCredentials {
	return PSPCredentials{
		MoMo: MoMoConfig{
			AVF: MoMoTenantCredentials{
				PartnerCode: strings.TrimSpace(os.Getenv("MOMO_PARTNER_CODE")),
				AccessKey:   strings.TrimSpace(os.Getenv("MOMO_ACCESS_KEY")),
				SecretKey:   strings.TrimSpace(os.Getenv("MOMO_SECRET_KEY")),
				Endpoint:    strings.TrimSpace(getenv("MOMO_END_POINT", "")),
				RedirectURL: strings.TrimSpace(os.Getenv("MOMO_REDIRECT_URL")),
				IPNURL:      strings.TrimSpace(os.Getenv("MOMO_IPN_URL")),
				TerminalID:  strings.TrimSpace(os.Getenv("MOMO_TERMINAL_ID")),
			},
			TFO: MoMoTenantCredentials{
				PartnerCode: strings.TrimSpace(os.Getenv("TFO_MOMO_PARTNER_CODE")),
				AccessKey:   strings.TrimSpace(os.Getenv("TFO_MOMO_ACCESS_KEY")),
				SecretKey:   strings.TrimSpace(os.Getenv("TFO_MOMO_SECRET_KEY")),
				Endpoint:    strings.TrimSpace(getenv("TFO_MOMO_END_POINT", "")),
				RedirectURL: strings.TrimSpace(os.Getenv("TFO_MOMO_REDIRECT_URL")),
				IPNURL:      strings.TrimSpace(os.Getenv("TFO_MOMO_IPN_URL")),
				TerminalID:  strings.TrimSpace(os.Getenv("TFO_MOMO_TERMINAL_ID")),
			},
		},
		ZaloPay: ZaloPayConfig{
			AppID:       strings.TrimSpace(os.Getenv("ZALOPAY_APP_ID")),
			Key1:        strings.TrimSpace(os.Getenv("ZALOPAY_KEY1")),
			Key2:        strings.TrimSpace(os.Getenv("ZALOPAY_KEY2")),
			Endpoint:    strings.TrimSpace(os.Getenv("ZALOPAY_ENDPOINT")),
			CallbackURL: strings.TrimSpace(os.Getenv("ZALOPAY_CALLBACK_URL")),
		},
		VNPay: VNPayConfig{
			AppID:               strings.TrimSpace(os.Getenv("VNP_APP_ID")),
			SecretKey:           strings.TrimSpace(os.Getenv("VNP_SECRET_KEY")),
			SecretKeyCheckTrans: strings.TrimSpace(os.Getenv("VNP_SECRET_KEY_CHECK_TRANS")),
			SecretKeyRefund:     strings.TrimSpace(os.Getenv("VNP_SECRET_KEY_REFUND")),
			MasterMerCode:       strings.TrimSpace(firstNonEmptyTrimmed(os.Getenv("VPN_MASTER_MER_CODE"), os.Getenv("VNP_MASTER_MER_CODE"))),
			MerchantCode:        strings.TrimSpace(firstNonEmptyTrimmed(os.Getenv("VPN_MERCHANT_CODE"), os.Getenv("VNP_MERCHANT_CODE"))),
			TerminalID:          strings.TrimSpace(os.Getenv("VNP_TERMINAL_ID")),
			EndpointURL:         strings.TrimSpace(os.Getenv("VNP_END_POINT_URL")),
			EndpointQueryURL:    strings.TrimSpace(os.Getenv("VNP_END_POINT_QUERY_URL")),
			ReturnURL:           strings.TrimSpace(os.Getenv("VNP_RETURN_URL")),
		},
		ShopeePay: ShopeePayConfig{
			AVF: ShopeePayTenantCredentials{
				ClientID:      strings.TrimSpace(os.Getenv("SHOPEEPAY_CLIENT_ID")),
				SecretKey:     strings.TrimSpace(os.Getenv("SHOPEEPAY_SECRET_KEY")),
				Endpoint:      strings.TrimSpace(os.Getenv("SHOPEEPAY_END_POINT")),
				MerchantExtID: strings.TrimSpace(os.Getenv("SHOPEEPAY_MERCHANT_EXT_ID")),
				StoreExtID:    strings.TrimSpace(os.Getenv("SHOPEEPAY_STORE_EXT_ID")),
				CallbackURL:   strings.TrimSpace(os.Getenv("SHOPEEPAY_CALLBACK_URL")),
				TerminalID:    strings.TrimSpace(os.Getenv("SHOPEEPAY_TERMINAL_ID")),
			},
			TFO: ShopeePayTenantCredentials{
				ClientID:      strings.TrimSpace(os.Getenv("TFO_SHOPEEPAY_CLIENT_ID")),
				SecretKey:     strings.TrimSpace(os.Getenv("TFO_SHOPEEPAY_SECRET_KEY")),
				Endpoint:      strings.TrimSpace(os.Getenv("TFO_SHOPEEPAY_END_POINT")),
				MerchantExtID: strings.TrimSpace(os.Getenv("TFO_SHOPEEPAY_MERCHANT_EXT_ID")),
				StoreExtID:    strings.TrimSpace(os.Getenv("TFO_SHOPEEPAY_STORE_EXT_ID")),
				CallbackURL:   strings.TrimSpace(os.Getenv("TFO_SHOPEEPAY_CALLBACK_URL")),
				TerminalID:    strings.TrimSpace(os.Getenv("TFO_SHOPEEPAY_TERMINAL_ID")),
			},
			CallbackIPWhitelist: splitCSV(os.Getenv("SHOPEEPAY_CALLBACK_IP_WHITELIST")),
		},
	}
}

// ParsePaymentProvidersCSV parses COMMERCE_PAYMENT_PROVIDERS into lowercased unique keys.
func ParsePaymentProvidersCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		k := strings.ToLower(strings.TrimSpace(p))
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

// NormalizePaymentProviderKey maps client aliases to registry keys.
func NormalizePaymentProviderKey(key string) string {
	k := strings.ToLower(strings.TrimSpace(key))
	switch k {
	case "momo_qr", "momo-qr":
		return "momo"
	case "zalo", "zalo_pay", "zalo-pay":
		return "zalopay"
	case "vnpay_qr", "vn_pay":
		return "vnpay"
	case "shopee", "shopee_pay", "shopee-pay":
		return "shopeepay"
	case "viet_qr", "viet-qr":
		return "vietqr"
	default:
		return k
	}
}
