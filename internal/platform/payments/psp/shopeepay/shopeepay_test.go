package shopeepay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
)

func TestSignBody_AndVerify_Golden(t *testing.T) {
	secret := "test_secret_key"
	body := []byte(`{"request_id":"test-123","amount":500000}`)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	got := SignBody(secret, body)
	if got != want {
		t.Fatalf("SignBody = %q, want %q", got, want)
	}
	if !VerifySignature(secret, body, want) {
		t.Fatal("VerifySignature should accept")
	}
	if VerifySignature(secret, []byte(`{"request_id":"tampered"}`), want) {
		t.Fatal("VerifySignature should reject tampered body")
	}
	if VerifySignature("", body, want) {
		t.Fatal("empty secret should fail")
	}
}

func TestAmountConversion(t *testing.T) {
	if ToAPIAmount(5000) != 500000 {
		t.Fatal("ToAPIAmount(5000)")
	}
	if FromAPIAmount(500000) != 5000 {
		t.Fatal("FromAPIAmount(500000)")
	}
	if FromAPIAmount(ToAPIAmount(99999)) != 99999 {
		t.Fatal("round-trip")
	}
	if AmountFactor != 100 {
		t.Fatal("AmountFactor")
	}
	if PaymentReferenceIDMaxLength != 21 {
		t.Fatal("PaymentReferenceIDMaxLength")
	}
}

func TestResolveTenant(t *testing.T) {
	if ResolveTenant("TFO000001") != TenantTFO {
		t.Fatal("tfo")
	}
	if ResolveTenant("AVF000073") != TenantAVF {
		t.Fatal("avf")
	}
}

func TestParseWhitelist_AndClientIPAllowed(t *testing.T) {
	wl := ParseWhitelist("143.92.69.7, 203.162.56.17")
	if !ClientIPAllowed("143.92.69.7", wl) {
		t.Fatal("listed ip")
	}
	if ClientIPAllowed("1.2.3.4", wl) {
		t.Fatal("unlisted ip")
	}
	if !ClientIPAllowed("1.2.3.4", ParseWhitelist("")) {
		t.Fatal("empty whitelist allows all")
	}
	if !ClientIPAllowed("1.2.3.4", nil) {
		t.Fatal("nil whitelist allows all")
	}
	if ClientIPAllowed("", wl) {
		t.Fatal("empty ip with whitelist should deny")
	}
}

func TestMapTxStatus(t *testing.T) {
	cases := []struct {
		tx     int
		status string
		legacy int
	}{
		{3, StatusCaptured, 0},
		{1, StatusPending, 1},
		{2, StatusPending, 1},
		{4, StatusFailed, -1},
		{6, StatusFailed, -1},
		{7, StatusFailed, -1},
		{99, StatusPending, 1},
	}
	for _, tc := range cases {
		st, lg := MapTxStatus(tc.tx)
		if st != tc.status || lg != tc.legacy {
			t.Fatalf("MapTxStatus(%d) = (%q,%d), want (%q,%d)", tc.tx, st, lg, tc.status, tc.legacy)
		}
	}
}

func TestSoftACK(t *testing.T) {
	var m map[string]any
	if err := json.Unmarshal(SoftACK, &m); err != nil {
		t.Fatal(err)
	}
	if m["errcode"].(float64) != 0 || m["debug_msg"].(string) != "success" {
		t.Fatalf("SoftACK = %v", m)
	}
	ack := SoftACKMap()
	if ack["errcode"] != 0 || ack["debug_msg"] != "success" {
		t.Fatalf("SoftACKMap = %v", ack)
	}
}

func TestBuildHeaders(t *testing.T) {
	body := []byte(`{"a":1}`)
	h := BuildHeaders("client", "secret", body)
	if h[HeaderClientID] != "client" {
		t.Fatal("client id")
	}
	if h[HeaderReqH] != SignBody("secret", body) {
		t.Fatal("signature header")
	}
}

func TestResolveStoreExtID(t *testing.T) {
	if got := ResolveStoreExtID("AVF000079", "fallback"); got != "AVF000079" {
		t.Fatalf("avf: %q", got)
	}
	if got := ResolveStoreExtID("TFO000001", "fallback"); got != "TFO000001" {
		t.Fatalf("tfo: %q", got)
	}
	if got := ResolveStoreExtID("", "AVF000079"); got != "AVF000079" {
		t.Fatalf("empty: %q", got)
	}
	if got := ResolveStoreExtID("UNKNOWN", "AVF000079"); got != "AVF000079" {
		t.Fatalf("unknown: %q", got)
	}
}

func TestValidateEndpoint(t *testing.T) {
	if err := ValidateEndpoint("https://"+LiveHost, true); err != nil {
		t.Fatalf("live prod: %v", err)
	}
	if err := ValidateEndpoint("https://"+UATHost, true); !errors.Is(err, ErrUATEndpointOnProduction) {
		t.Fatalf("uat prod: %v", err)
	}
	if err := ValidateEndpoint("https://"+LiveHost, false); !errors.Is(err, ErrLiveEndpointBlocked) {
		t.Fatalf("live non-prod: %v", err)
	}
	if err := ValidateEndpoint("https://"+UATHost, false); err != nil {
		t.Fatalf("uat non-prod: %v", err)
	}
	if err := ValidateEndpoint("http://127.0.0.1:1234", false); err != nil {
		t.Fatalf("loopback non-prod: %v", err)
	}
}

func TestStoreAllowlist(t *testing.T) {
	list := ParseAllowlistCSV("AVF000079, TFO000001")
	if !StoreAllowed("AVF000079", list, true) {
		t.Fatal("listed")
	}
	if StoreAllowed("AVF999999", list, true) {
		t.Fatal("unlisted")
	}
	if StoreAllowed("AVF000079", nil, true) {
		t.Fatal("empty fail-closed")
	}
	if !StoreAllowed("AVF000079", nil, false) {
		t.Fatal("empty fail-open")
	}
	embedded := ParseAllowlistLines(DefaultStoreAllowlistFile)
	if _, ok := embedded["AVF000079"]; !ok {
		t.Fatal("embedded missing AVF000079")
	}
	if _, ok := embedded["AVF000195"]; !ok {
		t.Fatal("embedded missing AVF000195")
	}
}

func TestClientIPPolicy_FailClosed(t *testing.T) {
	if ClientIPPolicy("1.2.3.4", nil, true) {
		t.Fatal("empty whitelist fail-closed should deny")
	}
	if !ClientIPPolicy("1.2.3.4", nil, false) {
		t.Fatal("empty whitelist fail-open should allow")
	}
}
