package sellreadiness

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/google/uuid"
)

// Snapshot is backend-derived operator/kiosk hints (device cache state is not persisted server-side).
type Snapshot struct {
	CatalogSynced   bool     `json:"catalogSynced"`
	MediaSynced     bool     `json:"mediaSynced"`
	InventorySynced bool     `json:"inventorySynced"`
	ReadyForSale    bool     `json:"readyForSale"`
	Issues          []string `json:"readinessIssues"`
}

// PrimaryMediaReadyMap returns ready flags keyed by product id (missing ids default to false).
func PrimaryMediaReadyMap(ctx context.Context, q *db.Queries, productIDs []uuid.UUID) (map[uuid.UUID]bool, error) {
	out := make(map[uuid.UUID]bool, len(productIDs))
	if len(productIDs) == 0 || q == nil {
		return out, nil
	}
	rows, err := q.RuntimeProductPrimaryMediaReady(ctx, productIDs)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.ProductID] = r.Ready
	}
	return out, nil
}

// SellableSlotProductIDs returns products assigned to slots that are intended to vend (Mirrors sale-catalog “priceOK && stock path” gate for assignment policy).
func SellableSlotProductIDs(save setupapp.SlotConfigSaveInput) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{})
	for _, it := range save.Items {
		if it.ProductID == nil || *it.ProductID == uuid.Nil {
			continue
		}
		if it.MaxQuantity <= 0 || it.PriceMinor <= 0 {
			continue
		}
		if _, ok := seen[*it.ProductID]; ok {
			continue
		}
		seen[*it.ProductID] = struct{}{}
	}
	out := make([]uuid.UUID, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].String() < out[j].String() })
	return out
}

// ValidateSellableSlotProductsPrimaryMedia enforces ready primary media for sellable slot assignments (publish / planogram validate).
func ValidateSellableSlotProductsPrimaryMedia(ctx context.Context, q *db.Queries, save setupapp.SlotConfigSaveInput) error {
	pids := SellableSlotProductIDs(save)
	if len(pids) == 0 {
		return nil
	}
	m, err := PrimaryMediaReadyMap(ctx, q, pids)
	if err != nil {
		return err
	}
	for _, p := range pids {
		if !m[p] {
			return fmt.Errorf("product %s requires ready primary media before it can occupy a sellable slot", p.String())
		}
	}
	return nil
}

// Compute derives readiness from bootstrap + snapshot ack rows + optional legacy inventory projection.
func Compute(ctx context.Context, q *db.Queries, slotView setupapp.MachineSlotView, b setupapp.MachineBootstrap) (Snapshot, error) {
	var out Snapshot
	if q == nil {
		out.Issues = append(out.Issues, "readiness_queries_unavailable")
		return out, nil
	}
	mid := b.Machine.ID
	ackRow, err := q.RuntimeGetMachineSellReadinessAcks(ctx, mid)
	if err != nil {
		return out, err
	}

	pub := ackRow.PublishedPlanogramVersionID
	gotAck := ackRow.LastAcknowledgedPlanogramVersionID
	if pub.Valid && gotAck.Valid && uuid.UUID(pub.Bytes) == uuid.UUID(gotAck.Bytes) {
		out.CatalogSynced = true
	} else {
		out.Issues = append(out.Issues, "catalog_planogram_ack_pending")
	}

	if ackRow.LatestConfigRevision > 0 {
		if !ackRow.LastAcknowledgedConfigRevision.Valid {
			out.CatalogSynced = false
			out.Issues = append(out.Issues, "catalog_config_ack_missing")
		} else if ackRow.LastAcknowledgedConfigRevision.Int32 < ackRow.LatestConfigRevision {
			out.CatalogSynced = false
			out.Issues = append(out.Issues, "catalog_config_revision_pending")
		}
	}

	legacyByIndex := make(map[int32]setupapp.LegacySlotRow)
	for _, l := range slotView.LegacySlots {
		legacyByIndex[l.SlotIndex] = l
	}

	var sellablePids []uuid.UUID
	pidSeen := make(map[uuid.UUID]struct{})
	out.InventorySynced = true
	for _, sl := range b.CurrentCabinetSlots {
		if !sl.IsCurrent || sl.ProductID == nil {
			continue
		}
		if sl.MaxQuantity <= 0 || sl.PriceMinor <= 0 {
			continue
		}
		if _, ok := pidSeen[*sl.ProductID]; !ok {
			pidSeen[*sl.ProductID] = struct{}{}
			sellablePids = append(sellablePids, *sl.ProductID)
		}
		if sl.SlotIndex == nil {
			continue
		}
		if _, ok := legacyByIndex[*sl.SlotIndex]; !ok {
			out.InventorySynced = false
			out.Issues = append(out.Issues, fmt.Sprintf("inventory_projection_missing:slot_index_%d", *sl.SlotIndex))
		}
	}

	readyMap, err := PrimaryMediaReadyMap(ctx, q, sellablePids)
	if err != nil {
		return out, err
	}
	out.MediaSynced = true
	for _, pid := range sellablePids {
		if !readyMap[pid] {
			out.MediaSynced = false
			out.Issues = append(out.Issues, fmt.Sprintf("primary_media_not_ready:%s", pid.String()))
		}
	}

	st := strings.TrimSpace(b.Machine.Status)
	statusOK := strings.EqualFold(st, "active")

	out.ReadyForSale = out.CatalogSynced && out.MediaSynced && out.InventorySynced && statusOK && pub.Valid
	if !statusOK {
		out.Issues = append(out.Issues, "machine_not_active")
	}
	if !pub.Valid {
		out.Issues = append(out.Issues, "no_published_planogram")
	}

	return out, nil
}
