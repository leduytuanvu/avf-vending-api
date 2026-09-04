package postgres

import (
	"context"
	"encoding/json"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgjson"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ appcommerce.FinancialCorrectnessStore = (*Store)(nil)

func (s *Store) ClaimWinningPayment(ctx context.Context, paymentID, orderID uuid.UUID) (domaincommerce.Order, bool, error) {
	row, err := db.New(s.pool).ClaimWinningPayment(ctx, db.ClaimWinningPaymentParams{
		WinningPaymentID: uuidToPg(paymentID),
		ID:               orderID,
	})
	if err != nil {
		if isNoRows(err) {
			ord, gerr := s.GetOrderByID(ctx, orderID)
			if gerr != nil {
				return domaincommerce.Order{}, false, gerr
			}
			return ord, false, nil
		}
		return domaincommerce.Order{}, false, err
	}
	return mapOrder(row), true, nil
}

func (s *Store) GetWinningPaymentForOrder(ctx context.Context, orderID uuid.UUID) (domaincommerce.Payment, error) {
	row, err := db.New(s.pool).GetWinningPaymentForOrder(ctx, orderID)
	if err != nil {
		if isNoRows(err) {
			return domaincommerce.Payment{}, appcommerce.ErrNotFound
		}
		return domaincommerce.Payment{}, err
	}
	return mapPayment(row), nil
}

func (s *Store) UpdatePaymentOutcome(ctx context.Context, paymentID uuid.UUID, outcome string) (domaincommerce.Payment, error) {
	row, err := db.New(s.pool).UpdatePaymentOutcome(ctx, db.UpdatePaymentOutcomeParams{
		ID:      paymentID,
		Outcome: outcome,
	})
	if err != nil {
		if isNoRows(err) {
			return domaincommerce.Payment{}, appcommerce.ErrNotFound
		}
		return domaincommerce.Payment{}, err
	}
	return mapPayment(row), nil
}

func (s *Store) CancelPaymentByID(ctx context.Context, paymentID uuid.UUID) (domaincommerce.Payment, error) {
	row, err := db.New(s.pool).CancelPaymentByID(ctx, paymentID)
	if err != nil {
		if isNoRows(err) {
			return domaincommerce.Payment{}, appcommerce.ErrNotFound
		}
		return domaincommerce.Payment{}, err
	}
	return mapPayment(row), nil
}

func (s *Store) GetLatestNonCapturedPaymentForOrder(ctx context.Context, orderID uuid.UUID) (domaincommerce.Payment, error) {
	row, err := db.New(s.pool).GetLatestNonCapturedPaymentForOrder(ctx, orderID)
	if err != nil {
		if isNoRows(err) {
			return domaincommerce.Payment{}, appcommerce.ErrNotFound
		}
		return domaincommerce.Payment{}, err
	}
	return mapPayment(row), nil
}

func (s *Store) ListPaymentsForOrder(ctx context.Context, orderID uuid.UUID) ([]domaincommerce.Payment, error) {
	rows, err := db.New(s.pool).ListPaymentsForOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	out := make([]domaincommerce.Payment, 0, len(rows))
	for _, r := range rows {
		out = append(out, mapPayment(r))
	}
	return out, nil
}

func (s *Store) RecordCashAcceptanceEvents(ctx context.Context, in appcommerce.RecordCashAcceptanceEventsInput) error {
	q := db.New(s.pool)
	for _, ev := range in.Events {
		_, err := q.InsertCashAcceptanceEvent(ctx, db.InsertCashAcceptanceEventParams{
			MachineID:         in.MachineID,
			OrderID:           uuidToPg(in.OrderID),
			DeviceEventID:     ev.DeviceEventID,
			DenominationMinor: ev.DenominationMinor,
			CreditSource:      ev.CreditSource,
			Currency:          in.Currency,
			AcceptedAt:        ev.AcceptedAt.UTC(),
			RawMetadata:       pgjson.TextJSON(ev.RawMetadata),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) RecordCashAllocation(ctx context.Context, in appcommerce.RecordCashAllocationInput) (appcommerce.CashAllocationView, error) {
	row, err := db.New(s.pool).InsertCashAllocation(ctx, db.InsertCashAllocationParams{
		OrderID:                in.OrderID,
		PaymentID:              uuidToPg(in.PaymentID),
		MachineID:              in.MachineID,
		AmountMinor:            in.AmountMinor,
		PreOrderCreditMinor:    in.PreOrderCreditMinor,
		PostOrderInsertedMinor: in.PostOrderInsertedMinor,
		ConsentSource:          in.ConsentSource,
		ConsentedAt:            optionalTimeToPgTimestamptz(in.ConsentedAt),
		Currency:               in.Currency,
		IdempotencyKey:         pgtype.Text{String: in.IdempotencyKey, Valid: in.IdempotencyKey != ""},
	})
	if err != nil {
		return appcommerce.CashAllocationView{}, err
	}
	return mapCashAllocation(row), nil
}

func (s *Store) RecordCashChangeEvent(ctx context.Context, in appcommerce.RecordCashChangeEventInput) (appcommerce.CashChangeEventView, error) {
	row, err := db.New(s.pool).InsertCashChangeEvent(ctx, db.InsertCashChangeEventParams{
		OrderID:              in.OrderID,
		PaymentID:            uuidToPg(in.PaymentID),
		MachineID:            in.MachineID,
		ChangeDueMinor:       in.ChangeDueMinor,
		ChangeDispensedMinor: in.ChangeDispensedMinor,
		Outcome:              in.Outcome,
		LiabilityMinor:       in.LiabilityMinor,
		Currency:             in.Currency,
		IdempotencyKey:       pgtype.Text{String: in.IdempotencyKey, Valid: in.IdempotencyKey != ""},
	})
	if err != nil {
		return appcommerce.CashChangeEventView{}, err
	}
	return mapCashChange(row), nil
}

func (s *Store) GetOrderMoneyView(ctx context.Context, orderID uuid.UUID) (appcommerce.OrderMoneyView, error) {
	q := db.New(s.pool)
	ord, err := q.GetOrderByID(ctx, orderID)
	if err != nil {
		if isNoRows(err) {
			return appcommerce.OrderMoneyView{}, appcommerce.ErrNotFound
		}
		return appcommerce.OrderMoneyView{}, err
	}
	payments, err := q.ListPaymentsForOrder(ctx, orderID)
	if err != nil {
		return appcommerce.OrderMoneyView{}, err
	}
	view := appcommerce.OrderMoneyView{OrderID: orderID}
	if ord.WinningPaymentID.Valid {
		id := uuid.UUID(ord.WinningPaymentID.Bytes)
		view.WinningPaymentID = &id
	}
	for _, p := range payments {
		pm := appcommerce.PaymentMoneyView{Payment: mapPayment(p)}
		if view.WinningPaymentID != nil && p.ID == *view.WinningPaymentID {
			pm.IsWinner = true
		} else if p.State == "captured" && view.WinningPaymentID != nil && p.ID != *view.WinningPaymentID {
			pm.IsLosingCapture = true
		}
		view.Payments = append(view.Payments, pm)
	}
	if alloc, err := q.GetCashAllocationForOrder(ctx, orderID); err == nil {
		a := mapCashAllocation(alloc)
		view.CashAllocation = &a
	} else if !isNoRows(err) {
		return appcommerce.OrderMoneyView{}, err
	}
	if chg, err := q.GetCashChangeEventForOrder(ctx, orderID); err == nil {
		c := mapCashChange(chg)
		view.CashChange = &c
		view.OutstandingLiability = c.LiabilityMinor
	} else if !isNoRows(err) {
		return appcommerce.OrderMoneyView{}, err
	}
	events, err := q.ListCashAcceptanceEventsForOrder(ctx, uuidToPg(orderID))
	if err != nil {
		return appcommerce.OrderMoneyView{}, err
	}
	for _, ev := range events {
		view.AcceptanceEvents = append(view.AcceptanceEvents, appcommerce.CashAcceptanceEventView{
			DeviceEventID:     ev.DeviceEventID,
			DenominationMinor: ev.DenominationMinor,
			CreditSource:      ev.CreditSource,
			AcceptedAt:        ev.AcceptedAt,
		})
	}
	return view, nil
}

func (s *Store) InsertLedgerEntry(ctx context.Context, in appcommerce.LedgerEntryInput) error {
	_, err := db.New(s.pool).InsertFinancialLedgerEntry(ctx, db.InsertFinancialLedgerEntryParams{
		EntryType:         in.EntryType,
		SignedAmountMinor: in.SignedAmountMinor,
		Currency:          in.Currency,
		OccurredAt:        in.OccurredAt.UTC(),
		MachineID:         optionalUUIDToPg(in.MachineID),
		OrderID:           optionalUUIDToPg(in.OrderID),
		PaymentID:         optionalUUIDToPg(in.PaymentID),
		Metadata:          pgjson.TextJSON(in.Metadata),
	})
	return err
}

func (s *Store) UpsertReconciliationCase(ctx context.Context, in domaincommerce.ReconciliationCaseInput) (domaincommerce.ReconciliationCase, error) {
	return (&CommerceReconcileRepository{pool: s.pool}).UpsertReconciliationCase(ctx, in)
}

func mapCashAllocation(row db.CashAllocation) appcommerce.CashAllocationView {
	return appcommerce.CashAllocationView{
		ID:                     row.ID,
		OrderID:                row.OrderID,
		AmountMinor:            row.AmountMinor,
		PreOrderCreditMinor:    row.PreOrderCreditMinor,
		PostOrderInsertedMinor: row.PostOrderInsertedMinor,
		ConsentSource:          row.ConsentSource,
	}
}

func mapCashChange(row db.CashChangeEvent) appcommerce.CashChangeEventView {
	return appcommerce.CashChangeEventView{
		ID:                   row.ID,
		ChangeDueMinor:       row.ChangeDueMinor,
		ChangeDispensedMinor: row.ChangeDispensedMinor,
		Outcome:              row.Outcome,
		LiabilityMinor:       row.LiabilityMinor,
	}
}

func reconciliationCaseMetadata(v map[string]any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
}
