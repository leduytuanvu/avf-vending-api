package momo

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

const testSecret = "secretKey123"

func TestSignCreate_Golden(t *testing.T) {
	r := CreateRequest{
		AccessKey:   "accessKey",
		Amount:      "15000",
		ExtraData:   "",
		IPNURL:      "https://example.com/ipn",
		OrderID:     "ORDER001",
		OrderInfo:   "Thanh toan don hang MoMo ORDER001",
		PartnerCode: "MOMO",
		RedirectURL: "https://example.com/redirect",
		RequestID:   "req-001",
		RequestType: RequestTypeCreate,
	}
	raw := "accessKey=accessKey&amount=15000&extraData=&ipnUrl=https://example.com/ipn&orderId=ORDER001&orderInfo=Thanh toan don hang MoMo ORDER001&partnerCode=MOMO&redirectUrl=https://example.com/redirect&requestId=req-001&requestType=captureWallet"
	want := hmacHex(testSecret, raw)
	got := SignCreate(testSecret, r)
	if got != want {
		t.Fatalf("SignCreate = %q, want %q", got, want)
	}
}

func TestSignQuery_Golden(t *testing.T) {
	raw := "accessKey=ak&orderId=OID&partnerCode=PC&requestId=RID"
	want := hmacHex(testSecret, raw)
	got := SignQuery(testSecret, "ak", "OID", "PC", "RID")
	if got != want {
		t.Fatalf("SignQuery = %q, want %q", got, want)
	}
}

func TestSignIPN_Golden(t *testing.T) {
	f := IPNFields{
		AccessKey:    "ak",
		Amount:       "10000",
		ExtraData:    "",
		Message:      "Success",
		OrderID:      "OID",
		OrderInfo:    "info",
		OrderType:    "momo_wallet",
		PartnerCode:  "PC",
		PayType:      "qr",
		RequestID:    "RID",
		ResponseTime: "1700000000000",
		ResultCode:   "0",
		TransID:      "12345",
	}
	raw := "accessKey=ak&amount=10000&extraData=&message=Success&orderId=OID&orderInfo=info&orderType=momo_wallet&partnerCode=PC&payType=qr&requestId=RID&responseTime=1700000000000&resultCode=0&transId=12345"
	want := hmacHex(testSecret, raw)
	got := SignIPN(testSecret, f)
	if got != want {
		t.Fatalf("SignIPN = %q, want %q", got, want)
	}
	if !VerifyIPNSignature(testSecret, f, want) {
		t.Fatal("VerifyIPNSignature should accept matching signature")
	}
	if VerifyIPNSignature(testSecret, f, "deadbeef") {
		t.Fatal("VerifyIPNSignature should reject mismatch")
	}
	if VerifyIPNSignature("", f, want) {
		t.Fatal("empty secret should fail")
	}
}

func TestBuildCreateBody(t *testing.T) {
	r := CreateRequest{
		AccessKey:   "ak",
		Amount:      "5000",
		IPNURL:      "https://ipn",
		OrderID:     "O1",
		OrderInfo:   "pay",
		PartnerCode: "PC",
		RedirectURL: "https://re",
		RequestID:   "R1",
		StoreID:     "STORE",
	}
	body := BuildCreateBody(testSecret, r)
	if body["requestType"] != RequestTypeCreate {
		t.Fatalf("requestType = %v", body["requestType"])
	}
	if body["lang"] != "vi" {
		t.Fatalf("lang = %v", body["lang"])
	}
	sig, _ := body["signature"].(string)
	if sig == "" {
		t.Fatal("missing signature")
	}
	r.RequestType = RequestTypeCreate
	r.ExtraData = ""
	if sig != SignCreate(testSecret, r) {
		t.Fatal("body signature mismatch")
	}
}

func TestResolveTenant(t *testing.T) {
	if got := ResolveTenant("TFO000001"); got != TenantTFO {
		t.Fatalf("got %q", got)
	}
	if got := ResolveTenant("AVF000073"); got != TenantAVF {
		t.Fatalf("got %q", got)
	}
	if got := ResolveTenant(""); got != TenantAVF {
		t.Fatalf("got %q", got)
	}
}

func TestMapResultCode(t *testing.T) {
	cases := []struct {
		code   int
		status string
		ret    int
	}{
		{0, StatusCaptured, 0},
		{9000, StatusPending, 1},
		{1000, StatusPending, 1},
		{7000, StatusPending, 1},
		{-1, StatusPending, 1},
	}
	for _, tc := range cases {
		if got := MapResultCode(tc.code); got != tc.status {
			t.Fatalf("MapResultCode(%d) = %q, want %q", tc.code, got, tc.status)
		}
		if got := MapResultCodeToReturnCode(tc.code); got != tc.ret {
			t.Fatalf("MapResultCodeToReturnCode(%d) = %d, want %d", tc.code, got, tc.ret)
		}
	}
}

func TestTenantKeys_ForTenant(t *testing.T) {
	keys := TenantKeys{
		AVF: Credentials{PartnerCode: "AVF"},
		TFO: Credentials{PartnerCode: "TFO"},
	}
	if keys.ForTenant(TenantTFO).PartnerCode != "TFO" {
		t.Fatal("tfo")
	}
	if keys.ForTenant(TenantAVF).PartnerCode != "AVF" {
		t.Fatal("avf")
	}
	if keys.ForTenant("other").PartnerCode != "AVF" {
		t.Fatal("default avf")
	}
}

func hmacHex(secret, raw string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(raw))
	return hex.EncodeToString(mac.Sum(nil))
}
