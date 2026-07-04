package activation

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/machineruntime"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/gen/db"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func activationServiceWithRuntime(t *testing.T, pool *pgxpool.Pool) (*Service, *machineruntime.Service) {
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
	svc := NewService(pool, issuer, plauth.TrimSecret(cfg.JWTSecret), nil)
	rt, err := machineruntime.NewService(machineruntime.Deps{Pool: pool})
	require.NoError(t, err)
	svc.SetMachineRuntime(rt)
	return svc, rt
}

func insertActivationTestMachine(t *testing.T, pool *pgxpool.Pool) (siteID, machineID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	siteID = id.NewUUIDV7()
	machineID = id.NewUUIDV7()
	_, err := pool.Exec(ctx, `INSERT INTO sites (id, name, code, status) VALUES ($1, 's', '', 'active')`, siteID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, site_id, serial_number, status, credential_version)
VALUES ($1, $2, $3, 'online', 0)`, machineID, siteID, "sn-attach-"+uuid.NewString()[:8])
	require.NoError(t, err)
	return siteID, machineID
}

func TestClaim_WithRuntime_FirstClaimCreatesDeviceAttachment(t *testing.T) {
	t.Parallel()
	pool := activationTestPool(t)
	ctx := context.Background()
	svc, _ := activationServiceWithRuntime(t, pool)
	_, machineID := insertActivationTestMachine(t, pool)

	create, err := svc.CreateCode(ctx, CreateInput{MachineID: machineID, ExpiresInMinutes: 60, MaxUses: 1})
	require.NoError(t, err)

	fp := DeviceFingerprint{AndroidID: "aid-first", BoardSerial: "board-first"}
	out, err := svc.Claim(ctx, ClaimInput{ActivationCode: create.PlaintextCode, DeviceFingerprint: fp}, "mqtt://x", "pfx", "legacy")
	require.NoError(t, err)
	require.NotNil(t, out.DeviceAttachmentID)

	var currentAttach uuid.UUID
	err = pool.QueryRow(ctx, `SELECT current_device_attachment_id FROM machines WHERE id = $1`, machineID).Scan(&currentAttach)
	require.NoError(t, err)
	require.Equal(t, *out.DeviceAttachmentID, currentAttach)

	var activeCount int64
	err = pool.QueryRow(ctx, `SELECT count(*) FROM machine_device_attachments WHERE machine_id = $1 AND status = 'active'`, machineID).Scan(&activeCount)
	require.NoError(t, err)
	require.Equal(t, int64(1), activeCount)
}

func TestClaim_WithRuntime_ReplayReusesAttachment(t *testing.T) {
	t.Parallel()
	pool := activationTestPool(t)
	ctx := context.Background()
	svc, _ := activationServiceWithRuntime(t, pool)
	_, machineID := insertActivationTestMachine(t, pool)

	create, err := svc.CreateCode(ctx, CreateInput{MachineID: machineID, ExpiresInMinutes: 60, MaxUses: 2})
	require.NoError(t, err)

	fp := DeviceFingerprint{AndroidID: "aid-replay", BoardSerial: "board-replay"}
	in := ClaimInput{ActivationCode: create.PlaintextCode, DeviceFingerprint: fp}

	out1, err := svc.Claim(ctx, in, "mqtt://x", "pfx", "legacy")
	require.NoError(t, err)
	require.NotNil(t, out1.DeviceAttachmentID)

	out2, err := svc.Claim(ctx, in, "mqtt://x", "pfx", "legacy")
	require.NoError(t, err)
	require.NotNil(t, out2.DeviceAttachmentID)
	require.Equal(t, *out1.DeviceAttachmentID, *out2.DeviceAttachmentID)

	var activeCount int64
	err = pool.QueryRow(ctx, `SELECT count(*) FROM machine_device_attachments WHERE machine_id = $1 AND status = 'active'`, machineID).Scan(&activeCount)
	require.NoError(t, err)
	require.Equal(t, int64(1), activeCount)
}

func TestClaim_WithRuntime_DifferentBoardReplacesAttachmentAndClosesSession(t *testing.T) {
	t.Parallel()
	pool := activationTestPool(t)
	ctx := context.Background()
	svc, rt := activationServiceWithRuntime(t, pool)
	_, machineID := insertActivationTestMachine(t, pool)

	create, err := svc.CreateCode(ctx, CreateInput{MachineID: machineID, ExpiresInMinutes: 60, MaxUses: 2})
	require.NoError(t, err)

	fp1 := DeviceFingerprint{AndroidID: "aid-board", BoardSerial: "board-old"}
	out1, err := svc.Claim(ctx, ClaimInput{ActivationCode: create.PlaintextCode, DeviceFingerprint: fp1}, "mqtt://x", "pfx", "legacy")
	require.NoError(t, err)
	require.NotNil(t, out1.DeviceAttachmentID)

	sess, err := rt.StartRuntimeAppSession(ctx, machineruntime.StartInput{
		MachineID:          machineID,
		DeviceAttachmentID: out1.DeviceAttachmentID,
		BootID:             "boot-1",
		AppStartID:         "start-1",
		AppInstanceID:      "inst-1",
		PackageName:        "com.avf.vending",
		AppVersion:         "1.0.0",
		StartReason:        "COLD_START",
	})
	require.NoError(t, err)

	fp2 := DeviceFingerprint{AndroidID: "aid-board", BoardSerial: "board-new"}
	out2, err := svc.Claim(ctx, ClaimInput{ActivationCode: create.PlaintextCode, DeviceFingerprint: fp2}, "mqtt://x", "pfx", "legacy")
	require.NoError(t, err)
	require.NotNil(t, out2.DeviceAttachmentID)
	require.NotEqual(t, *out1.DeviceAttachmentID, *out2.DeviceAttachmentID)

	oldAttach, err := db.New(pool).GetMachineDeviceAttachmentByID(ctx, *out1.DeviceAttachmentID)
	require.NoError(t, err)
	require.Equal(t, "replaced", oldAttach.Status)

	newAttach, err := db.New(pool).GetMachineDeviceAttachmentByID(ctx, *out2.DeviceAttachmentID)
	require.NoError(t, err)
	require.Equal(t, "active", newAttach.Status)

	var currentAttach uuid.UUID
	err = pool.QueryRow(ctx, `SELECT current_device_attachment_id FROM machines WHERE id = $1`, machineID).Scan(&currentAttach)
	require.NoError(t, err)
	require.Equal(t, *out2.DeviceAttachmentID, currentAttach)

	closed, err := db.New(pool).GetMachineRuntimeAppSessionByID(ctx, sess.ID)
	require.NoError(t, err)
	require.Equal(t, "REPLACED", closed.Status)
	require.True(t, closed.EndReason.Valid)
	require.Equal(t, "BOARD_REPLACED", closed.EndReason.String)
}

func TestClaim_WithRuntime_NoOperatorSessionRequired(t *testing.T) {
	t.Parallel()
	pool := activationTestPool(t)
	ctx := context.Background()
	svc, _ := activationServiceWithRuntime(t, pool)
	_, machineID := insertActivationTestMachine(t, pool)

	create, err := svc.CreateCode(ctx, CreateInput{MachineID: machineID, ExpiresInMinutes: 60, MaxUses: 1})
	require.NoError(t, err)

	out, err := svc.Claim(ctx, ClaimInput{
		ActivationCode:    create.PlaintextCode,
		DeviceFingerprint: DeviceFingerprint{AndroidID: "aid-no-op", BoardSerial: "board-no-op"},
		ClaimContext:      ClaimContext{ActivationSource: "activation_code"},
	}, "mqtt://x", "pfx", "legacy")
	require.NoError(t, err)
	require.NotNil(t, out.DeviceAttachmentID)
}
