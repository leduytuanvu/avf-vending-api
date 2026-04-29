package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appotaadmin "github.com/avf/avf-vending-api/internal/app/otaadmin"
)

func TestWriteOTAAdminError_mapsKnownErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rr := httptest.NewRecorder()
	writeOTAAdminError(rr, ctx, appotaadmin.ErrInvalidTransition)
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.Code != "illegal_transition" {
		t.Fatalf("code %q", body.Error.Code)
	}
	if rr.Code != http.StatusConflict {
		t.Fatalf("status %d", rr.Code)
	}
}
