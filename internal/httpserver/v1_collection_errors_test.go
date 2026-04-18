package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/api"
)

func TestWriteCapabilityNotConfigured(t *testing.T) {
	rec := httptest.NewRecorder()
	writeCapabilityNotConfigured(rec, "mqtt_command_dispatch", "MQTT broker client is not configured")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusServiceUnavailable)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error object: %#v", body)
	}
	if errObj["code"] != "capability_not_configured" {
		t.Fatalf("code: got %v", errObj["code"])
	}
	if errObj["implemented"] != false {
		t.Fatalf("implemented: got %v", errObj["implemented"])
	}
	if errObj["capability"] != "mqtt_command_dispatch" {
		t.Fatalf("capability: got %v", errObj["capability"])
	}
}

func TestWriteV1ListError_NotImplemented(t *testing.T) {
	rec := httptest.NewRecorder()
	writeV1ListError(rec, &api.CapabilityError{
		Capability: "v1.admin.commands.list",
		Message:    "command listing is not implemented for this API revision",
	})
	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusNotImplemented)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	errObj, ok := body["error"].(map[string]any)
	if !ok {
		t.Fatalf("missing error object: %#v", body)
	}
	if errObj["code"] != "not_implemented" {
		t.Fatalf("code: got %v", errObj["code"])
	}
	if errObj["implemented"] != false {
		t.Fatalf("implemented: got %v", errObj["implemented"])
	}
	if errObj["capability"] != "v1.admin.commands.list" {
		t.Fatalf("capability: got %v", errObj["capability"])
	}
}

func TestWriteV1ListError_TenantScopeRequired(t *testing.T) {
	rec := httptest.NewRecorder()
	writeV1ListError(rec, api.ErrAdminTenantScopeRequired)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestWriteMachineShadowLoadError_NotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	writeMachineShadowLoadError(rec, api.ErrMachineShadowNotFound)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusNotFound)
	}
}
