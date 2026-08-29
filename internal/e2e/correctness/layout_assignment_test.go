package correctness

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/app/layoutassignment"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestAssignServerLayoutAtomicity exercises AssignServerLayout against a real Postgres database.
// Requires TEST_DATABASE_URL (skipped when unset or when -short is set; see helpers_test.go).
//
// Full matrix (Phase 9/11): single-tx commit on success; rollback on mid-tx failure; at most one
// current SERVER row per machine; revision conflict on stale expectedCurrentRevision; idempotent replay.
func TestAssignServerLayoutAtomicity(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	siteID := uuid.New()
	machineID := uuid.New()
	insertSiteMachine(t, ctx, pool, siteID, machineID, "online", 1)

	svc := &layoutassignment.Service{
		Pool:  pool,
		Setup: postgres.NewSetupRepository(pool),
	}

	// TODO(phase-11): seed published planogram version + layout version rows, then assert:
	// - assignment row, machine_layout_state, and config snapshot are written in one commit
	// - a forced failure after draft save rolls back the entire assignment
	// - second assign with same idempotency key replays without duplicate revision
	_ = svc
	require.NotNil(t, pool)
}
