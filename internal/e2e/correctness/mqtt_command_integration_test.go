package correctness

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/domain/device"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestP06_E2E_MQTTCommand_* validates command ledger persistence vs MQTT ACK semantics.

func TestP06_E2E_MQTTCommand_ackWrongMachineRejectedUnknownSequence(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orgID := uuid.New()
	siteID := uuid.New()
	machineA := uuid.New()
	machineB := uuid.New()
	insertOrganizationAndSite(t, ctx, pool, orgID, siteID)
	insertMachine(t, ctx, pool, orgID, siteID, machineA, "online", 1)
	insertMachine(t, ctx, pool, orgID, siteID, machineB, "online", 1)

	appendRes, err := store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:      machineA,
		CommandType:    "noop",
		Payload:        []byte(`{}`),
		IdempotencyKey: "p06-wrong-mac-" + uuid.NewString(),
		DesiredState:   []byte(`{}`),
	})
	require.NoError(t, err)

	_, err = store.ApplyCommandReceiptTransition(ctx, postgres.CommandReceiptTransitionParams{
		MachineID:  machineB,
		Sequence:   appendRes.Sequence,
		Status:     "acked",
		Payload:    []byte(`{}`),
		DedupeKey:  "dedupe-wrong-mac-" + uuid.NewString(),
		CommandID:  appendRes.CommandID,
		OccurredAt: time.Now().UTC(),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown sequence")
}

func TestP06_E2E_MQTTCommand_publishAttemptCreatesLedgerBeforeAckWindow(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	insertOrganizationSiteMachine(t, ctx, pool, orgID, siteID, machineID, "online", 1)

	appendRes, err := store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:      machineID,
		CommandType:    "noop",
		Payload:        []byte(`{}`),
		IdempotencyKey: "p06-led-before-" + uuid.NewString(),
		DesiredState:   []byte(`{}`),
	})
	require.NoError(t, err)

	cmdRow, err := store.GetCommandLedgerByMachineSequence(ctx, machineID, appendRes.Sequence)
	require.NoError(t, err)

	deadline := time.Now().UTC().Add(time.Hour)
	att, err := store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, cmdRow.ID, machineID, nil, []byte(`{}`), deadline, "")
	require.NoError(t, err)

	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM command_ledger WHERE id = $1 AND machine_id = $2`,
		cmdRow.ID, machineID).Scan(&n))
	require.Equal(t, 1, n)

	var attemptMachine uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT machine_id FROM machine_command_attempts WHERE id = $1`, att.ID).Scan(&attemptMachine))
	require.Equal(t, machineID, attemptMachine)
}

func TestP06_E2E_MQTTCommand_expiresStaleLedgerRows(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	insertOrganizationSiteMachine(t, ctx, pool, orgID, siteID, machineID, "online", 1)

	appendRes, err := store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:      machineID,
		CommandType:    "noop",
		Payload:        []byte(`{}`),
		IdempotencyKey: "p06-exp-" + uuid.NewString(),
		DesiredState:   []byte(`{}`),
	})
	require.NoError(t, err)

	cmdRow, err := store.GetCommandLedgerByMachineSequence(ctx, machineID, appendRes.Sequence)
	require.NoError(t, err)

	past := time.Now().UTC().Add(-48 * time.Hour)
	att, err := store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, cmdRow.ID, machineID, nil, []byte(`{}`), past, "")
	require.NoError(t, err)
	require.NoError(t, store.MarkMQTTDispatchAttemptSent(ctx, att.ID, past))

	require.NoError(t, store.ApplyMQTTCommandAckTimeouts(ctx, time.Now().UTC()))

	var attemptStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM machine_command_attempts WHERE id = $1`, att.ID).Scan(&attemptStatus))
	require.Equal(t, "expired", attemptStatus)
}

func TestP06_E2E_MQTTCommand_ackDeadlineTimeoutsWhenLedgerSLAStillFuture(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	insertOrganizationSiteMachine(t, ctx, pool, orgID, siteID, machineID, "online", 1)

	appendRes, err := store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:      machineID,
		CommandType:    "noop",
		Payload:        []byte(`{}`),
		IdempotencyKey: "p06-ackto-" + uuid.NewString(),
		DesiredState:   []byte(`{}`),
	})
	require.NoError(t, err)

	cmdRow, err := store.GetCommandLedgerByMachineSequence(ctx, machineID, appendRes.Sequence)
	require.NoError(t, err)

	ledgerDeadline := time.Now().UTC().Add(time.Hour) // command_ledger SLA — must not expire first
	pastAck := time.Now().UTC().Add(-time.Minute)

	att, err := store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, cmdRow.ID, machineID, nil, []byte(`{}`), ledgerDeadline, "")
	require.NoError(t, err)
	require.NoError(t, store.MarkMQTTDispatchAttemptSent(ctx, att.ID, pastAck))

	require.NoError(t, store.ApplyMQTTCommandAckTimeouts(ctx, time.Now().UTC()))

	var attemptStatus string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status FROM machine_command_attempts WHERE id = $1`, att.ID).Scan(&attemptStatus))
	require.Equal(t, "ack_timeout", attemptStatus)
}

func TestP06_E2E_MQTTCommand_commandIDMismatchRejected(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	insertOrganizationSiteMachine(t, ctx, pool, orgID, siteID, machineID, "online", 1)

	appendRes, err := store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:      machineID,
		CommandType:    "noop",
		Payload:        []byte(`{}`),
		IdempotencyKey: "p06-cmdid-mismatch-" + uuid.NewString(),
		DesiredState:   []byte(`{}`),
	})
	require.NoError(t, err)

	wrongID := uuid.New()
	_, err = store.ApplyCommandReceiptTransition(ctx, postgres.CommandReceiptTransitionParams{
		MachineID:  machineID,
		Sequence:   appendRes.Sequence,
		Status:     "acked",
		Payload:    []byte(`{}`),
		DedupeKey:  "dedupe-cmdid-mismatch-" + uuid.NewString(),
		CommandID:  wrongID,
		OccurredAt: time.Now().UTC(),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "command_id mismatch")
}

func TestP06_E2E_MQTTCommand_lateAckRejectedWhenLedgerTimeoutAtPassed(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	insertOrganizationSiteMachine(t, ctx, pool, orgID, siteID, machineID, "online", 1)

	appendRes, err := store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:      machineID,
		CommandType:    "noop",
		Payload:        []byte(`{}`),
		IdempotencyKey: "p06-ledger-to-" + uuid.NewString(),
		DesiredState:   []byte(`{}`),
	})
	require.NoError(t, err)

	cmdRow, err := store.GetCommandLedgerByMachineSequence(ctx, machineID, appendRes.Sequence)
	require.NoError(t, err)

	futureAck := time.Now().UTC().Add(time.Hour)
	att, err := store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, cmdRow.ID, machineID, nil, []byte(`{}`), futureAck, "")
	require.NoError(t, err)
	require.NoError(t, store.MarkMQTTDispatchAttemptSent(ctx, att.ID, futureAck))

	pastLedger := time.Now().UTC().Add(-2 * time.Minute)
	_, err = pool.Exec(ctx, `UPDATE command_ledger SET timeout_at = $1 WHERE id = $2`, pastLedger, cmdRow.ID)
	require.NoError(t, err)

	_, err = store.ApplyCommandReceiptTransition(ctx, postgres.CommandReceiptTransitionParams{
		MachineID:  machineID,
		Sequence:   appendRes.Sequence,
		Status:     "acked",
		Payload:    []byte(`{}`),
		DedupeKey:  "dedupe-late-ledger-to-" + uuid.NewString(),
		CommandID:  appendRes.CommandID,
		OccurredAt: time.Now().UTC(),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "ledger timeout")
}

func TestP06_E2E_MQTTCommand_duplicateAckSameDedupeKeyIsReplay(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	insertOrganizationSiteMachine(t, ctx, pool, orgID, siteID, machineID, "online", 1)

	appendRes, err := store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:      machineID,
		CommandType:    "noop",
		Payload:        []byte(`{}`),
		IdempotencyKey: "p06-ack-dedupe-" + uuid.NewString(),
		DesiredState:   []byte(`{}`),
	})
	require.NoError(t, err)

	cmdRow, err := store.GetCommandLedgerByMachineSequence(ctx, machineID, appendRes.Sequence)
	require.NoError(t, err)
	deadline := time.Now().UTC().Add(time.Hour)
	att, err := store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, cmdRow.ID, machineID, nil, []byte(`{}`), deadline, "")
	require.NoError(t, err)
	require.NoError(t, store.MarkMQTTDispatchAttemptSent(ctx, att.ID, deadline))

	dedupe := "dedupe-p06-" + uuid.NewString()
	p := postgres.CommandReceiptTransitionParams{
		MachineID:  machineID,
		Sequence:   appendRes.Sequence,
		Status:     "acked",
		Payload:    []byte(`{}`),
		DedupeKey:  dedupe,
		CommandID:  appendRes.CommandID,
		OccurredAt: time.Now().UTC(),
	}
	r1, err := store.ApplyCommandReceiptTransition(ctx, p)
	require.NoError(t, err)
	require.False(t, r1.ReceiptReplay)

	r2, err := store.ApplyCommandReceiptTransition(ctx, p)
	require.NoError(t, err)
	require.True(t, r2.ReceiptReplay)
}

func TestP06_E2E_MQTTCommand_publishFailureAllowsRetryAttempt(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	insertOrganizationSiteMachine(t, ctx, pool, orgID, siteID, machineID, "online", 1)

	appendRes, err := store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:      machineID,
		CommandType:    "noop",
		Payload:        []byte(`{}`),
		IdempotencyKey: "p06-pub-fail-" + uuid.NewString(),
		DesiredState:   []byte(`{}`),
	})
	require.NoError(t, err)

	cmdRow, err := store.GetCommandLedgerByMachineSequence(ctx, machineID, appendRes.Sequence)
	require.NoError(t, err)
	deadline := time.Now().UTC().Add(time.Hour)
	att1, err := store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, cmdRow.ID, machineID, nil, []byte(`{}`), deadline, "")
	require.NoError(t, err)
	require.NoError(t, store.MarkMQTTDispatchAttemptPublishFailed(ctx, att1.ID, "mqtt_publish_failed"))

	att2, err := store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, cmdRow.ID, machineID, nil, []byte(`{}`), deadline, "")
	require.NoError(t, err)
	require.NotEqual(t, att1.ID, att2.ID)
}

func TestP06_E2E_MQTTCommand_maxDispatchAttemptsStopsRetries(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)

	orgID := uuid.New()
	siteID := uuid.New()
	machineID := uuid.New()
	insertOrganizationSiteMachine(t, ctx, pool, orgID, siteID, machineID, "online", 1)

	appendRes, err := store.AppendCommandUpdateShadow(ctx, device.AppendCommandInput{
		MachineID:      machineID,
		CommandType:    "noop",
		Payload:        []byte(`{}`),
		IdempotencyKey: "p06-max-att-" + uuid.NewString(),
		DesiredState:   []byte(`{}`),
	})
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE command_ledger SET max_dispatch_attempts = 2 WHERE id = $1`, appendRes.CommandID)
	require.NoError(t, err)
	cmdRow, err := store.GetCommandLedgerByMachineSequence(ctx, machineID, appendRes.Sequence)
	require.NoError(t, err)
	deadline := time.Now().UTC().Add(time.Hour)

	_, err = store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, cmdRow.ID, machineID, nil, []byte(`{}`), deadline, "")
	require.NoError(t, err)
	_, err = store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, cmdRow.ID, machineID, nil, []byte(`{}`), deadline, "")
	require.NoError(t, err)
	_, err = store.InsertMQTTDispatchAttemptWithLedgerMeta(ctx, cmdRow.ID, machineID, nil, []byte(`{}`), deadline, "")
	require.ErrorIs(t, err, postgres.ErrMQTTMaxDispatchAttemptsExceeded)
}
