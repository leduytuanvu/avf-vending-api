package httpserver

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestPatchAdminProduct_primaryImageUrlWithoutMediaAdmin_capabilityNotConfigured(t *testing.T) {
	pool := planogramHTTPTestPool(t)
	svc := planogramHTTPTestCatalogAdmin(t, pool)
	r := planogramHTTPTestRouter(t, svc)

	createBody, err := json.Marshal(map[string]any{
		"sku":           "IMGURL-" + uuid.NewString()[:8],
		"name":          "Draft Image URL",
		"active":        false,
		"ageRestricted": false,
		"allergenCodes": []string{},
	})
	require.NoError(t, err)

	createReq := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Idempotency-Key", "product-create-"+uuid.NewString())
	createReq = withCatalogAdminPrincipal(createReq)
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusOK, createRec.Code, createRec.Body.String())

	var created V1AdminProduct
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)

	patchBody, err := json.Marshal(map[string]any{
		"name":            created.Name,
		"active":          false,
		"ageRestricted":   false,
		"allergenCodes":   []string{},
		"primaryImageUrl": "https://cdn.example.com/product.png",
	})
	require.NoError(t, err)

	patchReq := httptest.NewRequest(http.MethodPatch, "/products/"+created.ID, bytes.NewReader(patchBody))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq.Header.Set("Idempotency-Key", "product-patch-url-"+uuid.NewString())
	patchReq = withCatalogAdminPrincipal(patchReq)
	patchRec := httptest.NewRecorder()
	r.ServeHTTP(patchRec, patchReq)
	require.Equal(t, http.StatusServiceUnavailable, patchRec.Code, patchRec.Body.String())
	require.Contains(t, patchRec.Body.String(), "capability_not_configured")
}

func TestPatchAdminProduct_omitPrimaryImageUrl_keepsDraft(t *testing.T) {
	pool := planogramHTTPTestPool(t)
	svc := planogramHTTPTestCatalogAdmin(t, pool)
	r := planogramHTTPTestRouter(t, svc)

	createBody, err := json.Marshal(map[string]any{
		"sku":           "NOIMG-" + uuid.NewString()[:8],
		"name":          "Keep Image",
		"active":        false,
		"ageRestricted": false,
		"allergenCodes": []string{},
	})
	require.NoError(t, err)

	createReq := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Idempotency-Key", "product-create-keep-"+uuid.NewString())
	createReq = withCatalogAdminPrincipal(createReq)
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusOK, createRec.Code, createRec.Body.String())

	var created V1AdminProduct
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	patchBody, err := json.Marshal(map[string]any{
		"name":          "Renamed Keep Image",
		"active":        false,
		"ageRestricted": false,
		"allergenCodes": []string{},
	})
	require.NoError(t, err)

	patchReq := httptest.NewRequest(http.MethodPatch, "/products/"+created.ID, bytes.NewReader(patchBody))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq.Header.Set("Idempotency-Key", "product-patch-keep-"+uuid.NewString())
	patchReq = withCatalogAdminPrincipal(patchReq)
	patchRec := httptest.NewRecorder()
	r.ServeHTTP(patchRec, patchReq)
	require.Equal(t, http.StatusOK, patchRec.Code, patchRec.Body.String())

	var updated V1AdminProduct
	require.NoError(t, json.Unmarshal(patchRec.Body.Bytes(), &updated))
	require.Equal(t, "Renamed Keep Image", updated.Name)
	require.Nil(t, updated.PrimaryMediaID)
}

func TestPostAdminProduct_activeWithoutMedia_invalidArgument(t *testing.T) {
	pool := planogramHTTPTestPool(t)
	svc := planogramHTTPTestCatalogAdmin(t, pool)
	r := planogramHTTPTestRouter(t, svc)

	createBody, err := json.Marshal(map[string]any{
		"sku":           "ACTIVE-" + uuid.NewString()[:8],
		"name":          "Active No Image",
		"active":        true,
		"ageRestricted": false,
		"allergenCodes": []string{},
	})
	require.NoError(t, err)

	createReq := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Idempotency-Key", "product-create-active-"+uuid.NewString())
	createReq = withCatalogAdminPrincipal(createReq)
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusBadRequest, createRec.Code, createRec.Body.String())
	require.Contains(t, createRec.Body.String(), "invalid_argument")
	require.Contains(t, createRec.Body.String(), "primaryMediaId")
}

func TestPatchAdminProduct_primaryMediaIdDoesNotRegisterExternalURL(t *testing.T) {
	pool := planogramHTTPTestPool(t)
	svc := planogramHTTPTestCatalogAdmin(t, pool)
	r := planogramHTTPTestRouter(t, svc)

	createBody, err := json.Marshal(map[string]any{
		"sku":           "MEDID-" + uuid.NewString()[:8],
		"name":          "Media ID Wins",
		"active":        false,
		"ageRestricted": false,
		"allergenCodes": []string{},
	})
	require.NoError(t, err)

	createReq := httptest.NewRequest(http.MethodPost, "/products", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Idempotency-Key", "product-create-mediaid-"+uuid.NewString())
	createReq = withCatalogAdminPrincipal(createReq)
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusOK, createRec.Code, createRec.Body.String())

	var created V1AdminProduct
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))

	mediaID := uuid.NewString()
	patchBody, err := json.Marshal(map[string]any{
		"name":            created.Name,
		"active":          false,
		"ageRestricted":   false,
		"allergenCodes":   []string{},
		"primaryMediaId":  mediaID,
		"primaryImageUrl": "https://cdn.example.com/ignored.png",
	})
	require.NoError(t, err)

	patchReq := httptest.NewRequest(http.MethodPatch, "/products/"+created.ID, bytes.NewReader(patchBody))
	patchReq.Header.Set("Content-Type", "application/json")
	patchReq.Header.Set("Idempotency-Key", "product-patch-mediaid-"+uuid.NewString())
	patchReq = withCatalogAdminPrincipal(patchReq)
	patchRec := httptest.NewRecorder()
	r.ServeHTTP(patchRec, patchReq)
	require.NotEqual(t, http.StatusServiceUnavailable, patchRec.Code, patchRec.Body.String())
	require.Equal(t, http.StatusBadRequest, patchRec.Code, patchRec.Body.String())
	require.Contains(t, patchRec.Body.String(), "invalid_argument")
	require.NotContains(t, patchRec.Body.String(), "capability_not_configured")
}
