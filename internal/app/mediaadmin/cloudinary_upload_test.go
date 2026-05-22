package mediaadmin

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	platformcloudinary "github.com/avf/avf-vending-api/internal/platform/cloudinary"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCloudinaryImageCacheKey_format(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	key := CloudinaryImageCacheKey(id, 2, "sha256:abc123")
	require.Equal(t, "11111111-1111-1111-1111-111111111111:2:abc123", key)
}

func TestMediaUploadConfig_CloudinaryConfigured(t *testing.T) {
	cfg := config.MediaUploadConfig{
		Enabled:  true,
		Provider: "cloudinary",
		Cloudinary: config.CloudinaryConfig{
			CloudName: "demo",
			APIKey:    "key",
			APISecret: "secret",
		},
	}
	require.True(t, cfg.CloudinaryConfigured())
	require.False(t, config.MediaUploadConfig{Enabled: true, Provider: "cloudinary"}.CloudinaryConfigured())
}

type fakeCloudinaryUploader struct {
	result platformcloudinary.UploadResult
	err    error
}

func (f *fakeCloudinaryUploader) UploadProductImage(_ context.Context, _ platformcloudinary.UploadInput) (platformcloudinary.UploadResult, error) {
	if f.err != nil {
		return platformcloudinary.UploadResult{}, f.err
	}
	return f.result, nil
}

func TestService_CloudinaryConfigured_requiresUploader(t *testing.T) {
	svc := &Service{
		uploadCfg: config.MediaUploadConfig{
			Enabled:  true,
			Provider: "cloudinary",
			Cloudinary: config.CloudinaryConfig{
				CloudName: "demo",
				APIKey:    "key",
				APISecret: "secret",
			},
		},
		cloudinary: &fakeCloudinaryUploader{},
	}
	require.True(t, svc.CloudinaryConfigured())
}

func TestNewService_cloudinaryConfigWithoutPoolNotConfigured(t *testing.T) {
	_, err := NewService(Deps{
		Upload: config.MediaUploadConfig{
			Enabled:  true,
			Provider: "cloudinary",
			Cloudinary: config.CloudinaryConfig{
				CloudName: "demo",
				APIKey:    "key",
				APISecret: "secret",
			},
		},
		Cloudinary: &fakeCloudinaryUploader{},
	})
	require.ErrorIs(t, err, ErrNotConfigured)
}
