package correctness

import (
	"context"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/assortmentapp"
	appcatalogadmin "github.com/avf/avf-vending-api/internal/app/catalogadmin"
	appmediaadmin "github.com/avf/avf-vending-api/internal/app/mediaadmin"
	appsalecatalog "github.com/avf/avf-vending-api/internal/app/salecatalog"
	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/avf/avf-vending-api/internal/config"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/modules/postgres"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"github.com/avf/avf-vending-api/internal/testfixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestExternalProductImage_registerBindMachineCatalog(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	// Production-like allowlisted URL; RemoteProbe skipped (real HEAD to adm.avf.vn is production/manual).
	imageURL := "https://adm.avf.vn/storage/photos/1/Product/69f0e277129d9.png"

	company := id.NewUUIDV7()
	mediaSvc, err := appmediaadmin.NewService(appmediaadmin.Deps{
		Pool: pool,
		External: config.ExternalProductImageConfig{
			Enabled:       true,
			AllowedHosts:  []string{"adm.avf.vn"},
			RequireHTTPS:  true,
			MaxBytes:      5 << 20,
			RemoteTimeout: 5 * time.Second,
		},
		RemoteProbe: func(context.Context, string, string, config.ExternalProductImageConfig) error { return nil },
	})
	require.NoError(t, err)
	require.False(t, mediaSvc.UploadConfigured(), "external flow must not require object storage")

	reg, err := mediaSvc.RegisterExternalProductImage(ctx, appmediaadmin.RegisterExternalProductImageInput{
		CompanyID: company,
		URL:       imageURL,
		Purpose:   "product_image",
		Filename:  "69f0e277129d9.png",
	})
	require.NoError(t, err)
	require.Equal(t, "external_url", reg.SourceType)
	require.Equal(t, "ready", reg.Status)
	require.NotEmpty(t, reg.CacheKey)
	require.Equal(t, int32(1), reg.Version)
	require.Equal(t, imageURL, reg.DisplayURL)
	testfixtures.AssertResourceUUIDV7(t, reg.MediaID)

	reg2, err := mediaSvc.RegisterExternalProductImage(ctx, appmediaadmin.RegisterExternalProductImageInput{
		CompanyID:   company,
		URL:         imageURL,
		Purpose:     "product_image",
		Filename:    "69f0e277129d9.png",
		ContentType: "image/png",
	})
	require.NoError(t, err)
	require.True(t, reg2.Replay)
	require.Equal(t, reg.MediaID, reg2.MediaID)

	catSvc, err := appcatalogadmin.NewService(db.New(pool), pool, nil)
	require.NoError(t, err)
	catSvc.SetMediaBinder(mediaSvc)

	sku := "EXTIMG-" + uuid.NewString()[:8]
	mid := reg.MediaID
	prod, err := catSvc.CreateProduct(ctx, appcatalogadmin.CreateProductInput{
		Sku:            sku,
		Name:           "External Image Test Product",
		Description:    "integration",
		Active:         true,
		CompanyID:      company,
		PrimaryMediaID: &mid,
	})
	require.NoError(t, err)
	require.True(t, prod.PrimaryImageID.Valid)

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM product_images WHERE product_id = $1`, prod.ID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM product_media WHERE product_id = $1`, prod.ID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM products WHERE id = $1`, prod.ID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM media_assets WHERE id = $1`, mid)
	})

	q := db.New(pool)
	assRow, err := q.FleetAdminInsertAssortment(ctx, db.FleetAdminInsertAssortmentParams{
		Name:   "ext-img-" + uuid.NewString(),
		Status: "published",
		Meta:   []byte(`{}`),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM assortment_items WHERE assortment_id = $1`, assRow.ID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM assortments WHERE id = $1`, assRow.ID)
	})
	_, err = q.FleetAdminUpsertAssortmentItem(ctx, db.FleetAdminUpsertAssortmentItemParams{
		AssortmentID: assRow.ID,
		ProductID:    prod.ID,
		SortOrder:    1,
		Notes:        []byte(`{}`),
	})
	require.NoError(t, err)
	arepo := postgres.NewAssortmentRepository(pool)
	require.NoError(t, arepo.BindMachineAssortment(ctx, assortmentapp.BindMachineAssortmentInput{
		MachineID:    testfixtures.DevMachineID,
		AssortmentID: assRow.ID,
	}))
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM machine_assortment_bindings WHERE machine_id = $1 AND assortment_id = $2`, testfixtures.DevMachineID, assRow.ID)
	})

	repo := postgres.NewSetupRepository(pool)
	require.NoError(t, repo.UpsertMachineTopology(ctx, testfixtures.DevMachineID,
		[]setupapp.CabinetUpsert{{
			Code: "A", Title: "Alpha", SortOrder: 1, Metadata: []byte(`{}`),
		}},
		[]setupapp.TopologyLayoutUpsert{{
			CabinetCode: "A", LayoutKey: "default", Revision: 1, LayoutSpec: []byte(`{"rows":1}`), Status: "published",
		}},
	))
	slotIdx := int32(99)
	require.NoError(t, repo.SaveDraftOrCurrentSlotConfigs(ctx, testfixtures.DevMachineID, setupapp.SlotConfigSaveInput{
		PlanogramID:         testfixtures.DevPlanogramID,
		PlanogramRevision:   1,
		PublishAsCurrent:    true,
		SyncLegacyReadModel: true,
		Items: []setupapp.SlotConfigSaveItem{{
			CabinetCode: "A", LayoutKey: "default", LayoutRevision: 1,
			SlotCode: "EXT1", LegacySlotIndex: &slotIdx, ProductID: &prod.ID,
			MaxQuantity: 5, PriceMinor: 15000, EffectiveFrom: time.Now().UTC(), Metadata: []byte(`{}`),
		}},
	}))
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM machine_slot_configs WHERE machine_id = $1 AND slot_code = 'EXT1'`, testfixtures.DevMachineID)
	})

	cat := appsalecatalog.NewService(pool)
	snap, err := cat.BuildSnapshot(ctx, testfixtures.DevMachineID, appsalecatalog.Options{
		IncludeUnavailable: true,
		IncludeImages:      true,
	})
	require.NoError(t, err)

	var found *appsalecatalog.Item
	for i := range snap.Items {
		if snap.Items[i].ProductID == prod.ID {
			found = &snap.Items[i]
			break
		}
	}
	require.NotNil(t, found, "product must appear on assigned machine catalog")
	require.NotNil(t, found.Image)
	require.Equal(t, mid, found.Image.MediaID)
	require.Equal(t, "external_url", found.Image.SourceType)
	require.NotEmpty(t, found.Image.DisplayURL)
	require.NotEmpty(t, found.Image.ThumbURL)
	require.NotEmpty(t, found.Image.CacheKey)
	require.Equal(t, int32(1), found.Image.MediaVersion)
	require.True(t, found.Image.OfflineRequired)
	require.Equal(t, "download_when_online_use_local_when_offline", found.Image.DownloadStrategy)
}
