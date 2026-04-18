package artifacts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/platform/objectstore"
	"github.com/google/uuid"
)

type stubStore struct {
	putMeta map[string]map[string]string
	putKeys []string
}

func (s *stubStore) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	return s.PutWithUserMetadata(ctx, key, body, size, contentType, nil)
}

func (s *stubStore) PutWithUserMetadata(ctx context.Context, key string, body io.Reader, size int64, contentType string, userMetadata map[string]string) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if int64(len(b)) != size {
		return fmt.Errorf("size mismatch got %d want %d", len(b), size)
	}
	if s.putMeta == nil {
		s.putMeta = make(map[string]map[string]string)
	}
	cp := map[string]string{}
	for k, v := range userMetadata {
		cp[k] = v
	}
	s.putMeta[key] = cp
	s.putKeys = append(s.putKeys, key)
	return nil
}

func (s *stubStore) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	return nil, "", fmt.Errorf("not implemented")
}

func (s *stubStore) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (objectstore.SignedHTTP, error) {
	return objectstore.SignedHTTP{}, fmt.Errorf("not implemented")
}

func (s *stubStore) PresignGet(ctx context.Context, key string, ttl time.Duration) (objectstore.SignedHTTP, error) {
	return objectstore.SignedHTTP{URL: "https://example/signed", Method: http.MethodGet}, nil
}

func (s *stubStore) Head(ctx context.Context, key string) (objectstore.ObjectMeta, error) {
	return objectstore.ObjectMeta{}, fmt.Errorf("nosuchkey")
}

func (s *stubStore) Delete(ctx context.Context, key string) error {
	return nil
}

func (s *stubStore) ListPrefix(ctx context.Context, prefix string, maxKeys int32) ([]objectstore.ObjectMeta, error) {
	return nil, nil
}

func TestPutContent_checksumOK(t *testing.T) {
	st := &stubStore{}
	svc := NewService(Deps{Store: st, MaxUploadBytes: 1 << 20})
	org := uuid.New()
	art := uuid.New()
	payload := []byte("hello-artifact-world")
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])
	err := svc.PutContent(context.Background(), org, art, bytes.NewReader(payload), int64(len(payload)), "application/octet-stream", hexSum, "f.bin")
	if err != nil {
		t.Fatal(err)
	}
	key := objectstore.BackendArtifactObjectKey(org, art)
	if len(st.putKeys) != 1 || st.putKeys[0] != key {
		t.Fatalf("puts: %#v", st.putKeys)
	}
	if st.putMeta[key][metaSHA256Hex] != hexSum {
		t.Fatalf("metadata sha256: %v", st.putMeta[key])
	}
}

func TestPutContent_checksumMismatchDeletes(t *testing.T) {
	st := &stubStore{}
	svc := NewService(Deps{Store: st, MaxUploadBytes: 1 << 20})
	org := uuid.New()
	art := uuid.New()
	payload := []byte("a")
	wrongHex := strings.Repeat("0", 64)
	err := svc.PutContent(context.Background(), org, art, bytes.NewReader(payload), int64(len(payload)), "application/octet-stream", wrongHex, "")
	if err != ErrChecksumMismatch {
		t.Fatalf("want ErrChecksumMismatch, got %v", err)
	}
}
