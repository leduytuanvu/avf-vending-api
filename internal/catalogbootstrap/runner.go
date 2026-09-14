package catalogbootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/avf/avf-vending-api/internal/app/catalogadmin"
	appmediaadmin "github.com/avf/avf-vending-api/internal/app/mediaadmin"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/gen/db"
	platformcloudinary "github.com/avf/avf-vending-api/internal/platform/cloudinary"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	StatePending            = "PENDING"
	StateVerified           = "VERIFIED"
	StateFailed             = "FAILED"
	StateCloudinaryUploaded = "CLOUDINARY_UPLOADED"
	StateProductCreated     = "PRODUCT_CREATED"
	StatePriceCreated       = "PRICE_CREATED"
)

// Runner executes catalog bootstrap phases.
type Runner struct {
	Pool       *pgxpool.Pool
	Catalog    *catalogadmin.Service
	Media      *appmediaadmin.Service
	Lookup     *Lookup
	Evidence   *EvidenceWriter
	Uploader   *platformcloudinary.Uploader
	Downloader *ImageDownloader
	CompanyID  uuid.UUID
	AppEnv     string
	Manifest   *Manifest
}

// Options configures bootstrap execution.
type Options struct {
	DryRun            bool
	UploadImages      bool
	ImportTaxonomy    bool
	ImportProducts    bool
	ImportPrices      bool
	Resume            bool
	ConfirmProduction bool
}

// Run executes selected bootstrap phases.
func (r *Runner) Run(ctx context.Context, opt Options) error {
	if r == nil || r.Manifest == nil {
		return errors.New("runner not configured")
	}
	issues := ValidateManifest(r.Manifest)
	if len(issues) > 0 {
		return fmt.Errorf("manifest validation failed: %s", strings.Join(issues, "; "))
	}
	if opt.DryRun {
		_, _, _, err := ImportTaxonomy(ctx, r.Catalog, r.Lookup, r.Manifest, true)
		if err != nil {
			return err
		}
		return r.Evidence.WriteJSON("09-dry-run-report.json", map[string]any{
			"dry_run":       true,
			"product_count": len(r.Manifest.Products),
			"would_import":  true,
			"upload_images": opt.UploadImages,
			"import_prices": opt.ImportPrices,
		})
	}

	cp, err := r.Evidence.LoadCheckpoint()
	if err != nil {
		return err
	}
	if !opt.Resume {
		cp = &Checkpoint{Items: map[string]CheckpointItem{}}
	}

	catIDs, brandIDs, tagIDs := map[string]uuid.UUID{}, map[string]uuid.UUID{}, map[string]uuid.UUID{}
	if opt.ImportTaxonomy {
		catIDs, brandIDs, tagIDs, err = ImportTaxonomy(ctx, r.Catalog, r.Lookup, r.Manifest, false)
		if err != nil {
			return err
		}
		_ = r.Evidence.WriteJSON("10-taxonomy-import-results.json", map[string]any{
			"categories": len(catIDs),
			"brands":     len(brandIDs),
			"tags":       len(tagIDs),
		})
	} else {
		for _, c := range r.Manifest.Taxonomy.Categories {
			id, err := r.Lookup.CategoryIDBySlug(ctx, c.Slug)
			if err != nil {
				return fmt.Errorf("lookup category %s: %w", c.Slug, err)
			}
			catIDs[c.Slug] = id
		}
		for _, b := range r.Manifest.Taxonomy.Brands {
			id, err := r.Lookup.BrandIDBySlug(ctx, b.Slug)
			if err != nil {
				return fmt.Errorf("lookup brand %s: %w", b.Slug, err)
			}
			brandIDs[b.Slug] = id
		}
		for _, t := range r.Manifest.Taxonomy.Tags {
			id, err := r.Lookup.TagIDBySlug(ctx, t.Slug)
			if err != nil {
				return fmt.Errorf("lookup tag %s: %w", t.Slug, err)
			}
			tagIDs[t.Slug] = id
		}
	}

	priceBookID, err := r.ensurePriceBook(ctx)
	if err != nil {
		return err
	}

	q := db.New(r.Pool)
	uploadMap := []map[string]any{}
	productResults := []map[string]any{}

	if opt.ImportProducts {
		for _, p := range r.Manifest.Products {
			item := cp.Items[p.SKU]
			if item.State == StateVerified || item.State == StatePriceCreated {
				continue
			}
			res, ures, err := r.importOne(ctx, q, opt, catIDs, brandIDs, tagIDs, priceBookID, p, &item)
			productResults = append(productResults, res)
			if ures != nil {
				uploadMap = append(uploadMap, ures)
			}
			if err != nil {
				item.State = StateFailed
				item.Error = err.Error()
				cp.Items[p.SKU] = item
				_ = r.Evidence.SaveCheckpoint(cp)
				return fmt.Errorf("sku %s: %w", p.SKU, err)
			}
			item.State = StateVerified
			cp.Items[p.SKU] = item
			_ = r.Evidence.SaveCheckpoint(cp)
		}
	}

	_ = r.Evidence.WriteJSON("11-product-import-results.json", productResults)
	_ = r.Evidence.WriteJSON("12-cloudinary-upload-map.json", uploadMap)

	metrics, err := VerifyDatabase(ctx, r.Lookup, len(r.Manifest.Products))
	if err != nil {
		return err
	}
	_ = r.Evidence.WriteJSON("15-database-final-audit.json", metrics)
	return nil
}

func (r *Runner) ensurePriceBook(ctx context.Context) (uuid.UUID, error) {
	id, err := r.Lookup.DefaultPriceBookID(ctx)
	if err == nil {
		return id, nil
	}
	row, err := r.Catalog.CreatePriceBook(ctx, catalogadmin.CreatePriceBookInput{
		Name:           "AVF Production Default",
		Currency:       "VND",
		EffectiveFrom:  time.Now().UTC(),
		IsDefault:      true,
		PriceBookLevel: "global",
	})
	if err != nil {
		return uuid.Nil, err
	}
	return row.ID, nil
}

func (r *Runner) importOne(
	ctx context.Context,
	q *db.Queries,
	opt Options,
	catIDs, brandIDs, tagIDs map[string]uuid.UUID,
	priceBookID uuid.UUID,
	p ProductRecord,
	item *CheckpointItem,
) (map[string]any, map[string]any, error) {
	result := map[string]any{"sku": p.SKU, "name": p.Name}
	var uploadRow map[string]any

	existingID, err := r.Lookup.ProductIDBySKU(ctx, p.SKU)
	if err == nil {
		result["action"] = "exists"
		item.ProductID = existingID.String()
		if opt.ImportPrices {
			_, perr := r.Catalog.UpsertPriceBookItem(ctx, r.CompanyID, priceBookID, existingID, p.PriceVND)
			if perr != nil {
				return result, uploadRow, perr
			}
		}
		return result, uploadRow, nil
	}

	var mediaID uuid.UUID
	if opt.UploadImages {
		if item.MediaAssetID != "" {
			mediaID, _ = uuid.Parse(item.MediaAssetID)
		}
		if mediaID == uuid.Nil {
			// Idempotent: skip upload if media already exists for public id.
			fullPublicID := strings.TrimSpace(p.CloudinaryPubID)
			existingMedia, lerr := r.Lookup.MediaAssetByProviderPublicID(ctx, fullPublicID)
			if lerr == nil {
				mediaID = existingMedia
			} else {
				up, uerr := UploadProductImage(ctx, q, r.Uploader, r.Downloader, r.CompanyID, p, r.AppEnv)
				if uerr != nil {
					return result, uploadRow, uerr
				}
				mediaID = up.MediaID
				item.MediaAssetID = mediaID.String()
				item.CloudinaryPubID = up.PublicID
				item.State = StateCloudinaryUploaded
				uploadRow = map[string]any{
					"sku":                   p.SKU,
					"product_name":          p.Name,
					"source_image_url":      p.SourceImageURL,
					"source_sha256":         up.SHA256,
					"cloudinary_asset_id":   up.AssetID,
					"cloudinary_public_id":  up.PublicID,
					"cloudinary_secure_url": up.SecureURL,
					"format":                up.Format,
					"bytes":                 up.Bytes,
					"width":                 up.Width,
					"height":                up.Height,
					"status":                "uploaded",
				}
			}
		}
	} else {
		return result, uploadRow, fmt.Errorf("upload-images required for new products")
	}

	catID := catIDs[p.CategorySlug]
	var brandID *uuid.UUID
	if p.BrandSlug != nil && strings.TrimSpace(*p.BrandSlug) != "" {
		bid := brandIDs[strings.TrimSpace(*p.BrandSlug)]
		if bid != uuid.Nil {
			brandID = &bid
		}
	}
	tagUUIDs := make([]uuid.UUID, 0, len(p.TagSlugs))
	for _, ts := range p.TagSlugs {
		if id := tagIDs[ts]; id != uuid.Nil {
			tagUUIDs = append(tagUUIDs, id)
		}
	}
	attrs, _ := json.Marshal(map[string]any{
		"import": map[string]any{
			"source_image_url": p.SourceImageURL,
			"legacy_sha256":    p.SourceSHA256,
		},
	})
	desc := p.Description
	if desc == "" {
		desc = p.Name
	}
	prod, err := r.Catalog.CreateProduct(ctx, catalogadmin.CreateProductInput{
		Sku:            p.SKU,
		Name:           p.Name,
		Description:    desc,
		Attrs:          attrs,
		Active:         p.Active,
		CategoryID:     &catID,
		BrandID:        brandID,
		TagIDs:         tagUUIDs,
		CompanyID:      r.CompanyID,
		PrimaryMediaID: &mediaID,
	})
	if err != nil {
		return result, uploadRow, err
	}
	item.ProductID = prod.ID.String()
	item.State = StateProductCreated
	result["action"] = "created"
	result["product_id"] = prod.ID.String()

	if opt.ImportPrices {
		_, err = r.Catalog.UpsertPriceBookItem(ctx, r.CompanyID, priceBookID, prod.ID, p.PriceVND)
		if err != nil {
			return result, uploadRow, err
		}
		item.State = StatePriceCreated
	}
	return result, uploadRow, nil
}

// NewRunner wires dependencies from config.
func NewRunner(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, manifest *Manifest, evidenceDir string) (*Runner, error) {
	q := db.New(pool)
	catSvc, err := catalogadmin.NewService(q, pool, nil)
	if err != nil {
		return nil, err
	}
	uploadCfg := cfg.MediaUpload
	var cldUploader *platformcloudinary.Uploader
	if uploadCfg.CloudinaryConfigured() {
		cldUploader, err = platformcloudinary.NewUploader(
			uploadCfg.Cloudinary.CloudName,
			uploadCfg.Cloudinary.APIKey,
			uploadCfg.Cloudinary.APISecret,
			uploadCfg.Cloudinary.Folder,
			uploadCfg.ThumbWidth,
			uploadCfg.ThumbHeight,
		)
		if err != nil {
			return nil, err
		}
	}
	var mediaSvc *appmediaadmin.Service
	if uploadCfg.CloudinaryConfigured() {
		mediaSvc, err = appmediaadmin.NewService(appmediaadmin.Deps{
			Pool:       pool,
			Upload:     uploadCfg,
			Cloudinary: cldUploader,
			AppEnv:     string(cfg.AppEnv),
		})
		if err != nil {
			return nil, fmt.Errorf("media service: %w", err)
		}
		catSvc.SetMediaBinder(mediaSvc)
	}
	ev, err := NewEvidenceWriter(evidenceDir)
	if err != nil {
		return nil, err
	}
	companyID := uploadCfg.CompanyID
	if companyID == uuid.Nil && uploadCfg.CloudinaryConfigured() {
		return nil, errors.New("MEDIA_COMPANY_ID is required for catalog bootstrap when Cloudinary is enabled")
	}
	return &Runner{
		Pool:       pool,
		Catalog:    catSvc,
		Media:      mediaSvc,
		Lookup:     NewLookup(pool),
		Evidence:   ev,
		Uploader:   cldUploader,
		Downloader: NewImageDownloader(),
		CompanyID:  companyID,
		AppEnv:     string(cfg.AppEnv),
		Manifest:   manifest,
	}, nil
}

// PreflightProduction records redacted production target verification.
func PreflightProduction(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, evidence *EvidenceWriter) error {
	lookup := NewLookup(pool)
	counts, err := lookup.CatalogCounts(ctx)
	if err != nil {
		return err
	}
	uploadCfg := cfg.MediaUpload
	fp := RedactCloudinaryFingerprint(
		uploadCfg.Cloudinary.CloudName,
		uploadCfg.Cloudinary.Folder,
		uploadCfg.Cloudinary.APIKey,
	)
	_ = evidence.WriteJSON("03-current-production-catalog-audit.json", map[string]any{
		"app_env":              cfg.AppEnv,
		"counts":               counts,
		"cloudinary":           fp,
		"media_company_id_set": uploadCfg.CompanyID != uuid.Nil,
	})
	return nil
}

// VerifyDatabase runs acceptance metrics.
func VerifyDatabase(ctx context.Context, lookup *Lookup, expectedProducts int) (map[string]any, error) {
	counts, err := lookup.CatalogCounts(ctx)
	if err != nil {
		return nil, err
	}
	var dupSKU int64
	err = lookup.pool.QueryRow(ctx, `
		SELECT count(*) FROM (
			SELECT sku FROM products GROUP BY sku HAVING count(*) > 1
		) d`).Scan(&dupSKU)
	if err != nil {
		return nil, err
	}
	var withoutPrimary int64
	err = lookup.pool.QueryRow(ctx, `
		SELECT count(*) FROM products p
		WHERE p.active AND (
			p.primary_image_id IS NULL OR NOT EXISTS (
				SELECT 1 FROM product_images pi
				WHERE pi.product_id = p.id AND pi.id = p.primary_image_id AND pi.is_primary AND pi.status = 'active'
			)
		)`).Scan(&withoutPrimary)
	if err != nil {
		return nil, err
	}
	var oldURL int64
	err = lookup.pool.QueryRow(ctx, `
		SELECT count(*) FROM product_images pi
		WHERE pi.cdn_url ILIKE '%adm.avf.vn%' OR pi.thumb_cdn_url ILIKE '%adm.avf.vn%'`).Scan(&oldURL)
	if err != nil {
		return nil, err
	}
	metrics := map[string]any{
		"EXPECTED_PRODUCT_COUNT":             expectedProducts,
		"ACTUAL_PRODUCT_COUNT":               counts.Products,
		"DUPLICATE_SKU_COUNT":                dupSKU,
		"PRODUCTS_WITHOUT_PRIMARY_IMAGE":     withoutPrimary,
		"PRODUCTS_WITH_OLD_SOURCE_IMAGE_URL": oldURL,
		"counts":                             counts,
	}
	return metrics, nil
}
