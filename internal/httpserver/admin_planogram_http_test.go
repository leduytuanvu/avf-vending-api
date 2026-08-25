package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func planogramHTTPTestDSN(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration tests in -short mode")
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	return dsn
}

func planogramHTTPTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := planogramHTTPTestDSN(t)
	testfixtures.EnsureTestMigrations(t, dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func planogramHTTPTestCatalogAdmin(t *testing.T, pool *pgxpool.Pool) *appcatalogadmin.Service {
	t.Helper()
	q := db.New(pool)
	svc, err := appcatalogadmin.NewService(q, pool, nil)
	require.NoError(t, err)
	return svc
}

func planogramHTTPTestRouter(t *testing.T, svc *appcatalogadmin.Service) chi.Router {
	t.Helper()
	app := &api.HTTPApplication{CatalogAdmin: svc}
	r := chi.NewRouter()
	writeRL := func(h http.Handler) http.Handler { return h }
	mountAdminCatalogRoutes(r, app, writeRL)
	return r
}

func TestMountAdminCatalogRoutes_planogramWriteRoutesRegistered(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	app := &api.HTTPApplication{CatalogAdmin: new(appcatalogadmin.Service)}
	writeRL := func(h http.Handler) http.Handler { return h }
	mountAdminCatalogRoutes(r, app, writeRL)

	want := []string{
		"POST /planograms",
		"PUT /planograms/{planogramId}/slots",
	}
	var routes []string
	require.NoError(t, chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, method+" "+route)
		return nil
	}))
	for _, w := range want {
		found := false
		for _, got := range routes {
			if strings.Contains(got, w) {
				found = true
				break
			}
		}
		require.True(t, found, "missing route %q in %v", w, routes)
	}
}

func TestPostAdminPlanogramCreate_withMeta(t *testing.T) {
	pool := planogramHTTPTestPool(t)
	svc := planogramHTTPTestCatalogAdmin(t, pool)
	r := planogramHTTPTestRouter(t, svc)

	machineID := uuid.NewString()
	name := "Meta Planogram " + uuid.NewString()[:8]
	createBody, err := json.Marshal(map[string]any{
		"name":   name,
		"status": "draft",
		"meta": map[string]any{
			"scope":     "machine",
			"machineId": machineID,
		},
	})
	require.NoError(t, err)

	createReq := httptest.NewRequest(http.MethodPost, "/planograms", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Idempotency-Key", "planogram-create-meta-"+uuid.NewString())
	createReq = withCatalogAdminPrincipal(createReq)

	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code, createRec.Body.String())

	var created V1AdminPlanogram
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	require.Equal(t, name, created.Name)
	require.Equal(t, "draft", created.Status)
	require.JSONEq(t, `{"scope":"machine","machineId":"`+machineID+`"}`, string(created.Meta))
}

func TestPostAdminPlanogramCreate_andPutSlots(t *testing.T) {
	pool := planogramHTTPTestPool(t)
	svc := planogramHTTPTestCatalogAdmin(t, pool)
	r := planogramHTTPTestRouter(t, svc)

	name := "UI Test Planogram " + uuid.NewString()[:8]
	createBody, err := json.Marshal(map[string]any{
		"name":   name,
		"status": "published",
	})
	require.NoError(t, err)

	createReq := httptest.NewRequest(http.MethodPost, "/planograms", bytes.NewReader(createBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Idempotency-Key", "planogram-create-"+uuid.NewString())
	createReq = withCatalogAdminPrincipal(createReq)

	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code, createRec.Body.String())

	var created V1AdminPlanogram
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	require.NotEmpty(t, created.ID)
	require.Equal(t, name, created.Name)

	slotsBody, err := json.Marshal(map[string]any{
		"slots": []map[string]any{
			{"slotIndex": 1, "maxQuantity": 10},
			{"slotIndex": 2, "maxQuantity": 8},
		},
	})
	require.NoError(t, err)

	slotsReq := httptest.NewRequest(http.MethodPut, "/planograms/"+created.ID+"/slots", bytes.NewReader(slotsBody))
	slotsReq.Header.Set("Content-Type", "application/json")
	slotsReq.Header.Set("Idempotency-Key", "planogram-slots-"+uuid.NewString())
	slotsReq = withCatalogAdminPrincipal(slotsReq)

	slotsRec := httptest.NewRecorder()
	r.ServeHTTP(slotsRec, slotsReq)
	require.Equal(t, http.StatusOK, slotsRec.Code, slotsRec.Body.String())

	var detail V1AdminPlanogramDetail
	require.NoError(t, json.Unmarshal(slotsRec.Body.Bytes(), &detail))
	require.Len(t, detail.Slots, 2)
}

func TestPutAdminPlanogramSlotsReplace_notFound(t *testing.T) {
	pool := planogramHTTPTestPool(t)
	svc := planogramHTTPTestCatalogAdmin(t, pool)
	r := planogramHTTPTestRouter(t, svc)

	missingID := id.NewUUIDV7()
	body, err := json.Marshal(map[string]any{"slots": []map[string]any{}})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPut, "/planograms/"+missingID.String()+"/slots", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "planogram-slots-missing-"+uuid.NewString())
	req = withCatalogAdminPrincipal(req)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func withCatalogAdminPrincipal(req *http.Request) *http.Request {
	p := plauth.Principal{
		Subject: uuid.NewString(),
		Roles:   []string{plauth.RoleOrgAdmin},
	}
	return req.WithContext(plauth.WithPrincipal(req.Context(), p))
}
