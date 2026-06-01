package testfixtures

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	testMigrateMu   sync.Mutex
	testMigratedDSN = map[string]struct{}{}
)

// EnsureTestMigrations runs goose up once per DSN for integration tests.
func EnsureTestMigrations(t *testing.T, dsn string) {
	t.Helper()
	testMigrateMu.Lock()
	defer testMigrateMu.Unlock()
	if _, ok := testMigratedDSN[dsn]; ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	goBin := os.Getenv("GO_BIN")
	if goBin == "" {
		goBin = "go"
	}
	repoRoot := RepoRoot(t)
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
	testMigratedDSN[dsn] = struct{}{}
}
