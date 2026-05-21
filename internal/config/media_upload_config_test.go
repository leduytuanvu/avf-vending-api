package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadMediaUploadConfig_cloudinary(t *testing.T) {
	t.Setenv("MEDIA_PROVIDER", "cloudinary")
	t.Setenv("MEDIA_UPLOAD_ENABLED", "true")
	t.Setenv("CLOUDINARY_CLOUD_NAME", "demo")
	t.Setenv("MEDIA_MAX_IMAGE_SIZE_MB", "5")
	cfg := loadMediaUploadConfig()
	require.True(t, cfg.Enabled)
	require.Equal(t, "cloudinary", cfg.Provider)
	require.Equal(t, "demo", cfg.Cloudinary.CloudName)
	require.Equal(t, int64(5<<20), cfg.MaxBytes)
}

func TestLoadMediaUploadConfig_missingSecretNotConfigured(t *testing.T) {
	_ = os.Unsetenv("CLOUDINARY_API_SECRET")
	t.Setenv("MEDIA_PROVIDER", "cloudinary")
	t.Setenv("MEDIA_UPLOAD_ENABLED", "true")
	t.Setenv("CLOUDINARY_CLOUD_NAME", "demo")
	t.Setenv("CLOUDINARY_API_KEY", "key")
	cfg := loadMediaUploadConfig()
	require.False(t, cfg.CloudinaryConfigured())
}
