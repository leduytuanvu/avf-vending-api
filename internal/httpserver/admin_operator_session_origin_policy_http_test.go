package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	appinventoryadmin "github.com/avf/avf-vending-api/internal/app/inventoryadmin"
	appoperator "github.com/avf/avf-vending-api/internal/app/operator"
	domainoperator "github.com/avf/avf-vending-api/internal/domain/operator"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func operatorSessionPolicyHTTPPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration tests in -short mode")
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	testfixtures.EnsureTestMigrations(t, dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func operatorSessionPolicyHTTPApp(t *testing.T, pool *pgxpool.Pool) *api.HTTPApplication {
	t.Helper()
	inv, err := appinventoryadmin.NewService(db.New(pool))
	require.NoError(t, err)
	op := appoperator.NewService(
		postgres.NewOperatorRepository(pool),
		postgres.NewMachineRepository(pool),
		postgres.NewTechnicianRepository(pool),
		postgres.NewTechnicianAssignmentRepository(pool),
	)
	return &api.HTTPApplication{
		InventoryAdmin:  inv,
		MachineOperator: op,
		TelemetryStore:  postgres.NewStore(pool),
	}
}

func withPlatformAdminPrincipal(req *http.Request) *http.Request {
	p := plauth.Principal{
		Subject: "operator-session-policy-admin",
		Roles:   []string{plauth.RolePlatformAdmin},
	}
	return req.WithContext(plauth.WithPrincipal(req.Context(), p))
}

func serveMachineHandler(t *testing.T, machineID uuid.UUID, method, pathSuffix string, h http.HandlerFunc, body []byte, hdr map[string]string, withPrincipal bool) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Method(method, "/machines/{machineId}"+pathSuffix, h)
	req := httptest.NewRequest(method, "/machines/"+machineID.String()+pathSuffix, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	if withPrincipal {
		req = withPlatformAdminPrincipal(req)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestAdminTopology_omitOperatorSession_succeedsWithAPIAttribution(t *testing.T) {
	pool := operatorSessionPolicyHTTPPool(t)
	app := operatorSessionPolicyHTTPApp(t, pool)
	mid := testfixtures.DevMachineID
	before := countOperatorSessions(t, pool, mid)

	body, err := json.Marshal(map[string]any{"cabinets": []any{}, "layouts": []any{}})
	require.NoError(t, err)
	rec := serveMachineHandler(t, mid, http.MethodPut, "/topology", putAdminMachineTopology(app), body, nil, true)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	require.Equal(t, before, countOperatorSessions(t, pool, mid), "remote topology must not insert machine_operator_sessions")
	origin, sessNull := latestAttributionOrigin(t, pool, mid, "machine_cabinets")
	require.Equal(t, domainoperator.ActionOriginAPI, origin)
	require.True(t, sessNull)
}

func TestAdminTopology_unauthenticated_rejected(t *testing.T) {
	pool := operatorSessionPolicyHTTPPool(t)
	app := operatorSessionPolicyHTTPApp(t, pool)
	body, err := json.Marshal(map[string]any{"cabinets": []any{}, "layouts": []any{}})
	require.NoError(t, err)
	rec := serveMachineHandler(t, testfixtures.DevMachineID, http.MethodPut, "/topology", putAdminMachineTopology(app), body, nil, false)
	require.True(t, rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden, rec.Body.String())
}

func TestAdminTopology_invalidSessionRejected(t *testing.T) {
	pool := operatorSessionPolicyHTTPPool(t)
	app := operatorSessionPolicyHTTPApp(t, pool)
	body, err := json.Marshal(map[string]any{
		"operator_session_id": uuid.NewString(),
		"cabinets":            []any{},
		"layouts":             []any{},
	})
	require.NoError(t, err)
	rec := serveMachineHandler(t, testfixtures.DevMachineID, http.MethodPut, "/topology", putAdminMachineTopology(app), body, nil, true)
	require.True(t, rec.Code >= 400 && rec.Code < 500, rec.Body.String())
}

func TestAdminCashCollection_omitSessionRejected(t *testing.T) {
	pool := operatorSessionPolicyHTTPPool(t)
	app := operatorSessionPolicyHTTPApp(t, pool)
	body, err := json.Marshal(map[string]any{"currency": "USD"})
	require.NoError(t, err)
	rec := serveMachineHandler(t, testfixtures.DevMachineID, http.MethodPost, "/cash-collections", postAdminMachineCashCollectionStart(app), body, map[string]string{
		"Idempotency-Key": uuid.NewString(),
	}, true)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestAdminTopology_doesNotTakeoverActiveTechnicianSession(t *testing.T) {
	pool := operatorSessionPolicyHTTPPool(t)
	app := operatorSessionPolicyHTTPApp(t, pool)
	ctx := context.Background()
	mid := testfixtures.DevMachineID
	tid := testfixtures.DevTechnicianID

	existing, err := app.MachineOperator.ActiveSessionForMachine(ctx, mid)
	require.NoError(t, err)
	if existing != nil {
		_, err = app.MachineOperator.EndOperatorSession(ctx, appoperator.EndOperatorSessionInput{
			MachineID:   mid,
			SessionID:   existing.ID,
			FinalStatus: domainoperator.SessionStatusEnded,
		})
		require.NoError(t, err)
	}

	sess, err := app.MachineOperator.StartOperatorSession(ctx, appoperator.StartOperatorSessionInput{
		MachineID:         mid,
		ActorType:         domainoperator.ActorTypeTechnician,
		TechnicianID:      &tid,
		InitialAuthMethod: domainoperator.AuthMethodBadge,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = app.MachineOperator.EndOperatorSession(context.Background(), appoperator.EndOperatorSessionInput{
			MachineID:   mid,
			SessionID:   sess.ID,
			FinalStatus: domainoperator.SessionStatusEnded,
		})
	})

	before := countOperatorSessions(t, pool, mid)
	body, err := json.Marshal(map[string]any{"cabinets": []any{}, "layouts": []any{}})
	require.NoError(t, err)
	rec := serveMachineHandler(t, mid, http.MethodPut, "/topology", putAdminMachineTopology(app), body, nil, true)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	active, err := app.MachineOperator.ActiveSessionForMachine(ctx, mid)
	require.NoError(t, err)
	require.NotNil(t, active)
	require.Equal(t, sess.ID, active.ID)
	require.Equal(t, before, countOperatorSessions(t, pool, mid))
}

func countOperatorSessions(t *testing.T, pool *pgxpool.Pool, machineID uuid.UUID) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(), `SELECT count(*) FROM machine_operator_sessions WHERE machine_id = $1`, machineID).Scan(&n)
	require.NoError(t, err)
	return n
}

func latestAttributionOrigin(t *testing.T, pool *pgxpool.Pool, machineID uuid.UUID, resourceType string) (origin string, sessionNull bool) {
	t.Helper()
	var sessID *uuid.UUID
	err := pool.QueryRow(context.Background(), `
SELECT action_origin_type, operator_session_id
FROM machine_action_attributions
WHERE machine_id = $1 AND resource_type = $2
ORDER BY occurred_at DESC
LIMIT 1`, machineID, resourceType).Scan(&origin, &sessID)
	require.NoError(t, err)
	return origin, sessID == nil
}
