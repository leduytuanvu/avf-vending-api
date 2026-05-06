package testfixtures

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

const repoGoModuleLine = "module github.com/avf/avf-vending-api"

// RepoRoot walks up from the caller's source file until it finds the repository go.mod.
// Use this for subprocesses (e.g. goose -dir) so tests work regardless of package depth.
func RepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(1)
	require.True(t, ok, "runtime.Caller(1) failed")
	dir := filepath.Dir(file)
	for {
		modPath := filepath.Join(dir, "go.mod")
		b, err := os.ReadFile(modPath)
		if err == nil && bytes.Contains(b, []byte(repoGoModuleLine)) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			require.Fail(t, "repository root (go.mod) not found walking up from "+file)
		}
		dir = parent
	}
}
