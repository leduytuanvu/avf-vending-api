package salecatalog

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/pricingengine"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SnapshotBuilder abstracts BuildSnapshot for transport layers and tests.
type SnapshotBuilder interface {
	BuildSnapshot(ctx context.Context, machineID uuid.UUID, opts Options) (Snapshot, error)
}

// Service builds the runtime sale catalog (planogram, price, stock, media metadata) for a machine.
type Service struct {
	pool *pgxpool.Pool
	repo *postgres.SetupRepository
}

// NewService returns a sale catalog builder backed by Postgres.
func NewService(pool *pgxpool.Pool) *Service {
	if pool == nil {
		panic("salecatalog.NewService: nil pool")
	}
	return &Service{
		pool: pool,
		repo: postgres.NewSetupRepository(pool),
	}
}

// Options mirrors HTTP query flags for /v1/machines/{id}/sale-catalog.
type Options struct {
	IncludeUnavailable       bool
	IncludeImages            bool
	IfNoneMatchConfigVersion *int64
}

// Snapshot is the structured sale catalog (no JSON).
type Snapshot struct {
	MachineID      uuid.UUID
	OrganizationID uuid.UUID
	SiteID         uuid.UUID
	ConfigVersion  int64

	// CatalogVersion is the composite runtime sale-catalog digest (canonical **catalog fingerprint**).
	// It is exposed on gRPC/HTTP as `catalog_version`; pair with `generated_at` for snapshot freshness
	// and response `Meta.server_time` for RPC wall-clock. The value changes when assortment, pricebook,
	// planogram lineage, media projection, inventory quantities, machine shadow config_version,
	// currency, or snapshot flags change.
	CatalogVersion string
	Currency       string
	GeneratedAt    time.Time
	NotModified    bool
	Items          []Item
	// Bootstrap is the loaded machine bootstrap (for media manifest / fingerprints). Nil if NotModified.
	Bootstrap *setupapp.MachineBootstrap
}

// Item is one slot line on the sale catalog.
type Item struct {
	SlotIndex         int32
	SlotCode          string
	CabinetCode       string
	ProductID         uuid.UUID
	SKU               string
	Name              string
	ShortName         string
	PriceMinor        int64
	AvailableQuantity int32
	MaxQuantity       int32
	IsAvailable       bool
	UnavailableReason string
	SortOrder         int32
	Image             *ImageMeta
	// BasePriceMinor is the register price per unit before promotions (after machine_price_overrides).
	BasePriceMinor int64
	// DiscountUnitMinor is promotion discount per unit; PriceMinor is the effective unit after discount.
	DiscountUnitMinor  int64
	PricingFingerprint string
}

// ImageMeta is HTTPS URL + integrity metadata (no bytes).
type ImageMeta struct {
	MediaID            uuid.UUID
	ThumbURL           string
	DisplayURL         string
	ContentHash        string
	Etag               string
	ContentType        string
	SizeBytes          int64
	ObjectVersion      int32
	MediaVersion       int32
	Width              int32
	Height             int32
	UpdatedAt          time.Time
	OriginalURL        string
	Deleted            bool
	ThumbStorageKey    string
	DisplayStorageKey  string
	OriginalStorageKey string
	URLExpiresAt       time.Time
	Variants           []ImageVariantMeta
}

// BuildSnapshot loads bootstrap + slots and projects sale items.
func (s *Service) BuildSnapshot(ctx context.Context, machineID uuid.UUID, opts Options) (Snapshot, error) {
	var out Snapshot
	if s == nil || s.pool == nil {
		return out, setupapp.ErrNotFound
	}
	q := db.New(s.pool)
	bootstrap, err := s.repo.GetMachineBootstrap(ctx, machineID)
	if err != nil {
		return out, err
	}
	slotView, err := s.repo.GetMachineSlotView(ctx, machineID)
	if err != nil {
		return out, err
	}

	legacyByIndex := make(map[int32]struct {
		qty       int32
		maxQty    int32
		price     int64
		planogram string
	})
	for _, l := range slotView.LegacySlots {
		legacyByIndex[l.SlotIndex] = struct {
			qty       int32
			maxQty    int32
			price     int64
			planogram string
		}{l.CurrentQuantity, l.MaxQuantity, l.PriceMinor, l.PlanogramName}
	}

	sortByProduct := make(map[uuid.UUID]int32)
	for _, ap := range bootstrap.AssortmentProducts {
		sortByProduct[ap.ProductID] = ap.SortOrder
	}

	var cfgVersion int64
	ver, err := q.GetMachineShadowVersion(ctx, machineID)
	if err != nil {
		if err != pgx.ErrNoRows {
			return out, err
		}
		cfgVersion = 0
	} else {
		cfgVersion = ver
	}

	cur, err := q.InventoryAdminGetOrgDefaultCurrency(ctx, bootstrap.Machine.OrganizationID)
	if err != nil {
		return out, err
	}
	currencyUpper := strings.ToUpper(strings.TrimSpace(cur))

	if opts.IfNoneMatchConfigVersion != nil && *opts.IfNoneMatchConfigVersion == cfgVersion {
		notLoaded := Snapshot{
			ConfigVersion: cfgVersion,
			Currency:      currencyUpper,
			Items:         nil,
		}
		return Snapshot{
			MachineID:      machineID,
			OrganizationID: bootstrap.Machine.OrganizationID,
			SiteID:         bootstrap.Machine.SiteID,
			ConfigVersion:  cfgVersion,
			CatalogVersion: RuntimeSaleCatalogFingerprint(bootstrap, notLoaded, opts),
			NotModified:    true,
			GeneratedAt:    time.Now().UTC(),
		}, nil
	}

	productIDs := make([]uuid.UUID, 0)
	for _, sl := range bootstrap.CurrentCabinetSlots {
		if !sl.IsCurrent || sl.ProductID == nil {
			continue
		}
		productIDs = append(productIDs, *sl.ProductID)
	}
	prodByID := make(map[uuid.UUID]db.RuntimeGetProductsByIDsRow)
	if len(productIDs) > 0 {
		prodRows, err := q.RuntimeGetProductsByIDs(ctx, db.RuntimeGetProductsByIDsParams{
			OrganizationID: bootstrap.Machine.OrganizationID,
			Column2:        productIDs,
		})
		if err != nil {
			return out, err
		}
		for _, p := range prodRows {
			prodByID[p.ID] = p
		}
	}

	imgByProduct := make(map[uuid.UUID][]db.RuntimeListProductImagesForProductsRow)
	if opts.IncludeImages && len(productIDs) > 0 {
		imgs, ierr := q.RuntimeListProductImagesForProducts(ctx, productIDs)
		if ierr != nil {
			return out, ierr
		}
		for _, im := range imgs {
			imgByProduct[im.ProductID] = append(imgByProduct[im.ProductID], im)
		}
	}

	items := make([]Item, 0)
	genAt := time.Now().UTC()
	priceBatch, batchErr := pricingengine.New(s.pool).NewBatch(ctx, bootstrap.Machine.OrganizationID, machineID, genAt)
	if batchErr != nil {
		return out, batchErr
	}
	for _, sl := range bootstrap.CurrentCabinetSlots {
		if !sl.IsCurrent || sl.ProductID == nil {
			continue
		}
		pid := *sl.ProductID
		pmeta, ok := prodByID[pid]
		if !ok {
			continue
		}
		var slotIdx int32 = -1
		if sl.SlotIndex != nil {
			slotIdx = *sl.SlotIndex
		}
		leg, hasLeg := legacyByIndex[slotIdx]
		qty := int32(0)
		maxQ := sl.MaxQuantity
		price := sl.PriceMinor
		if hasLeg {
			qty = leg.qty
			if maxQ <= 0 {
				maxQ = leg.maxQty
			}
			if price == 0 {
				price = leg.price
			}
		}
		priceOK := price > 0
		stockOK := qty > 0
		activeOK := pmeta.Active
		reasons := make([]string, 0)
		if !activeOK {
			reasons = append(reasons, "product_inactive")
		}
		if !priceOK {
			reasons = append(reasons, "no_price")
		}
		if !stockOK {
			reasons = append(reasons, "out_of_stock")
		}
		available := activeOK && priceOK && stockOK
		if !available && !opts.IncludeUnavailable {
			continue
		}
		unavail := ""
		if !available {
			unavail = strings.Join(reasons, ",")
		}
		shortName := shortNameFromAttrs(pmeta.Name, pmeta.Attrs)
		var effPrice, baseReg, discUnit int64
		var pfp string
		if price > 0 {
			pl, perr := priceBatch.PriceLine(ctx, pricingengine.PriceLineInput{
				OrganizationID:    bootstrap.Machine.OrganizationID,
				MachineID:         machineID,
				ProductID:         pid,
				SlotListUnitMinor: price,
				SlotConfigID:      sl.ConfigID,
				CabinetCode:       sl.CabinetCode,
				SlotCode:          sl.SlotCode,
				SlotIndex:         slotIdx,
				Quantity:          1,
			})
			if perr != nil {
				return out, perr
			}
			effPrice = pl.EffectiveUnitMinor
			baseReg = pl.RegisterUnitMinor
			discUnit = pl.DiscountUnitMinor
			pfp = pl.PricingFingerprint
		}
		it := Item{
			SlotIndex:          slotIdx,
			SlotCode:           sl.SlotCode,
			CabinetCode:        sl.CabinetCode,
			ProductID:          pid,
			SKU:                pmeta.Sku,
			Name:               pmeta.Name,
			ShortName:          shortName,
			PriceMinor:         effPrice,
			BasePriceMinor:     baseReg,
			DiscountUnitMinor:  discUnit,
			PricingFingerprint: pfp,
			AvailableQuantity:  qty,
			MaxQuantity:        maxQ,
			IsAvailable:        available,
			UnavailableReason:  unavail,
			SortOrder:          sortByProduct[pid],
		}
		if opts.IncludeImages {
			if imgs := imgByProduct[pid]; len(imgs) > 0 {
				im := pickDisplayImage(imgs)
				thumb := productImageThumbURL(im)
				disp := productImageDisplayURL(im)
				if disp == "" {
					disp = thumb
				}
				ch := productImageContentHash(im)
				etag := productImageEtag(im, ch)
				var mid uuid.UUID
				if im.MediaAssetID.Valid {
					mid = uuid.UUID(im.MediaAssetID.Bytes)
				}
				var sz int64
				if im.AssetSizeBytes.Valid {
					sz = im.AssetSizeBytes.Int64
				}
				var ov int32
				if im.AssetObjectVersion.Valid {
					ov = im.AssetObjectVersion.Int32
				}
				var width int32
				if im.Width.Valid {
					width = im.Width.Int32
				}
				var height int32
				if im.Height.Valid {
					height = im.Height.Int32
				}
				var ct string
				if im.MimeType.Valid {
					ct = strings.TrimSpace(im.MimeType.String)
				}
				tk := strings.TrimSpace(im.ThumbObjectKey)
				dk := strings.TrimSpace(im.DisplayObjectKey)
				ok := strings.TrimSpace(im.OriginalObjectKey)
				origCDN := strings.TrimSpace(im.OriginalCdnUrl)
				it.Image = &ImageMeta{
					MediaID:            mid,
					ThumbURL:           thumb,
					DisplayURL:         disp,
					OriginalURL:        origCDN,
					ContentHash:        ch,
					Etag:               etag,
					ContentType:        ct,
					SizeBytes:          sz,
					ObjectVersion:      ov,
					MediaVersion:       im.MediaVersion,
					Width:              width,
					Height:             height,
					UpdatedAt:          im.UpdatedAt.UTC(),
					ThumbStorageKey:    tk,
					DisplayStorageKey:  dk,
					OriginalStorageKey: ok,
					Variants: buildMediaVariants(
						im, mid, thumb, disp, ct, width, height, sz, im.MediaVersion, im.UpdatedAt.UTC(), ch,
					),
				}
			} else {
				it.Image = &ImageMeta{
					Deleted:   true,
					UpdatedAt: time.Now().UTC(),
				}
			}
		}
		items = append(items, it)
	}

	bcopy := bootstrap
	partial := Snapshot{
		ConfigVersion: cfgVersion,
		Currency:      currencyUpper,
		Items:         items,
	}
	return Snapshot{
		MachineID:      machineID,
		OrganizationID: bootstrap.Machine.OrganizationID,
		SiteID:         bootstrap.Machine.SiteID,
		ConfigVersion:  cfgVersion,
		CatalogVersion: RuntimeSaleCatalogFingerprint(bootstrap, partial, opts),
		Currency:       currencyUpper,
		GeneratedAt:    genAt,
		Items:          items,
		Bootstrap:      &bcopy,
	}, nil
}

func mediaEtag(contentHash string, updatedAt time.Time) string {
	if strings.TrimSpace(contentHash) != "" {
		return `W/"` + strings.TrimPrefix(strings.TrimSpace(contentHash), "sha256:") + `"`
	}
	return `W/"` + updatedAt.Format(time.RFC3339Nano) + `"`
}

func shortNameFromAttrs(full string, attrs []byte) string {
	var m map[string]any
	if len(attrs) > 0 {
		_ = json.Unmarshal(attrs, &m)
	}
	if m != nil {
		if s, ok := m["short_name"].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
		if s, ok := m["shortName"].(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	if len(full) > 24 {
		return full[:24]
	}
	return full
}

var _ SnapshotBuilder = (*Service)(nil)
