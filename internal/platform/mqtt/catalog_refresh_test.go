package mqtt

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestValidateCatalogRefreshDispatchPayload_OK(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"type":"catalog.refresh","catalogVersion":124,"mediaManifestVersion":46,"reason":"product_media_updated"}`)
	if err := ValidateCatalogRefreshDispatchPayload(CommandTypeCatalogRefresh, raw); err != nil {
		t.Fatal(err)
	}
	raw2 := []byte(`{"type":"catalog.refresh","catalogVersion":"sync-v1","mediaManifestVersion":"mediafp","reason":"product_media_updated"}`)
	if err := ValidateCatalogRefreshDispatchPayload(CommandTypeCatalogRefresh, raw2); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCatalogRefreshDispatchPayload_RejectsUnknownField(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"type":"catalog.refresh","catalogVersion":1,"mediaManifestVersion":2,"reason":"x","imageUrl":"https://x"}`)
	err := ValidateCatalogRefreshDispatchPayload(CommandTypeCatalogRefresh, raw)
	if err == nil || !errors.Is(err, ErrCatalogRefreshPayloadInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestValidateCatalogRefreshDispatchPayload_RejectsDataURL(t *testing.T) {
	t.Parallel()
	raw2 := []byte(`{"type":"catalog.refresh","catalogVersion":1,"mediaManifestVersion":2,"reason":"data:image/png;base64,AAA"}`)
	err := ValidateCatalogRefreshDispatchPayload(CommandTypeCatalogRefresh, raw2)
	if err == nil || !strings.Contains(err.Error(), "image transport") {
		t.Fatalf("got %v", err)
	}
}

func TestValidateCatalogRefreshAckPayload_OK(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"catalogVersion":124,"mediaManifestVersion":46,"mediaSynced":true}`)
	if err := ValidateCatalogRefreshAckPayload(CommandTypeCatalogRefresh, "acked", raw); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCatalogRefreshAckPayload_RequiresMediaSyncedTrue(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"catalogVersion":1,"mediaManifestVersion":2,"mediaSynced":false}`)
	err := ValidateCatalogRefreshAckPayload(CommandTypeCatalogRefresh, "acked", raw)
	if err == nil || !errors.Is(err, ErrCatalogRefreshPayloadInvalid) {
		t.Fatalf("got %v", err)
	}
}

func TestValidateCatalogRefreshAckPayload_IgnoresNonSuccessStatus(t *testing.T) {
	t.Parallel()
	raw := []byte(`{}`)
	if err := ValidateCatalogRefreshAckPayload(CommandTypeCatalogRefresh, "nacked", raw); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCatalogRefreshAckPayload_RejectsNestedJSON(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"catalogVersion":1,"mediaManifestVersion":2,"mediaSynced":true,"detail":{"x":1}}`)
	err := ValidateCatalogRefreshAckPayload(CommandTypeCatalogRefresh, "acked", raw)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMQTTVersionToken_stringFingerprint(t *testing.T) {
	t.Parallel()
	var m map[string]any
	_ = json.Unmarshal([]byte(`{"catalogVersion":"runtime_sale_catalog_sync_v1:abc","mediaManifestVersion":"media_catalog_v3:def"}`), &m)
	a, err := requireMQTTVersionToken(m, "catalogVersion")
	if err != nil || !strings.Contains(a, "runtime_sale") {
		t.Fatalf("got %q %v", a, err)
	}
}
