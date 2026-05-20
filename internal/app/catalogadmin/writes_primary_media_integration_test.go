package catalogadmin

import (
	"bytes"
	"context"
	"github.com/avf/avf-vending-api/internal/platform/id"
	"image"
	"image/png"
	"io"
	"testing"
	"time"

	appmediaadmin "github.com/avf/avf-vending-api/internal/app/mediaadmin"
	"github.com/avf/avf-vending-api/internal/gen/db"
	"github.com/avf/avf-vending-api/internal/platform/objectstore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type pmMemObj struct {
	body []byte
	ct   string
	meta map[string]string
}

// phase2MemStore is an in-memory object store for primary-media integration tests.
type phase2MemStore struct {
	obj map[string]pmMemObj
}

func newPhase2MemStore() *phase2MemStore {
	return &phase2MemStore{obj: make(map[string]pmMemObj)}
}

func (m *phase2MemStore) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	return m.PutWithUserMetadata(ctx, key, body, size, contentType, nil)
}

func (m *phase2MemStore) PutWithUserMetadata(ctx context.Context, key string, body io.Reader, size int64, contentType string, userMetadata map[string]string) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	cp := map[string]string{}
	for k, v := range userMetadata {
		cp[k] = v
	}
	m.obj[key] = pmMemObj{body: b, ct: contentType, meta: cp}
	return nil
}

func (m *phase2MemStore) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	o, ok := m.obj[key]
	if !ok {
		return nil, "", io.EOF
	}
	return io.NopCloser(bytes.NewReader(o.body)), o.ct, nil
}

func (m *phase2MemStore) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (objectstore.SignedHTTP, error) {
	return objectstore.SignedHTTP{URL: "http://local/upload/" + key, Method: "PUT"}, nil
}

func (m *phase2MemStore) PresignGet(ctx context.Context, key string, ttl time.Duration) (objectstore.SignedHTTP, error) {
	return objectstore.SignedHTTP{URL: "http://local/get/" + key, Method: "GET"}, nil
}

func (m *phase2MemStore) Head(ctx context.Context, key string) (objectstore.ObjectMeta, error) {
	o, ok := m.obj[key]
	if !ok {
		return objectstore.ObjectMeta{}, io.EOF
	}
	return objectstore.ObjectMeta{
		Key:          key,
		Size:         int64(len(o.body)),
		ContentType:  o.ct,
		UserMetadata: o.meta,
	}, nil
}

func (m *phase2MemStore) Delete(ctx context.Context, key string) error {
	delete(m.obj, key)
	return nil
}

func (m *phase2MemStore) ListPrefix(ctx context.Context, prefix string, maxKeys int32) ([]objectstore.ObjectMeta, error) {
	return nil, nil
}

func pngUploadBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func TestPrimaryMedia_phase2_catalog_rules(t *testing.T) {
	t.Parallel()
	pool := catalogWritesPool(t)
	ctx := context.Background()
	st := newPhase2MemStore()
	mediaSvc, err := appmediaadmin.NewService(appmediaadmin.Deps{
		Pool:           pool,
		Store:          st,
		PresignPutTTL:  time.Minute,
		MaxUploadBytes: 5 << 20,
	})
	require.NoError(t, err)
	catSvc, err := NewService(db.New(pool), pool, nil)
	require.NoError(t, err)
	catSvc.SetMediaBinder(mediaSvc)

	company := id.NewUUIDV7()

	t.Run("inactive_without_primary_ok", func(t *testing.T) {
		_, err := catSvc.CreateProduct(ctx, CreateProductInput{
			Sku:         "SKU-DRAFT-" + uuid.NewString()[:8],
			Name:        "Draft",
			Description: "d",
			Active:      false,
			CompanyID:   uuid.Nil,
		})
		require.NoError(t, err)
	})

	t.Run("active_without_primary_fails", func(t *testing.T) {
		_, err := catSvc.CreateProduct(ctx, CreateProductInput{
			Sku:         "SKU-ACT-" + uuid.NewString()[:8],
			Name:        "Active",
			Description: "d",
			Active:      true,
			CompanyID:   company,
		})
		require.ErrorIs(t, err, ErrInvalidArgument)
	})

	t.Run("invalid_tag_ids_fail", func(t *testing.T) {
		badTag := id.NewUUIDV7()
		_, err := catSvc.CreateProduct(ctx, CreateProductInput{
			Sku:         "SKU-TAGBAD-" + uuid.NewString()[:8],
			Name:        "T",
			Description: "d",
			Active:      false,
			CompanyID:   uuid.Nil,
			TagIDs:      []uuid.UUID{badTag},
		})
		require.ErrorIs(t, err, ErrInvalidArgument)
	})

	t.Run("active_with_ready_primary_passes", func(t *testing.T) {
		init, err := mediaSvc.InitUpload(ctx, company, "phase2.png", "image/png", "product_image")
		require.NoError(t, err)
		png := pngUploadBytes(t)
		require.NoError(t, st.Put(ctx, init.OriginalKey, bytes.NewReader(png), int64(len(png)), "image/png"))
		_, err = mediaSvc.CompleteUpload(ctx, company, init.MediaID)
		require.NoError(t, err)

		mid := init.MediaID
		p, err := catSvc.CreateProduct(ctx, CreateProductInput{
			Sku:            "SKU-PRI-" + uuid.NewString()[:8],
			Name:           "With media",
			Description:    "d",
			Active:         true,
			CompanyID:      company,
			PrimaryMediaID: &mid,
		})
		require.NoError(t, err)
		require.True(t, p.PrimaryImageID.Valid)

		vars, err := mediaSvc.ListVariantsForAssets(ctx, []uuid.UUID{mid})
		require.NoError(t, err)
		require.NotEmpty(t, vars)
	})

	t.Run("update_to_active_without_primary_fails", func(t *testing.T) {
		sku := "SKU-UP-" + uuid.NewString()[:8]
		p, err := catSvc.CreateProduct(ctx, CreateProductInput{
			Sku:         sku,
			Name:        "Up",
			Description: "d",
			Active:      false,
			CompanyID:   uuid.Nil,
		})
		require.NoError(t, err)
		_, err = catSvc.UpdateProduct(ctx, UpdateProductInput{
			ProductID:     p.ID,
			Sku:           sku,
			Name:          "Up",
			Description:   "d",
			Active:        true,
			CompanyID:     company,
			AllergenCodes: []string{},
		})
		require.ErrorIs(t, err, ErrInvalidArgument)
	})

	t.Run("update_to_active_with_primary_passes", func(t *testing.T) {
		init, err := mediaSvc.InitUpload(ctx, company, "phase2b.png", "image/png", "product_image")
		require.NoError(t, err)
		b := pngUploadBytes(t)
		require.NoError(t, st.Put(ctx, init.OriginalKey, bytes.NewReader(b), int64(len(b)), "image/png"))
		_, err = mediaSvc.CompleteUpload(ctx, company, init.MediaID)
		require.NoError(t, err)
		mid := init.MediaID

		sku := "SKU-UP2-" + uuid.NewString()[:8]
		p, err := catSvc.CreateProduct(ctx, CreateProductInput{
			Sku:         sku,
			Name:        "Up2",
			Description: "d",
			Active:      false,
			CompanyID:   uuid.Nil,
		})
		require.NoError(t, err)

		p2, err := catSvc.UpdateProduct(ctx, UpdateProductInput{
			ProductID:             p.ID,
			Sku:                   sku,
			Name:                  "Up2",
			Description:           "d",
			Active:                true,
			CompanyID:             company,
			PrimaryMediaIDReplace: &mid,
			AllergenCodes:         []string{},
		})
		require.NoError(t, err)
		require.True(t, p2.Active)
		require.True(t, p2.PrimaryImageID.Valid)
	})
}

func TestPrimaryMedia_phase3_manifest_bind_complete_validation(t *testing.T) {
	t.Parallel()
	pool := catalogWritesPool(t)
	ctx := context.Background()
	st := newPhase2MemStore()
	mediaSvc, err := appmediaadmin.NewService(appmediaadmin.Deps{
		Pool:           pool,
		Store:          st,
		PresignPutTTL:  time.Minute,
		MaxUploadBytes: 5 << 20,
	})
	require.NoError(t, err)
	catSvc, err := NewService(db.New(pool), pool, nil)
	require.NoError(t, err)
	catSvc.SetMediaBinder(mediaSvc)

	company := id.NewUUIDV7()

	t.Run("init_upload_leaves_pending", func(t *testing.T) {
		init, err := mediaSvc.InitUpload(ctx, company, "p.png", "image/png", "product_image")
		require.NoError(t, err)
		q := db.New(pool)
		row, err := q.MediaAdminGetAssetForOrg(ctx, init.MediaID)
		require.NoError(t, err)
		require.Equal(t, "pending", row.Status)
	})

	t.Run("complete_marks_ready", func(t *testing.T) {
		init, err := mediaSvc.InitUpload(ctx, company, "r.png", "image/png", "product_image")
		require.NoError(t, err)
		b := pngUploadBytes(t)
		require.NoError(t, st.Put(ctx, init.OriginalKey, bytes.NewReader(b), int64(len(b)), "image/png"))
		row, err := mediaSvc.CompleteUpload(ctx, company, init.MediaID)
		require.NoError(t, err)
		require.Equal(t, "ready", row.Status)
	})

	t.Run("complete_rejects_non_image_head_type", func(t *testing.T) {
		init, err := mediaSvc.InitUpload(ctx, company, "bad.png", "image/png", "product_image")
		require.NoError(t, err)
		require.NoError(t, st.Put(ctx, init.OriginalKey, bytes.NewReader([]byte("%PDF-1.4")), 9, "application/pdf"))
		_, err = mediaSvc.CompleteUpload(ctx, company, init.MediaID)
		require.Error(t, err)
		q := db.New(pool)
		a, qerr := q.MediaAdminGetAssetForOrg(ctx, init.MediaID)
		require.NoError(t, qerr)
		require.NotEqual(t, "ready", a.Status)
	})

	t.Run("manifest_contains_sha_version_size_url", func(t *testing.T) {
		init, err := mediaSvc.InitUpload(ctx, company, "m.png", "image/png", "product_image")
		require.NoError(t, err)
		b := pngUploadBytes(t)
		require.NoError(t, st.Put(ctx, init.OriginalKey, bytes.NewReader(b), int64(len(b)), "image/png"))
		_, err = mediaSvc.CompleteUpload(ctx, company, init.MediaID)
		require.NoError(t, err)

		man, err := mediaSvc.MediaManifest(ctx, company, init.MediaID)
		require.NoError(t, err)
		require.NotEmpty(t, man)
		by := map[string]appmediaadmin.MediaManifestEntry{}
		for _, e := range man {
			by[e.Variant] = e
		}
		require.Contains(t, by, "original")
		o := by["original"]
		require.NotEmpty(t, o.SHA256)
		require.GreaterOrEqual(t, o.Version, int32(1))
		require.Positive(t, o.SizeBytes)
		require.NotEmpty(t, o.DownloadURL)
		require.Contains(t, o.MimeType, "image/")
	})

	t.Run("manifest_requires_ready", func(t *testing.T) {
		init, err := mediaSvc.InitUpload(ctx, company, "pend.png", "image/png", "product_image")
		require.NoError(t, err)
		_, err = mediaSvc.MediaManifest(ctx, company, init.MediaID)
		require.ErrorIs(t, err, appmediaadmin.ErrInvalidArgument)
	})

	t.Run("bind_requires_ready_media", func(t *testing.T) {
		init, err := mediaSvc.InitUpload(ctx, company, "nobytes.png", "image/png", "product_image")
		require.NoError(t, err)
		p, err := catSvc.CreateProduct(ctx, CreateProductInput{
			Sku:         "SKU-BIND-" + uuid.NewString()[:8],
			Name:        "B",
			Description: "d",
			Active:      false,
			CompanyID:   uuid.Nil,
		})
		require.NoError(t, err)
		_, err = mediaSvc.BindProductPrimaryMedia(ctx, company, p.ID, init.MediaID)
		require.Error(t, err)
		require.ErrorIs(t, err, appmediaadmin.ErrConflict)
	})

	t.Run("active_create_with_pending_primary_fails", func(t *testing.T) {
		init, err := mediaSvc.InitUpload(ctx, company, "pend-act.png", "image/png", "product_image")
		require.NoError(t, err)
		mid := init.MediaID
		_, err = catSvc.CreateProduct(ctx, CreateProductInput{
			Sku:            "SKU-ACT-PEND-" + uuid.NewString()[:8],
			Name:           "Act pend",
			Description:    "d",
			Active:         true,
			CompanyID:      company,
			PrimaryMediaID: &mid,
		})
		require.Error(t, err)
		require.ErrorIs(t, err, appmediaadmin.ErrConflict)
	})

	t.Run("ready_media_can_bind", func(t *testing.T) {
		init, err := mediaSvc.InitUpload(ctx, company, "bind.png", "image/png", "product_image")
		require.NoError(t, err)
		b := pngUploadBytes(t)
		require.NoError(t, st.Put(ctx, init.OriginalKey, bytes.NewReader(b), int64(len(b)), "image/png"))
		_, err = mediaSvc.CompleteUpload(ctx, company, init.MediaID)
		require.NoError(t, err)
		p, err := catSvc.CreateProduct(ctx, CreateProductInput{
			Sku:         "SKU-BIND2-" + uuid.NewString()[:8],
			Name:        "B2",
			Description: "d",
			Active:      false,
			CompanyID:   uuid.Nil,
		})
		require.NoError(t, err)
		_, err = mediaSvc.BindProductPrimaryMedia(ctx, company, p.ID, init.MediaID)
		require.NoError(t, err)
	})
}
