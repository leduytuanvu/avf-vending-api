package mediaadmin

import (
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/stretchr/testify/require"
)

var minimalPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

func defaultImageUploadCfg() config.MediaUploadConfig {
	return config.MediaUploadConfig{
		MaxBytes:     5 << 20,
		AllowedTypes: []string{"image/jpeg", "image/png", "image/webp", "image/gif"},
	}
}

func TestValidateProductImageFile_acceptsPNGWithExplicitMIME(t *testing.T) {
	cfg := defaultImageUploadCfg()
	ct, err := validateProductImageFile("test.png", "image/png", int64(len(minimalPNG)), minimalPNG, cfg)
	require.NoError(t, err)
	require.Equal(t, "image/png", ct)
}

func TestValidateProductImageFile_acceptsPNGWithOctetStream(t *testing.T) {
	cfg := defaultImageUploadCfg()
	ct, err := validateProductImageFile("test.png", "application/octet-stream", int64(len(minimalPNG)), minimalPNG, cfg)
	require.NoError(t, err)
	require.Equal(t, "image/png", ct)
}

func TestValidateProductImageFile_acceptsPNGWithMissingPartContentType(t *testing.T) {
	cfg := defaultImageUploadCfg()
	ct, err := validateProductImageFile("test.png", "", int64(len(minimalPNG)), minimalPNG, cfg)
	require.NoError(t, err)
	require.Equal(t, "image/png", ct)
}

func TestValidateProductImageFile_rejectsTextRenamedPNG(t *testing.T) {
	cfg := defaultImageUploadCfg()
	text := []byte("this is not an image file")
	_, err := validateProductImageFile("fake.png", "application/octet-stream", int64(len(text)), text, cfg)
	require.Error(t, err)
	inv, ok := AsInvalidImageFile(err)
	require.True(t, ok)
	require.False(t, inv.TooLarge)
	require.Contains(t, strings.ToLower(inv.Message), "unsupported")
}

func TestValidateProductImageFile_rejectsOversize(t *testing.T) {
	cfg := config.MediaUploadConfig{MaxBytes: 10, AllowedTypes: []string{"image/png", "image/jpeg"}}
	_, err := validateProductImageFile("big.png", "image/png", 11, minimalPNG[:8], cfg)
	require.Error(t, err)
	inv, ok := AsInvalidImageFile(err)
	require.True(t, ok)
	require.True(t, inv.TooLarge)
}

func TestValidateProductImageFile_rejectsSVG(t *testing.T) {
	cfg := defaultImageUploadCfg()
	_, err := validateProductImageFile("evil.svg", "image/svg+xml", 128, []byte(`<svg`), cfg)
	require.Error(t, err)
	_, ok := AsInvalidImageFile(err)
	require.True(t, ok)
}
