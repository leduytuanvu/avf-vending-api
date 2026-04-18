package httpserver

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/app/api"
	"github.com/avf/avf-vending-api/internal/app/artifacts"
	"github.com/avf/avf-vending-api/internal/platform/auth"
	"github.com/avf/avf-vending-api/internal/platform/objectstore"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func noopWriteRL(next http.Handler) http.Handler { return next }

func TestArtifactOrgAllowed(t *testing.T) {
	org := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	pOrg := auth.Principal{
		OrganizationID: org,
		Roles:          []string{auth.RoleOrgAdmin},
	}
	if !artifactOrgAllowed(pOrg, org) {
		t.Fatal("org admin same org")
	}
	if artifactOrgAllowed(pOrg, uuid.New()) {
		t.Fatal("org admin other org")
	}
	pPlat := auth.Principal{Roles: []string{auth.RolePlatformAdmin}}
	if !artifactOrgAllowed(pPlat, uuid.New()) {
		t.Fatal("platform admin any org")
	}
}

func TestMountArtifactRoutes_smokeReserve(t *testing.T) {
	stub := stubArtifactStore{}
	svc := artifacts.NewService(artifacts.Deps{Store: stub, MaxUploadBytes: 1024, DownloadPresignTTL: time.Minute, ListMaxKeys: 10})
	app := &api.HTTPApplication{Artifacts: svc}
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			org := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
			ctx := auth.WithPrincipal(r.Context(), auth.Principal{
				OrganizationID: org,
				Roles:          []string{auth.RoleOrgAdmin},
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	mountArtifactAdminRoutes(r, app, noopWriteRL)

	req := httptest.NewRequest(http.MethodPost, "/organizations/dddddddd-dddd-dddd-dddd-dddddddddddd/artifacts", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}

type stubArtifactStore struct{}

func (stubArtifactStore) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	return nil
}
func (stubArtifactStore) PutWithUserMetadata(ctx context.Context, key string, body io.Reader, size int64, contentType string, userMetadata map[string]string) error {
	return nil
}
func (stubArtifactStore) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	return nil, "", nil
}
func (stubArtifactStore) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (objectstore.SignedHTTP, error) {
	return objectstore.SignedHTTP{}, nil
}
func (stubArtifactStore) PresignGet(ctx context.Context, key string, ttl time.Duration) (objectstore.SignedHTTP, error) {
	return objectstore.SignedHTTP{}, nil
}
func (stubArtifactStore) Head(ctx context.Context, key string) (objectstore.ObjectMeta, error) {
	return objectstore.ObjectMeta{}, nil
}
func (stubArtifactStore) Delete(ctx context.Context, key string) error { return nil }
func (stubArtifactStore) ListPrefix(ctx context.Context, prefix string, maxKeys int32) ([]objectstore.ObjectMeta, error) {
	return nil, nil
}
