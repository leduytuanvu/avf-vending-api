package shopeepay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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
		{99, StatusFailed, -1},
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
	if h["X-Airpay-ClientId"] != "client" {
		t.Fatal("client id")
	}
	if h["X-Airpay-Req-H"] != SignBody("secret", body) {
		t.Fatal("signature header")
	}
}
