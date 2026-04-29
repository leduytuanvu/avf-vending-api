package planogram

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/google/uuid"
)

type snapshotBody struct {
	PlanogramID         string             `json:"planogramId"`
	PlanogramRevision   int32              `json:"planogramRevision"`
	SyncLegacyReadModel bool               `json:"syncLegacyReadModel"`
	Items               []snapshotSlotItem `json:"items"`
}

type snapshotSlotItem struct {
	CabinetCode     string          `json:"cabinetCode"`
	LayoutKey       string          `json:"layoutKey"`
	LayoutRevision  int32           `json:"layoutRevision"`
	SlotCode        string          `json:"slotCode"`
	LegacySlotIndex *int32          `json:"legacySlotIndex,omitempty"`
	ProductID       *string         `json:"productId,omitempty"`
	MaxQuantity     int32           `json:"maxQuantity"`
	PriceMinor      int64           `json:"priceMinor"`
	EffectiveFrom   *time.Time      `json:"effectiveFrom,omitempty"`
	Metadata        json.RawMessage `json:"metadata,omitempty"`
}

func snapshotBytesToSaveInput(snapshot []byte, publish bool) (setupapp.SlotConfigSaveInput, error) {
	var body snapshotBody
	if err := json.Unmarshal(snapshot, &body); err != nil {
		return setupapp.SlotConfigSaveInput{}, fmt.Errorf("%w: %w", ErrInvalidSnapshot, err)
	}
	pgID, err := uuid.Parse(strings.TrimSpace(body.PlanogramID))
	if err != nil || pgID == uuid.Nil {
		return setupapp.SlotConfigSaveInput{}, fmt.Errorf("%w: planogramId must be a UUID", ErrInvalidSnapshot)
	}
	items := make([]setupapp.SlotConfigSaveItem, 0, len(body.Items))
	for _, it := range body.Items {
		meta := []byte(it.Metadata)
		if len(meta) == 0 {
			meta = []byte("{}")
		}
		if !json.Valid(meta) {
			return setupapp.SlotConfigSaveInput{}, fmt.Errorf("%w: item metadata must be JSON", ErrInvalidSnapshot)
		}
		var pid *uuid.UUID
		if it.ProductID != nil && strings.TrimSpace(*it.ProductID) != "" {
			u, perr := uuid.Parse(strings.TrimSpace(*it.ProductID))
			if perr != nil {
				return setupapp.SlotConfigSaveInput{}, fmt.Errorf("%w: productId must be a UUID when set", ErrInvalidSnapshot)
			}
			pid = &u
		}
		eff := time.Time{}
		if it.EffectiveFrom != nil {
			eff = it.EffectiveFrom.UTC()
		}
		items = append(items, setupapp.SlotConfigSaveItem{
			CabinetCode:     strings.TrimSpace(it.CabinetCode),
			LayoutKey:       strings.TrimSpace(it.LayoutKey),
			LayoutRevision:  it.LayoutRevision,
			SlotCode:        strings.TrimSpace(it.SlotCode),
			LegacySlotIndex: it.LegacySlotIndex,
			ProductID:       pid,
			MaxQuantity:     it.MaxQuantity,
			PriceMinor:      it.PriceMinor,
			EffectiveFrom:   eff,
			Metadata:        meta,
		})
	}
	return setupapp.SlotConfigSaveInput{
		PlanogramID:         pgID,
		PlanogramRevision:   body.PlanogramRevision,
		PublishAsCurrent:    publish,
		SyncLegacyReadModel: body.SyncLegacyReadModel,
		Items:               items,
	}, nil
}
