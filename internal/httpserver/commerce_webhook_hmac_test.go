package httpserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"
	"time"
)

func signAVFCommerceWebhook(secret string, ts int64, body []byte) (tsHeader, sigHeader string) {
	tsHeader = strconv.FormatInt(ts, 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(tsHeader))
	mac.Write([]byte{'.'})
	mac.Write(body)
	return tsHeader, "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyCommerceWebhookHMAC_valid(t *testing.T) {
	t.Parallel()
	secret := "test-secret-fixture"
	body := []byte(`{"provider":"psp","provider_reference":"r1"}`)
	ts := time.Now().Unix()
	th, sh := signAVFCommerceWebhook(secret, ts, body)
	if err := verifyCommerceWebhookHMAC(secret, th, sh, body, 300*time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyCommerceWebhookHMAC_invalidSignature(t *testing.T) {
	t.Parallel()
	secret := "test-secret-fixture"
	body := []byte(`{"provider":"psp","provider_reference":"r1"}`)
	ts := time.Now().Unix()
	th, _ := signAVFCommerceWebhook(secret, ts, body)
	err := verifyCommerceWebhookHMAC(secret, th, "sha256=deadbeef", body, 300*time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid webhook signature") {
		t.Fatalf("got %v", err)
	}
}

func TestVerifyCommerceWebhookHMAC_staleTimestamp(t *testing.T) {
	t.Parallel()
	secret := "test-secret-fixture"
	body := []byte(`{}`)
	ts := time.Now().Add(-400 * time.Second).Unix()
	th, sh := signAVFCommerceWebhook(secret, ts, body)
	err := verifyCommerceWebhookHMAC(secret, th, sh, body, 120*time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "skew") {
		t.Fatalf("got %v", err)
	}
}

func TestVerifyCommerceWebhookHMAC_missingHeaders(t *testing.T) {
	t.Parallel()
	if err := verifyCommerceWebhookHMAC("sec", "", "", []byte("{}"), time.Minute); err == nil {
		t.Fatal("expected error")
	}
}
