package activation

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	appaudit "github.com/avf/avf-vending-api/internal/app/audit"
	"github.com/avf/avf-vending-api/internal/config"
	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func testActivationDSN(t *testing.T) string {
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

func activationModuleRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func activationMigrate(t *testing.T, dsn string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	goBin := os.Getenv("GO_BIN")
	if goBin == "" {
		goBin = "go"
	}
	cmd := exec.CommandContext(ctx, goBin, "run", "github.com/pressly/goose/v3/cmd/goose@v3.27.0",
		"-dir", filepath.Join(activationModuleRoot(t), "migrations"),
		"postgres", dsn, "up",
	)
	cmd.Dir = activationModuleRoot(t)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", string(out))
}

func activationTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := testActivationDSN(t)
	activationMigrate(t, dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func TestClaim_IdempotentReplaySameFingerprint(t *testing.T) {
	t.Parallel()

	pool := activationTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()

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

	_, err = pool.Exec(ctx, `INSERT INTO organizations (id, name, slug, status) VALUES ($1, 'act', 'act-org', 'active')`, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO sites (id, organization_id, name, code, status) VALUES ($1, $2, 's', '', 'active')`, siteID, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, serial_number, status, credential_version)
VALUES ($1, $2, $3, $4, 'online', 0)`, machineID, orgID, siteID, "sn-act-1")
	require.NoError(t, err)

	create, err := svc.CreateCode(ctx, CreateInput{
		MachineID:        machineID,
		OrganizationID:   orgID,
		ExpiresInMinutes: 60,
		MaxUses:          2,
	})
	require.NoError(t, err)

	fp := DeviceFingerprint{SerialNumber: "SN-ACT", AndroidID: "aid-1"}
	in := ClaimInput{ActivationCode: create.PlaintextCode, DeviceFingerprint: fp}

	out1, err := svc.Claim(ctx, in, "mqtt://x", "pfx")
	require.NoError(t, err)
	require.Equal(t, machineID, out1.MachineID)

	out2, err := svc.Claim(ctx, in, "mqtt://x", "pfx")
	require.NoError(t, err)
	require.Equal(t, machineID, out2.MachineID)
}

func TestClaim_DifferentFingerprintRejectedWhenSingleUse(t *testing.T) {
	t.Parallel()

	pool := activationTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()

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

	_, err = pool.Exec(ctx, `INSERT INTO organizations (id, name, slug, status) VALUES ($1, 'act2', 'act2-org', 'active')`, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO sites (id, organization_id, name, code, status) VALUES ($1, $2, 's', '', 'active')`, siteID, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, serial_number, status, credential_version)
VALUES ($1, $2, $3, $4, 'online', 0)`, machineID, orgID, siteID, "sn-act-2")
	require.NoError(t, err)

	create, err := svc.CreateCode(ctx, CreateInput{
		MachineID:        machineID,
		OrganizationID:   orgID,
		ExpiresInMinutes: 60,
		MaxUses:          1,
	})
	require.NoError(t, err)

	_, err = svc.Claim(ctx, ClaimInput{
		ActivationCode:    create.PlaintextCode,
		DeviceFingerprint: DeviceFingerprint{SerialNumber: "A"},
	}, "mqtt://x", "pfx")
	require.NoError(t, err)

	_, err = svc.Claim(ctx, ClaimInput{
		ActivationCode:    create.PlaintextCode,
		DeviceFingerprint: DeviceFingerprint{SerialNumber: "B"},
	}, "mqtt://x", "pfx")
	require.ErrorIs(t, err, ErrInvalid)
}

func TestClaim_TwoDistinctFingerprintsWhenMaxUsesTwo(t *testing.T) {
	t.Parallel()

	pool := activationTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()

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

	_, err = pool.Exec(ctx, `INSERT INTO organizations (id, name, slug, status) VALUES ($1, 'act3', 'act3-org', 'active')`, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO sites (id, organization_id, name, code, status) VALUES ($1, $2, 's', '', 'active')`, siteID, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, serial_number, status, credential_version)
VALUES ($1, $2, $3, $4, 'online', 0)`, machineID, orgID, siteID, "sn-act-3")
	require.NoError(t, err)

	create, err := svc.CreateCode(ctx, CreateInput{
		MachineID:        machineID,
		OrganizationID:   orgID,
		ExpiresInMinutes: 60,
		MaxUses:          2,
	})
	require.NoError(t, err)
	code := create.PlaintextCode

	_, err = svc.Claim(ctx, ClaimInput{
		ActivationCode:    code,
		DeviceFingerprint: DeviceFingerprint{SerialNumber: "fp-one"},
	}, "mqtt://x", "pfx")
	require.NoError(t, err)
	_, err = svc.Claim(ctx, ClaimInput{
		ActivationCode:    code,
		DeviceFingerprint: DeviceFingerprint{SerialNumber: "fp-two"},
	}, "mqtt://x", "pfx")
	require.NoError(t, err)

	_, err = svc.Claim(ctx, ClaimInput{
		ActivationCode:    code,
		DeviceFingerprint: DeviceFingerprint{SerialNumber: "fp-three"},
	}, "mqtt://x", "pfx")
	require.ErrorIs(t, err, ErrInvalid)

	var n int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM machine_activation_claims WHERE activation_code_id = $1 AND result = 'succeeded'`, create.ID).Scan(&n))
	require.Equal(t, 2, n)
}

func TestClaim_ConcurrentClaimsRespectMaxUses(t *testing.T) {
	t.Parallel()

	pool := activationTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()

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

	_, err = pool.Exec(ctx, `INSERT INTO organizations (id, name, slug, status) VALUES ($1, 'act4', 'act4-org', 'active')`, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO sites (id, organization_id, name, code, status) VALUES ($1, $2, 's', '', 'active')`, siteID, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, serial_number, status, credential_version)
VALUES ($1, $2, $3, $4, 'online', 0)`, machineID, orgID, siteID, "sn-act-4")
	require.NoError(t, err)

	create, err := svc.CreateCode(ctx, CreateInput{
		MachineID:        machineID,
		OrganizationID:   orgID,
		ExpiresInMinutes: 60,
		MaxUses:          3,
	})
	require.NoError(t, err)
	code := create.PlaintextCode

	const goroutines = 12
	var wg sync.WaitGroup
	var okCount atomic.Int32
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := svc.Claim(ctx, ClaimInput{
				ActivationCode:    code,
				DeviceFingerprint: DeviceFingerprint{SerialNumber: fmt.Sprintf("conc-%d", i)},
			}, "mqtt://x", "pfx")
			if err == nil {
				okCount.Add(1)
			}
		}(i)
	}
	wg.Wait()
	require.Equal(t, int32(3), okCount.Load(), "exactly max_uses successes under contention")

	var n int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM machine_activation_claims WHERE activation_code_id = $1 AND result = 'succeeded'`, create.ID).Scan(&n))
	require.Equal(t, 3, n)
}

func TestClaim_PersistsSucceededClaimAndAudit(t *testing.T) {
	t.Parallel()

	pool := activationTestPool(t)
	ctx := context.Background()
	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()

	cfg := config.HTTPAuthConfig{
		Mode:            plauth.HTTPAuthModeHS256,
		JWTSecret:       bytes.Repeat([]byte("z"), 32),
		JWTLeeway:       30 * time.Second,
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 720 * time.Hour,
	}
	issuer, err := plauth.NewSessionIssuerFromHTTPAuth(cfg)
	require.NoError(t, err)
	auditSvc := appaudit.NewService(pool)
	svc := NewService(pool, issuer, plauth.TrimSecret(cfg.JWTSecret), auditSvc)

	_, err = pool.Exec(ctx, `INSERT INTO organizations (id, name, slug, status) VALUES ($1, 'act5', 'act5-org', 'active')`, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO sites (id, organization_id, name, code, status) VALUES ($1, $2, 's', '', 'active')`, siteID, orgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO machines (id, organization_id, site_id, serial_number, status, credential_version)
VALUES ($1, $2, $3, $4, 'online', 0)`, machineID, orgID, siteID, "sn-act-5")
	require.NoError(t, err)

	create, err := svc.CreateCode(ctx, CreateInput{
		MachineID:        machineID,
		OrganizationID:   orgID,
		ExpiresInMinutes: 60,
		MaxUses:          1,
	})
	require.NoError(t, err)

	_, err = svc.Claim(ctx, ClaimInput{
		ActivationCode:    create.PlaintextCode,
		DeviceFingerprint: DeviceFingerprint{SerialNumber: "aud-1"},
		ClientIP:          "203.0.113.10",
		UserAgent:         "integration-test",
	}, "mqtt://x", "pfx")
	require.NoError(t, err)

	var claimCount int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM machine_activation_claims WHERE activation_code_id = $1 AND result = 'succeeded'`, create.ID).Scan(&claimCount))
	require.Equal(t, 1, claimCount)

	var auditCount int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM audit_events
WHERE organization_id = $1 AND action = 'machine.activation.claimed'`, orgID).Scan(&auditCount))
	require.GreaterOrEqual(t, auditCount, 1)
}
