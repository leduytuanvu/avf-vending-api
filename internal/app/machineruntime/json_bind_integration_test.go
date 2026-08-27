package machineruntime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Uncast jsonb INSERT matching production sqlc before FIX-RS-01 ( []byte binds, no ::text::jsonb ).
const uncastStartRuntimeSessionSQL = `
INSERT INTO machine_runtime_app_sessions (
    machine_id, device_attachment_id, machine_session_id, operator_session_id,
    previous_runtime_session_id, boot_id, app_start_id, app_instance_id,
    package_name, app_version, app_build_sha, start_reason, status,
    last_network_state, last_mqtt_state, storefront_state, sell_ready,
    blockers, hardware_status, catalog_status, outbox_status, recovery_status, metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17,
    $18, $19, $20, $21, $22, $23
)
RETURNING id
`

func runtimeSessionTestDSN(t *testing.T) string {
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

func runtimeSessionPool(t *testing.T, mode pgx.QueryExecMode) *pgxpool.Pool {
	t.Helper()
	dsn := runtimeSessionTestDSN(t)
	testfixtures.EnsureTestMigrations(t, dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pcfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	pcfg.ConnConfig.DefaultQueryExecMode = mode
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func insertRuntimeSessionTestMachine(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	_, err := pool.Exec(ctx, `INSERT INTO sites (id, name, code, status) VALUES ($1, 'rs-site', '', 'active')`, siteID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, site_id, serial_number, code, status, credential_version)
VALUES ($1, $2, $3, $4, 'online', 0)`, machineID, siteID, "sn-rs-"+uuid.NewString()[:8], "RS"+uuid.NewString()[:8])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machine_current_snapshot (machine_id, site_id)
VALUES ($1, $2)
ON CONFLICT (machine_id) DO NOTHING`, machineID, siteID)
	require.NoError(t, err)
	return machineID
}

func pgSQLState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func TestPgxExecMode_UncastByteSliceJSON_Returns22P02(t *testing.T) {
	pool := runtimeSessionPool(t, pgx.QueryExecModeExec)
	ctx := context.Background()
	machineID := insertRuntimeSessionTestMachine(t, pool)

	emptyUUID := pgtype.UUID{}
	err := pool.QueryRow(ctx, uncastStartRuntimeSessionSQL,
		machineID,
		emptyUUID,
		emptyUUID,
		emptyUUID,
		emptyUUID,
		"boot-h3",
		"start-h3",
		"inst-h3",
		"com.avf.vending",
		"1.0.0",
		"deadbeef",
		"UNKNOWN",
		"ONLINE",
		"unknown",
		"unknown",
		"INITIALIZING",
		false,
		[]byte("[]"),
		[]byte("{}"),
		[]byte("{}"),
		[]byte("{}"),
		[]byte("{}"),
		[]byte("{}"),
	).Scan(new(uuid.UUID))

	require.Error(t, err, "H3: []byte jsonb under QueryExecModeExec must fail")
	require.Equal(t, "22P02", pgSQLState(err), "got %v", err)
	require.Contains(t, err.Error(), "json")
}

func TestStartRuntimeAppSession_Lifecycle(t *testing.T) {
	t.Run("query_exec_mode_exec", func(t *testing.T) {
		runRuntimeSessionLifecycle(t, pgx.QueryExecModeExec)
	})
	t.Run("query_exec_mode_cache_describe", func(t *testing.T) {
		runRuntimeSessionLifecycle(t, pgx.QueryExecModeCacheDescribe)
	})
}

func runRuntimeSessionLifecycle(t *testing.T, mode pgx.QueryExecMode) {
	t.Helper()
	pool := runtimeSessionPool(t, mode)
	ctx := context.Background()
	machineID := insertRuntimeSessionTestMachine(t, pool)
	svc, err := NewService(Deps{Pool: pool})
	require.NoError(t, err)

	started, err := svc.StartRuntimeAppSession(ctx, StartInput{
		MachineID:       machineID,
		BootID:          "boot-life",
		AppStartID:      "start-life",
		AppInstanceID:   "inst-life",
		PackageName:     "com.avf.vending",
		AppVersion:      "2.0.0",
		AppBuildSHA:     "abc123",
		StartReason:     "UNKNOWN",
		NetworkState:    "online",
		MqttState:       "connected",
		StorefrontState: "INITIALIZING",
		Metadata:        json.RawMessage(`{"ok":true,"n":1,"s":"cà phê"}`),
	})
	require.NoError(t, err)
	require.Equal(t, machineID, started.MachineID)
	require.Equal(t, "ONLINE", started.Status)
	require.True(t, json.Valid(started.Blockers))
	require.True(t, json.Valid(started.HardwareStatus))
	require.JSONEq(t, "[]", string(started.Blockers))
	require.JSONEq(t, "{}", string(started.HardwareStatus))

	current, err := svc.GetCurrentRuntimeAppSession(ctx, machineID)
	require.NoError(t, err)
	require.Equal(t, started.ID, current.ID)

	hb, err := svc.HeartbeatRuntimeAppSession(ctx, HeartbeatInput{
		SessionID:       started.ID,
		MachineID:       machineID,
		NetworkState:    "online",
		MqttState:       "connected",
		StorefrontState: "SELLABLE",
		SellReady:       false,
		Blockers:        json.RawMessage(`[{"code":"X","severity":"warn","message":"m"}]`),
		HardwareStatus:  json.RawMessage(`{"bill":{"ok":true}}`),
		CatalogStatus:   json.RawMessage(`{"items":[1,2]}`),
		OutboxStatus:    json.RawMessage(`{"pending":0}`),
		RecoveryStatus:  json.RawMessage(`{"needed":false}`),
	})
	require.NoError(t, err)
	require.JSONEq(t, `[{"code":"X","severity":"warn","message":"m"}]`, string(hb.Blockers))
	require.True(t, hb.LastHeartbeatAt.Valid)

	current, err = svc.GetCurrentRuntimeAppSession(ctx, machineID)
	require.NoError(t, err)
	require.Equal(t, started.ID, current.ID)

	ended, err := svc.EndRuntimeAppSession(ctx, EndInput{
		SessionID: started.ID,
		MachineID: machineID,
		EndReason: "NORMAL_SHUTDOWN",
		Status:    "ENDED",
	})
	require.NoError(t, err)
	require.Equal(t, "ENDED", ended.Status)
	require.True(t, ended.EndedAt.Valid)
}

func TestHeartbeatRuntimeAppSession_RejectsMalformedJSON(t *testing.T) {
	pool := runtimeSessionPool(t, pgx.QueryExecModeExec)
	ctx := context.Background()
	machineID := insertRuntimeSessionTestMachine(t, pool)
	svc, err := NewService(Deps{Pool: pool})
	require.NoError(t, err)
	started, err := svc.StartRuntimeAppSession(ctx, StartInput{
		MachineID: machineID, BootID: "b", AppStartID: "s", AppInstanceID: "i",
		StartReason: "UNKNOWN",
	})
	require.NoError(t, err)
	_, err = svc.HeartbeatRuntimeAppSession(ctx, HeartbeatInput{
		SessionID: started.ID, MachineID: machineID,
		StorefrontState: "INITIALIZING",
		Blockers:        json.RawMessage(`not-json`),
	})
	require.Error(t, err)
	require.NotEqual(t, "22P02", pgSQLState(err))
	require.ErrorIs(t, err, ErrInvalidRuntimeJSON)
}

// Uncast jsonb INSERT matching production sqlc before the device-attachment metadata text→jsonb cast.
const uncastInsertMachineDeviceAttachmentSQL = `
INSERT INTO machine_device_attachments (
    machine_id, previous_attachment_id, status, reason,
    attached_by_account_id, operator_session_id, correlation_id,
    android_id, android_serial, board_serial, device_serial,
    sim_serial, sim_iccid, sim_operator, sim_country_iso,
    manufacturer, brand, model, device_model, hardware, product,
    android_release, sdk_int, package_name, version_name, version_code,
    app_build_sha, boot_id, network_type, network_state, ip_address, user_agent,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
    $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33
)
RETURNING id
`

func TestPgxExecMode_UncastDeviceAttachmentMetadata_Returns22P02(t *testing.T) {
	pool := runtimeSessionPool(t, pgx.QueryExecModeExec)
	ctx := context.Background()
	machineID := insertRuntimeSessionTestMachine(t, pool)
	emptyUUID := pgtype.UUID{}
	emptyText := pgtype.Text{}
	err := pool.QueryRow(ctx, uncastInsertMachineDeviceAttachmentSQL,
		machineID,
		emptyUUID,
		"active",
		"first_install",
		emptyUUID,
		emptyUUID,
		emptyUUID,
		emptyText, emptyText, emptyText, emptyText,
		emptyText, emptyText, emptyText, emptyText,
		emptyText, emptyText, emptyText, emptyText, emptyText, emptyText,
		emptyText, pgtype.Int4{}, emptyText, emptyText, pgtype.Int8{},
		emptyText, emptyText, emptyText, emptyText, nil, emptyText,
		[]byte(`{"activation_source":"activation_code"}`),
	).Scan(new(uuid.UUID))
	require.Error(t, err, "[]byte jsonb under QueryExecModeExec must fail")
	require.Equal(t, "22P02", pgSQLState(err), "got %v", err)
	require.Contains(t, err.Error(), "json")
}

func TestEnsureActivationDeviceAttachment_QueryExecModeExec(t *testing.T) {
	pool := runtimeSessionPool(t, pgx.QueryExecModeExec)
	ctx := context.Background()
	machineID := insertRuntimeSessionTestMachine(t, pool)
	svc, err := NewService(Deps{Pool: pool})
	require.NoError(t, err)
	q := db.New(pool)
	row, err := svc.EnsureActivationDeviceAttachmentInTx(ctx, q, ActivationAttachInput{
		MachineID:        machineID,
		FingerprintJSON:  json.RawMessage(`{"android_id":"aid-jsonb","board_serial":"board-jsonb"}`),
		Reason:           "first_install",
		ActivationSource: "activation_code",
	})
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, row.ID)
	require.JSONEq(t, `{"activation_source":"activation_code"}`, string(row.Metadata))
}
