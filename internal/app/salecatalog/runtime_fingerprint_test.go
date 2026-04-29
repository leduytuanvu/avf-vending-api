package salecatalog

import (
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/setupapp"
	domainfleet "github.com/avf/avf-vending-api/internal/domain/fleet"
	"github.com/google/uuid"
)

func baseBootstrap(pid uuid.UUID, priceMinor int64) setupapp.MachineBootstrap {
	cid := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	return setupapp.MachineBootstrap{
		Machine: domainfleet.Machine{
			ID:             uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
			OrganizationID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
			SiteID:         uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		},
		AssortmentProducts: []setupapp.AssortmentProductView{
			{ProductID: pid, SortOrder: 1, SKU: "SKU"},
		},
		CurrentCabinetSlots: []setupapp.CabinetSlotConfigView{
			{
				ConfigID:      cid,
				CabinetCode:   "A",
				SlotCode:      "1",
				SlotIndex:     ptrI32(0),
				ProductID:     &pid,
				PriceMinor:    priceMinor,
				EffectiveFrom: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
				IsCurrent:     true,
				MaxQuantity:   10,
			},
		},
	}
}

func ptrI32(v int32) *int32 { return &v }

func TestRuntimeSaleCatalogFingerprint_stableWhenRepeatable(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	b := baseBootstrap(pid, 9900)
	mid := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	snap := Snapshot{
		ConfigVersion: 7,
		Currency:      "THB",
		Items: []Item{
			{
				SlotIndex: 0, SlotCode: "1", CabinetCode: "A",
				ProductID: pid, SKU: "SKU",
				PriceMinor: 9900, AvailableQuantity: 3, MaxQuantity: 10,
				IsAvailable: true,
				Image: &ImageMeta{
					MediaID: mid, MediaVersion: 1, ContentHash: "sha256:x",
					ContentType: "image/webp", SizeBytes: 100, Etag: `W/"a"`,
					UpdatedAt: time.Unix(1, 0).UTC(),
				},
			},
		},
	}
	opts := Options{IncludeUnavailable: false, IncludeImages: true}
	a := RuntimeSaleCatalogFingerprint(b, snap, opts)
	b2 := RuntimeSaleCatalogFingerprint(b, snap, opts)
	if a != b2 || a == "" {
		t.Fatalf("expected stable nonempty fingerprint got %q %q", a, b2)
	}
}

func TestRuntimeSaleCatalogFingerprint_changesOnBootstrapPriceNotItem(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	b1 := baseBootstrap(pid, 100)
	b2 := baseBootstrap(pid, 200)
	mid := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	snap := Snapshot{ConfigVersion: 1, Currency: "USD", Items: []Item{{
		SlotIndex: 0, SlotCode: "1", CabinetCode: "A",
		ProductID: pid, SKU: "SKU", PriceMinor: 100,
		AvailableQuantity: 1, MaxQuantity: 10, IsAvailable: true,
		Image: &ImageMeta{
			MediaID: mid, MediaVersion: 1, ContentHash: "sha256:x",
			Etag: `W/"a"`, UpdatedAt: time.Unix(1, 0).UTC(),
		},
	}}}
	opts := Options{IncludeImages: true}
	fp1 := RuntimeSaleCatalogFingerprint(b1, snap, opts)
	fp2 := RuntimeSaleCatalogFingerprint(b2, snap, opts)
	if fp1 == fp2 {
		t.Fatal("expected fingerprint change when bootstrap pricing differs")
	}
}

func TestRuntimeSaleCatalogFingerprint_changesOnInventoryQuantity(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	b := baseBootstrap(pid, 500)
	baseSnap := Snapshot{
		ConfigVersion: 3,
		Currency:      "THB",
		Items: []Item{{
			SlotIndex: 0, SlotCode: "1", CabinetCode: "A",
			ProductID: pid, SKU: "SKU",
			PriceMinor: 500, AvailableQuantity: 2, MaxQuantity: 10,
			IsAvailable: true,
		}},
	}
	opts := Options{IncludeImages: false}
	fp1 := RuntimeSaleCatalogFingerprint(b, baseSnap, opts)
	changedQty := baseSnap
	changedQty.Items[0].AvailableQuantity = 9
	fp2 := RuntimeSaleCatalogFingerprint(b, changedQty, opts)
	if fp1 == fp2 {
		t.Fatal("expected fingerprint change when inventory quantity changes")
	}
}

func TestRuntimeSaleCatalogFingerprint_changesOnProjectionFlags(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	b := baseBootstrap(pid, 400)
	snap := Snapshot{
		ConfigVersion: 1, Currency: "USD",
		Items: []Item{{
			SlotIndex: 0, CabinetCode: "A", SlotCode: "1",
			ProductID: pid, SKU: "SKU",
			UnavailableReason: "product_inactive", IsAvailable: false,
			AvailableQuantity: 0, PriceMinor: 400,
		}},
	}
	fpPrivate := RuntimeSaleCatalogFingerprint(b, snap, Options{IncludeUnavailable: false})
	fpAll := RuntimeSaleCatalogFingerprint(b, snap, Options{IncludeUnavailable: true})
	if fpPrivate == fpAll {
		t.Fatal("include_unavailable bit is part of fingerprint; values must differ")
	}
}

func TestRuntimeSaleCatalogFingerprint_emptyVsInactiveLine(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	b := baseBootstrap(pid, 400)
	opts := Options{IncludeUnavailable: true, IncludeImages: false}
	noItems := Snapshot{ConfigVersion: 1, Currency: "THB", Items: nil}
	withInactive := Snapshot{ConfigVersion: 1, Currency: "THB", Items: []Item{{
		ProductID: pid, SKU: "SKU", CabinetCode: "A", SlotCode: "1",
		IsAvailable: false, UnavailableReason: "product_inactive",
	}}}
	if RuntimeSaleCatalogFingerprint(b, noItems, opts) == RuntimeSaleCatalogFingerprint(b, withInactive, opts) {
		t.Fatal("inactive line vs empty projection should diverge")
	}
}

func TestInventorySnapshotFingerprint_orderIndependent(t *testing.T) {
	t.Parallel()
	pid1 := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	pid2 := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	a := Snapshot{Items: []Item{
		{ProductID: pid1, SlotCode: "a", CabinetCode: "A", AvailableQuantity: 1},
		{ProductID: pid2, SlotCode: "b", CabinetCode: "A", AvailableQuantity: 2},
	}}
	b := Snapshot{Items: []Item{
		{ProductID: pid2, SlotCode: "b", CabinetCode: "A", AvailableQuantity: 2},
		{ProductID: pid1, SlotCode: "a", CabinetCode: "A", AvailableQuantity: 1},
	}}
	if InventorySnapshotFingerprint(a) != InventorySnapshotFingerprint(b) {
		t.Fatal("slot lines are sorted internally; order should not matter")
	}
}

func TestRuntimeSaleCatalogFingerprint_changesWhenMediaVariantKeyChanges(t *testing.T) {
	t.Parallel()
	pid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	b := baseBootstrap(pid, 500)
	mid := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	opts := Options{IncludeImages: true, IncludeUnavailable: false}
	s1 := Snapshot{
		ConfigVersion: 1, Currency: "THB",
		Items: []Item{{
			SlotIndex: 0, SlotCode: "1", CabinetCode: "A",
			ProductID: pid, SKU: "SKU",
			PriceMinor: 500, AvailableQuantity: 2, MaxQuantity: 10, IsAvailable: true,
			Image: &ImageMeta{
				MediaID: mid, MediaVersion: 1,
				Variants: []ImageVariantMeta{
					{Kind: MediaVariantKindDisplay, MediaAssetID: mid, StorageKey: "a/obj", ChecksumSHA256: "sha256:u", Etag: `W/"e"`},
				},
			},
		}},
	}
	s2 := Snapshot{
		ConfigVersion: 1, Currency: "THB",
		Items: []Item{{
			SlotIndex: 0, SlotCode: "1", CabinetCode: "A",
			ProductID: pid, SKU: "SKU",
			PriceMinor: 500, AvailableQuantity: 2, MaxQuantity: 10, IsAvailable: true,
			Image: &ImageMeta{
				MediaID: mid, MediaVersion: 1,
				Variants: []ImageVariantMeta{
					{Kind: MediaVariantKindDisplay, MediaAssetID: mid, StorageKey: "b/obj", ChecksumSHA256: "sha256:u", Etag: `W/"e"`},
				},
			},
		}},
	}
	if RuntimeSaleCatalogFingerprint(b, s1, opts) == RuntimeSaleCatalogFingerprint(b, s2, opts) {
		t.Fatal("expected catalog_version to change when runtime media variant key changes")
	}
}
