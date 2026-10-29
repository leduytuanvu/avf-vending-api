package postgres_test

import (
	"context"
	"testing"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/app/machineruntime"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRecordCashMovements_duplicateSameIdNoSecondLedger(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	boot := uuid.NewString()
	deviceID := boot + ":A1B204"
	occurred := time.Now().UTC().Truncate(time.Millisecond)
	ev := appcommerce.CashMovementEventInput{
		Kind:              "bill_credit",
		DeviceEventID:     deviceID,
		BootID:            boot,
		DenominationMinor: 10000,
		CreditSource:      "stored_in_recycler",
		Currency:          "VND",
		OccurredAt:        occurred,
		RawMetadata:       []byte(`{"rawRecordHex":"A1B204"}`),
	}
	in := appcommerce.RecordCashMovementsInput{
		MachineID:      testfixtures.DevMachineID,
		IdempotencyKey: "cash_movement-dup-" + uuid.NewString(),
		Events:         []appcommerce.CashMovementEventInput{ev},
	}

	first, err := store.RecordCashMovements(ctx, in)
	require.NoError(t, err)
	require.Equal(t, int32(1), first.AcceptedCount)
	require.Equal(t, int32(0), first.DuplicateCount)

	second, err := store.RecordCashMovements(ctx, in)
	require.NoError(t, err)
	require.Equal(t, int32(0), second.AcceptedCount)
	require.Equal(t, int32(1), second.DuplicateCount)

	var acceptCount int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM cash_acceptance_events
WHERE machine_id = $1 AND device_event_id = $2`,
		testfixtures.DevMachineID, deviceID,
	).Scan(&acceptCount))
	require.Equal(t, 1, acceptCount)

	var ledgerCount int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM financial_ledger_entries
WHERE machine_id = $1
  AND entry_type = 'cash_accepted'
  AND signed_amount_minor = 10000
  AND occurred_at = $2`,
		testfixtures.DevMachineID, occurred,
	).Scan(&ledgerCount))
	require.Equal(t, 1, ledgerCount)
}

func TestRecordCashMovements_sameHexDifferentBootInsertsTwo(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	store := postgres.NewStore(pool)
	hex := "C2D204"
	boot1 := uuid.NewString()
	boot2 := uuid.NewString()
	occurred := time.Now().UTC().Truncate(time.Millisecond)
	mk := func(boot string) appcommerce.CashMovementEventInput {
		return appcommerce.CashMovementEventInput{
			Kind:              "bill_credit",
			DeviceEventID:     hex,
			BootID:            boot,
			DenominationMinor: 10000,
			CreditSource:      "stored_in_recycler",
			Currency:          "VND",
			OccurredAt:        occurred,
			RawMetadata:       []byte(`{"rawRecordHex":"C2D204"}`),
		}
	}

	first, err := store.RecordCashMovements(ctx, appcommerce.RecordCashMovementsInput{
		MachineID:      testfixtures.DevMachineID,
		IdempotencyKey: "cash_movement-boot1-" + uuid.NewString(),
		Events:         []appcommerce.CashMovementEventInput{mk(boot1)},
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), first.AcceptedCount)

	second, err := store.RecordCashMovements(ctx, appcommerce.RecordCashMovementsInput{
		MachineID:      testfixtures.DevMachineID,
		IdempotencyKey: "cash_movement-boot2-" + uuid.NewString(),
		Events:         []appcommerce.CashMovementEventInput{mk(boot2)},
	})
	require.NoError(t, err)
	require.Equal(t, int32(1), second.AcceptedCount)
	require.Equal(t, int32(0), second.DuplicateCount)

	var acceptCount int
	require.NoError(t, pool.QueryRow(ctx, `
SELECT COUNT(*) FROM cash_acceptance_events
WHERE machine_id = $1 AND device_event_id IN ($2, $3)`,
		testfixtures.DevMachineID, boot1+":"+hex, boot2+":"+hex,
	).Scan(&acceptCount))
	require.Equal(t, 2, acceptCount)
}

func TestLookupOfflineEventReconcileStatus_cashMovementProcessedWithoutAcceptanceIsRetryable(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	boot := uuid.NewString()
	hex := "E3F204"
	deviceID := boot + ":" + hex
	key := "cash_movement:" + testfixtures.DevMachineID.String() + ":" + boot + ":" + hex + ":bill_credit"
	seq := time.Now().UnixNano()

	_, err := pool.Exec(ctx, `
INSERT INTO machine_offline_events (
    machine_id, offline_sequence, event_type, event_id, client_event_id,
    occurred_at, payload, processing_status, processing_error, idempotency_key
) VALUES (
    $1, $2, 'commerce.report_cash_movements', $3, $3,
    now(), '{}'::jsonb, 'processed', '', $4
)`, testfixtures.DevMachineID, seq, "cli-"+uuid.NewString(), key)
	require.NoError(t, err)

	q := db.New(pool)
	got, err := machineruntime.LookupOfflineEventReconcileStatus(ctx, q, testfixtures.DevMachineID, key)
	require.NoError(t, err)
	require.True(t, got.Found)
	require.Equal(t, "failed_retryable", got.Status)
	require.True(t, got.Retryable)

	store := postgres.NewStore(pool)
	_, err = store.RecordCashMovements(ctx, appcommerce.RecordCashMovementsInput{
		MachineID:      testfixtures.DevMachineID,
		IdempotencyKey: key + "-ingest",
		Events: []appcommerce.CashMovementEventInput{{
			Kind:              "bill_credit",
			DeviceEventID:     deviceID,
			BootID:            boot,
			DenominationMinor: 10000,
			CreditSource:      "stored_in_recycler",
			Currency:          "VND",
			OccurredAt:        time.Now().UTC(),
		}},
	})
	require.NoError(t, err)

	got, err = machineruntime.LookupOfflineEventReconcileStatus(ctx, q, testfixtures.DevMachineID, key)
	require.NoError(t, err)
	require.True(t, got.Found)
	require.Equal(t, "processed", got.Status)
	require.False(t, got.Retryable)
}
