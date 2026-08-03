package zalopay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"
)

const (
	testKey1 = "key1secret"
	testKey2 = "key2secret"
)

func TestCreateOrderMAC_Golden(t *testing.T) {
	raw := "2553|250801_ORDER1|store1|15000|1700000000000|{\"preferred_payment_method\":[\"zalopay_wallet\"]}|[]"
	want := hmacHex(testKey1, raw)
	got := CreateOrderMAC(
		testKey1,
		"2553",
		"250801_ORDER1",
		"store1",
		"15000",
		1700000000000,
		`{"preferred_payment_method":["zalopay_wallet"]}`,
		"[]",
	)
	if got != want {
		t.Fatalf("CreateOrderMAC = %q, want %q", got, want)
	}
}

func TestQueryMAC_Golden(t *testing.T) {
	raw := "2553|250801_ORDER1|" + testKey1
	want := hmacHex(testKey1, raw)
	got := QueryMAC(testKey1, "2553", "250801_ORDER1")
	if got != want {
		t.Fatalf("QueryMAC = %q, want %q", got, want)
	}
}

func TestCallbackMAC_Golden(t *testing.T) {
	data := `{"app_id":2553,"app_trans_id":"250801_ORDER1"}`
	want := hmacHex(testKey2, data)
	got := CallbackMAC(testKey2, data)
	if got != want {
		t.Fatalf("CallbackMAC = %q, want %q", got, want)
	}
	if !VerifyCallbackMAC(testKey2, data, want) {
		t.Fatal("VerifyCallbackMAC should accept")
	}
	if VerifyCallbackMAC(testKey2, data, "bad") {
		t.Fatal("VerifyCallbackMAC should reject")
	}
}

func TestAppTransID(t *testing.T) {
	now := time.Date(2025, 8, 1, 15, 30, 0, 0, time.UTC)
	got := AppTransID("ORDER_ABC", now)
	if got != "250801_ORDER_ABC" {
		t.Fatalf("AppTransID = %q, want 250801_ORDER_ABC", got)
	}
}

func TestEmbedData(t *testing.T) {
	wallet := EmbedData(false)
	if wallet != `{"preferred_payment_method":["zalopay_wallet"]}` {
		t.Fatalf("wallet embed = %q", wallet)
	}
	vietqr := EmbedData(true)
	if vietqr != `{"preferred_payment_method":["vietqr"]}` {
		t.Fatalf("vietqr embed = %q", vietqr)
	}
}

func TestMapReturnCode(t *testing.T) {
	cases := []struct {
		code   int
		status string
		legacy int
	}{
		{1, StatusCaptured, 0},
		{3, StatusPending, 1},
		{2, StatusFailed, -1},
		{-53, StatusFailed, -1},
	}
	for _, tc := range cases {
		st, lg := MapReturnCode(tc.code)
		if st != tc.status || lg != tc.legacy {
			t.Fatalf("MapReturnCode(%d) = (%q,%d), want (%q,%d)", tc.code, st, lg, tc.status, tc.legacy)
		}
	}
}

func hmacHex(key, data string) string {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}
