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

	"github.com/avf/avf-vending-api/internal/app/activation"
	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/config"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func activationHTTPTestDSN(t *testing.T) string {
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

func activationHTTPTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := activationHTTPTestDSN(t)
	testfixtures.EnsureTestMigrations(t, dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func activationHTTPTestService(t *testing.T, pool *pgxpool.Pool) *activation.Service {
	t.Helper()
	cfg := config.HTTPAuthConfig{
		Mode:            plauth.HTTPAuthModeHS256,
		JWTSecret:       bytes.Repeat([]byte("z"), 32),
		JWTLeeway:       30 * time.Second,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 720 * time.Hour,
	}
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg)
	require.NoError(t, err)
	return activation.NewService(pool, issuer, plauth.TrimSecret(cfg.JWTSecret), nil)
}

func activationHTTPTestRouter(t *testing.T, svc *activation.Service) chi.Router {
	t.Helper()
	app := &api.HTTPApplication{Activation: svc}
	r := chi.NewRouter()
	writeRL := func(h http.Handler) http.Handler { return h }
	mountAdminActivationRoutes(r, app, writeRL)
	mountAdminCompanyScopedActivationRoutes(r, app, writeRL)
	return r
}

func withAdminPrincipal(req *http.Request) *http.Request {
	p := plauth.Principal{Subject: uuid.NewString(), Roles: []string{"org_admin"}}
	return req.WithContext(plauth.WithPrincipal(req.Context(), p))
}

func insertActivationHTTPTestMachine(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, string) {
	t.Helper()
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	code := "AVF000301"
	_, err := pool.Exec(ctx, `INSERT INTO sites (id, name, code, status) VALUES ($1, 's', '', 'active')`, siteID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, site_id, serial_number, code, status, credential_version)
VALUES ($1, $2, $3, $4, 'online', 0)`, machineID, siteID, "sn-http-"+uuid.NewString()[:8], code)
	require.NoError(t, err)
	return machineID, code
}

func TestMountV1_adminMachineCodeActivationRoutesRegistered(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	app := &api.HTTPApplication{Activation: &activation.Service{}}
	writeRL := func(h http.Handler) http.Handler { return h }
	mountAdminActivationRoutes(r, app, writeRL)

	want := []string{
		"GET /machine-codes/{machineCode}/activation-codes",
		"POST /machine-codes/{machineCode}/activation-codes",
		"DELETE /machine-codes/{machineCode}/activation-codes/{activationCodeId}",
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

func TestAdminCreateActivationCode_byMachineCodePath(t *testing.T) {
	t.Parallel()
	pool := activationHTTPTestPool(t)
	svc := activationHTTPTestService(t, pool)
	r := activationHTTPTestRouter(t, svc)
	machineID, code := insertActivationHTTPTestMachine(t, pool)

	body := bytes.NewBufferString(`{"expiresInMinutes":60,"maxUses":1}`)
	req := httptest.NewRequest(http.MethodPost, "/machine-codes/"+code+"/activation-codes", body)
	req.Header.Set("Content-Type", "application/json")
	req = withAdminPrincipal(req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, code, resp["machineCode"])
	require.Equal(t, machineID.String(), resp["machineId"])
	require.NotEmpty(t, resp["activationCode"])
	require.Regexp(t, `^[0-9]{6}$`, resp["activationCode"])
	_, ok := resp["activationCode"].(string)
	require.True(t, ok, "activationCode must be JSON string")
}

func TestAdminCreateActivationCode_byMachineUUIDPath(t *testing.T) {
	t.Parallel()
	pool := activationHTTPTestPool(t)
	svc := activationHTTPTestService(t, pool)
	r := activationHTTPTestRouter(t, svc)
	machineID, code := insertActivationHTTPTestMachine(t, pool)

	body := bytes.NewBufferString(`{"expiresInMinutes":60,"maxUses":1}`)
	req := httptest.NewRequest(http.MethodPost, "/machines/"+machineID.String()+"/activation-codes", body)
	req.Header.Set("Content-Type", "application/json")
	req = withAdminPrincipal(req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, code, resp["machineCode"])
}

func TestAdminCreateActivationCode_byMachineCodeInMachinePath(t *testing.T) {
	t.Parallel()
	pool := activationHTTPTestPool(t)
	svc := activationHTTPTestService(t, pool)
	r := activationHTTPTestRouter(t, svc)
	machineID, code := insertActivationHTTPTestMachine(t, pool)

	body := bytes.NewBufferString(`{"expiresInMinutes":60,"maxUses":1}`)
	req := httptest.NewRequest(http.MethodPost, "/machines/"+code+"/activation-codes", body)
	req.Header.Set("Content-Type", "application/json")
	req = withAdminPrincipal(req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, code, resp["machineCode"])
	require.Equal(t, machineID.String(), resp["machineId"])
}

func TestAdminCreateActivationCode_invalidMachineIdentifier(t *testing.T) {
	t.Parallel()
	pool := activationHTTPTestPool(t)
	svc := activationHTTPTestService(t, pool)
	r := activationHTTPTestRouter(t, svc)

	body := bytes.NewBufferString(`{"expiresInMinutes":60,"maxUses":1}`)
	req := httptest.NewRequest(http.MethodPost, "/machine-codes/NOTVALID/activation-codes", body)
	req.Header.Set("Content-Type", "application/json")
	req = withAdminPrincipal(req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "invalid_machine_identifier", resp["code"])
}

func TestAdminCreateActivationCode_machineNotFound(t *testing.T) {
	t.Parallel()
	pool := activationHTTPTestPool(t)
	svc := activationHTTPTestService(t, pool)
	r := activationHTTPTestRouter(t, svc)

	body := bytes.NewBufferString(`{"expiresInMinutes":60,"maxUses":1}`)
	req := httptest.NewRequest(http.MethodPost, "/machine-codes/AVF999999/activation-codes", body)
	req.Header.Set("Content-Type", "application/json")
	req = withAdminPrincipal(req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminListActivationCodes_excludesPlaintextAndHash(t *testing.T) {
	t.Parallel()
	pool := activationHTTPTestPool(t)
	svc := activationHTTPTestService(t, pool)
	r := activationHTTPTestRouter(t, svc)
	_, code := insertActivationHTTPTestMachine(t, pool)

	createBody := bytes.NewBufferString(`{"expiresInMinutes":60,"maxUses":1}`)
	createReq := httptest.NewRequest(http.MethodPost, "/machine-codes/"+code+"/activation-codes", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withAdminPrincipal(createReq)
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	plain := created["activationCode"].(string)

	listReq := httptest.NewRequest(http.MethodGet, "/machine-codes/"+code+"/activation-codes", nil)
	listReq = withAdminPrincipal(listReq)
	listRec := httptest.NewRecorder()
	r.ServeHTTP(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code)

	body := listRec.Body.String()
	require.NotContains(t, body, plain)
	require.NotContains(t, strings.ToLower(body), "codehash")
	require.Contains(t, body, `"machineCode":"`+code+`"`)
}

func TestAdminCatalogCreateActivationCode_bodyMachineCode(t *testing.T) {
	t.Parallel()
	pool := activationHTTPTestPool(t)
	svc := activationHTTPTestService(t, pool)
	r := activationHTTPTestRouter(t, svc)
	machineID, code := insertActivationHTTPTestMachine(t, pool)

	body := bytes.NewBufferString(`{"machineCode":"` + code + `","expiresInMinutes":60,"maxUses":1}`)
	req := httptest.NewRequest(http.MethodPost, "/activation-codes", body)
	req.Header.Set("Content-Type", "application/json")
	req = withAdminPrincipal(req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, machineID.String(), resp["machineId"])
	require.Equal(t, code, resp["machineCode"])
}

func TestAdminCatalogCreateActivationCode_identifierConflict(t *testing.T) {
	t.Parallel()
	pool := activationHTTPTestPool(t)
	svc := activationHTTPTestService(t, pool)
	r := activationHTTPTestRouter(t, svc)
	machineA, _ := insertActivationHTTPTestMachine(t, pool)
	_, codeB := insertActivationHTTPTestMachine(t, pool)

	body := bytes.NewBufferString(`{"machineId":"` + machineA.String() + `","machineCode":"` + codeB + `","expiresInMinutes":60,"maxUses":1}`)
	req := httptest.NewRequest(http.MethodPost, "/activation-codes", body)
	req.Header.Set("Content-Type", "application/json")
	req = withAdminPrincipal(req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, "machine_identifier_conflict", resp["code"])
}

func TestAdminCatalogCreateActivationCode_bodyMachineCodeSnake(t *testing.T) {
	t.Parallel()
	pool := activationHTTPTestPool(t)
	svc := activationHTTPTestService(t, pool)
	r := activationHTTPTestRouter(t, svc)
	machineID, code := insertActivationHTTPTestMachine(t, pool)

	body := bytes.NewBufferString(`{"machine_code":"` + code + `","expiresInMinutes":60,"maxUses":1}`)
	req := httptest.NewRequest(http.MethodPost, "/activation-codes", body)
	req.Header.Set("Content-Type", "application/json")
	req = withAdminPrincipal(req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, machineID.String(), resp["machineId"])
	require.Equal(t, code, resp["machineCode"])
}

func TestAdminCatalogCreateActivationCode_bodyMachineIDSnake(t *testing.T) {
	t.Parallel()
	pool := activationHTTPTestPool(t)
	svc := activationHTTPTestService(t, pool)
	r := activationHTTPTestRouter(t, svc)
	machineID, code := insertActivationHTTPTestMachine(t, pool)

	body := bytes.NewBufferString(`{"machine_id":"` + machineID.String() + `","expiresInMinutes":60,"maxUses":1}`)
	req := httptest.NewRequest(http.MethodPost, "/activation-codes", body)
	req.Header.Set("Content-Type", "application/json")
	req = withAdminPrincipal(req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Equal(t, machineID.String(), resp["machineId"])
	require.Equal(t, code, resp["machineCode"])
}

func TestAdminDeleteActivationCode_byMachineCodePath(t *testing.T) {
	t.Parallel()
	pool := activationHTTPTestPool(t)
	svc := activationHTTPTestService(t, pool)
	r := activationHTTPTestRouter(t, svc)
	_, code := insertActivationHTTPTestMachine(t, pool)

	createBody := bytes.NewBufferString(`{"expiresInMinutes":60,"maxUses":1}`)
	createReq := httptest.NewRequest(http.MethodPost, "/machine-codes/"+code+"/activation-codes", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createReq = withAdminPrincipal(createReq)
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)
	require.Equal(t, http.StatusCreated, createRec.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(createRec.Body.Bytes(), &created))
	codeID := created["activationCodeId"].(string)

	delReq := httptest.NewRequest(http.MethodDelete, "/machine-codes/"+code+"/activation-codes/"+codeID, nil)
	delReq = withAdminPrincipal(delReq)
	delRec := httptest.NewRecorder()
	r.ServeHTTP(delRec, delReq)
	require.Equal(t, http.StatusNoContent, delRec.Code)
}
