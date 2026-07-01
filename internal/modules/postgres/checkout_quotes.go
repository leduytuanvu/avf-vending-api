package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	appcommerce "github.com/avf/avf-vending-api/internal/app/commerce"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// ResolveSaleLineWithQuantity implements quantity-aware pricing for multi-cart quotes.
func (s *Store) ResolveSaleLineWithQuantity(ctx context.Context, in appcommerce.ResolveSaleLineInput, qty int32) (appcommerce.ResolvedSaleLine, error) {
	if qty <= 0 {
		qty = 1
	}
	return s.EvaluateSaleLineAt(ctx, in, time.Now().UTC(), qty)
}

func (s *Store) CreateQuoteWithLines(ctx context.Context, in appcommerce.PersistQuoteInput) (appcommerce.PersistQuoteResult, error) {
	if s == nil || s.pool == nil {
		return appcommerce.PersistQuoteResult{}, errors.New("postgres: nil store")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return appcommerce.PersistQuoteResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)
	key := strings.TrimSpace(in.IdempotencyKey)
	if key != "" {
		existing, err := q.GetCheckoutQuoteByMachineAndIdempotencyKey(ctx, db.GetCheckoutQuoteByMachineAndIdempotencyKeyParams{
			MachineID:      in.MachineID,
			IdempotencyKey: optionalStringToPgText(key),
		})
		if err == nil {
			lines, lerr := q.ListCheckoutQuoteLines(ctx, existing.ID)
			if lerr != nil {
				return appcommerce.PersistQuoteResult{}, lerr
			}
			if err := tx.Commit(ctx); err != nil {
				return appcommerce.PersistQuoteResult{}, err
			}
			return mapPersistQuote(existing, lines, true), nil
		}
		if !isNoRows(err) {
			return appcommerce.PersistQuoteResult{}, err
		}
	}
	quoteRow, err := q.InsertCheckoutQuote(ctx, db.InsertCheckoutQuoteParams{
		MachineID:      in.MachineID,
		Currency:       in.Currency,
		PaymentMethod:  in.PaymentMethod,
		SubtotalMinor:  in.SubtotalMinor,
		DiscountMinor:  in.DiscountMinor,
		PayableMinor:   in.PayableMinor,
		State:          "active",
		IdempotencyKey: optionalStringToPgText(key),
		ExpiresAt:      in.ExpiresAt.UTC(),
	})
	if err != nil {
		return appcommerce.PersistQuoteResult{}, err
	}
	var lineRows []db.CheckoutQuoteLine
	for _, line := range in.Lines {
		row, err := q.InsertCheckoutQuoteLine(ctx, db.InsertCheckoutQuoteLineParams{
			QuoteID:            quoteRow.ID,
			LineSequence:       line.LineSequence,
			ProductID:          line.ProductID,
			SlotConfigID:       uuidPtrToPgUUID(line.SlotConfigID),
			CabinetCode:        line.CabinetCode,
			SlotCode:           line.SlotCode,
			SlotIndex:          line.SlotIndex,
			Quantity:           line.Quantity,
			UnitPriceMinor:     line.UnitPriceMinor,
			LineSubtotalMinor:  line.LineSubtotalMinor,
			PricingFingerprint: line.PricingFingerprint,
		})
		if err != nil {
			return appcommerce.PersistQuoteResult{}, err
		}
		lineRows = append(lineRows, row)
	}
	if err := tx.Commit(ctx); err != nil {
		return appcommerce.PersistQuoteResult{}, err
	}
	return mapPersistQuote(quoteRow, lineRows, false), nil
}

func (s *Store) TryReplayQuoteByIdempotency(ctx context.Context, machineID uuid.UUID, idempotencyKey string) (appcommerce.PersistQuoteResult, bool, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" || machineID == uuid.Nil {
		return appcommerce.PersistQuoteResult{}, false, nil
	}
	q := db.New(s.pool)
	row, err := q.GetCheckoutQuoteByMachineAndIdempotencyKey(ctx, db.GetCheckoutQuoteByMachineAndIdempotencyKeyParams{
		MachineID:      machineID,
		IdempotencyKey: optionalStringToPgText(key),
	})
	if err != nil {
		if isNoRows(err) {
			return appcommerce.PersistQuoteResult{}, false, nil
		}
		return appcommerce.PersistQuoteResult{}, false, err
	}
	lines, err := q.ListCheckoutQuoteLines(ctx, row.ID)
	if err != nil {
		return appcommerce.PersistQuoteResult{}, false, err
	}
	return mapPersistQuote(row, lines, true), true, nil
}

func (s *Store) GetQuoteWithLines(ctx context.Context, quoteID uuid.UUID) (appcommerce.PersistQuoteResult, error) {
	q := db.New(s.pool)
	row, err := q.GetCheckoutQuoteByID(ctx, quoteID)
	if err != nil {
		return appcommerce.PersistQuoteResult{}, err
	}
	lines, err := q.ListCheckoutQuoteLines(ctx, row.ID)
	if err != nil {
		return appcommerce.PersistQuoteResult{}, err
	}
	return mapPersistQuote(row, lines, false), nil
}

func (s *Store) TryReplayOrderFromQuote(ctx context.Context, machineID uuid.UUID, idempotencyKey string) (appcommerce.PersistOrderFromQuoteResult, bool, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return appcommerce.PersistOrderFromQuoteResult{}, false, nil
	}
	replay, ok, err := s.TryReplayCreateOrderWithVend(ctx, machineID, key)
	if err != nil || !ok {
		return appcommerce.PersistOrderFromQuoteResult{}, ok, err
	}
	vends, err := db.New(s.pool).ListVendSessionsByOrder(ctx, replay.Order.ID)
	if err != nil {
		return appcommerce.PersistOrderFromQuoteResult{}, false, err
	}
	return appcommerce.PersistOrderFromQuoteResult{
		Order:  replay.Order,
		Lines:  mapPersistVendLines(vends),
		Replay: true,
	}, true, nil
}

func (s *Store) CreateOrderFromQuoteWithVendSessions(ctx context.Context, in appcommerce.PersistOrderFromQuoteInput) (appcommerce.PersistOrderFromQuoteResult, error) {
	quote := in.Quote
	if quote.QuoteID == uuid.Nil {
		return appcommerce.PersistOrderFromQuoteResult{}, errors.New("postgres: quote_id required")
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if key == "" {
		return appcommerce.PersistOrderFromQuoteResult{}, errors.New("postgres: idempotency_key required")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return appcommerce.PersistOrderFromQuoteResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	existingOrder, err := q.GetOrderByMachineAndIdempotencyKey(ctx, db.GetOrderByMachineAndIdempotencyKeyParams{
		MachineID:      quote.MachineID,
		IdempotencyKey: optionalStringToPgText(key),
	})
	if err == nil {
		vends, verr := q.ListVendSessionsByOrder(ctx, existingOrder.ID)
		if verr != nil {
			return appcommerce.PersistOrderFromQuoteResult{}, verr
		}
		if err := tx.Commit(ctx); err != nil {
			return appcommerce.PersistOrderFromQuoteResult{}, err
		}
		return appcommerce.PersistOrderFromQuoteResult{
			Order:  mapOrder(existingOrder),
			Lines:  mapPersistVendLines(vends),
			Replay: true,
		}, nil
	}
	if !isNoRows(err) {
		return appcommerce.PersistOrderFromQuoteResult{}, err
	}

	lockedQuote, err := q.GetCheckoutQuoteByID(ctx, quote.QuoteID)
	if err != nil {
		return appcommerce.PersistOrderFromQuoteResult{}, err
	}
	if strings.ToLower(strings.TrimSpace(lockedQuote.State)) != "active" {
		return appcommerce.PersistOrderFromQuoteResult{}, fmt.Errorf("postgres: quote not active")
	}
	if time.Now().UTC().After(lockedQuote.ExpiresAt.UTC()) {
		return appcommerce.PersistOrderFromQuoteResult{}, fmt.Errorf("postgres: quote expired")
	}

	orderRow, err := q.InsertOrder(ctx, db.InsertOrderParams{
		MachineID:          quote.MachineID,
		Status:             "created",
		Currency:           quote.Currency,
		SubtotalMinor:      quote.SubtotalMinor,
		TaxMinor:           0,
		TotalMinor:         quote.PayableMinor,
		IdempotencyKey:     optionalStringToPgText(key),
		Simulated:          in.Simulated,
		SimulationRunID:    optionalStringToPgText(in.SimulationRunID),
		SimulationScenario: optionalStringToPgText(in.SimulationScenario),
		FakeBill:           in.FakeBill,
		FakeBoard:          in.FakeBoard,
	})
	if err != nil {
		return appcommerce.PersistOrderFromQuoteResult{}, err
	}

	quoteLines, err := q.ListCheckoutQuoteLines(ctx, quote.QuoteID)
	if err != nil {
		return appcommerce.PersistOrderFromQuoteResult{}, err
	}

	var vendLines []appcommerce.PersistOrderFromQuoteVendLine
	lineSeq := int32(0)
	for _, ql := range quoteLines {
		qty := ql.Quantity
		if qty <= 0 {
			qty = 1
		}
		for u := int32(0); u < qty; u++ {
			lineSeq++
			vRow, err := q.InsertVendSessionWithLineSequence(ctx, db.InsertVendSessionWithLineSequenceParams{
				OrderID:            orderRow.ID,
				MachineID:          quote.MachineID,
				SlotIndex:          ql.SlotIndex,
				ProductID:          ql.ProductID,
				State:              "pending",
				Simulated:          in.Simulated,
				SimulationRunID:    optionalStringToPgText(in.SimulationRunID),
				SimulationScenario: optionalStringToPgText(in.SimulationScenario),
				LineSequence:       lineSeq,
			})
			if err != nil {
				return appcommerce.PersistOrderFromQuoteResult{}, err
			}
			vendLines = append(vendLines, appcommerce.PersistOrderFromQuoteVendLine{
				VendSessionID: vRow.ID,
				LineSequence:  vRow.LineSequence,
				SlotIndex:     vRow.SlotIndex,
				ProductID:     vRow.ProductID,
				CabinetCode:   ql.CabinetCode,
				SlotCode:      ql.SlotCode,
				VendState:     vRow.State,
			})
		}
	}

	if _, err := q.MarkCheckoutQuoteConsumed(ctx, quote.QuoteID); err != nil {
		return appcommerce.PersistOrderFromQuoteResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return appcommerce.PersistOrderFromQuoteResult{}, err
	}
	return appcommerce.PersistOrderFromQuoteResult{
		Order:  mapOrder(orderRow),
		Lines:  vendLines,
		Replay: false,
	}, nil
}

func mapPersistQuote(row db.CheckoutQuote, lines []db.CheckoutQuoteLine, replay bool) appcommerce.PersistQuoteResult {
	out := appcommerce.PersistQuoteResult{
		QuoteID:       row.ID,
		MachineID:     row.MachineID,
		Currency:      row.Currency,
		PaymentMethod: row.PaymentMethod,
		SubtotalMinor: row.SubtotalMinor,
		DiscountMinor: row.DiscountMinor,
		PayableMinor:  row.PayableMinor,
		ExpiresAt:     row.ExpiresAt,
		State:         row.State,
		Replay:        replay,
	}
	for _, l := range lines {
		slotCfg := uuid.Nil
		if l.SlotConfigID.Valid {
			slotCfg = l.SlotConfigID.Bytes
		}
		out.Lines = append(out.Lines, appcommerce.PersistQuoteLineInput{
			LineSequence:       l.LineSequence,
			ProductID:          l.ProductID,
			SlotConfigID:       slotCfg,
			CabinetCode:        l.CabinetCode,
			SlotCode:           l.SlotCode,
			SlotIndex:          l.SlotIndex,
			Quantity:           l.Quantity,
			UnitPriceMinor:     l.UnitPriceMinor,
			LineSubtotalMinor:  l.LineSubtotalMinor,
			PricingFingerprint: l.PricingFingerprint,
		})
	}
	return out
}

func mapPersistVendLines(vends []db.ListVendSessionsByOrderRow) []appcommerce.PersistOrderFromQuoteVendLine {
	out := make([]appcommerce.PersistOrderFromQuoteVendLine, 0, len(vends))
	for _, v := range vends {
		out = append(out, appcommerce.PersistOrderFromQuoteVendLine{
			VendSessionID: v.ID,
			LineSequence:  v.LineSequence,
			SlotIndex:     v.SlotIndex,
			ProductID:     v.ProductID,
			VendState:     v.State,
		})
	}
	return out
}

func uuidPtrToPgUUID(id uuid.UUID) pgtype.UUID {
	if id == uuid.Nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}
