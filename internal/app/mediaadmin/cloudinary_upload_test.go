package mediaadmin

import (
	"context"
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	platformcloudinary "github.com/avf/avf-vending-api/internal/platform/cloudinary"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestValidateProductImageFile_acceptsPNG(t *testing.T) {
	// Minimal valid 1x1 PNG
	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
		0x42, 0x60, 0x82,
	}
	cfg := config.MediaUploadConfig{MaxBytes: 5 << 20, AllowedTypes: []string{"image/png"}}
	err := validateProductImageFile("test.png", "image/png", int64(len(png)), png, cfg)
	require.NoError(t, err)
}

func TestValidateProductImageFile_rejectsSVG(t *testing.T) {
	cfg := config.MediaUploadConfig{MaxBytes: 5 << 20, AllowedTypes: []string{"image/png", "image/jpeg"}}
	err := validateProductImageFile("evil.svg", "image/svg+xml", 128, []byte(`<svg`), cfg)
	require.Error(t, err)
	_, ok := AsInvalidImageFile(err)
	require.True(t, ok)
}

func TestValidateProductImageFile_rejectsOversize(t *testing.T) {
	cfg := config.MediaUploadConfig{MaxBytes: 10, AllowedTypes: []string{"image/png"}}
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	err := validateProductImageFile("big.png", "image/png", 11, png, cfg)
	require.Error(t, err)
	inv, ok := AsInvalidImageFile(err)
	require.True(t, ok)
	require.True(t, inv.TooLarge)
}

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
