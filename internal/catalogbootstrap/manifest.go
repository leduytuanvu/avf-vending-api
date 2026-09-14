package catalogbootstrap

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Manifest is the normalized import input.
type Manifest struct {
	Taxonomy Taxonomy
	Products []ProductRecord
}

type Taxonomy struct {
	Categories []TaxonCategory `json:"categories"`
	Brands     []TaxonBrand    `json:"brands"`
	Tags       []TaxonTag      `json:"tags"`
}

type TaxonCategory struct {
	Slug       string  `json:"slug"`
	Name       string  `json:"name"`
	ParentSlug *string `json:"parent_slug"`
	Active     bool    `json:"active"`
}

type TaxonBrand struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

type TaxonTag struct {
	Slug   string `json:"slug"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// ProductRecord is one SKU import row.
type ProductRecord struct {
	SKU             string
	Name            string
	Description     string
	Active          bool
	CategorySlug    string
	BrandSlug       *string
	TagSlugs        []string
	PriceVND        int64
	SourceImageURL  string
	CloudinaryPubID string
	SourceSHA256    string
	SourceMIME      string
	ImageStatus     string
}

type rawFinalManifest struct {
	Taxonomy Taxonomy          `json:"taxonomy"`
	Products []rawFinalProduct `json:"products"`
}

type rawFinalProduct struct {
	SKU         string `json:"sku"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Active      bool   `json:"active"`
	Category    struct {
		FinalDecision string `json:"final_decision"`
		Slug          string `json:"slug"`
	} `json:"category"`
	Brand struct {
		FinalDecision *string `json:"final_decision"`
		Slug          *string `json:"slug"`
		Approved      bool    `json:"approved"`
	} `json:"brand"`
	Tags struct {
		FinalList []string `json:"final_list"`
	} `json:"tags"`
	Pricing struct {
		UnitPriceMinor int64 `json:"unit_price_minor"`
		SourceVND      int64 `json:"source_vnd"`
	} `json:"pricing"`
	SourceImage struct {
		URL              string `json:"url"`
		SHA256           string `json:"sha256"`
		MIME             string `json:"mime"`
		ValidationStatus string `json:"validation_status"`
	} `json:"source_image"`
	Cloudinary struct {
		PlannedPublicID string `json:"planned_public_id"`
	} `json:"cloudinary"`
}

type rawEnrichedManifest struct {
	Taxonomy Taxonomy             `json:"taxonomy"`
	Products []rawEnrichedProduct `json:"products"`
}

type rawEnrichedProduct struct {
	SKU            string   `json:"sku"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Active         bool     `json:"active"`
	CategorySlug   string   `json:"category_slug"`
	BrandSlug      *string  `json:"brand_slug"`
	TagSlugs       []string `json:"tag_slugs"`
	PriceVND       int64    `json:"price_vnd"`
	SourceImageURL string   `json:"source_image_url"`
	Cloudinary     struct {
		PublicID string `json:"public_id"`
	} `json:"cloudinary"`
}

// LoadManifest reads final or enriched manifest JSON.
func LoadManifest(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var probe struct {
		Products []json.RawMessage `json:"products"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if len(probe.Products) == 0 {
		return nil, fmt.Errorf("manifest has no products")
	}
	var sample map[string]json.RawMessage
	if err := json.Unmarshal(probe.Products[0], &sample); err != nil {
		return nil, err
	}
	if _, ok := sample["category"]; ok {
		return loadFinalManifest(b)
	}
	return loadEnrichedManifest(b)
}

func loadFinalManifest(b []byte) (*Manifest, error) {
	var raw rawFinalManifest
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	out := &Manifest{Taxonomy: raw.Taxonomy}
	for _, p := range raw.Products {
		cat := strings.TrimSpace(p.Category.FinalDecision)
		if cat == "" {
			cat = strings.TrimSpace(p.Category.Slug)
		}
		var brand *string
		if p.Brand.FinalDecision != nil && strings.TrimSpace(*p.Brand.FinalDecision) != "" {
			s := strings.TrimSpace(*p.Brand.FinalDecision)
			brand = &s
		}
		pub := strings.TrimSpace(p.Cloudinary.PlannedPublicID)
		if pub == "" {
			pub = "import-" + strings.TrimSpace(p.SKU)
		}
		price := p.Pricing.UnitPriceMinor
		if price == 0 {
			price = p.Pricing.SourceVND
		}
		out.Products = append(out.Products, ProductRecord{
			SKU:             strings.TrimSpace(p.SKU),
			Name:            strings.TrimSpace(p.Name),
			Description:     strings.TrimSpace(p.Description),
			Active:          p.Active,
			CategorySlug:    cat,
			BrandSlug:       brand,
			TagSlugs:        p.Tags.FinalList,
			PriceVND:        price,
			SourceImageURL:  strings.TrimSpace(p.SourceImage.URL),
			CloudinaryPubID: pub,
			SourceSHA256:    strings.TrimSpace(p.SourceImage.SHA256),
			SourceMIME:      strings.TrimSpace(p.SourceImage.MIME),
			ImageStatus:     strings.TrimSpace(p.SourceImage.ValidationStatus),
		})
	}
	return out, nil
}

func loadEnrichedManifest(b []byte) (*Manifest, error) {
	var raw rawEnrichedManifest
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	out := &Manifest{Taxonomy: raw.Taxonomy}
	for _, p := range raw.Products {
		pub := strings.TrimSpace(p.Cloudinary.PublicID)
		if pub == "" {
			pub = "import-" + strings.TrimSpace(p.SKU)
		}
		out.Products = append(out.Products, ProductRecord{
			SKU:             strings.TrimSpace(p.SKU),
			Name:            strings.TrimSpace(p.Name),
			Description:     strings.TrimSpace(p.Description),
			Active:          p.Active,
			CategorySlug:    strings.TrimSpace(p.CategorySlug),
			BrandSlug:       p.BrandSlug,
			TagSlugs:        p.TagSlugs,
			PriceVND:        p.PriceVND,
			SourceImageURL:  strings.TrimSpace(p.SourceImageURL),
			CloudinaryPubID: pub,
			ImageStatus:     "PENDING",
		})
	}
	return out, nil
}

// ValidateManifest checks referential integrity before import.
func ValidateManifest(m *Manifest) []string {
	var issues []string
	if m == nil {
		return []string{"nil manifest"}
	}
	catSlugs := map[string]bool{}
	for _, c := range m.Taxonomy.Categories {
		catSlugs[strings.ToLower(c.Slug)] = true
	}
	tagSlugs := map[string]bool{}
	for _, t := range m.Taxonomy.Tags {
		tagSlugs[strings.ToLower(t.Slug)] = true
	}
	brandSlugs := map[string]bool{}
	for _, b := range m.Taxonomy.Brands {
		brandSlugs[strings.ToLower(b.Slug)] = true
	}
	seenSKU := map[string]bool{}
	seenPub := map[string]string{}
	for _, p := range m.Products {
		if p.SKU == "" {
			issues = append(issues, "product missing sku")
			continue
		}
		if seenSKU[p.SKU] {
			issues = append(issues, "duplicate sku "+p.SKU)
		}
		seenSKU[p.SKU] = true
		if p.CategorySlug == "" || !catSlugs[strings.ToLower(p.CategorySlug)] {
			issues = append(issues, fmt.Sprintf("sku %s invalid category %q", p.SKU, p.CategorySlug))
		}
		if p.BrandSlug != nil && *p.BrandSlug != "" && !brandSlugs[strings.ToLower(*p.BrandSlug)] {
			issues = append(issues, fmt.Sprintf("sku %s brand %q not in taxonomy", p.SKU, *p.BrandSlug))
		}
		for _, ts := range p.TagSlugs {
			if !tagSlugs[strings.ToLower(ts)] {
				issues = append(issues, fmt.Sprintf("sku %s unknown tag %q", p.SKU, ts))
			}
		}
		if p.SourceImageURL == "" {
			issues = append(issues, "sku "+p.SKU+" missing source_image_url")
		}
		if p.ImageStatus != "" && p.ImageStatus != "OK" && p.ImageStatus != "PENDING" {
			issues = append(issues, fmt.Sprintf("sku %s image status %s", p.SKU, p.ImageStatus))
		}
		if p.PriceVND <= 0 {
			issues = append(issues, "sku "+p.SKU+" invalid price")
		}
		if prev, ok := seenPub[p.CloudinaryPubID]; ok {
			issues = append(issues, fmt.Sprintf("duplicate cloudinary public_id %s (%s,%s)", p.CloudinaryPubID, prev, p.SKU))
		}
		seenPub[p.CloudinaryPubID] = p.SKU
	}
	return issues
}
