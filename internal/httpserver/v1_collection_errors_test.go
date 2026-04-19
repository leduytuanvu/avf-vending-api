package httpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/api"
)

func TestWriteCapabilityNotConfigured(t *testing.T) {
	rec := httptest.NewRecorder()
	writeCapabilityNotConfigured(rec, context.Background(), "mqtt_command_dispatch", "MQTT broker client is not configured")
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
	details, ok := errObj["details"].(map[string]any)
	if !ok {
		t.Fatalf("missing details: %#v", errObj)
	}
	if details["implemented"] != false {
		t.Fatalf("implemented: got %v", details["implemented"])
	}
	if details["capability"] != "mqtt_command_dispatch" {
		t.Fatalf("capability: got %v", details["capability"])
	}
	if _, ok := errObj["requestId"].(string); !ok {
		t.Fatalf("requestId: got %v", errObj["requestId"])
	}
}

func TestWriteV1ListError_NotImplemented(t *testing.T) {
	rec := httptest.NewRecorder()
	writeV1ListError(rec, context.Background(), &api.CapabilityError{
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
	details, ok := errObj["details"].(map[string]any)
	if !ok {
		t.Fatalf("missing details: %#v", errObj)
	}
	if details["implemented"] != false {
		t.Fatalf("implemented: got %v", details["implemented"])
	}
	if details["capability"] != "v1.admin.commands.list" {
		t.Fatalf("capability: got %v", details["capability"])
	}
}

func TestWriteV1ListError_TenantScopeRequired(t *testing.T) {
	rec := httptest.NewRecorder()
	writeV1ListError(rec, context.Background(), api.ErrAdminTenantScopeRequired)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestWriteMachineShadowLoadError_NotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	writeMachineShadowLoadError(rec, context.Background(), api.ErrMachineShadowNotFound)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusNotFound)
	}
}
