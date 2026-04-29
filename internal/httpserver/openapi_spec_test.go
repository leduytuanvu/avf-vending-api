package httpserver

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/docs/swagger"
)

func TestOpenAPI_embeddedJSON_validAndProductionServerFirst(t *testing.T) {
	t.Parallel()
	raw := swagger.OpenAPIJSON()
	if !json.Valid(raw) {
		t.Fatal("embedded OpenAPI document is not valid JSON")
	}
	var spec map[string]any
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	servers, ok := spec["servers"].([]any)
	if !ok || len(servers) < 2 {
		t.Fatalf("expected at least two servers, got %#v", spec["servers"])
	}
	s0, ok := servers[0].(map[string]any)
	if !ok || s0["url"] != "https://api.ldtv.dev" {
		t.Fatalf("servers[0] must be production https://api.ldtv.dev, got %#v", servers[0])
	}
	s1, ok := servers[1].(map[string]any)
	if !ok || s1["url"] != "http://localhost:8080" {
		t.Fatalf("servers[1] must be local http://localhost:8080, got %#v", servers[1])
	}
}

func TestOpenAPI_allLocalRefsResolve(t *testing.T) {
	t.Parallel()
	var spec map[string]any
	if err := json.Unmarshal(swagger.OpenAPIJSON(), &spec); err != nil {
		t.Fatal(err)
	}

	var resolveJSONPointer = func(ref string) (any, bool) {
		if !strings.HasPrefix(ref, "#/") {
			return nil, false
		}
		cur := any(spec)
		for _, raw := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
			part := strings.ReplaceAll(strings.ReplaceAll(raw, "~1", "/"), "~0", "~")
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, false
			}
			cur, ok = m[part]
			if !ok {
				return nil, false
			}
		}
		return cur, true
	}

	var walk func(path string, v any)
	walk = func(path string, v any) {
		switch x := v.(type) {
		case map[string]any:
			if ref, ok := x["$ref"].(string); ok && strings.HasPrefix(ref, "#/") {
				if _, ok := resolveJSONPointer(ref); !ok {
					t.Errorf("unresolved local OpenAPI $ref at %s: %s", path, ref)
				}
			}
			for k, vv := range x {
				walk(path+"."+k, vv)
			}
		case []any:
			for i, vv := range x {
				walk(fmt.Sprintf("%s[%d]", path, i), vv)
			}
		}
	}
	walk("$", spec)
}

func TestOpenAPI_duplicateOperationIDsAbsent(t *testing.T) {
	t.Parallel()
	var spec map[string]any
	if err := json.Unmarshal(swagger.OpenAPIJSON(), &spec); err != nil {
		t.Fatal(err)
	}
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("missing paths")
	}
	seen := make(map[string]string)
	for path, pm := range paths {
		methods, ok := pm.(map[string]any)
		if !ok {
			continue
		}
		for method, opAny := range methods {
			method = strings.ToLower(method)
			if strings.HasPrefix(method, "x-") {
				continue
			}
			op, ok := opAny.(map[string]any)
			if !ok {
				continue
			}
			raw, ok := op["operationId"].(string)
			if !ok || strings.TrimSpace(raw) == "" {
				continue
			}
			id := strings.TrimSpace(raw)
			label := strings.ToUpper(method) + " " + path
			if prev, dup := seen[id]; dup {
				t.Fatalf("duplicate operationId %q: %s vs %s", id, prev, label)
			}
			seen[id] = label
		}
	}
}

func TestOpenAPI_machineLegacyRESTMarkedDeprecated(t *testing.T) {
	t.Parallel()
	var spec map[string]any
	if err := json.Unmarshal(swagger.OpenAPIJSON(), &spec); err != nil {
		t.Fatal(err)
	}
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("missing paths")
	}
	check := func(method, path string) {
		t.Helper()
		pm, ok := paths[path].(map[string]any)
		if !ok {
			t.Fatalf("missing path %s", path)
		}
		opAny, ok := pm[method].(map[string]any)
		if !ok {
			t.Fatalf("missing %s %s", strings.ToUpper(method), path)
		}
		if dep, ok := opAny["deprecated"].(bool); !ok || !dep {
			t.Fatalf("%s %s: want deprecated=true", strings.ToUpper(method), path)
		}
	}
	check("post", "/v1/machines/{machineId}/check-ins")
	check("post", "/v1/device/machines/{machineId}/vend-results")
}

func TestOpenAPI_securitySchemesBearerAuthPresent(t *testing.T) {
	t.Parallel()
	var spec map[string]any
	if err := json.Unmarshal(swagger.OpenAPIJSON(), &spec); err != nil {
		t.Fatal(err)
	}
	comps, ok := spec["components"].(map[string]any)
	if !ok {
		t.Fatal("missing components")
	}
	schemes, ok := comps["securitySchemes"].(map[string]any)
	if !ok {
		t.Fatal("missing securitySchemes")
	}
	bearer, ok := schemes["bearerAuth"].(map[string]any)
	if !ok {
		t.Fatal("missing bearerAuth")
	}
	if bearer["type"] != "http" || bearer["scheme"] != "bearer" {
		t.Fatalf("unexpected bearerAuth: %#v", bearer)
	}
}

func TestOpenAPI_plannedOnlyPathsNotDocumented(t *testing.T) {
	t.Parallel()
	var spec map[string]any
	if err := json.Unmarshal(swagger.OpenAPIJSON(), &spec); err != nil {
		t.Fatal(err)
	}
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("missing paths")
	}
	forbiddenFragments := []string{
		"/v1/activation",
		"/v1/machines/{machineId}/activation",
		"/v1/runtime/catalog",
		"/v1/telemetry/reconcile",
		"/v1/cash-collection",
	}
	forbiddenSuffixes := []string{"/{orderId}/refund"} // singular stub only; `/refunds` is shipped
	for p := range paths {
		for _, suf := range forbiddenSuffixes {
			if strings.HasSuffix(p, suf) {
				t.Fatalf("path %q must not appear in OpenAPI (planned-only); see docs/api/roadmap.md", p)
			}
		}
		for _, frag := range forbiddenFragments {
			if strings.Contains(p, frag) {
				t.Fatalf("path %q must not appear in OpenAPI (planned-only); see docs/api/roadmap.md", p)
			}
		}
	}
}

func TestOpenAPI_requestBodyExamplesForJSONWrites(t *testing.T) {
	t.Parallel()
	var spec map[string]any
	if err := json.Unmarshal(swagger.OpenAPIJSON(), &spec); err != nil {
		t.Fatal(err)
	}
	paths, _ := spec["paths"].(map[string]any)
	for path, methods := range paths {
		for method, opAny := range methods.(map[string]any) {
			method = strings.ToLower(method)
			if method != "post" && method != "put" && method != "patch" {
				continue
			}
			op := opAny.(map[string]any)
			rb, ok := op["requestBody"].(map[string]any)
			if !ok {
				continue
			}
			content, _ := rb["content"].(map[string]any)
			aj, ok := content["application/json"].(map[string]any)
			if !ok {
				continue
			}
			if _, ok := aj["example"]; !ok {
				t.Errorf("%s %s: application/json request body must include an example for Swagger UI", strings.ToUpper(method), path)
			}
		}
	}
}

func TestOpenAPI_bearerAuthOnProtectedV1Routes(t *testing.T) {
	t.Parallel()
	var spec map[string]any
	if err := json.Unmarshal(swagger.OpenAPIJSON(), &spec); err != nil {
		t.Fatal(err)
	}
	paths, _ := spec["paths"].(map[string]any)
	noBearer := map[string]map[string]struct{}{
		"/v1/auth/login":                                              {"post": {}},
		"/v1/auth/refresh":                                            {"post": {}},
		"/v1/auth/password/reset/request":                             {"post": {}},
		"/v1/auth/password/reset/confirm":                             {"post": {}},
		"/v1/setup/activation-codes/claim":                            {"post": {}},
		"/v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks": {"post": {}},
	}
	for path, methods := range paths {
		if !strings.HasPrefix(path, "/v1/") {
			continue
		}
		for method, opAny := range methods.(map[string]any) {
			method = strings.ToLower(method)
			if skip, ok := noBearer[path]; ok {
				if _, ok := skip[method]; ok {
					continue
				}
			}
			op := opAny.(map[string]any)
			sec, ok := op["security"].([]any)
			if !ok || len(sec) == 0 {
				t.Errorf("%s %s: expected security (bearerAuth) on protected /v1 route", strings.ToUpper(method), path)
				continue
			}
			found := false
			for _, s := range sec {
				m, ok := s.(map[string]any)
				if !ok {
					continue
				}
				if _, ok := m["bearerAuth"]; ok {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s %s: security must include bearerAuth", strings.ToUpper(method), path)
			}
		}
	}
}

func TestOpenAPI_idempotencyParameterOnRetryableWrites(t *testing.T) {
	t.Parallel()
	var spec map[string]any
	if err := json.Unmarshal(swagger.OpenAPIJSON(), &spec); err != nil {
		t.Fatal(err)
	}
	paths, _ := spec["paths"].(map[string]any)
	// Keep in sync with tools/build_openapi.py IDEMPOTENCY_OPS (webhook is intentionally excluded).
	want := []struct {
		method, path string
	}{
		{"post", "/v1/commerce/orders"},
		{"post", "/v1/commerce/cash-checkout"},
		{"post", "/v1/commerce/orders/{orderId}/payment-session"},
		{"post", "/v1/commerce/orders/{orderId}/vend/start"},
		{"post", "/v1/commerce/orders/{orderId}/vend/success"},
		{"post", "/v1/commerce/orders/{orderId}/vend/failure"},
		{"post", "/v1/commerce/orders/{orderId}/cancel"},
		{"post", "/v1/commerce/orders/{orderId}/refunds"},
		{"post", "/v1/device/machines/{machineId}/vend-results"},
		{"post", "/v1/machines/{machineId}/commands/dispatch"},
		{"post", "/v1/admin/machines/{machineId}/planograms/publish"},
		{"post", "/v1/admin/machines/{machineId}/sync"},
		{"post", "/v1/admin/machines/{machineId}/stock-adjustments"},
		{"post", "/v1/admin/machines/{machineId}/cash-collections"},
		{"post", "/v1/admin/machines/{machineId}/diagnostics/requests"},
		{"post", "/v1/admin/products"},
		{"put", "/v1/admin/products/{productId}"},
		{"patch", "/v1/admin/products/{productId}"},
		{"delete", "/v1/admin/products/{productId}"},
		{"post", "/v1/admin/products/{productId}/image"},
		{"put", "/v1/admin/products/{productId}/image"},
		{"delete", "/v1/admin/products/{productId}/image"},
		{"post", "/v1/admin/brands"},
		{"put", "/v1/admin/brands/{brandId}"},
		{"patch", "/v1/admin/brands/{brandId}"},
		{"delete", "/v1/admin/brands/{brandId}"},
		{"post", "/v1/admin/categories"},
		{"put", "/v1/admin/categories/{categoryId}"},
		{"patch", "/v1/admin/categories/{categoryId}"},
		{"delete", "/v1/admin/categories/{categoryId}"},
		{"post", "/v1/admin/tags"},
		{"put", "/v1/admin/tags/{tagId}"},
		{"patch", "/v1/admin/tags/{tagId}"},
		{"delete", "/v1/admin/tags/{tagId}"},
	}
	for _, w := range want {
		entry, ok := paths[w.path].(map[string]any)
		if !ok {
			t.Fatalf("missing path %q", w.path)
		}
		op, ok := entry[w.method].(map[string]any)
		if !ok {
			t.Fatalf("missing %s %q", w.method, w.path)
		}
		params, ok := op["parameters"].([]any)
		if !ok {
			t.Fatalf("%s %q: missing parameters", w.method, w.path)
		}
		found := false
		for _, p := range params {
			m, ok := p.(map[string]any)
			if !ok {
				continue
			}
			if m["name"] == "Idempotency-Key" {
				found = true
				break
			}
			if ref, ok := m["$ref"].(string); ok && strings.HasSuffix(ref, "/IdempotencyKeyHeader") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s %q: expected Idempotency-Key parameter", w.method, w.path)
		}
	}
}

func TestOpenAPI_successAndErrorJSONExamples(t *testing.T) {
	t.Parallel()
	var spec map[string]any
	if err := json.Unmarshal(swagger.OpenAPIJSON(), &spec); err != nil {
		t.Fatal(err)
	}
	paths, _ := spec["paths"].(map[string]any)
	// Liveness/metrics probes intentionally use plain-text bodies for some statuses (not the JSON error envelope).
	jsonErrExampleExempt := map[string]map[string]struct{}{
		"/health/ready": {"get": {}},
		"/metrics":      {"get": {}},
	}
	for path, methods := range paths {
		for method, opAny := range methods.(map[string]any) {
			method = strings.ToLower(method)
			op := opAny.(map[string]any)
			responses, ok := op["responses"].(map[string]any)
			if !ok {
				t.Errorf("%s %s: missing responses", strings.ToUpper(method), path)
				continue
			}
			var has2xx, has2xxExample bool
			var has4xxJSONErrExample bool
			for code, respAny := range responses {
				var codeN int
				_, _ = fmt.Sscanf(code, "%d", &codeN)
				resp, ok := respAny.(map[string]any)
				if !ok {
					continue
				}
				content, _ := resp["content"].(map[string]any)
				if codeN >= 200 && codeN < 300 {
					has2xx = true
					if aj, ok := content["application/json"].(map[string]any); ok {
						if _, ok := aj["example"]; ok {
							has2xxExample = true
						}
					}
					if _, ok := content["text/plain"]; ok {
						has2xxExample = true
					}
					if _, ok := content["text/html"]; ok {
						has2xxExample = true
					}
					continue
				}
				if codeN < 400 {
					continue
				}
				if aj, ok := content["application/json"].(map[string]any); ok {
					if _, ok := aj["example"]; ok {
						has4xxJSONErrExample = true
					}
				}
			}
			if !has2xx {
				t.Errorf("%s %s: expected at least one 2xx response", strings.ToUpper(method), path)
				continue
			}
			if !has2xxExample {
				t.Errorf("%s %s: expected a 2xx example (JSON, text/plain, or text/html)", strings.ToUpper(method), path)
			}
			hasAnyErr := false
			for code := range responses {
				var codeN int
				_, _ = fmt.Sscanf(code, "%d", &codeN)
				if codeN >= 400 {
					hasAnyErr = true
					break
				}
			}
			if !hasAnyErr {
				continue
			}
			if !has4xxJSONErrExample {
				if skip, ok := jsonErrExampleExempt[path]; ok {
					if _, ok := skip[method]; ok {
						continue
					}
				}
				t.Errorf("%s %s: declare error responses with application/json examples", strings.ToUpper(method), path)
			}
		}
	}
}

// Keep in sync with tools/build_openapi.py REQUIRED_OPERATIONS (pilot / P0 subset below).
var requiredP0Operations = []struct {
	method, path string
}{
	{"post", "/v1/admin/machines/{machineId}/activation-codes"},
	{"get", "/v1/admin/machines/{machineId}/activation-codes"},
	{"delete", "/v1/admin/machines/{machineId}/activation-codes/{activationCodeId}"},
	{"post", "/v1/setup/activation-codes/claim"},
	{"get", "/v1/machines/{machineId}/sale-catalog"},
	{"post", "/v1/device/machines/{machineId}/events/reconcile"},
	{"get", "/v1/device/machines/{machineId}/events/{idempotencyKey}/status"},
	{"post", "/v1/commerce/orders/{orderId}/cancel"},
	{"post", "/v1/commerce/orders/{orderId}/refunds"},
	{"get", "/v1/commerce/orders/{orderId}/refunds"},
	{"get", "/v1/commerce/orders/{orderId}/refunds/{refundId}"},
	{"get", "/v1/admin/machines/{machineId}/cashbox"},
	{"post", "/v1/admin/machines/{machineId}/cash-collections"},
	{"post", "/v1/admin/machines/{machineId}/cash-collections/{collectionId}/close"},
	{"get", "/v1/admin/machines/{machineId}/cash-collections"},
	{"get", "/v1/admin/machines/{machineId}/cash-collections/{collectionId}"},
	{"post", "/v1/admin/products"},
	{"put", "/v1/admin/products/{productId}"},
	{"patch", "/v1/admin/products/{productId}"},
	{"delete", "/v1/admin/products/{productId}"},
	{"post", "/v1/admin/products/{productId}/image"},
	{"put", "/v1/admin/products/{productId}/image"},
	{"delete", "/v1/admin/products/{productId}/image"},
	{"post", "/v1/admin/media/assets"},
	{"post", "/v1/admin/media/uploads"},
	{"post", "/v1/admin/media/{mediaId}/complete"},
	{"get", "/v1/admin/media/assets"},
	{"get", "/v1/admin/media/assets/{mediaId}"},
	{"get", "/v1/admin/media"},
	{"get", "/v1/admin/media/{mediaId}"},
	{"delete", "/v1/admin/media/assets/{mediaId}"},
	{"delete", "/v1/admin/media/{mediaId}"},
	{"post", "/v1/admin/products/{productId}/media"},
	{"put", "/v1/admin/products/{productId}/media"},
	{"delete", "/v1/admin/products/{productId}/media/{mediaId}"},
	{"post", "/v1/admin/brands"},
	{"put", "/v1/admin/brands/{brandId}"},
	{"patch", "/v1/admin/brands/{brandId}"},
	{"delete", "/v1/admin/brands/{brandId}"},
	{"post", "/v1/admin/categories"},
	{"put", "/v1/admin/categories/{categoryId}"},
	{"patch", "/v1/admin/categories/{categoryId}"},
	{"delete", "/v1/admin/categories/{categoryId}"},
	{"post", "/v1/admin/tags"},
	{"put", "/v1/admin/tags/{tagId}"},
	{"patch", "/v1/admin/tags/{tagId}"},
	{"delete", "/v1/admin/tags/{tagId}"},
}

func TestOpenAPI_embeddedJSON_requiredP0PathsPresent(t *testing.T) {
	t.Parallel()
	var spec map[string]any
	if err := json.Unmarshal(swagger.OpenAPIJSON(), &spec); err != nil {
		t.Fatal(err)
	}
	paths, _ := spec["paths"].(map[string]any)
	for _, op := range requiredP0Operations {
		entry, ok := paths[op.path].(map[string]any)
		if !ok {
			t.Fatalf("missing path %q", op.path)
		}
		if _, ok := entry[op.method]; !ok {
			t.Fatalf("missing %s %q", op.method, op.path)
		}
	}
}

func TestOpenAPI_adminCatalogReadDocumentedForPortal(t *testing.T) {
	t.Parallel()
	var spec map[string]any
	if err := json.Unmarshal(swagger.OpenAPIJSON(), &spec); err != nil {
		t.Fatal(err)
	}
	paths, _ := spec["paths"].(map[string]any)
	entry, ok := paths["/v1/admin/products"].(map[string]any)
	if !ok {
		t.Fatal(`missing /v1/admin/products in OpenAPI`)
	}
	op, ok := entry["get"].(map[string]any)
	if !ok {
		t.Fatal(`missing GET /v1/admin/products`)
	}
	if strings.TrimSpace(fmt.Sprint(op["operationId"])) == "" {
		t.Fatal("missing operationId for GET /v1/admin/products")
	}
}

func TestOpenAPI_machineInternalGRPCNotDocumentedAsPublicHTTP(t *testing.T) {
	t.Parallel()
	var spec map[string]any
	if err := json.Unmarshal(swagger.OpenAPIJSON(), &spec); err != nil {
		t.Fatal(err)
	}
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("missing paths")
	}
	for p := range paths {
		pl := strings.ToLower(p)
		if strings.HasPrefix(pl, "/v1/internal") {
			t.Fatalf("must not document internal avf.internal.v1 bridge as public HTTP: %q", p)
		}
		if strings.Contains(pl, "/grpc/") || strings.HasSuffix(pl, "/grpc") {
			t.Fatalf("must not document gRPC transports in OpenAPI paths: %q", p)
		}
	}
}
