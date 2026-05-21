package postman_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func productionFullSuiteCollectionPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "production-full-suite", "avf-production-full.postman_collection.json")
}

func walkPostmanItems(items []any, fn func(name string, req map[string]any) bool) bool {
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := item["name"].(string)
		if req, ok := item["request"].(map[string]any); ok {
			if fn(name, req) {
				return true
			}
		}
		if nested, ok := item["item"].([]any); ok {
			if walkPostmanItems(nested, fn) {
				return true
			}
		}
	}
	return false
}

func headerValue(req map[string]any, key string) string {
	headers, ok := req["header"].([]any)
	if !ok {
		return ""
	}
	for _, h := range headers {
		row, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if row["disabled"] == true {
			continue
		}
		if strings.EqualFold(row["key"].(string), key) {
			v, _ := row["value"].(string)
			return v
		}
	}
	return ""
}

func TestProductionFullSuite_mediaUploadInitPresentWithDirectGuidIdempotency(t *testing.T) {
	raw, err := os.ReadFile(productionFullSuiteCollectionPath(t))
	if err != nil {
		t.Fatalf("read collection: %v", err)
	}
	var coll map[string]any
	if err := json.Unmarshal(raw, &coll); err != nil {
		t.Fatalf("parse collection: %v", err)
	}
	items, ok := coll["item"].([]any)
	if !ok {
		t.Fatal("collection missing item array")
	}

	found := walkPostmanItems(items, func(name string, req map[string]any) bool {
		method, _ := req["method"].(string)
		url, _ := req["url"].(map[string]any)
		rawURL, _ := url["raw"].(string)
		if method != "POST" || !strings.Contains(rawURL, "/v1/admin/media/uploads/init") {
			return false
		}
		if headerValue(req, "Idempotency-Key") != "{{$guid}}" {
			t.Fatalf("%q: Idempotency-Key must be {{$guid}}, got %q", name, headerValue(req, "Idempotency-Key"))
		}
		if v := headerValue(req, "X-Request-ID"); v != "{{$guid}}" {
			t.Fatalf("%q: X-Request-ID must be {{$guid}}, got %q", name, v)
		}
		if v := headerValue(req, "X-Correlation-ID"); v != "{{$guid}}" {
			t.Fatalf("%q: X-Correlation-ID must be {{$guid}}, got %q", name, v)
		}
		body, _ := req["body"].(map[string]any)
		rawBody, _ := body["raw"].(string)
		for _, want := range []string{`"filename"`, `"contentType"`, `"purpose"`, "coca-330ml.png", "image/png", "product_image"} {
			if !strings.Contains(rawBody, want) {
				t.Fatalf("%q body missing %q: %s", name, want, rawBody)
			}
		}
		return true
	})
	if !found {
		t.Fatal("POST /v1/admin/media/uploads/init not found in production-full-suite collection")
	}
}
