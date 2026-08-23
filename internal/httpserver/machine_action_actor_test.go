package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestParseOptionalOperatorSessionIDField_omitted(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/", nil)
	id, ok := parseOptionalOperatorSessionIDField(rec, req, "")
	if !ok || id != nil {
		t.Fatalf("omitted session should succeed with nil id, ok=%v id=%v", ok, id)
	}
}

func TestParseOptionalOperatorSessionIDField_zeroRejected(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/", nil)
	_, ok := parseOptionalOperatorSessionIDField(rec, req, uuid.Nil.String())
	if ok {
		t.Fatal("zero UUID must be rejected")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestParseOptionalOperatorSessionIDField_invalidRejected(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/", nil)
	_, ok := parseOptionalOperatorSessionIDField(rec, req, "not-a-uuid")
	if ok {
		t.Fatal("expected reject")
	}
}
