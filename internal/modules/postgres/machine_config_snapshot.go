package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// InsertMachineConfigSnapshotTx appends a machine_configs row with the next monotonic config_revision for this machine.
func InsertMachineConfigSnapshotTx(ctx context.Context, tx pgx.Tx, orgID, machineID uuid.UUID, operatorSessionID pgtype.UUID, planogramID string, planogramRevision int32, publishedPlanogramVersionID *uuid.UUID) (db.MachineConfig, int32, error) {
	var maxRev int32
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(config_revision), 0) FROM machine_configs WHERE machine_id = $1`, machineID).Scan(&maxRev); err != nil {
		return db.MachineConfig{}, 0, err
	}
	next := maxRev + 1
	meta := map[string]any{
		"planogramId":       planogramID,
		"planogramRevision": planogramRevision,
	}
	if publishedPlanogramVersionID != nil && *publishedPlanogramVersionID != uuid.Nil {
		meta["publishedPlanogramVersionId"] = publishedPlanogramVersionID.String()
	}
	metaBytes, _ := json.Marshal(meta)

	cfgPayload := map[string]any{
		"kind":                 "planogram_publish",
		"planogramId":          planogramID,
		"planogramRevision":    planogramRevision,
		"desiredConfigVersion": next,
	}
	if publishedPlanogramVersionID != nil && *publishedPlanogramVersionID != uuid.Nil {
		cfgPayload["publishedPlanogramVersionId"] = publishedPlanogramVersionID.String()
	}
	cfgBytes, _ := json.Marshal(cfgPayload)

	q := db.New(tx)
	row, err := q.InsertMachineConfigApplication(ctx, db.InsertMachineConfigApplicationParams{
		OrganizationID:    orgID,
		MachineID:         machineID,
		AppliedAt:         time.Now().UTC(),
		ConfigRevision:    next,
		ConfigPayload:     cfgBytes,
		OperatorSessionID: operatorSessionID,
		Metadata:          metaBytes,
	})
	if err != nil {
		return db.MachineConfig{}, 0, err
	}
	return row, next, nil
}
