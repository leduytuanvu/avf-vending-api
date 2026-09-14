package productmastercatalog

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/pgxutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultPageSize = 500

// Record is one active product master row for machine distribution.
type Record struct {
	ProductID      uuid.UUID
	SKU            string
	Barcode        string
	Name           string
	ShortName      string
	Description    string
	BasePriceMinor int64
	Currency       string
	Active         bool
	CategoryID     *uuid.UUID
	BrandID        *uuid.UUID
	UpdatedAt      time.Time
	Image          *ImageMeta
}

// ImageMeta is HTTPS URL + integrity metadata (no bytes).
type ImageMeta struct {
	MediaID      uuid.UUID
	ThumbURL     string
	DisplayURL   string
	ContentHash  string
	Etag         string
	MediaVersion int32
	CacheKey     string
}

// Snapshot is the full active product master catalog.
type Snapshot struct {
	CatalogVersion   string
	GeneratedAt      time.Time
	TotalActiveCount int32
	Products         []Record
}

// Delta is a versioned product master change set.
type Delta struct {
	BasisMatches                    bool
	BasisCatalogVersion             string
	ToCatalogVersion                string
	TotalActiveCount                int32
	Upserts                         []Record
	DeletedOrDeactivatedProductIDs  []uuid.UUID
	GeneratedAt                     time.Time
	ResetRequired                   bool
}

// Service builds machine-facing global product master catalogs.
type Service struct {
	pool *pgxpool.Pool
}

// NewService returns a product master catalog builder.
func NewService(pool *pgxpool.Pool) *Service {
	if pool == nil {
		panic("productmastercatalog.NewService: nil pool")
	}
	return &Service{pool: pool}
}

// BuildSnapshot returns the authoritative active product master set.
func (s *Service) BuildSnapshot(ctx context.Context, pageSize int32, pageToken string) (Snapshot, string, error) {
	if s == nil || s.pool == nil {
		return Snapshot{}, "", fmt.Errorf("productmastercatalog: nil service")
	}
	q := pgxutil.NewQueries(s.pool)
	total, err := q.ProductMasterCountActiveProducts(ctx)
	if err != nil {
		return Snapshot{}, "", err
	}
	offset, err := parsePageToken(pageToken)
	if err != nil {
		return Snapshot{}, "", err
	}
	limit := int32(defaultPageSize)
	if pageSize > 0 {
		limit = pageSize
	}
	rows, err := q.ProductMasterListActiveProductsPage(ctx, db.ProductMasterListActiveProductsPageParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return Snapshot{}, "", err
	}
	records, _, err := s.mapRows(ctx, q, rows)
	if err != nil {
		return Snapshot{}, "", err
	}
	nextToken := ""
	if int64(offset)+int64(len(rows)) < total {
		nextToken = strconv.FormatInt(int64(offset)+int64(len(rows)), 10)
	}
	// Version always reflects the full active catalog, not just this page.
	fullRows, err := s.loadAllActiveRows(ctx, q)
	if err != nil {
		return Snapshot{}, "", err
	}
	fullRecords, _, err := s.mapRows(ctx, q, fullRows)
	if err != nil {
		return Snapshot{}, "", err
	}
	version := catalogVersion(fullRecords)
	return Snapshot{
		CatalogVersion:   version,
		GeneratedAt:      time.Now().UTC(),
		TotalActiveCount: int32(total),
		Products:         records,
	}, nextToken, nil
}

// BuildDelta compares basis version to the current active catalog.
func (s *Service) BuildDelta(ctx context.Context, basisVersion string, basisProductIDs []uuid.UUID) (Delta, error) {
	if s == nil || s.pool == nil {
		return Delta{}, fmt.Errorf("productmastercatalog: nil service")
	}
	q := pgxutil.NewQueries(s.pool)
	rows, err := s.loadAllActiveRows(ctx, q)
	if err != nil {
		return Delta{}, err
	}
	records, currency, err := s.mapRows(ctx, q, rows)
	if err != nil {
		return Delta{}, err
	}
	_ = currency
	currentVersion := catalogVersion(records)
	basis := strings.TrimSpace(basisVersion)
	if basis != "" && basis == currentVersion {
		return Delta{
			BasisMatches:        true,
			BasisCatalogVersion: basis,
			ToCatalogVersion:    currentVersion,
			TotalActiveCount:    int32(len(records)),
			GeneratedAt:         time.Now().UTC(),
		}, nil
	}
	activeIDs := make(map[uuid.UUID]struct{}, len(records))
	for _, r := range records {
		activeIDs[r.ProductID] = struct{}{}
	}
	var tombstones []uuid.UUID
	for _, id := range basisProductIDs {
		if id == uuid.Nil {
			continue
		}
		if _, ok := activeIDs[id]; !ok {
			tombstones = append(tombstones, id)
		}
	}
	resetRequired := basis == ""
	return Delta{
		BasisMatches:                   false,
		BasisCatalogVersion:            basis,
		ToCatalogVersion:               currentVersion,
		TotalActiveCount:               int32(len(records)),
		Upserts:                        records,
		DeletedOrDeactivatedProductIDs: tombstones,
		GeneratedAt:                    time.Now().UTC(),
		ResetRequired:                  resetRequired,
	}, nil
}

func (s *Service) loadAllActiveRows(ctx context.Context, q *db.Queries) ([]db.ProductMasterListActiveProductsPageRow, error) {
	total, err := q.ProductMasterCountActiveProducts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]db.ProductMasterListActiveProductsPageRow, 0, total)
	var offset int32
	for int64(offset) < total {
		rows, err := q.ProductMasterListActiveProductsPage(ctx, db.ProductMasterListActiveProductsPageParams{
			Limit:  defaultPageSize,
			Offset: offset,
		})
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}
		out = append(out, rows...)
		offset += int32(len(rows))
	}
	return out, nil
}

func (s *Service) mapRows(ctx context.Context, q *db.Queries, rows []db.ProductMasterListActiveProductsPageRow) ([]Record, string, error) {
	if len(rows) == 0 {
		cur, err := q.InventoryAdminGetOrgDefaultCurrency(ctx)
		if err != nil {
			return nil, "", err
		}
		return nil, strings.ToUpper(strings.TrimSpace(cur)), nil
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	priceByID, err := s.loadDefaultPrices(ctx, q, ids)
	if err != nil {
		return nil, "", err
	}
	imgByProduct, err := q.RuntimeListProductImagesForProducts(ctx, ids)
	if err != nil {
		return nil, "", err
	}
	primaryByProduct := map[uuid.UUID]db.RuntimeListProductImagesForProductsRow{}
	for _, im := range imgByProduct {
		if _, exists := primaryByProduct[im.ProductID]; !exists || im.IsPrimary {
			primaryByProduct[im.ProductID] = im
		}
	}
	cur, err := q.InventoryAdminGetOrgDefaultCurrency(ctx)
	if err != nil {
		return nil, "", err
	}
	currency := strings.ToUpper(strings.TrimSpace(cur))
	out := make([]Record, 0, len(rows))
	for _, row := range rows {
		var catID *uuid.UUID
		if row.CategoryID.Valid {
			id := uuid.UUID(row.CategoryID.Bytes)
			catID = &id
		}
		var brandID *uuid.UUID
		if row.BrandID.Valid {
			id := uuid.UUID(row.BrandID.Bytes)
			brandID = &id
		}
		barcode := ""
		if row.Barcode.Valid {
			barcode = strings.TrimSpace(row.Barcode.String)
		}
		rec := Record{
			ProductID:      row.ID,
			SKU:            row.Sku,
			Barcode:        barcode,
			Name:           row.Name,
			Description:    row.Description,
			BasePriceMinor: priceByID[row.ID],
			Currency:       currency,
			Active:         row.Active,
			CategoryID:     catID,
			BrandID:        brandID,
			UpdatedAt:      row.UpdatedAt.UTC(),
		}
		if im, ok := primaryByProduct[row.ID]; ok {
			rec.Image = imageMetaFromRuntimeRow(im)
		}
		out = append(out, rec)
	}
	return out, currency, nil
}

func (s *Service) loadDefaultPrices(ctx context.Context, q *db.Queries, ids []uuid.UUID) (map[uuid.UUID]int64, error) {
	out := make(map[uuid.UUID]int64, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.ProductMasterDefaultPriceByProductIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ProductID] = row.UnitPriceMinor
	}
	return out, nil
}

func imageMetaFromRuntimeRow(im db.RuntimeListProductImagesForProductsRow) *ImageMeta {
	thumb := strings.TrimSpace(im.ThumbCdnUrl)
	if thumb == "" {
		thumb = strings.TrimSpace(im.CdnUrl)
	}
	disp := strings.TrimSpace(im.CdnUrl)
	if disp == "" {
		disp = thumb
	}
	ch := ""
	if im.ContentHash.Valid {
		ch = strings.TrimSpace(im.ContentHash.String)
	}
	if im.AssetSha256.Valid && strings.TrimSpace(im.AssetSha256.String) != "" {
		ch = strings.TrimSpace(im.AssetSha256.String)
	}
	etag := ch
	if im.AssetEtag.Valid && strings.TrimSpace(im.AssetEtag.String) != "" {
		etag = strings.TrimSpace(im.AssetEtag.String)
	}
	var mid uuid.UUID
	if im.MediaAssetID.Valid {
		mid = uuid.UUID(im.MediaAssetID.Bytes)
	}
	cacheKey := ""
	if mid != uuid.Nil {
		cacheKey = fmt.Sprintf("media:%s:v%d", mid.String(), im.MediaVersion)
	}
	return &ImageMeta{
		MediaID:      mid,
		ThumbURL:     thumb,
		DisplayURL:   disp,
		ContentHash:  ch,
		Etag:         etag,
		MediaVersion: im.MediaVersion,
		CacheKey:     cacheKey,
	}
}

func catalogVersion(records []Record) string {
	parts := make([]string, 0, len(records))
	for _, r := range records {
		img := ""
		if r.Image != nil {
			img = r.Image.MediaID.String() + ":" + strconv.Itoa(int(r.Image.MediaVersion)) + ":" + r.Image.ContentHash
		}
		cat := ""
		if r.CategoryID != nil {
			cat = r.CategoryID.String()
		}
		brand := ""
		if r.BrandID != nil {
			brand = r.BrandID.String()
		}
		parts = append(parts, strings.Join([]string{
			r.ProductID.String(),
			r.SKU,
			r.Name,
			strconv.FormatBool(r.Active),
			strconv.FormatInt(r.BasePriceMinor, 10),
			r.Currency,
			cat,
			brand,
			r.UpdatedAt.UTC().Format(time.RFC3339Nano),
			img,
		}, "|"))
	}
	return setupapp.SortedKeyFingerprint("product_master_catalog", parts)
}

func parsePageToken(token string) (int32, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(token, 10, 32)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("invalid page_token")
	}
	return int32(v), nil
}
