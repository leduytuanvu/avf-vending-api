package inventoryapp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ComputeAdjustmentPayloadSHA256 returns a deterministic hash of the idempotent payload
// (reason, occurred_at, client_event_id, and slot lines). Used to reject conflicting retries
// under the same idempotency key.
func ComputeAdjustmentPayloadSHA256(in AdjustmentBatchInput) string {
	occ := ""
	if in.OccurredAt != nil {
		occ = in.OccurredAt.UTC().Format(time.RFC3339Nano)
	}
	items := append([]AdjustmentItem(nil), in.Items...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].PlanogramID != items[j].PlanogramID {
			return items[i].PlanogramID.String() < items[j].PlanogramID.String()
		}
		return items[i].SlotIndex < items[j].SlotIndex
	})
	lines := make([]struct {
		P uuid.UUID `json:"planogram_id"`
		I int32     `json:"slot_index"`
		B int32     `json:"quantity_before"`
		A int32     `json:"quantity_after"`
	}, len(items))
	for i, it := range items {
		lines[i].P = it.PlanogramID
		lines[i].I = it.SlotIndex
		lines[i].B = it.QuantityBefore
		lines[i].A = it.QuantityAfter
	}
	canon := struct {
		Reason        string `json:"reason"`
		OccurredAt    string `json:"occurred_at"`
		ClientEventID string `json:"client_event_id"`
		Items         []struct {
			P uuid.UUID `json:"planogram_id"`
			I int32     `json:"slot_index"`
			B int32     `json:"quantity_before"`
			A int32     `json:"quantity_after"`
		} `json:"items"`
	}{
		Reason:        strings.ToLower(strings.TrimSpace(in.Reason)),
		OccurredAt:    occ,
		ClientEventID: strings.TrimSpace(in.ClientEventID),
		Items:         lines,
	}
	b, _ := json.Marshal(canon)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
