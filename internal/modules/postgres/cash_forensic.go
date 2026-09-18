package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgjson"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ appcommerce.CashForensicStore = (*Store)(nil)

func normalizeMovementKind(kind string) string {
	return strings.TrimSpace(strings.ToLower(kind))
}

func normalizeOutcomeFinality(s string) string {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "command_accepted", "confirmed", "ambiguous", "failed", "not_delivered", "requested":
		return strings.TrimSpace(strings.ToLower(s))
	default:
		return "requested"
	}
}

func mapPayoutJournalToFinality(eventType, outcome string) string {
	if o := normalizeOutcomeFinality(outcome); o != "requested" {
		return o
	}
	switch strings.TrimSpace(strings.ToUpper(eventType)) {
	case "NOTE_COMMAND_ACCEPTED":
		return "command_accepted"
	case "NOTE_CONFIRMED", "NOTE_DELIVERED_AFTER_FAULT", "COMPLETED":
		return "confirmed"
	case "NOTE_AMBIGUOUS", "AMBIGUOUS":
		return "ambiguous"
	case "NOTE_FAILED", "FAILED", "NOTE_NOT_DELIVERED":
		return "not_delivered"
	default:
		return "requested"
	}
}

func (s *Store) RecordCashMovements(ctx context.Context, in appcommerce.RecordCashMovementsInput) (appcommerce.RecordCashMovementsResult, error) {
	if s == nil || s.pool == nil {
		return appcommerce.RecordCashMovementsResult{}, appcommerce.ErrNotConfigured
	}
	q := db.New(s.pool)
	var accepted, dup int32
	for _, ev := range in.Events {
		deviceID := strings.TrimSpace(ev.DeviceEventID)
		if deviceID == "" {
			continue
		}
		cur := strings.ToUpper(strings.TrimSpace(ev.Currency))
		if cur == "" {
			cur = "VND"
		}
		at := ev.OccurredAt.UTC()
		if at.IsZero() {
			at = time.Now().UTC()
		}
		kind := normalizeMovementKind(ev.Kind)
		switch kind {
		case "bill_credit", "acceptance", "":
			kind = "bill_credit"
			_, err := q.InsertCashAcceptanceEvent(ctx, db.InsertCashAcceptanceEventParams{
				MachineID:         in.MachineID,
				OrderID:           optionalUUIDToPg(ev.OrderID),
				DeviceEventID:     deviceID,
				DenominationMinor: ev.DenominationMinor,
				CreditSource:      strings.TrimSpace(ev.CreditSource),
				Currency:          cur,
				AcceptedAt:        at,
				BootID:            optionalStringToPgText(ev.BootID),
				OccurredAtDevice:  optionalTimeToPgTimestamptz(&at),
				RawMetadata:       pgjson.TextJSON(ev.RawMetadata),
			})
			if err != nil {
				return appcommerce.RecordCashMovementsResult{}, err
			}
			accepted++
			_ = s.InsertLedgerEntry(ctx, appcommerce.LedgerEntryInput{
				MachineID:         &in.MachineID,
				OrderID:           ev.OrderID,
				EntryType:         "cash_accepted",
				SignedAmountMinor: ev.DenominationMinor,
				Currency:          cur,
				OccurredAt:        at,
				Metadata:          ev.RawMetadata,
			})
		case "bill_lifecycle", "lifecycle":
			_, err := q.InsertCashBillLifecycleEvent(ctx, db.InsertCashBillLifecycleEventParams{
				MachineID:         in.MachineID,
				OrderID:           optionalUUIDToPg(ev.OrderID),
				DeviceEventID:     deviceID,
				LifecycleType:     strings.TrimSpace(strings.ToLower(ev.LifecycleType)),
				DenominationMinor: ev.DenominationMinor,
				RawRecordHex:      strings.TrimSpace(ev.RawRecordHex),
				Currency:          cur,
				OccurredAtDevice:  at,
				RawMetadata:       pgjson.TextJSON(ev.RawMetadata),
			})
			if err != nil {
				return appcommerce.RecordCashMovementsResult{}, err
			}
			accepted++
		case "payout", "withdrawal":
			var rcBefore, rcAfter pgtype.Int4
			rcBefore = optionalInt32ToPgInt4(ev.RecyclerCountBefore)
			rcAfter = optionalInt32ToPgInt4(ev.RecyclerCountAfter)
			withdrawalID := strings.TrimSpace(ev.WithdrawalID)
			if withdrawalID == "" {
				withdrawalID = "unknown"
			}
			_, err := q.InsertCashPayoutEvent(ctx, db.InsertCashPayoutEventParams{
				MachineID:           in.MachineID,
				OrderID:             optionalUUIDToPg(ev.OrderID),
				WithdrawalID:        withdrawalID,
				NoteSequence:        ev.NoteSequence,
				EventType:           strings.TrimSpace(strings.ToUpper(ev.EventType)),
				DeviceEventID:       deviceID,
				DenominationMinor:   ev.DenominationMinor,
				AmountMinor:         ev.AmountMinor,
				RecyclerCountBefore: rcBefore,
				RecyclerCountAfter:  rcAfter,
				OutcomeFinality:     mapPayoutJournalToFinality(ev.EventType, ev.OutcomeFinality),
				Currency:            cur,
				OccurredAtDevice:    at,
				RawMetadata:         pgjson.TextJSON(ev.RawMetadata),
			})
			if err != nil {
				return appcommerce.RecordCashMovementsResult{}, err
			}
			accepted++
			if mapPayoutJournalToFinality(ev.EventType, ev.OutcomeFinality) == "confirmed" && ev.AmountMinor > 0 {
				_ = s.InsertLedgerEntry(ctx, appcommerce.LedgerEntryInput{
					MachineID:         &in.MachineID,
					OrderID:           ev.OrderID,
					EntryType:         "change_dispensed",
					SignedAmountMinor: -ev.AmountMinor,
					Currency:          cur,
					OccurredAt:        at,
					Metadata:          ev.RawMetadata,
				})
			}
		case "recycler_observation", "observation":
			denom := ev.RecyclerDenominationMinor
			if denom <= 0 {
				denom = ev.DenominationMinor
			}
			recyclerCount := int32(ev.AmountMinor)
			if ev.RecyclerCountAfter != nil {
				recyclerCount = *ev.RecyclerCountAfter
			}
			var cb pgtype.Int4
			cb = optionalInt32ToPgInt4(ev.CashboxCount)
			src := strings.TrimSpace(strings.ToLower(ev.ObservationSource))
			if src == "" {
				src = "poll"
			}
			_, err := q.InsertCashHardwareObservation(ctx, db.InsertCashHardwareObservationParams{
				MachineID:                 in.MachineID,
				DeviceEventID:             deviceID,
				ObservedAtDevice:          at,
				RecyclerDenominationMinor: denom,
				RecyclerCount:             recyclerCount,
				CashboxCount:              cb,
				Source:                    src,
				Currency:                  cur,
				RawMetadata:               pgjson.TextJSON(ev.RawMetadata),
			})
			if err != nil {
				return appcommerce.RecordCashMovementsResult{}, err
			}
			accepted++
		default:
			return appcommerce.RecordCashMovementsResult{}, fmt.Errorf("unknown cash movement kind %q", ev.Kind)
		}
	}
	return appcommerce.RecordCashMovementsResult{AcceptedCount: accepted, DuplicateCount: dup}, nil
}

func (s *Store) BindAcceptanceEventsToOrder(ctx context.Context, machineID, orderID uuid.UUID, deviceEventIDs []string) error {
	q := db.New(s.pool)
	for _, id := range deviceEventIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if err := q.UpdateCashAcceptanceEventOrder(ctx, db.UpdateCashAcceptanceEventOrderParams{
			MachineID:     machineID,
			DeviceEventID: id,
			OrderID:       uuidToPg(orderID),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetMachinePhysicalCashPosition(ctx context.Context, machineID uuid.UUID, currency string, asOf *time.Time) (appcommerce.MachinePhysicalCashPosition, error) {
	cur := strings.ToUpper(strings.TrimSpace(currency))
	if cur == "" {
		cur = "VND"
	}
	at := time.Now().UTC()
	if asOf != nil && !asOf.IsZero() {
		at = asOf.UTC()
	}
	q := db.New(s.pool)
	lastClosed, err := q.CashSettlementLastClosedAt(ctx, machineID)
	if err != nil {
		return appcommerce.MachinePhysicalCashPosition{}, err
	}
	since := lastClosed.UTC()
	if since.IsZero() {
		since = time.Unix(0, 0).UTC()
	}
	acceptSums, err := q.SumCashAcceptanceByMachineSince(ctx, db.SumCashAcceptanceByMachineSinceParams{
		MachineID:  machineID,
		Currency:   cur,
		AcceptedAt: since,
	})
	if err != nil {
		return appcommerce.MachinePhysicalCashPosition{}, err
	}
	payoutSum, err := q.SumCashPayoutConfirmedByMachineSince(ctx, db.SumCashPayoutConfirmedByMachineSinceParams{
		MachineID:        machineID,
		Currency:         cur,
		OccurredAtDevice: since,
	})
	if err != nil {
		return appcommerce.MachinePhysicalCashPosition{}, err
	}
	adjSums, err := q.SumCashAdjustmentsByMachineSince(ctx, db.SumCashAdjustmentsByMachineSinceParams{
		MachineID: machineID,
		Currency:  cur,
		CreatedAt: since,
	})
	if err != nil {
		return appcommerce.MachinePhysicalCashPosition{}, err
	}
	collected, err := q.SumCollectedCashByMachineSince(ctx, db.SumCollectedCashByMachineSinceParams{
		MachineID: machineID,
		Currency:  cur,
		ClosedAt:  pgtype.Timestamptz{Time: since, Valid: true},
	})
	if err != nil {
		return appcommerce.MachinePhysicalCashPosition{}, err
	}
	salesNet, err := q.CashSettlementNetExpectedMinor(ctx, db.CashSettlementNetExpectedMinorParams{
		MachineID: machineID,
		Currency:  cur,
	})
	if err != nil {
		return appcommerce.MachinePhysicalCashPosition{}, err
	}
	obs, obsErr := q.GetLatestCashHardwareObservation(ctx, db.GetLatestCashHardwareObservationParams{
		MachineID: machineID,
		Currency:  cur,
	})
	var observedMinor int64
	var observedCount int32
	var observedDenom int64
	if obsErr == nil {
		observedCount = obs.RecyclerCount
		observedDenom = obs.RecyclerDenominationMinor
		observedMinor = int64(observedCount) * observedDenom
	}
	cashboxExpected := acceptSums.CashboxMinor + adjSums.CashboxMinor - collected
	recyclerExpected := acceptSums.RecyclerMinor + adjSums.RecyclerMinor - payoutSum
	if cashboxExpected < 0 {
		cashboxExpected = 0
	}
	if recyclerExpected < 0 {
		recyclerExpected = 0
	}
	evidence := "ACCOUNTING_ONLY"
	if obsErr == nil {
		evidence = "OBSERVED"
	}
	liabilityRows, _ := q.ListUnresolvedChangeLiability(ctx, db.ListUnresolvedChangeLiabilityParams{
		CreatedAt: at,
		Limit:     100,
	})
	var liability int64
	for _, row := range liabilityRows {
		if row.MachineID == machineID {
			liability += row.LiabilityMinor
		}
	}
	ambiguous, _ := q.ListUnresolvedCashPayoutAmbiguous(ctx, db.ListUnresolvedCashPayoutAmbiguousParams{
		MachineID:        machineID,
		OccurredAtDevice: at,
		Limit:            100,
	})
	return appcommerce.MachinePhysicalCashPosition{
		MachineID:                    machineID,
		Currency:                     cur,
		AsOf:                         at,
		PhysicalExpectedCashboxMinor: cashboxExpected,
		PhysicalExpectedRecyclerMinor: recyclerExpected,
		PhysicalExpectedMachineMinor: cashboxExpected + recyclerExpected,
		SalesNetExpectedMinor:        salesNet,
		ObservedRecyclerMinor:        observedMinor,
		ObservedRecyclerCount:        observedCount,
		ObservedRecyclerDenomMinor:   observedDenom,
		UnresolvedLiabilityMinor:     liability,
		AmbiguousEventCount:          int64(len(ambiguous)),
		EvidenceStatus:               evidence,
		Disclosure:                   "Cashbox expected is accounting-derived; recycler observed only when hardware snapshots exist.",
	}, nil
}

func (s *Store) InsertCashAdjustment(ctx context.Context, in appcommerce.InsertCashAdjustmentInput) (appcommerce.CashAdjustmentView, error) {
	row, err := db.New(s.pool).InsertCashAdjustment(ctx, db.InsertCashAdjustmentParams{
		MachineID:         in.MachineID,
		AmountMinor:       in.AmountMinor,
		Bucket:            in.Bucket,
		Reason:            in.Reason,
		OperatorAccountID: optionalUUIDToPg(in.OperatorAccountID),
		IdempotencyKey:    in.IdempotencyKey,
		Currency:          in.Currency,
		Metadata:          pgjson.TextJSON(in.Metadata),
	})
	if err != nil {
		return appcommerce.CashAdjustmentView{}, err
	}
	_ = s.InsertLedgerEntry(ctx, appcommerce.LedgerEntryInput{
		MachineID:         &in.MachineID,
		EntryType:         "adjustment",
		SignedAmountMinor: in.AmountMinor,
		Currency:          in.Currency,
		OccurredAt:        row.CreatedAt.UTC(),
		Metadata:          in.Metadata,
	})
	return appcommerce.CashAdjustmentView{
		ID:          row.ID,
		MachineID:   row.MachineID,
		AmountMinor: row.AmountMinor,
		Bucket:      row.Bucket,
		Reason:      row.Reason,
		Currency:    row.Currency,
		CreatedAt:   row.CreatedAt.UTC(),
	}, nil
}

func (s *Store) ListCashLedger(ctx context.Context, in appcommerce.ListCashLedgerInput) (appcommerce.ListCashLedgerResult, error) {
	if in.Limit <= 0 {
		in.Limit = 50
	}
	if in.Limit > 500 {
		in.Limit = 500
	}
	if in.MachineID == nil || *in.MachineID == uuid.Nil {
		return appcommerce.ListCashLedgerResult{}, fmt.Errorf("machine_id required")
	}
	machineID := *in.MachineID
	q := db.New(s.pool)
	params := db.ListCashLedgerAcceptanceEventsParams{
		MachineID: uuidToPg(machineID),
		FromTime:  pgtype.Timestamptz{Time: in.From.UTC(), Valid: true},
		ToTime:    pgtype.Timestamptz{Time: in.To.UTC(), Valid: true},
		OrderID:   optionalUUIDToPg(in.OrderID),
		AfterTime: optionalTimeToPgTimestamptz(in.AfterOccurredAt),
		AfterID:   ledgerAfterID(in.AfterID),
		Limit:     in.Limit,
	}
	rows, err := q.ListCashLedgerAcceptanceEvents(ctx, params)
	if err != nil {
		return appcommerce.ListCashLedgerResult{}, err
	}
	items := make([]appcommerce.CashLedgerRow, 0, len(rows))
	for _, r := range rows {
		var orderID *uuid.UUID
		orderID = pgUUIDToPtr(r.OrderID)
		items = append(items, appcommerce.CashLedgerRow{
			ID:                r.ID,
			MachineID:         r.MachineID,
			OrderID:           orderID,
			MovementClass:     r.MovementClass,
			EventType:         "BILL_" + strings.ToUpper(strings.TrimSpace(r.CreditSource)),
			Destination:       r.Destination,
			DenominationMinor: r.DenominationMinor,
			AmountMinor:       r.DenominationMinor,
			Currency:          r.Currency,
			OccurredAtDevice:  r.AcceptedAt.UTC(),
			RecordedAt:        r.CreatedAt.UTC(),
			EvidenceStatus:    "CONFIRMED",
			DeviceEventID:     r.DeviceEventID,
		})
	}
	payoutParams := db.ListCashLedgerPayoutEventsParams{
		MachineID: uuidToPg(machineID),
		FromTime:  pgtype.Timestamptz{Time: in.From.UTC(), Valid: true},
		ToTime:    pgtype.Timestamptz{Time: in.To.UTC(), Valid: true},
		OrderID:   optionalUUIDToPg(in.OrderID),
		AfterTime: optionalTimeToPgTimestamptz(in.AfterOccurredAt),
		AfterID:   ledgerAfterID(in.AfterID),
		Limit:     in.Limit,
	}
	payoutRows, err := q.ListCashLedgerPayoutEvents(ctx, payoutParams)
	if err != nil {
		return appcommerce.ListCashLedgerResult{}, err
	}
	for _, r := range payoutRows {
		var orderID *uuid.UUID
		orderID = pgUUIDToPtr(r.OrderID)
		evidence := "CONFIRMED"
		if r.OutcomeFinality == "command_accepted" {
			evidence = "DERIVED"
		} else if r.OutcomeFinality == "ambiguous" {
			evidence = "AMBIGUOUS"
		}
		items = append(items, appcommerce.CashLedgerRow{
			ID:                r.ID,
			MachineID:         r.MachineID,
			OrderID:           orderID,
			MovementClass:     r.MovementClass,
			EventType:         r.EventType,
			Destination:       r.Destination,
			DenominationMinor: r.DenominationMinor,
			AmountMinor:       r.AmountMinor,
			Currency:          r.Currency,
			OccurredAtDevice:  r.OccurredAtDevice.UTC(),
			RecordedAt:        r.CreatedAt.UTC(),
			EvidenceStatus:    evidence,
			DeviceEventID:     r.DeviceEventID,
		})
	}
	lifecycleRows, err := q.ListCashBillLifecycleEventsForLedger(ctx, db.ListCashBillLifecycleEventsForLedgerParams{
		MachineID: uuidToPg(machineID),
		FromTime:  pgtype.Timestamptz{Time: in.From.UTC(), Valid: true},
		ToTime:    pgtype.Timestamptz{Time: in.To.UTC(), Valid: true},
		OrderID:   optionalUUIDToPg(in.OrderID),
		AfterTime: optionalTimeToPgTimestamptz(in.AfterOccurredAt),
		AfterID:   ledgerAfterID(in.AfterID),
		Limit:     in.Limit,
	})
	if err != nil {
		return appcommerce.ListCashLedgerResult{}, err
	}
	for _, r := range lifecycleRows {
		items = append(items, appcommerce.CashLedgerRow{
			ID:                r.ID,
			MachineID:         r.MachineID,
			OrderID:           pgUUIDToPtr(r.OrderID),
			MovementClass:     r.MovementClass,
			EventType:         strings.ToUpper(strings.TrimSpace(r.LifecycleType)),
			Destination:       r.Destination,
			DenominationMinor: r.DenominationMinor,
			AmountMinor:       r.DenominationMinor,
			Currency:          r.Currency,
			OccurredAtDevice:  r.OccurredAtDevice.UTC(),
			RecordedAt:        r.CreatedAt.UTC(),
			EvidenceStatus:    "OBSERVED",
			DeviceEventID:     r.DeviceEventID,
		})
	}
	obsRows, err := q.ListCashHardwareObservationsForLedger(ctx, db.ListCashHardwareObservationsForLedgerParams{
		MachineID: uuidToPg(machineID),
		FromTime:  pgtype.Timestamptz{Time: in.From.UTC(), Valid: true},
		ToTime:    pgtype.Timestamptz{Time: in.To.UTC(), Valid: true},
		AfterTime: optionalTimeToPgTimestamptz(in.AfterOccurredAt),
		AfterID:   ledgerAfterID(in.AfterID),
		Limit:     in.Limit,
	})
	if err != nil {
		return appcommerce.ListCashLedgerResult{}, err
	}
	for _, r := range obsRows {
		items = append(items, appcommerce.CashLedgerRow{
			ID:                r.ID,
			MachineID:         r.MachineID,
			MovementClass:     r.MovementClass,
			EventType:         strings.ToUpper(strings.TrimSpace(r.Source)),
			Destination:       r.Destination,
			DenominationMinor: r.RecyclerDenominationMinor,
			AmountMinor:       int64(r.RecyclerCount) * r.RecyclerDenominationMinor,
			Currency:          r.Currency,
			OccurredAtDevice:  r.ObservedAtDevice.UTC(),
			RecordedAt:        r.CreatedAt.UTC(),
			EvidenceStatus:    "OBSERVED",
			DeviceEventID:     r.DeviceEventID,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].OccurredAtDevice.Equal(items[j].OccurredAtDevice) {
			return items[i].ID.String() > items[j].ID.String()
		}
		return items[i].OccurredAtDevice.After(items[j].OccurredAtDevice)
	})
	if int32(len(items)) > in.Limit {
		items = items[:in.Limit]
	}
	var next *appcommerce.CashLedgerCursor
	if len(items) > 0 {
		last := items[len(items)-1]
		next = &appcommerce.CashLedgerCursor{OccurredAt: last.OccurredAtDevice, ID: last.ID}
	}
	return appcommerce.ListCashLedgerResult{Items: items, NextCursor: next}, nil
}

func (s *Store) GetCashLedgerEventDetail(ctx context.Context, eventID uuid.UUID, movementClass string) (appcommerce.CashLedgerEventDetail, error) {
	q := db.New(s.pool)
	switch strings.TrimSpace(strings.ToLower(movementClass)) {
	case "payout":
		row, err := q.GetCashPayoutEventByID(ctx, eventID)
		if err != nil {
			if isNoRows(err) {
				return appcommerce.CashLedgerEventDetail{}, appcommerce.ErrNotFound
			}
			return appcommerce.CashLedgerEventDetail{}, err
		}
		orderID := pgUUIDToPtr(row.OrderID)
		evidence := "CONFIRMED"
		if row.OutcomeFinality == "command_accepted" {
			evidence = "DERIVED"
		} else if row.OutcomeFinality == "ambiguous" {
			evidence = "AMBIGUOUS"
		}
		extra := map[string]any{
			"withdrawalId":        row.WithdrawalID,
			"noteSequence":          row.NoteSequence,
			"outcomeFinality":       row.OutcomeFinality,
			"recyclerCountBefore":   nullInt(row.RecyclerCountBefore),
			"recyclerCountAfter":    nullInt(row.RecyclerCountAfter),
		}
		meta := map[string]any{}
		_ = json.Unmarshal(row.RawMetadata, &meta)
		return appcommerce.CashLedgerEventDetail{
			Row: appcommerce.CashLedgerRow{
				ID:                row.ID,
				MachineID:         row.MachineID,
				OrderID:           orderID,
				MovementClass:     "payout",
				EventType:         row.EventType,
				Destination:       "customer",
				DenominationMinor: row.DenominationMinor,
				AmountMinor:       row.AmountMinor,
				Currency:          row.Currency,
				OccurredAtDevice:  row.OccurredAtDevice.UTC(),
				RecordedAt:        row.CreatedAt.UTC(),
				EvidenceStatus:    evidence,
				DeviceEventID:     row.DeviceEventID,
			},
			RawMetadata: meta,
			Extra:       extra,
		}, nil
	default:
		row, err := q.GetCashAcceptanceEventByID(ctx, eventID)
		if err != nil {
			if isNoRows(err) {
				return appcommerce.CashLedgerEventDetail{}, appcommerce.ErrNotFound
			}
			return appcommerce.CashLedgerEventDetail{}, err
		}
		orderID := pgUUIDToPtr(row.OrderID)
		dest := "cashbox"
		if row.CreditSource == "stored_in_recycler" {
			dest = "recycler"
		}
		meta := map[string]any{}
		_ = json.Unmarshal(row.RawMetadata, &meta)
		return appcommerce.CashLedgerEventDetail{
			Row: appcommerce.CashLedgerRow{
				ID:                row.ID,
				MachineID:         row.MachineID,
				OrderID:           orderID,
				MovementClass:     "acceptance",
				EventType:         row.CreditSource,
				Destination:       dest,
				DenominationMinor: row.DenominationMinor,
				AmountMinor:       row.DenominationMinor,
				Currency:          row.Currency,
				OccurredAtDevice:  row.AcceptedAt.UTC(),
				RecordedAt:        row.CreatedAt.UTC(),
				EvidenceStatus:    "CONFIRMED",
				DeviceEventID:     row.DeviceEventID,
			},
			RawMetadata: meta,
		}, nil
	}
}

func (s *Store) GetOrderCashForensics(ctx context.Context, orderID uuid.UUID) (appcommerce.OrderCashForensicsView, error) {
	money, err := s.GetOrderMoneyView(ctx, orderID)
	if err != nil {
		return appcommerce.OrderCashForensicsView{}, err
	}
	q := db.New(s.pool)
	lifecycleRows, err := q.ListCashBillLifecycleEventsForOrder(ctx, uuidToPg(orderID))
	if err != nil {
		return appcommerce.OrderCashForensicsView{}, err
	}
	payoutRows, err := q.ListCashPayoutEventsForOrder(ctx, uuidToPg(orderID))
	if err != nil {
		return appcommerce.OrderCashForensicsView{}, err
	}
	lifecycle := make([]appcommerce.CashLifecycleEventView, 0, len(lifecycleRows))
	for _, row := range lifecycleRows {
		lifecycle = append(lifecycle, appcommerce.CashLifecycleEventView{
			DeviceEventID:     row.DeviceEventID,
			LifecycleType:     row.LifecycleType,
			DenominationMinor: row.DenominationMinor,
			RawRecordHex:      row.RawRecordHex,
			OccurredAt:        row.OccurredAtDevice.UTC(),
		})
	}
	payouts := make([]appcommerce.CashPayoutEventView, 0, len(payoutRows))
	for _, row := range payoutRows {
		payouts = append(payouts, appcommerce.CashPayoutEventView{
			ID:                  row.ID,
			WithdrawalID:        row.WithdrawalID,
			NoteSequence:        row.NoteSequence,
			EventType:           row.EventType,
			DenominationMinor:   row.DenominationMinor,
			AmountMinor:         row.AmountMinor,
			OutcomeFinality:     row.OutcomeFinality,
			RecyclerCountBefore: pgInt4ToPtr(row.RecyclerCountBefore),
			RecyclerCountAfter:  pgInt4ToPtr(row.RecyclerCountAfter),
			OccurredAt:          row.OccurredAtDevice.UTC(),
		})
	}
	var gross int64
	for _, ev := range money.AcceptanceEvents {
		gross += ev.DenominationMinor
	}
	remainder := int64(0)
	if money.CashAllocation != nil {
		remainder = gross - money.CashAllocation.AmountMinor
		if remainder < 0 {
			remainder = 0
		}
	}
	return appcommerce.OrderCashForensicsView{
		OrderMoneyView:  money,
		RemainderMinor:  remainder,
		LifecycleEvents: lifecycle,
		PayoutEvents:    payouts,
	}, nil
}

func pgInt4ToPtr(v pgtype.Int4) *int32 {
	if !v.Valid {
		return nil
	}
	x := v.Int32
	return &x
}

func ledgerAfterID(id *uuid.UUID) pgtype.Text {
	if id == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: id.String(), Valid: true}
}

func nullInt(v pgtype.Int4) any {
	if !v.Valid {
		return nil
	}
	return v.Int32
}
