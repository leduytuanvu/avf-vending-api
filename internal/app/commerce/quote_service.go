package commerce

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domaincommerce "github.com/avf/avf-vending-api/internal/domain/commerce"
	"github.com/google/uuid"
)

const defaultQuoteTTL = 15 * time.Minute

// QuoteStore persists checkout quotes and multi-vend order creation from quotes.
type QuoteStore interface {
	CreateQuoteWithLines(ctx context.Context, in PersistQuoteInput) (PersistQuoteResult, error)
	TryReplayQuoteByIdempotency(ctx context.Context, machineID uuid.UUID, idempotencyKey string) (PersistQuoteResult, bool, error)
	GetQuoteWithLines(ctx context.Context, quoteID uuid.UUID) (PersistQuoteResult, error)
	CreateOrderFromQuoteWithVendSessions(ctx context.Context, in PersistOrderFromQuoteInput) (PersistOrderFromQuoteResult, error)
	TryReplayOrderFromQuote(ctx context.Context, machineID uuid.UUID, idempotencyKey string) (PersistOrderFromQuoteResult, bool, error)
}

// PersistQuoteInput is the persistence payload for quote creation.
type PersistQuoteInput struct {
	MachineID      uuid.UUID
	Currency       string
	PaymentMethod  string
	SubtotalMinor  int64
	DiscountMinor  int64
	PayableMinor   int64
	IdempotencyKey string
	ExpiresAt      time.Time
	Lines          []PersistQuoteLineInput
}

type PersistQuoteLineInput struct {
	LineSequence       int32
	ProductID          uuid.UUID
	SlotConfigID       uuid.UUID
	CabinetCode        string
	SlotCode           string
	SlotIndex          int32
	Quantity           int32
	UnitPriceMinor     int64
	LineSubtotalMinor  int64
	PricingFingerprint string
}

type PersistQuoteResult struct {
	QuoteID       uuid.UUID
	MachineID     uuid.UUID
	Currency      string
	PaymentMethod string
	SubtotalMinor int64
	DiscountMinor int64
	PayableMinor  int64
	ExpiresAt     time.Time
	State         string
	Lines         []PersistQuoteLineInput
	Replay        bool
}

type PersistOrderFromQuoteInput struct {
	Quote              PersistQuoteResult
	IdempotencyKey     string
	Simulated          bool
	SimulationRunID    string
	SimulationScenario string
	FakeBill           bool
	FakeBoard          bool
}

type PersistOrderFromQuoteVendLine struct {
	VendSessionID uuid.UUID
	LineSequence  int32
	SlotIndex     int32
	ProductID     uuid.UUID
	CabinetCode   string
	SlotCode      string
	VendState     string
}

type PersistOrderFromQuoteResult struct {
	Order  domaincommerce.Order
	Lines  []PersistOrderFromQuoteVendLine
	Replay bool
}

// CreateQuote prices a multi-line cart and persists an auditable quote snapshot.
func (s *Service) CreateQuote(ctx context.Context, in CreateQuoteInput) (CreateQuoteResult, error) {
	if err := validateCreateQuote(in); err != nil {
		return CreateQuoteResult{}, err
	}
	store := s.quoteStore()
	if store == nil {
		return CreateQuoteResult{}, ErrNotConfigured
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if key != "" {
		if replay, ok, err := store.TryReplayQuoteByIdempotency(ctx, in.MachineID, key); err != nil {
			return CreateQuoteResult{}, err
		} else if ok {
			return mapPersistQuoteResult(replay), nil
		}
	}
	ttl := in.QuoteTTL
	if ttl <= 0 {
		ttl = defaultQuoteTTL
	}
	expiresAt := time.Now().UTC().Add(ttl)
	var (
		persistLines []PersistQuoteLineInput
		views        []QuoteLineView
		subtotal     int64
	)
	for i, line := range in.Lines {
		qty := line.Quantity
		if qty <= 0 {
			qty = 1
		}
		resIn := ResolveSaleLineInput{
			MachineID:   in.MachineID,
			ProductID:   line.ProductID,
			SlotID:      line.SlotID,
			CabinetCode: line.CabinetCode,
			SlotCode:    line.SlotCode,
			SlotIndex:   line.SlotIndex,
		}
		var resolved ResolvedSaleLine
		var err error
		if qa, ok := s.saleLines.(QuantityAwareSaleLineResolver); ok {
			resolved, err = qa.ResolveSaleLineWithQuantity(ctx, resIn, qty)
		} else {
			resolved, err = s.saleLines.ResolveSaleLine(ctx, resIn)
			if err == nil && qty > 1 {
				resolved.TotalMinor = resolved.PriceMinor * int64(qty)
				resolved.SubtotalMinor = resolved.TotalMinor
			}
		}
		if err != nil {
			return CreateQuoteResult{}, err
		}
		seq := int32(i + 1)
		lineSub := resolved.TotalMinor
		subtotal += lineSub
		persistLines = append(persistLines, PersistQuoteLineInput{
			LineSequence:       seq,
			ProductID:          line.ProductID,
			SlotConfigID:       resolved.SlotConfigID,
			CabinetCode:        resolved.CabinetCode,
			SlotCode:           resolved.SlotCode,
			SlotIndex:          resolved.SlotIndex,
			Quantity:           qty,
			UnitPriceMinor:     resolved.PriceMinor,
			LineSubtotalMinor:  lineSub,
			PricingFingerprint: resolved.PricingFingerprint,
		})
		views = append(views, QuoteLineView{
			LineSequence:       seq,
			ProductID:          line.ProductID,
			SlotConfigID:       resolved.SlotConfigID,
			CabinetCode:        resolved.CabinetCode,
			SlotCode:           resolved.SlotCode,
			SlotIndex:          resolved.SlotIndex,
			Quantity:           qty,
			UnitPriceMinor:     resolved.PriceMinor,
			LineSubtotalMinor:  lineSub,
			PricingFingerprint: resolved.PricingFingerprint,
			PromotionLabel:     resolved.PromotionLabel,
		})
	}
	discountMinor := int64(0)
	payable := subtotal - discountMinor
	persisted, err := store.CreateQuoteWithLines(ctx, PersistQuoteInput{
		MachineID:      in.MachineID,
		Currency:       strings.ToUpper(strings.TrimSpace(in.Currency)),
		PaymentMethod:  strings.TrimSpace(in.PaymentMethod),
		SubtotalMinor:  subtotal,
		DiscountMinor:  discountMinor,
		PayableMinor:   payable,
		IdempotencyKey: key,
		ExpiresAt:      expiresAt,
		Lines:          persistLines,
	})
	if err != nil {
		return CreateQuoteResult{}, err
	}
	out := mapPersistQuoteResult(persisted)
	out.Lines = views
	return out, nil
}

// CreateOrderFromQuote creates an order and expanded vend sessions from a quote.
func (s *Service) CreateOrderFromQuote(ctx context.Context, in CreateOrderFromQuoteInput) (CreateOrderFromQuoteResult, error) {
	if err := validateCreateOrderFromQuote(in); err != nil {
		return CreateOrderFromQuoteResult{}, err
	}
	store := s.quoteStore()
	if store == nil {
		return CreateOrderFromQuoteResult{}, ErrNotConfigured
	}
	key := strings.TrimSpace(in.IdempotencyKey)
	if key != "" {
		if replay, ok, err := store.TryReplayOrderFromQuote(ctx, in.MachineID, key); err != nil {
			return CreateOrderFromQuoteResult{}, err
		} else if ok {
			return mapPersistOrderFromQuote(replay), nil
		}
	}
	quote, err := store.GetQuoteWithLines(ctx, in.QuoteID)
	if err != nil {
		return CreateOrderFromQuoteResult{}, err
	}
	if quote.MachineID != in.MachineID {
		return CreateOrderFromQuoteResult{}, fmt.Errorf("quote machine mismatch")
	}
	if strings.ToLower(strings.TrimSpace(quote.State)) != "active" {
		return CreateOrderFromQuoteResult{}, fmt.Errorf("quote is not active")
	}
	if time.Now().UTC().After(quote.ExpiresAt.UTC()) {
		return CreateOrderFromQuoteResult{}, fmt.Errorf("quote expired")
	}
	created, err := store.CreateOrderFromQuoteWithVendSessions(ctx, PersistOrderFromQuoteInput{
		Quote:              quote,
		IdempotencyKey:     key,
		Simulated:          in.Simulated,
		SimulationRunID:    strings.TrimSpace(in.SimulationRunID),
		SimulationScenario: strings.TrimSpace(in.SimulationScenario),
		FakeBill:           in.FakeBill,
		FakeBoard:          in.FakeBoard,
	})
	if err != nil {
		return CreateOrderFromQuoteResult{}, err
	}
	return mapPersistOrderFromQuote(created), nil
}

func (s *Service) quoteStore() QuoteStore {
	if qs, ok := s.orders.(QuoteStore); ok {
		return qs
	}
	return nil
}

func mapPersistQuoteResult(p PersistQuoteResult) CreateQuoteResult {
	lines := make([]QuoteLineView, 0, len(p.Lines))
	for _, l := range p.Lines {
		lines = append(lines, QuoteLineView{
			LineSequence:       l.LineSequence,
			ProductID:          l.ProductID,
			SlotConfigID:       l.SlotConfigID,
			CabinetCode:        l.CabinetCode,
			SlotCode:           l.SlotCode,
			SlotIndex:          l.SlotIndex,
			Quantity:           l.Quantity,
			UnitPriceMinor:     l.UnitPriceMinor,
			LineSubtotalMinor:  l.LineSubtotalMinor,
			PricingFingerprint: l.PricingFingerprint,
		})
	}
	return CreateQuoteResult{
		QuoteID:       p.QuoteID,
		MachineID:     p.MachineID,
		Currency:      p.Currency,
		PaymentMethod: p.PaymentMethod,
		SubtotalMinor: p.SubtotalMinor,
		DiscountMinor: p.DiscountMinor,
		PayableMinor:  p.PayableMinor,
		ExpiresAt:     p.ExpiresAt,
		Lines:         lines,
		Replay:        p.Replay,
	}
}

func mapPersistOrderFromQuote(p PersistOrderFromQuoteResult) CreateOrderFromQuoteResult {
	lines := make([]OrderVendLineView, 0, len(p.Lines))
	for _, l := range p.Lines {
		lines = append(lines, OrderVendLineView{
			VendSessionID: l.VendSessionID,
			LineSequence:  l.LineSequence,
			SlotIndex:     l.SlotIndex,
			ProductID:     l.ProductID,
			CabinetCode:   l.CabinetCode,
			SlotCode:      l.SlotCode,
			VendState:     l.VendState,
		})
	}
	o := p.Order
	return CreateOrderFromQuoteResult{
		OrderID:       o.ID,
		OrderStatus:   o.Status,
		Currency:      o.Currency,
		SubtotalMinor: o.SubtotalMinor,
		TaxMinor:      o.TaxMinor,
		TotalMinor:    o.TotalMinor,
		Lines:         lines,
		Replay:        p.Replay,
	}
}

func validateCreateQuote(in CreateQuoteInput) error {
	if in.MachineID == uuid.Nil {
		return errors.New("machine_id is required")
	}
	if len(in.Lines) == 0 {
		return errors.New("at least one cart line is required")
	}
	for _, l := range in.Lines {
		if l.ProductID == uuid.Nil {
			return errors.New("product_id is required on each line")
		}
	}
	return nil
}

func validateCreateOrderFromQuote(in CreateOrderFromQuoteInput) error {
	if in.MachineID == uuid.Nil {
		return errors.New("machine_id is required")
	}
	if in.QuoteID == uuid.Nil {
		return errors.New("quote_id is required")
	}
	if strings.TrimSpace(in.IdempotencyKey) == "" {
		return errors.New("idempotency_key is required")
	}
	return nil
}
