package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPostMachineCommandDispatch_nilApp(t *testing.T) {
	h := postMachineCommandDispatch(nil)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rec.Code)
	}
}
