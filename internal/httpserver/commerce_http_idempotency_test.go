package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireWriteIdempotencyKey_PrimaryHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(headerIdempotencyKey, " k1 ")
	got, err := requireWriteIdempotencyKey(req)
	if err != nil {
		t.Fatal(err)
	}
	if got != "k1" {
		t.Fatalf("got %q", got)
	}
}

func TestRequireWriteIdempotencyKey_AltHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set(headerIdempotencyKeyAlt, "k2")
	got, err := requireWriteIdempotencyKey(req)
	if err != nil {
		t.Fatal(err)
	}
	if got != "k2" {
		t.Fatalf("got %q", got)
	}
}

func TestRequireWriteIdempotencyKey_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	_, err := requireWriteIdempotencyKey(req)
	if err == nil {
		t.Fatal("expected error")
	}
}
