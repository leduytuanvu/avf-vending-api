package salecatalog

import (
	"sort"
	"strconv"
	"strings"

	"github.com/avf/avf-vending-api/internal/app/setupapp"
)

// PromotionsSnapshotFingerprint fingerprints applied promotion material per catalog line (effective register + promos).
func PromotionsSnapshotFingerprint(s Snapshot) string {
	parts := make([]string, 0, len(s.Items))
	for _, it := range s.Items {
		fp := strings.TrimSpace(it.PricingFingerprint)
		if fp == "" {
			fp = "na"
		}
		parts = append(parts,
			it.ProductID.String()+":"+strconv.FormatInt(it.BasePriceMinor, 10)+":"+
				strconv.FormatInt(it.DiscountUnitMinor, 10)+":"+strconv.FormatInt(it.PriceMinor, 10)+":"+fp)
	}
	return setupapp.SortedKeyFingerprint("runtime_promo_v1", parts)
}

// MediaFingerprint fingerprints the **sale catalog media projection** for a machine (bindings, hashes,
// MIME, etag, and per-variant **storage keys** — not presigned URLs). Embed `media_version` +
// per-variant `checksum_sha256` on the client cache key alongside this string.
// It must be computed from the same Snapshot Options (include_unavailable / slot filter) as GetMediaManifest.
func MediaFingerprint(s Snapshot) string {
	parts := make([]string, 0, len(s.Items))
	for _, it := range s.Items {
		var img string
		switch {
		case it.Image == nil:
			img = "none"
		case it.Image.Deleted:
			img = "deleted"
		default:
			if len(it.Image.Variants) > 0 {
				chunks := make([]string, 0, len(it.Image.Variants))
				for _, v := range it.Image.Variants {
					chunks = append(chunks,
						strconv.Itoa(int(v.Kind))+":"+
							strings.TrimSpace(v.StorageKey)+":"+
							strings.TrimSpace(v.ChecksumSHA256)+":"+
							strings.TrimSpace(v.Etag))
				}
				sort.Strings(chunks)
				img = it.Image.MediaID.String() + ":" +
					strconv.Itoa(int(it.Image.MediaVersion)) + ":" +
					strings.Join(chunks, ";")
			} else {
				img = it.Image.MediaID.String() + ":" +
					strconv.Itoa(int(it.Image.MediaVersion)) + ":" +
					strings.TrimSpace(it.Image.ContentHash) + ":" +
					strconv.FormatInt(it.Image.SizeBytes, 10) + ":" +
					strings.TrimSpace(it.Image.ContentType) + ":" +
					strings.TrimSpace(it.Image.Etag)
			}
		}
		parts = append(parts, it.ProductID.String()+":"+strings.TrimSpace(it.SKU)+":"+img)
	}
	return setupapp.SortedKeyFingerprint("media_catalog_v3", parts)
}

// InventorySnapshotFingerprint hashes slot lines (price, stock envelope, availability reasons)
// for sync; it must change when quantities or sellability of a line change.
func InventorySnapshotFingerprint(s Snapshot) string {
	parts := make([]string, 0, len(s.Items))
	for _, it := range s.Items {
		parts = append(parts,
			it.CabinetCode+":"+it.SlotCode+":"+strconv.FormatInt(int64(it.SlotIndex), 10)+":"+
				it.ProductID.String()+":"+
				strconv.FormatInt(int64(it.AvailableQuantity), 10)+":"+
				strconv.FormatInt(int64(it.MaxQuantity), 10)+":"+
				strconv.FormatBool(it.IsAvailable)+":"+
				strconv.FormatInt(it.PriceMinor, 10)+":"+
				strconv.FormatInt(it.BasePriceMinor, 10)+":"+
				strconv.FormatInt(it.DiscountUnitMinor, 10)+":"+
				strings.TrimSpace(it.PricingFingerprint)+":"+
				it.UnavailableReason,
		)
	}
	return setupapp.SortedKeyFingerprint("runtime_inv_v1", parts)
}

// RuntimeSaleCatalogFingerprint is the canonical value for Snapshot.CatalogVersion (gRPC/HTTP
// `catalog_version`): a stable digest of assortment + pricebook + planogram lineage + promotion
// placeholder + media projection + inventory lines + machine shadow config + currency +
// include_unavailable/include_images flags. Treat it as **catalog_fingerprint** in client docs.
func RuntimeSaleCatalogFingerprint(bootstrap setupapp.MachineBootstrap, snap Snapshot, opts Options) string {
	parts := []string{
		"asm:" + setupapp.CatalogFingerprint(bootstrap),
		"prc:" + setupapp.PricingFingerprint(bootstrap),
		"plg:" + setupapp.PlanogramFingerprint(bootstrap),
		"prm:" + PromotionsSnapshotFingerprint(snap),
		"med:" + MediaFingerprint(snap),
		"inv:" + InventorySnapshotFingerprint(snap),
		"cfg:" + strconv.FormatInt(snap.ConfigVersion, 10),
		"cur:" + strings.ToUpper(strings.TrimSpace(snap.Currency)),
		"uav:" + strconv.FormatBool(opts.IncludeUnavailable),
		"img:" + strconv.FormatBool(opts.IncludeImages),
	}
	return setupapp.SortedKeyFingerprint("runtime_sale_catalog_v6", parts)
}
