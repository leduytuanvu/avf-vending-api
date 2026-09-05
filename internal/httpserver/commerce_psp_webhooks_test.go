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
