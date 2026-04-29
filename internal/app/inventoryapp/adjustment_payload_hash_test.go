package inventoryapp

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestComputeAdjustmentPayloadSHA256_stableOrder(t *testing.T) {
	t.Parallel()
	pg := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	a := AdjustmentBatchInput{
		Reason: "restock",
		Items: []AdjustmentItem{
			{PlanogramID: pg, SlotIndex: 2, QuantityBefore: 1, QuantityAfter: 3},
			{PlanogramID: pg, SlotIndex: 1, QuantityBefore: 0, QuantityAfter: 1},
		},
	}
	b := AdjustmentBatchInput{
		Reason: "restock",
		Items: []AdjustmentItem{
			{PlanogramID: pg, SlotIndex: 1, QuantityBefore: 0, QuantityAfter: 1},
			{PlanogramID: pg, SlotIndex: 2, QuantityBefore: 1, QuantityAfter: 3},
		},
	}
	if ComputeAdjustmentPayloadSHA256(a) != ComputeAdjustmentPayloadSHA256(b) {
		t.Fatal("order should not affect hash")
	}
}

func TestComputeAdjustmentPayloadSHA256_occurredAtMatters(t *testing.T) {
	t.Parallel()
	pg := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	a := AdjustmentBatchInput{Reason: "restock", Items: []AdjustmentItem{{PlanogramID: pg, SlotIndex: 0, QuantityBefore: 0, QuantityAfter: 1}}}
	b := a
	b.OccurredAt = &ts
	if ComputeAdjustmentPayloadSHA256(a) == ComputeAdjustmentPayloadSHA256(b) {
		t.Fatal("occurred_at should affect hash")
	}
}
