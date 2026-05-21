package config

import (
	"os"
	"testing"

	"github.com/google/uuid"
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

func TestLoadMediaUploadConfig_fullCredentialsConfigured(t *testing.T) {
	t.Setenv("MEDIA_PROVIDER", "cloudinary")
	t.Setenv("MEDIA_UPLOAD_ENABLED", "true")
	t.Setenv("CLOUDINARY_CLOUD_NAME", "demo")
	t.Setenv("CLOUDINARY_API_KEY", "key123")
	t.Setenv("CLOUDINARY_API_SECRET", "secret456")
	t.Setenv("CLOUDINARY_FOLDER", "avf-vending/products")
	cfg := loadMediaUploadConfig()
	require.True(t, cfg.CloudinaryConfigured())
	require.Equal(t, "avf-vending/products", cfg.Cloudinary.Folder)
}

func TestLoadMediaUploadConfig_placeholderCredentialsNotConfigured(t *testing.T) {
	t.Setenv("MEDIA_PROVIDER", "cloudinary")
	t.Setenv("MEDIA_UPLOAD_ENABLED", "true")
	t.Setenv("CLOUDINARY_CLOUD_NAME", "your-cloudinary-cloud-name")
	t.Setenv("CLOUDINARY_API_KEY", "your-cloudinary-api-key")
	t.Setenv("CLOUDINARY_API_SECRET", "your-cloudinary-api-secret")
	cfg := loadMediaUploadConfig()
	require.False(t, cfg.CloudinaryConfigured())
}

func TestParseMediaCompanyIDEnv_validUUID(t *testing.T) {
	t.Setenv("MEDIA_COMPANY_ID", "0194a1b2-c3d4-7890-abcd-ef1234567890")
	t.Setenv("MEDIA_SCOPE_ID", "")
	id, err := parseMediaCompanyIDEnv()
	require.NoError(t, err)
	require.Equal(t, uuid.MustParse("0194a1b2-c3d4-7890-abcd-ef1234567890"), id)
}

func TestParseMediaCompanyIDEnv_rejectsNilUUID(t *testing.T) {
	t.Setenv("MEDIA_COMPANY_ID", "00000000-0000-0000-0000-000000000000")
	_, err := parseMediaCompanyIDEnv()
	require.Error(t, err)
	require.Contains(t, err.Error(), "nil UUID")
}

func TestParseMediaCompanyIDEnv_priorityOverScopeID(t *testing.T) {
	t.Setenv("MEDIA_COMPANY_ID", "0194a1b2-c3d4-7890-abcd-ef1234567890")
	t.Setenv("MEDIA_SCOPE_ID", "11111111-1111-1111-1111-111111111111")
	id, err := parseMediaCompanyIDEnv()
	require.NoError(t, err)
	require.Equal(t, uuid.MustParse("0194a1b2-c3d4-7890-abcd-ef1234567890"), id)
}

func TestParseMediaCompanyIDEnv_deprecatedScopeAlias(t *testing.T) {
	t.Setenv("MEDIA_COMPANY_ID", "")
	t.Setenv("MEDIA_SCOPE_ID", "0194a1b2-c3d4-7890-abcd-ef1234567890")
	id, err := parseMediaCompanyIDEnv()
	require.NoError(t, err)
	require.Equal(t, uuid.MustParse("0194a1b2-c3d4-7890-abcd-ef1234567890"), id)
}

func TestMediaUploadConfig_validate_productionRequiresCompanyID(t *testing.T) {
	cfg := MediaUploadConfig{
		Enabled:  true,
		Provider: "cloudinary",
		Cloudinary: CloudinaryConfig{
			CloudName: "demo",
			APIKey:    "key",
			APISecret: "secret",
		},
	}
	t.Setenv("MEDIA_COMPANY_ID", "")
	t.Setenv("MEDIA_SCOPE_ID", "")
	require.Error(t, cfg.validate(AppEnvProduction))
}

func TestMediaUploadConfig_validate_productionWithCompanyID(t *testing.T) {
	cfg := MediaUploadConfig{
		Enabled:  true,
		Provider: "cloudinary",
		Cloudinary: CloudinaryConfig{
			CloudName: "demo",
			APIKey:    "key",
			APISecret: "secret",
		},
	}
	t.Setenv("MEDIA_COMPANY_ID", "0194a1b2-c3d4-7890-abcd-ef1234567890")
	require.NoError(t, cfg.validate(AppEnvProduction))
}
