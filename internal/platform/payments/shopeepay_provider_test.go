package payments

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/platform/payments/psp/httpx"
	"github.com/avf/avf-vending-api/internal/platform/payments/psp/shopeepay"
	"github.com/stretchr/testify/require"
)

const testShopeeSecret = "test-secret"

type shopeeCapture struct {
	path       string
	storeExtID string
	merchant   string
	body       map[string]any
}

func signedJSON(t *testing.T, w http.ResponseWriter, secret string, payload any) {
	t.Helper()
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	w.Header().Set(shopeepay.HeaderRespH, shopeepay.SignBody(secret, body))
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body)
}

func testShopeeProvider(t *testing.T, handler http.HandlerFunc) (*ShopeePayProvider, *shopeeCapture) {
	t.Helper()
	cap := &shopeeCapture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		cap.path = r.URL.Path
		cap.storeExtID = asString(m["store_ext_id"])
		cap.merchant = asString(m["merchant_ext_id"])
		cap.body = m
		if handler != nil {
			handler.ServeHTTP(w, r.WithContext(r.Context()))
			return
		}
		signedJSON(t, w, testShopeeSecret, map[string]any{"errcode": 0, "qr_content": "spp-qr", "qr_url": "https://img/qr.png"})
	}))
	t.Cleanup(srv.Close)

	client := httpx.New(httpx.DefaultTimeout)
	client.HTTP.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	p := &ShopeePayProvider{
		wired:  true,
		client: client,
		keys: shopeepay.TenantKeys{
			AVF: shopeepay.Credentials{
				ClientID:      "avf-client",
				SecretKey:     testShopeeSecret,
				Endpoint:      srv.URL,
				MerchantExtID: "AVF_MERCHANT",
				StoreExtID:    "FALLBACK_STORE",
			},
			TFO: shopeepay.Credentials{
				ClientID:      "tfo-client",
				SecretKey:     testShopeeSecret,
				Endpoint:      srv.URL,
				MerchantExtID: "TFO_MERCHANT",
				StoreExtID:    "TFO_FALLBACK",
			},
		},
		storeAllowlist: shopeepay.ParseAllowlistCSV("AVF000079,TFO000001"),
		whitelist:      shopeepay.ParseWhitelist("203.162.56.17"),
	}
	return p, cap
}

func TestShopeePayCreate_UsesMachineCodeAsStoreExtID(t *testing.T) {
	t.Parallel()
	p, cap := testShopeeProvider(t, nil)
	out, err := p.CreatePaymentSession(context.Background(), CreatePaymentSessionInput{
		AmountMinor:         5000,
		MachineExternalCode: "AVF000079",
		ProviderReference:   "AVFref123456789012",
	})
	require.NoError(t, err)
	require.Equal(t, "AVF000079", cap.storeExtID)
	require.Equal(t, "AVF_MERCHANT", cap.merchant)
	require.Equal(t, "spp-qr", out.QRPayloadOrURL)
	require.Equal(t, shopeepay.QRCreatePath, cap.path)
}

func TestShopeePayCreate_RejectsUnlistedStore(t *testing.T) {
	t.Parallel()
	p, _ := testShopeeProvider(t, nil)
	_, err := p.CreatePaymentSession(context.Background(), CreatePaymentSessionInput{
		AmountMinor:         5000,
		MachineExternalCode: "AVF999999",
		ProviderReference:   "AVFref123456789012",
	})
	require.ErrorIs(t, err, shopeepay.ErrStoreNotAllowlisted)
}

func TestShopeePayCreate_PrefersQrContent(t *testing.T) {
	t.Parallel()
	p, _ := testShopeeProvider(t, nil)
	out, err := p.CreatePaymentSession(context.Background(), CreatePaymentSessionInput{
		AmountMinor:         1000,
		MachineExternalCode: "AVF000079",
		ProviderReference:   "AVFref123456789012",
	})
	require.NoError(t, err)
	require.Equal(t, "spp-qr", out.QRPayloadOrURL)
}

func TestShopeePayQueryAndRefund_UseTenantAndStore(t *testing.T) {
	t.Parallel()
	p, cap := testShopeeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		signedJSON(t, w, testShopeeSecret, map[string]any{
			"errcode": 0,
			"transaction": map[string]any{
				"status": 3,
			},
		})
	})
	_, err := p.QueryPaymentStatus(context.Background(), domaincommerce.PaymentProviderLookup{
		ProviderReference:   "AVFref123456789012",
		AmountMinor:         5000,
		MachineExternalCode: "TFO000001",
	})
	require.NoError(t, err)
	require.Equal(t, "TFO000001", cap.storeExtID)
	require.Equal(t, "TFO_MERCHANT", cap.merchant)

	require.NoError(t, p.RefundPayment(context.Background(), RefundPaymentInput{
		ProviderReference:   "AVFref123456789012",
		AmountMinor:         5000,
		MachineExternalCode: "TFO000001",
		IdempotencyKey:      "refundref12345678901",
	}))
	require.Equal(t, shopeepay.RefundCreatePath, cap.path)
	require.Equal(t, "TFO000001", cap.storeExtID)
}

func TestShopeePayCancel_InvalidatesQR(t *testing.T) {
	t.Parallel()
	p, cap := testShopeeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		signedJSON(t, w, testShopeeSecret, map[string]any{"errcode": 0})
	})
	require.NoError(t, p.CancelPayment(context.Background(), CancelPaymentInput{
		ProviderReference:   "AVFref123456789012",
		MachineExternalCode: "AVF000079",
	}))
	require.Equal(t, shopeepay.QRInvalidatePath, cap.path)
	require.Equal(t, "AVF000079", cap.storeExtID)
}

func TestShopeePayQuery_InvalidatesOnFailedStatus(t *testing.T) {
	t.Parallel()
	var invalidate atomic.Bool
	p, _ := testShopeeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == shopeepay.QRInvalidatePath {
			invalidate.Store(true)
			signedJSON(t, w, testShopeeSecret, map[string]any{"errcode": 0})
			return
		}
		signedJSON(t, w, testShopeeSecret, map[string]any{
			"errcode": 0,
			"transaction": map[string]any{
				"status": shopeepay.TXStatusExpired,
			},
		})
	})
	snap, err := p.QueryPaymentStatus(context.Background(), domaincommerce.PaymentProviderLookup{
		ProviderReference:   "AVFref123456789012",
		AmountMinor:         5000,
		MachineExternalCode: "AVF000079",
	})
	require.NoError(t, err)
	require.Equal(t, shopeepay.StatusFailed, snap.NormalizedState)
	require.True(t, invalidate.Load())
}

func TestShopeePayCallback_ForbiddenIPAndInvalidSig(t *testing.T) {
	t.Parallel()
	p, _ := testShopeeProvider(t, nil)
	body := []byte(`{"reference_id":"AVFref123456789012","transaction_status":3,"amount":500000}`)
	_, _, _, _, err := p.VerifyAndParseCallback(body, "avf-client", "bad", "1.2.3.4")
	require.ErrorIs(t, err, shopeepay.ErrCallbackForbiddenIP)
	code, _ := ShopeeCallbackHTTPStatus(err)
	require.Equal(t, http.StatusForbidden, code)

	sig := shopeepay.SignBody(testShopeeSecret, body)
	_, _, _, _, err = p.VerifyAndParseCallback(body, "avf-client", "not-the-sig", "203.162.56.17")
	require.ErrorIs(t, err, shopeepay.ErrCallbackInvalidSignature)
	code, _ = ShopeeCallbackHTTPStatus(err)
	require.Equal(t, http.StatusUnauthorized, code)

	_, status, _, event, err := p.VerifyAndParseCallback(body, "avf-client", sig, "203.162.56.17")
	require.NoError(t, err)
	require.Equal(t, "captured", status)
	require.Equal(t, "AVFref123456789012", event.ProviderReference)
}

func TestShopeePayCreate_RejectsMissingResponseSignature(t *testing.T) {
	t.Parallel()
	p, _ := testShopeeProvider(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"qr_content":"x"}`))
	})
	_, err := p.CreatePaymentSession(context.Background(), CreatePaymentSessionInput{
		AmountMinor:         1000,
		MachineExternalCode: "AVF000079",
		ProviderReference:   "AVFref123456789012",
	})
	require.ErrorIs(t, err, shopeepay.ErrInvalidResponseSignature)
}

func TestNewShopeePayProvider_LoadsAllowlistAndStrict(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{AppEnv: config.AppEnvProduction}
	cfg.PSP.ShopeePay.AVF = config.ShopeePayTenantCredentials{
		ClientID: "c", SecretKey: "s", Endpoint: "https://api.wallet.airpay.vn",
		MerchantExtID: "m", StoreExtID: "AVF000079", CallbackURL: "https://api.ldtv.dev/v1/commerce/webhooks/shopeepay",
	}
	p := NewShopeePayProvider(cfg)
	require.True(t, p.strict)
	require.True(t, p.production)
	require.True(t, shopeepay.StoreAllowed("AVF000079", p.storeAllowlist, true))
	require.False(t, shopeepay.StoreAllowed("NOPE", p.storeAllowlist, true))
}

func TestShopeeCallbackHTTPStatus_OrderNotFoundMappedByCaller(t *testing.T) {
	t.Parallel()
	code, msg := ShopeeCallbackHTTPStatus(shopeepay.ErrCallbackForbiddenIP)
	require.Equal(t, http.StatusForbidden, code)
	require.Equal(t, "forbidden ip", msg)
	code, msg = ShopeeCallbackHTTPStatus(errors.New("invalid json"))
	require.Equal(t, http.StatusBadRequest, code)
}
