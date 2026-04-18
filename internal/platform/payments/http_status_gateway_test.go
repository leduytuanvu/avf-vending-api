package payments

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
)

func TestHTTPStatusGateway_FetchPaymentStatus_decodesBody(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	oid := uuid.MustParse("88888888-8888-8888-8888-888888888888")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != fmt.Sprintf("/payments/%s", pid.String()) {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"normalized_state":"SUCCEEDED","provider_hint":{"ref":"abc"}}`))
	}))
	t.Cleanup(srv.Close)

	gw, err := NewHTTPStatusGateway(srv.URL+"/payments/%s", "")
	if err != nil {
		t.Fatal(err)
	}
	snap, err := gw.FetchPaymentStatus(context.Background(), domaincommerce.PaymentProviderLookup{
		Provider:  "test",
		PaymentID: pid,
		OrderID:   oid,
	})
	if err != nil {
		t.Fatal(err)
	}
	if snap.NormalizedState != "succeeded" {
		t.Fatalf("normalized: %q", snap.NormalizedState)
	}
	if len(snap.ProviderHint) == 0 {
		t.Fatal("expected provider_hint bytes")
	}
}

func TestHTTPStatusGateway_rejectsMissingNormalizedState(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	gw, err := NewHTTPStatusGateway(srv.URL+"/p/%s", "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = gw.FetchPaymentStatus(context.Background(), domaincommerce.PaymentProviderLookup{
		PaymentID: uuid.New(),
		OrderID:   uuid.New(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
