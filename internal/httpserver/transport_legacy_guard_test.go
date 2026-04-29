package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
)

func TestMachineLegacyRESTGuard_blocksWhenDisabled(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		TransportBoundary: config.TransportBoundaryConfig{
			MachineRESTLegacyEnabled: false,
		},
	}
	h := machineLegacyRESTGuard(cfg)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next must not run")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/machines/x/check-ins", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMachineLegacyRESTGuard_allowsWhenEnabled(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		TransportBoundary: config.TransportBoundaryConfig{
			MachineRESTLegacyEnabled: true,
		},
	}
	var ran bool
	h := machineLegacyRESTGuard(cfg)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ran = true
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !ran || rec.Code != http.StatusOK {
		t.Fatalf("handler ran=%v code=%d", ran, rec.Code)
	}
}
