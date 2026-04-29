package mediaadmin

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/platform/objectstore"
	"github.com/google/uuid"
)

type memObj struct {
	body []byte
	ct   string
	meta map[string]string
}

// mapObjectStore is a minimal in-memory store for variant generation tests.
type mapObjectStore struct {
	obj map[string]memObj
}

func newMapObjectStore() *mapObjectStore {
	return &mapObjectStore{obj: make(map[string]memObj)}
}

func (m *mapObjectStore) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	return m.PutWithUserMetadata(ctx, key, body, size, contentType, nil)
}

func (m *mapObjectStore) PutWithUserMetadata(ctx context.Context, key string, body io.Reader, size int64, contentType string, userMetadata map[string]string) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	cp := map[string]string{}
	for k, v := range userMetadata {
		cp[k] = v
	}
	m.obj[key] = memObj{body: b, ct: contentType, meta: cp}
	return nil
}

func (m *mapObjectStore) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	o, ok := m.obj[key]
	if !ok {
		return nil, "", io.EOF
	}
	return io.NopCloser(bytes.NewReader(o.body)), o.ct, nil
}

func (m *mapObjectStore) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (objectstore.SignedHTTP, error) {
	return objectstore.SignedHTTP{}, io.EOF
}

func (m *mapObjectStore) PresignGet(ctx context.Context, key string, ttl time.Duration) (objectstore.SignedHTTP, error) {
	return objectstore.SignedHTTP{}, io.EOF
}

func (m *mapObjectStore) Head(ctx context.Context, key string) (objectstore.ObjectMeta, error) {
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

func (m *mapObjectStore) Delete(ctx context.Context, key string) error {
	delete(m.obj, key)
	return nil
}

func (m *mapObjectStore) ListPrefix(ctx context.Context, prefix string, maxKeys int32) ([]objectstore.ObjectMeta, error) {
	return nil, nil
}

func TestWebPVariantGenerator_outputsWebP(t *testing.T) {
	t.Parallel()
	const size = 64
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	var enc bytes.Buffer
	if err := png.Encode(&enc, img); err != nil {
		t.Fatal(err)
	}
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	ok := objectstore.MediaAssetOriginalKey(org, mid)
	st := newMapObjectStore()
	if err := st.Put(context.Background(), ok, bytes.NewReader(enc.Bytes()), int64(enc.Len()), "image/png"); err != nil {
		t.Fatal(err)
	}
	g := WebPVariantGenerator{ThumbMax: 32, DisplayMax: 48}
	va, err := g.GenerateWebPVariants(context.Background(), st, org, mid, ok, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if va.DisplayBytes <= 0 || va.ThumbBytes <= 0 {
		t.Fatalf("expected non-empty variants: %+v", va)
	}
	tk := objectstore.MediaAssetThumbWebpKey(org, mid)
	head, err := st.Head(context.Background(), tk)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(head.ContentType, webpContentType) {
		t.Fatalf("thumb content type %q", head.ContentType)
	}
}

func TestWebPVariantGenerator_rejectsNonCanonicalOriginalKey(t *testing.T) {
	t.Parallel()
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	var enc bytes.Buffer
	if err := png.Encode(&enc, img); err != nil {
		t.Fatal(err)
	}
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	ok := objectstore.MediaAssetOriginalKey(org, mid)
	st := newMapObjectStore()
	if err := st.Put(context.Background(), ok, bytes.NewReader(enc.Bytes()), int64(enc.Len()), "image/png"); err != nil {
		t.Fatal(err)
	}
	g := WebPVariantGenerator{}
	wrongMid := uuid.New()
	wrongKey := objectstore.MediaAssetOriginalKey(org, wrongMid)
	_, err := g.GenerateWebPVariants(context.Background(), st, org, mid, wrongKey, 1<<20)
	if err == nil {
		t.Fatal("expected error for key mismatch")
	}
}

func TestWebPVariantGenerator_rejectsNonImageSniff(t *testing.T) {
	t.Parallel()
	org := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mid := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	ok := objectstore.MediaAssetOriginalKey(org, mid)
	st := newMapObjectStore()
	payload := []byte("not-an-image-binary")
	if err := st.Put(context.Background(), ok, bytes.NewReader(payload), int64(len(payload)), "text/plain"); err != nil {
		t.Fatal(err)
	}
	g := WebPVariantGenerator{}
	if _, err := g.GenerateWebPVariants(context.Background(), st, org, mid, ok, 1<<20); err == nil {
		t.Fatal("expected error for non-image payload")
	}
}
