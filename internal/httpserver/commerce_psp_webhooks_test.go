package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/config"
	platformpayments "github.com/avf/avf-vending-api/internal/platform/payments"
)

func TestReconcileNativeWebhookProvider(t *testing.T) {
	t.Parallel()
	cases := []struct {
		event, stored, want string
	}{
		{"zalopay", "vietqr", "vietqr"},
		{"vietqr", "zalopay", "zalopay"},
		{"momo", "momo", "momo"},
		{"", "vietqr", "vietqr"},
		{"zalopay", "", "zalopay"},
		{"momo", "zalopay", "momo"},
	}
	for _, tc := range cases {
		if got := reconcileNativeWebhookProvider(tc.event, tc.stored); got != tc.want {
			t.Fatalf("reconcileNativeWebhookProvider(%q,%q)=%q want %q", tc.event, tc.stored, got, tc.want)
		}
	}
}

func TestShopeePayNativeCallback_forbiddenIPAndInvalidSignature(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.PSP.ShopeePay.AVF.ClientID = "cid"
	cfg.PSP.ShopeePay.AVF.SecretKey = "sk"
	cfg.PSP.ShopeePay.AVF.Endpoint = "https://api.uat.wallet.airpay.vn"
	cfg.PSP.ShopeePay.AVF.MerchantExtID = "AVF_M"
	cfg.PSP.ShopeePay.AVF.StoreExtID = "AVF000079"
	cfg.PSP.ShopeePay.CallbackIPWhitelist = []string{"203.162.56.17"}
	reg := platformpayments.NewRegistry(cfg)
	app := &api.HTTPApplication{PaymentProviders: reg}

	body := `{"reference_id":"AVFref123456789012","transaction_status":3,"amount":500000}`
	req := httptest.NewRequest(http.MethodPost, "/v1/commerce/webhooks/shopeepay", strings.NewReader(body))
	req.RemoteAddr = "1.2.3.4:9999"
	req.Header.Set("X-Airpay-ClientId", "cid")
	req.Header.Set("X-Airpay-Req-H", "deadbeef")
	rec := httptest.NewRecorder()
	ShopeePayNativeCallbackHandler(app, cfg).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("forbidden status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "forbidden ip") {
		t.Fatalf("expected forbidden ip, got %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/commerce/webhooks/shopeepay", strings.NewReader(body))
	req.RemoteAddr = "203.162.56.17:443"
	req.Header.Set("X-Airpay-ClientId", "cid")
	req.Header.Set("X-Airpay-Req-H", "deadbeef")
	rec = httptest.NewRecorder()
	ShopeePayNativeCallbackHandler(app, cfg).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid sig status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestShopeeCallbackClientIP_TrustProxy(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "10.0.0.1:80"
	req.Header.Set("X-Forwarded-For", "203.162.56.17, 10.0.0.1")
	if got := shopeeCallbackClientIP(req, false); got != "10.0.0.1" {
		t.Fatalf("no trust: %q", got)
	}
	if got := shopeeCallbackClientIP(req, true); got != "203.162.56.17" {
		t.Fatalf("trust: %q", got)
	}
}

func TestMoMoNativeIPN_invalidSignature(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.PSP.MoMo.AVF.PartnerCode = "PC"
	cfg.PSP.MoMo.AVF.AccessKey = "ak"
	cfg.PSP.MoMo.AVF.SecretKey = "sk"
	reg := platformpayments.NewRegistry(cfg)
	app := &api.HTTPApplication{
		PaymentProviders: reg,
	}

	body := `{"partnerCode":"PC","orderId":"OID","requestId":"RID","amount":"1000","extraData":"","message":"Success","orderInfo":"i","orderType":"momo_wallet","payType":"qr","responseTime":"1","resultCode":"0","transId":"1","signature":"deadbeef"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/commerce/webhooks/momo", strings.NewReader(body))
	rec := httptest.NewRecorder()
	MoMoNativeIPNHandler(app, cfg).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid signature") && !strings.Contains(rec.Body.String(), "resultCode") {
		t.Fatalf("expected invalid signature response, got %s", rec.Body.String())
	}
}
