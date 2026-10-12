package salecatalog

import (
	"testing"

	"github.com/avf/avf-vending-api/internal/app/setupapp"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestIfNoneMatchCatalogVersion_usesRuntimeFingerprintNotCfgLiteral(t *testing.T) {
	t.Parallel()
	cfg := int64(42)
	opts := Options{IfNoneMatchConfigVersion: &cfg, IncludeImages: true}
	bootstrap := setupapp.MachineBootstrap{}
	partial := Snapshot{ConfigVersion: cfg, Currency: "VND"}
	fp := RuntimeSaleCatalogFingerprint(bootstrap, partial, opts)
	require.NotEqual(t, "cfg:42", fp)
	require.NotEmpty(t, fp)
}

func TestIfNoneMatch_notCfgPrefix(t *testing.T) {
	t.Parallel()
	mid := uuid.New()
	cfg := int64(7)
	opts := Options{IfNoneMatchConfigVersion: &cfg}
	bootstrap := setupapp.MachineBootstrap{}
	partial := Snapshot{MachineID: mid, ConfigVersion: cfg, Currency: "VND"}
	snap := Snapshot{
		MachineID:      mid,
		ConfigVersion:  cfg,
		CatalogVersion: RuntimeSaleCatalogFingerprint(bootstrap, partial, opts),
		NotModified:    true,
	}
	require.False(t, snap.CatalogVersion == "cfg:7")
}
