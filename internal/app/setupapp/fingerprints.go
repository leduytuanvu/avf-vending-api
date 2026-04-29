package setupapp

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// CatalogFingerprint is a stable hash of assortment membership/order (matches CheckForUpdates / bootstrap).
func CatalogFingerprint(b MachineBootstrap) string {
	ids := make([]string, 0, len(b.AssortmentProducts))
	for _, p := range b.AssortmentProducts {
		ids = append(ids, p.ProductID.String()+":"+strconv.Itoa(int(p.SortOrder)))
	}
	sort.Strings(ids)
	return hashStringsFingerprint("catalog", ids)
}

// PricingFingerprint hashes current slot prices and effective times.
func PricingFingerprint(b MachineBootstrap) string {
	parts := make([]string, 0, len(b.CurrentCabinetSlots))
	for _, s := range b.CurrentCabinetSlots {
		parts = append(parts, s.ConfigID.String()+":"+strconv.FormatInt(s.PriceMinor, 10)+":"+s.EffectiveFrom.UTC().Format("2006-01-02T15:04:05Z"))
	}
	sort.Strings(parts)
	return hashStringsFingerprint("pricing", parts)
}

// PlanogramFingerprint hashes cabinet/slot layout identifiers.
func PlanogramFingerprint(b MachineBootstrap) string {
	var pvPart string
	if b.PublishedPlanogramVersionID != nil && *b.PublishedPlanogramVersionID != uuid.Nil {
		pvPart = "pv:" + b.PublishedPlanogramVersionID.String() + ":" + strconv.Itoa(int(b.PublishedPlanogramVersionNo))
	}
	parts := make([]string, 0, len(b.CurrentCabinetSlots)+1)
	if pvPart != "" {
		parts = append(parts, pvPart)
	}
	for _, s := range b.CurrentCabinetSlots {
		idx := int32(-1)
		if s.SlotIndex != nil {
			idx = *s.SlotIndex
		}
		pid := ""
		if s.ProductID != nil {
			pid = s.ProductID.String()
		}
		parts = append(parts, s.CabinetCode+":"+s.SlotCode+":"+strconv.Itoa(int(idx))+":"+pid+":"+strconv.FormatInt(int64(s.MaxQuantity), 10))
	}
	sort.Strings(parts)
	return hashStringsFingerprint("planogram", parts)
}

// MediaFingerprint hashes product ids and SKUs (image binding changes).
func MediaFingerprint(b MachineBootstrap) string {
	ids := make([]string, 0, len(b.AssortmentProducts))
	for _, p := range b.AssortmentProducts {
		ids = append(ids, p.ProductID.String()+":"+strings.TrimSpace(p.SKU))
	}
	sort.Strings(ids)
	return hashStringsFingerprint("media", ids)
}

func hashStringsFingerprint(prefix string, parts []string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(prefix))
	_, _ = h.Write([]byte{0})
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// SortedKeyFingerprint returns a deterministic SHA-256 hex digest over sorted string parts.
// The prefix namespaces the fingerprint from other hashes in the system.
func SortedKeyFingerprint(prefix string, parts []string) string {
	if len(parts) == 0 {
		return hashStringsFingerprint(prefix, nil)
	}
	cp := append([]string(nil), parts...)
	sort.Strings(cp)
	return hashStringsFingerprint(prefix, cp)
}
