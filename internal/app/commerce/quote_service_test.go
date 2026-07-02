package commerce

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type stubQtySaleLines struct {
	stubSaleLineResolver
}

func (stubQtySaleLines) ResolveSaleLineWithQuantity(_ context.Context, in ResolveSaleLineInput, qty int32) (ResolvedSaleLine, error) {
	line, err := stubSaleLineResolver{}.ResolveSaleLine(context.Background(), in)
	if err != nil {
		return ResolvedSaleLine{}, err
	}
	if qty > 1 {
		line.TotalMinor = line.PriceMinor * int64(qty)
		line.SubtotalMinor = line.TotalMinor
	}
	return line, nil
}

func TestValidateCreateQuote_requiresLines(t *testing.T) {
	err := validateCreateQuote(CreateQuoteInput{MachineID: uuid.New()})
	if err == nil {
		t.Fatal("expected error for empty lines")
	}
}

func TestValidateCreateOrderFromQuote_requiresIdempotency(t *testing.T) {
	err := validateCreateOrderFromQuote(CreateOrderFromQuoteInput{
		MachineID: uuid.New(),
		QuoteID:   uuid.New(),
	})
	if err == nil {
		t.Fatal("expected idempotency error")
	}
}

func TestCreateQuote_rejectsNilStore(t *testing.T) {
	svc := NewService(Deps{
		OrderVend: stubOrderVendWorkflow{},
		SaleLines: stubQtySaleLines{},
	})
	_, err := svc.CreateQuote(context.Background(), CreateQuoteInput{
		MachineID: uuid.New(),
		Lines:     []QuoteLineInput{{ProductID: uuid.New()}},
	})
	if err == nil {
		t.Fatal("expected not configured without quote store")
	}
}
