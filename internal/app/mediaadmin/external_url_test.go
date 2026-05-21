package mediaadmin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestValidateExternalProductImageURL_acceptsAllowlistedHTTPS(t *testing.T) {
	cfg := config.ExternalProductImageConfig{
		Enabled:      true,
		AllowedHosts: []string{"adm.avf.vn"},
		RequireHTTPS: true,
	}
	u, err := validateExternalProductImageURL("https://adm.avf.vn/storage/photos/1/Product/69f0e277129d9.png", cfg)
	require.NoError(t, err)
	require.Equal(t, "adm.avf.vn", u.Hostname())
}

func TestValidateExternalProductImageURL_rejectsHTTPWhenRequired(t *testing.T) {
	cfg := config.ExternalProductImageConfig{
		Enabled:      true,
		AllowedHosts: []string{"adm.avf.vn"},
		RequireHTTPS: true,
	}
	_, err := validateExternalProductImageURL("http://adm.avf.vn/x.png", cfg)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)
}

func TestValidateExternalProductImageURL_rejectsPrivateAndLocalhost(t *testing.T) {
	cfg := config.ExternalProductImageConfig{
		Enabled:      true,
		AllowedHosts: []string{"adm.avf.vn", "127.0.0.1"},
		RequireHTTPS: false,
	}
	for _, raw := range []string{
		"http://127.0.0.1/x.png",
		"http://localhost/x.png",
		"http://169.254.169.254/latest/meta-data",
		"http://10.0.0.1/x.png",
		"file:///etc/passwd",
		"data:image/png;base64,abc",
	} {
		_, err := validateExternalProductImageURL(raw, cfg)
		require.Error(t, err, raw)
	}
}

func TestValidateExternalProductImageURL_rejectsNonAllowlistedHost(t *testing.T) {
	cfg := config.ExternalProductImageConfig{
		Enabled:      true,
		AllowedHosts: []string{"adm.avf.vn"},
		RequireHTTPS: true,
	}
	_, err := validateExternalProductImageURL("https://evil.example.com/x.png", cfg)
	require.Error(t, err)
}

func TestExternalImageCacheKey_isDeterministic(t *testing.T) {
	url := "https://adm.avf.vn/storage/photos/1/Product/69f0e277129d9.png"
	a := ExternalImageCacheKey(url, 1)
	b := ExternalImageCacheKey(url, 1)
	require.Equal(t, a, b)
	require.True(t, strings.HasPrefix(a, "external-image:"))
	require.True(t, strings.HasSuffix(a, ":v1"))
}

func TestProbeExternalImageHEAD_acceptsImageServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	host := strings.TrimPrefix(strings.TrimPrefix(srv.URL, "https://"), "http://")
	host = strings.Split(host, "/")[0]
	cfg := config.ExternalProductImageConfig{
		Enabled:       true,
		AllowedHosts:  []string{strings.Split(host, ":")[0]},
		RequireHTTPS:  false,
		MaxBytes:      5 << 20,
		RemoteTimeout: 0,
	}
	require.NoError(t, probeExternalImageHEAD(t.Context(), srv.URL+"/test.png", "image/png", cfg))
}
