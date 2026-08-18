package machineidempotency

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/avf/avf-vending-api/internal/platform/pgjson"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	machinev1 "github.com/avf/avf-vending-api/proto/avf/machine/v1"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Uncast UPDATE matching production sqlc before FIX-CAT-01 ([]byte jsonb, no ::text::jsonb).
const uncastMarkMachineIdempotencySucceededSQL = `
UPDATE machine_idempotency_keys
SET
    status = 'succeeded',
    response_snapshot = $1,
    last_seen_at = now(),
    trace_id = $2
WHERE
    machine_id = $3
    AND operation = $4
    AND idempotency_key = $5
RETURNING id
`

func idempotencyTestDSN(t *testing.T) string {
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

func idempotencyPool(t *testing.T, mode pgx.QueryExecMode) *pgxpool.Pool {
	t.Helper()
	dsn := idempotencyTestDSN(t)
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

func insertIdempotencyTestMachine(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	siteID := id.NewUUIDV7()
	machineID := id.NewUUIDV7()
	_, err := pool.Exec(ctx, `INSERT INTO sites (id, name, code, status) VALUES ($1, 'idem-site', '', 'active')`, siteID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, site_id, serial_number, code, status, credential_version)
VALUES ($1, $2, $3, $4, 'online', 1)`, machineID, siteID, "sn-idem-"+uuid.NewString()[:8], "ID"+uuid.NewString()[:8])
	require.NoError(t, err)
	return machineID
}

func insertInProgressIdempotencyRow(t *testing.T, pool *pgxpool.Pool, machineID uuid.UUID, op, key string) {
	t.Helper()
	ctx := context.Background()
	q := db.New(pool)
	_, err := q.UpsertMachineIdempotencyKey(ctx, db.UpsertMachineIdempotencyKeyParams{
		MachineID:      machineID,
		Operation:      op,
		IdempotencyKey: key,
		RequestHash:    []byte("hash-idem-h3"),
		ExpiresAt:      time.Now().UTC().Add(time.Hour),
		TraceID:        "trace-h3",
	})
	require.NoError(t, err)
}

func pgSQLState(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func TestPgxExecMode_UncastIdempotencyResponseSnapshot_Returns22P02(t *testing.T) {
	pool := idempotencyPool(t, pgx.QueryExecModeExec)
	ctx := context.Background()
	machineID := insertIdempotencyTestMachine(t, pool)
	op := machinev1.MachineInventoryService_AckInventorySync_FullMethodName
	key := "inventory_ack-uncast-" + uuid.NewString()
	insertInProgressIdempotencyRow(t, pool, machineID, op, key)

	snap, err := protojson.MarshalOptions{EmitUnpopulated: true}.Marshal(&machinev1.AckInventorySyncResponse{})
	require.NoError(t, err)
	require.True(t, len(snap) > 0)

	err = pool.QueryRow(ctx, uncastMarkMachineIdempotencySucceededSQL,
		snap, "trace-h3", machineID, op, key,
	).Scan(new(uuid.UUID))
	require.Error(t, err, "H3: []byte jsonb under QueryExecModeExec must fail")
	require.Equal(t, "22P02", pgSQLState(err), "got %v", err)
	require.Contains(t, err.Error(), "json")
	t.Logf("CONFIRMED_BY_REPRODUCTION sqlstate=%s err=%v go_type=[]byte dest=jsonb query=MarkMachineIdempotencySucceeded_uncast", pgSQLState(err), err)
}

func TestMarkSucceeded_QueryExecMode(t *testing.T) {
	for _, mode := range []pgx.QueryExecMode{pgx.QueryExecModeExec, pgx.QueryExecModeCacheDescribe, pgx.QueryExecModeSimpleProtocol} {
		t.Run(mode.String(), func(t *testing.T) {
			runMarkSucceededLifecycle(t, mode)
		})
	}
}

func runMarkSucceededLifecycle(t *testing.T, mode pgx.QueryExecMode) {
	t.Helper()
	pool := idempotencyPool(t, mode)
	ctx := context.Background()
	machineID := insertIdempotencyTestMachine(t, pool)
	ledger := NewLedger(pool, nil)
	claims := plauth.MachineAccessClaims{MachineID: machineID, CredentialVersion: 1}

	cases := []struct {
		name string
		resp proto.Message
		op   string
		key  string
	}{
		{
			name: "empty_ack_inventory",
			resp: &machinev1.AckInventorySyncResponse{},
			op:   machinev1.MachineInventoryService_AckInventorySync_FullMethodName,
			key:  "inventory_ack-empty-" + uuid.NewString(),
		},
		{
			name: "utf8_nested_order",
			resp: &machinev1.CreateOrderResponse{OrderId: "đơn-hàng-" + uuid.NewString()[:8]},
			op:   machinev1.MachineCommerceService_CreateOrder_FullMethodName,
			key:  "order-utf8-" + uuid.NewString(),
		},
		{
			name: "ack_catalog",
			resp: &machinev1.AckCatalogVersionResponse{},
			op:   machinev1.MachineCatalogService_AckCatalogVersion_FullMethodName,
			key:  "catalog_ack-" + uuid.NewString(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hash := []byte("hash-" + tc.name + "-" + mode.String())
			row, replay, err := ledger.BeginMutation(ctx, claims, tc.op, tc.key, hash, "trace-ok", func(string) proto.Message {
				return proto.Clone(tc.resp)
			})
			require.NoError(t, err)
			require.Nil(t, replay)
			require.True(t, row.Inserted)
			require.NoError(t, ledger.MarkSucceeded(ctx, claims, tc.op, tc.key, tc.resp, "trace-ok"))

			row2, replay2, err := ledger.BeginMutation(ctx, claims, tc.op, tc.key, hash, "trace-replay", func(string) proto.Message {
				return proto.Clone(tc.resp)
			})
			require.NoError(t, err)
			require.False(t, row2.Inserted)
			require.NotNil(t, replay2)
			require.Equal(t, "succeeded", row2.Status)
			require.True(t, json.Valid(row2.ResponseSnapshot), "response_snapshot must be JSON")
			require.NotEmpty(t, pgjson.RequiredString(row2.ResponseSnapshot))
		})
	}
}

func TestMarkSucceeded_ConflictSameKeyDifferentPayload(t *testing.T) {
	pool := idempotencyPool(t, pgx.QueryExecModeExec)
	ctx := context.Background()
	machineID := insertIdempotencyTestMachine(t, pool)
	ledger := NewLedger(pool, nil)
	claims := plauth.MachineAccessClaims{MachineID: machineID, CredentialVersion: 1}
	op := machinev1.MachineInventoryService_AckInventorySync_FullMethodName
	key := "inventory_ack-conflict-" + uuid.NewString()

	reqA := &machinev1.AckInventorySyncRequest{
		Meta:       &machinev1.MachineRequestMeta{MachineId: machineID.String(), IdempotencyKey: key},
		SyncCursor: "v1|100",
	}
	hashA, err := HashMutationRequest(reqA)
	require.NoError(t, err)
	_, _, err = ledger.BeginMutation(ctx, claims, op, key, hashA, "t1", func(string) proto.Message {
		return &machinev1.AckInventorySyncResponse{}
	})
	require.NoError(t, err)
	require.NoError(t, ledger.MarkSucceeded(ctx, claims, op, key, &machinev1.AckInventorySyncResponse{}, "t1"))

	reqB := &machinev1.AckInventorySyncRequest{
		Meta:       &machinev1.MachineRequestMeta{MachineId: machineID.String(), IdempotencyKey: key},
		SyncCursor: "v1|999",
	}
	hashB, err := HashMutationRequest(reqB)
	require.NoError(t, err)
	require.False(t, bytes.Equal(hashA, hashB))
	_, _, err = ledger.BeginMutation(ctx, claims, op, key, hashB, "t2", func(string) proto.Message {
		return &machinev1.AckInventorySyncResponse{}
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Contains(t, status.Convert(err).Message(), ErrMsgIdempotencyPayloadMismatch)
}

func TestHashMutationRequest_AckInventorySyncDistinctCursors(t *testing.T) {
	a := &machinev1.AckInventorySyncRequest{SyncCursor: "a", Meta: &machinev1.MachineRequestMeta{IdempotencyKey: "k"}}
	b := &machinev1.AckInventorySyncRequest{SyncCursor: "b", Meta: &machinev1.MachineRequestMeta{IdempotencyKey: "k"}}
	ha, err := HashMutationRequest(a)
	require.NoError(t, err)
	hb, err := HashMutationRequest(b)
	require.NoError(t, err)
	require.False(t, bytes.Equal(ha, hb))
}
