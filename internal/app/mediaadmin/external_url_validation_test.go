package mediaadmin

import (
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/stretchr/testify/require"
)

func baseExternalCfg() config.ExternalProductImageConfig {
	return config.ExternalProductImageConfig{
		Enabled:      true,
		AllowedHosts: []string{"adm.avf.vn"},
		RequireHTTPS: true,
		MaxBytes:     5 << 20,
	}
}

func TestValidateExternalProductImageURL_acceptsProductionURLs(t *testing.T) {
	cfg := baseExternalCfg()
	for _, raw := range []string{
		"https://adm.avf.vn/storage/photos/1/Product/69f0e277129d9.png",
		"https://adm.avf.vn/storage/photos/1/Product/69f5c789105c0.png",
		"https://adm.avf.vn/storage/photos/1/Product/68833a13a45e5.png",
	} {
		u, err := validateExternalProductImageURL(raw, cfg)
		require.NoError(t, err, raw)
		require.Equal(t, "adm.avf.vn", u.Hostname())
		require.Equal(t, "https", u.Scheme)
	}
}

func TestValidateExternalProductImageURL_acceptsImageMIMETypes(t *testing.T) {
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp"} {
		require.NoError(t, validateExternalFilenameExtension("image"+ext))
	}
	for _, ct := range []string{"image/png", "image/jpeg", "image/webp"} {
		require.NoError(t, validateRasterUploadMIME(ct))
	}
}

func TestValidateExternalProductImageURL_infersFilenameAndMIME(t *testing.T) {
	require.Equal(t, "image/png", inferMIMEFromFilename("69f0e277129d9.png"))
	require.Equal(t, "image/jpeg", inferMIMEFromFilename("photo.jpg"))
	require.Equal(t, "image/webp", inferMIMEFromFilename("x.webp"))
	require.Equal(t, "", inferMIMEFromFilename("noext"))
}

func TestValidateExternalProductImageURL_rejectsSSRFTargets(t *testing.T) {
	cfg := baseExternalCfg()
	fail := []string{
		"http://adm.avf.vn/x.png",
		"https://evil.com/image.png",
		"https://localhost/image.png",
		"https://127.0.0.1/image.png",
		"https://10.0.0.1/image.png",
		"https://172.16.0.1/image.png",
		"https://192.168.1.1/image.png",
		"https://169.254.169.254/latest/meta-data",
		"file:///etc/passwd",
		"data:image/png;base64,abc",
		"ftp://adm.avf.vn/image.png",
	}
	for _, raw := range fail {
		_, err := validateExternalProductImageURL(raw, cfg)
		require.Error(t, err, raw)
		require.ErrorIs(t, err, ErrInvalidArgument, raw)
	}
}

func TestValidateExternalProductImageURL_rejectsBadExtensionAndMIME(t *testing.T) {
	require.Error(t, validateExternalFilenameExtension("file.txt"))
	require.Error(t, validateExternalFilenameExtension(""))
	require.Error(t, validateRasterUploadMIME("text/html"))
}

func TestExternalImageCacheKey_versionAndStability(t *testing.T) {
	url := "https://adm.avf.vn/storage/photos/1/Product/69f0e277129d9.png"
	k1 := ExternalImageCacheKey(url, 1)
	k2 := ExternalImageCacheKey(url, 2)
	require.NotEqual(t, k1, k2)
	require.True(t, strings.HasSuffix(k1, ":v1"))
	require.Equal(t, k1, ExternalImageCacheKey(url, 1))
}

func TestNewService_externalOnlyWithoutObjectStore(t *testing.T) {
	// Pool nil should fail; external-only wiring is validated at HTTP app layer.
	_, err := NewService(Deps{
		External: config.ExternalProductImageConfig{Enabled: true, AllowedHosts: []string{"adm.avf.vn"}},
	})
	require.ErrorIs(t, err, ErrNotConfigured)
}

func TestRegisterExternalProductImage_featureDisabled(t *testing.T) {
	svc := &Service{
		pool:     nil,
		external: config.ExternalProductImageConfig{Enabled: false},
	}
	_, err := svc.RegisterExternalProductImage(t.Context(), RegisterExternalProductImageInput{
		URL: "https://adm.avf.vn/x.png",
	})
	require.ErrorIs(t, err, ErrNotConfigured)
}

func TestService_UploadConfigured_vs_ExternalConfigured(t *testing.T) {
	extOnly := &Service{
		external: config.ExternalProductImageConfig{Enabled: true},
	}
	require.True(t, extOnly.ExternalConfigured())
	require.False(t, extOnly.UploadConfigured())
}
