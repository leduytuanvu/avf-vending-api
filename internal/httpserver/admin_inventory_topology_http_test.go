package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/api"
	appinventoryadmin "github.com/avf/avf-vending-api/internal/app/inventoryadmin"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func topologyHTTPApp(t *testing.T) (*api.HTTPApplication, *pgxpool.Pool) {
	t.Helper()
	pool := operatorSessionPolicyHTTPPool(t)
	inv, err := appinventoryadmin.NewService(db.New(pool))
	require.NoError(t, err)
	app := operatorSessionPolicyHTTPApp(t, pool)
	app.InventoryAdmin = inv
	return app, pool
}

func canonicalTopologyBody(t *testing.T, status any) []byte {
	t.Helper()
	layout := map[string]any{
		"cabinetCode": "A",
		"layoutKey":   "grid-10x6",
		"revision":    1,
		"layoutSpec": map[string]any{
			"rows": 6,
			"cols": 10,
		},
	}
	if status != nil {
		layout["status"] = status
	}
	body, err := json.Marshal(map[string]any{
		"cabinets": []any{
			map[string]any{
				"code":      "A",
				"title":     "Cabinet A",
				"sortOrder": 1,
				"metadata":  map[string]any{},
			},
		},
		"layouts": []any{layout},
	})
	require.NoError(t, err)
	return body
}

func putTopologyHTTP(t *testing.T, app *api.HTTPApplication, machineID uuid.UUID, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Method(http.MethodPut, "/machines/{machineId}/topology", putAdminMachineTopology(app))
	req := httptest.NewRequest(http.MethodPut, "/machines/"+machineID.String()+"/topology", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withPlatformAdminPrincipal(req)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func cleanupTopologyHTTPArtifacts(ctx context.Context, t *testing.T, pool *pgxpool.Pool, machineID uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `DELETE FROM machine_slot_configs WHERE machine_id = $1`, machineID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM machine_slot_layouts WHERE machine_id = $1`, machineID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM machine_cabinets WHERE machine_id = $1`, machineID)
	require.NoError(t, err)
}

func TestPutAdminMachineTopology_layoutStatusPublished_succeeds(t *testing.T) {
	app, pool := topologyHTTPApp(t)
	ctx := context.Background()
	machineID := testfixtures.DevMachineID
	defer cleanupTopologyHTTPArtifacts(ctx, t, pool, machineID)

	rec := putTopologyHTTP(t, app, machineID, canonicalTopologyBody(t, "published"))
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	var status string
	err := pool.QueryRow(ctx, `
SELECT status FROM machine_slot_layouts
WHERE machine_id = $1 AND layout_key = 'grid-10x6' AND revision = 1
`, machineID).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "published", status)
}

func TestPutAdminMachineTopology_layoutStatusOmitted_defaultsPublished(t *testing.T) {
	app, pool := topologyHTTPApp(t)
	ctx := context.Background()
	machineID := testfixtures.DevMachineID
	defer cleanupTopologyHTTPArtifacts(ctx, t, pool, machineID)

	rec := putTopologyHTTP(t, app, machineID, canonicalTopologyBody(t, nil))
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	var status string
	err := pool.QueryRow(ctx, `
SELECT status FROM machine_slot_layouts
WHERE machine_id = $1 AND layout_key = 'grid-10x6' AND revision = 1
`, machineID).Scan(&status)
	require.NoError(t, err)
	require.Equal(t, "published", status)
}

func TestPutAdminMachineTopology_layoutStatusActive_rejected(t *testing.T) {
	app, pool := topologyHTTPApp(t)
	ctx := context.Background()
	machineID := testfixtures.DevMachineID
	defer cleanupTopologyHTTPArtifacts(ctx, t, pool, machineID)

	rec := putTopologyHTTP(t, app, machineID, canonicalTopologyBody(t, "active"))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	var errBody map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &errBody))
	errObj, ok := errBody["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "invalid_layout_status", errObj["code"])

	var count int
	err := pool.QueryRow(ctx, `
SELECT count(*) FROM machine_slot_layouts
WHERE machine_id = $1 AND layout_key = 'grid-10x6' AND revision = 1
`, machineID).Scan(&count)
	require.NoError(t, err)
	require.Zero(t, count)
}
