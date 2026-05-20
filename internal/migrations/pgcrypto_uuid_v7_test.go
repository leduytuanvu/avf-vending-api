package migrations_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func sqlLineUnqualifiedPgcrypto(line string) (bytes bool, uuidFn bool) {
	line = strings.Split(line, "--")[0]
	if strings.Contains(line, "gen_random_bytes(") && !strings.Contains(line, "extensions.gen_random_bytes(") {
		bytes = true
	}
	if strings.Contains(line, "gen_random_uuid(") && !strings.Contains(line, "extensions.gen_random_uuid(") {
		uuidFn = true
	}
	return bytes, uuidFn
}

func sqlHasUnqualifiedGenRandomBytes(raw string) bool {
	for _, line := range strings.Split(raw, "\n") {
		if b, _ := sqlLineUnqualifiedPgcrypto(line); b {
			return true
		}
	}
	return false
}

func sqlHasUnqualifiedGenRandomUUID(raw string) bool {
	for _, line := range strings.Split(raw, "\n") {
		if _, u := sqlLineUnqualifiedPgcrypto(line); u {
			return true
		}
	}
	return false
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	repoRoot := testfixtures.RepoRoot(t)
	b, err := os.ReadFile(filepath.Join(repoRoot, rel))
	require.NoError(t, err)
	return string(b)
}

func TestUUIDV7Migration_uses_schema_qualified_pgcrypto(t *testing.T) {
	for _, rel := range []string{
		"migrations/00005_uuid_v7_defaults.sql",
		"migrations/00006_fix_pgcrypto_uuid_v7_schema_qualification.sql",
		"db/schema/01_platform.sql",
	} {
		raw := readRepoFile(t, rel)
		if sqlHasUnqualifiedGenRandomBytes(raw) {
			t.Fatalf("%s: unqualified gen_random_bytes() call; use extensions.gen_random_bytes()", rel)
		}
		if strings.Contains(raw, "SET search_path = public, pg_temp") &&
			strings.Contains(raw, "uuid_generate_v7") {
			t.Fatalf("%s: uuid_generate_v7 search_path must include extensions", rel)
		}
		if !strings.Contains(raw, "extensions.gen_random_bytes") {
			t.Fatalf("%s: expected extensions.gen_random_bytes qualification", rel)
		}
	}
}

func TestForwardMigration00006_exists_and_verifies_uuid_v7(t *testing.T) {
	raw := readRepoFile(t, "migrations/00006_fix_pgcrypto_uuid_v7_schema_qualification.sql")
	require.Contains(t, raw, "CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA extensions")
	require.Contains(t, raw, "PERFORM extensions.gen_random_bytes(16)")
	require.Contains(t, raw, "v := public.uuid_generate_v7()")
}

func TestUUIDV7DownMigration_uses_schema_qualified_gen_random_uuid(t *testing.T) {
	raw := readRepoFile(t, "migrations/00005_uuid_v7_defaults.sql")
	downIdx := strings.Index(raw, "-- +goose Down")
	require.Greater(t, downIdx, 0, "expected goose Down section")
	down := raw[downIdx:]
	if sqlHasUnqualifiedGenRandomUUID(down) {
		t.Fatal("00005 Down: unqualified gen_random_uuid(); use extensions.gen_random_uuid()")
	}
}

func testDSN(t *testing.T) string {
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

func migrateUp(t *testing.T, dsn string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	goBin := os.Getenv("GO_BIN")
	if goBin == "" {
		goBin = "go"
	}
	repoRoot := testfixtures.RepoRoot(t)
	absRoot, err := filepath.Abs(repoRoot)
	require.NoError(t, err)
	migrationsDir := filepath.Join(absRoot, "migrations")
	cmd := exec.CommandContext(ctx, goBin, "run", "github.com/pressly/goose/v3/cmd/goose@v3.27.0",
		"-dir", migrationsDir,
		"postgres", dsn, "up",
	)
	cmd.Dir = absRoot
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", string(out))
}

func TestUUIDV7Function_andAuditInsert_integration(t *testing.T) {
	dsn := testDSN(t)
	migrateUp(t, dsn)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	var pgcryptoOK bool
	err = pool.QueryRow(ctx, `SELECT extensions.gen_random_bytes(16) IS NOT NULL`).Scan(&pgcryptoOK)
	require.NoError(t, err)
	require.True(t, pgcryptoOK)

	var idStr string
	err = pool.QueryRow(ctx, `SELECT public.uuid_generate_v7()::text`).Scan(&idStr)
	require.NoError(t, err)
	parsed, err := uuid.Parse(idStr)
	require.NoError(t, err)
	testfixtures.AssertResourceUUIDV7(t, parsed)

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()

	var auditID uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO audit_events (
			actor_type, action, resource_type, outcome
		) VALUES (
			'system', 'auth.login.test', 'auth_session', 'success'
		)
		RETURNING id
	`).Scan(&auditID)
	require.NoError(t, err, "audit_events INSERT must not fail on gen_random_bytes")
	testfixtures.AssertResourceUUIDV7(t, auditID)
}
