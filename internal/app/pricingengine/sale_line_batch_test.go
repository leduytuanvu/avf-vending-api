package pricingengine

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestEvaluateSaleLines_rejectsEmpty(t *testing.T) {
	eng := &Engine{}
	_, err := eng.EvaluateSaleLines(context.Background(), nil, time.Now().UTC())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestEvaluateSaleLines_rejectsMixedMachines(t *testing.T) {
	eng := &Engine{pool: nil}
	m1 := uuid.New()
	m2 := uuid.New()
	_, err := eng.EvaluateSaleLines(context.Background(), []EvaluateSaleLineRequest{
		{Selector: SaleLineSelector{MachineID: m1, ProductID: uuid.New()}},
		{Selector: SaleLineSelector{MachineID: m2, ProductID: uuid.New()}},
	}, time.Now().UTC())
	if err == nil {
		t.Fatal("expected error")
	}
}
