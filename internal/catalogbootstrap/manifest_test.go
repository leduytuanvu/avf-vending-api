package catalogbootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadManifest_enriched(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", "..", "docs", "avf_products_enriched_cloudinary_import_manifest.json")
	if _, err := os.Stat(path); err != nil {
		t.Skip("enriched manifest not present")
	}
	m, err := LoadManifest(path)
	require.NoError(t, err)
	require.Len(t, m.Products, 117)
	issues := ValidateManifest(m)
	require.Empty(t, issues)
}

func TestLoadManifest_final(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "..", "..", "docs", "catalog-import-final.json")
	if _, err := os.Stat(path); err != nil {
		t.Skip("final manifest not present")
	}
	m, err := LoadManifest(path)
	require.NoError(t, err)
	require.Len(t, m.Products, 117)
	issues := ValidateManifest(m)
	require.Empty(t, issues)
}

func TestValidateManifest_duplicateSKU(t *testing.T) {
	t.Parallel()
	m := &Manifest{
		Taxonomy: Taxonomy{
			Categories: []TaxonCategory{{Slug: "beverage", Name: "Bev"}},
			Brands:     []TaxonBrand{{Slug: "coca-cola", Name: "Coca"}},
			Tags:       []TaxonTag{{Slug: "beverage", Name: "Bev"}},
		},
		Products: []ProductRecord{
			{SKU: "1", Name: "A", CategorySlug: "beverage", TagSlugs: []string{"beverage"}, PriceVND: 1000, SourceImageURL: "https://example.com/a.png", CloudinaryPubID: "import-1"},
			{SKU: "1", Name: "B", CategorySlug: "beverage", TagSlugs: []string{"beverage"}, PriceVND: 1000, SourceImageURL: "https://example.com/b.png", CloudinaryPubID: "import-2"},
		},
	}
	issues := ValidateManifest(m)
	require.NotEmpty(t, issues)
}
