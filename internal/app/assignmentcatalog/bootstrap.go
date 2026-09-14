package assignmentcatalog

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const bootstrapManifestName = "manifest.json"

// BootstrapManifestDTO matches the Android ProductBootstrapBundleManifestDto JSON shape.
type BootstrapManifestDTO struct {
	CatalogVersion int32  `json:"catalogVersion"`
	GeneratedAt    string `json:"generatedAt"`
	Variant        string `json:"variant"`
	ProductCount   int    `json:"productCount"`
	Products       []BootstrapProductDTO `json:"products"`
}

// BootstrapProductDTO matches Android ProductBootstrapBundleProductDto.
type BootstrapProductDTO struct {
	ID                   string  `json:"id"`
	SKU                  string  `json:"sku"`
	Barcode              *string `json:"barcode,omitempty"`
	Name                 string  `json:"name"`
	ShortName            *string `json:"shortName,omitempty"`
	Description          *string `json:"description,omitempty"`
	IsActive             bool    `json:"isActive"`
	BasePriceMinor       *int64  `json:"basePriceMinor,omitempty"`
	Currency             *string `json:"currency,omitempty"`
	ImageKey             *string `json:"imageKey,omitempty"`
	ImageHash            *string `json:"imageHash,omitempty"`
	ImageURL             *string `json:"imageUrl,omitempty"`
	ThumbURL             *string `json:"thumbUrl,omitempty"`
	DisplayURL           *string `json:"displayUrl,omitempty"`
	ImageContentRevision *int64  `json:"imageContentRevision,omitempty"`
	CatalogVersion       *int32  `json:"catalogVersion,omitempty"`
	ThumbFile            *string `json:"thumbFile,omitempty"`
}

// ManifestHeaderDTO matches Android ProductBootstrapManifestDto without bundle URLs.
type ManifestHeaderDTO struct {
	CatalogVersion  int32                        `json:"catalogVersion"`
	GeneratedAt     string                       `json:"generatedAt"`
	ProductCount    int                          `json:"productCount"`
	AssetCount      int                          `json:"assetCount"`
	BundleURL       string                       `json:"bundleUrl"`
	ThumbBundleURL  string                       `json:"thumbBundleUrl"`
	DisplayBundleURL string                      `json:"displayBundleUrl"`
	EstimatedBytes  BootstrapEstimatedBytesDTO   `json:"estimatedBytes"`
}

type BootstrapEstimatedBytesDTO struct {
	Thumb   int64 `json:"thumb"`
	Display int64 `json:"display"`
}

// DeltaDTO matches Android ProductDeltaResponseDto.
type DeltaDTO struct {
	FromCatalogVersion int32                 `json:"fromCatalogVersion"`
	ToCatalogVersion   int32                 `json:"toCatalogVersion"`
	GeneratedAt        string                `json:"generatedAt"`
	Upserts            []BootstrapProductDTO `json:"upserts"`
	DeletedProductIDs  []string              `json:"deletedProductIds"`
}

// BuildManifestHeader builds the REST bootstrap manifest header for a snapshot.
func BuildManifestHeader(baseURL string, snap *Snapshot) ManifestHeaderDTO {
	count := len(snap.Products)
	bundleURL := fmt.Sprintf("%s/admin/products/bootstrap-bundle?catalogVersion=%d&variant=thumb", trimSlash(baseURL), snap.CatalogVersion)
	return ManifestHeaderDTO{
		CatalogVersion:   snap.CatalogVersion,
		GeneratedAt:      snap.GeneratedAt.UTC().Format(time.RFC3339Nano),
		ProductCount:     count,
		AssetCount:       countWithImageKey(snap.Products),
		BundleURL:        bundleURL,
		ThumbBundleURL:   bundleURL,
		DisplayBundleURL: bundleURL,
		EstimatedBytes: BootstrapEstimatedBytesDTO{
			Thumb:   int64(count) * 32_768,
			Display: int64(count) * 131_072,
		},
	}
}

// BuildBootstrapBundleZip encodes assignment catalog products into the Android bootstrap zip format.
func BuildBootstrapBundleZip(snap *Snapshot, variant string) ([]byte, error) {
	if snap == nil {
		return nil, fmt.Errorf("assignmentcatalog: nil snapshot")
	}
	manifest := BootstrapManifestDTO{
		CatalogVersion: snap.CatalogVersion,
		GeneratedAt:    snap.GeneratedAt.UTC().Format(time.RFC3339Nano),
		Variant:        variant,
		ProductCount:   len(snap.Products),
		Products:       mapBootstrapProducts(snap),
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(nil)
	zw := zip.NewWriter(buf)
	w, err := zw.Create(bootstrapManifestName)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(manifestBytes); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// BuildDeltaDTO maps a delta to the REST response DTO.
func BuildDeltaDTO(delta *Delta) DeltaDTO {
	if delta == nil {
		return DeltaDTO{}
	}
	deleted := make([]string, 0, len(delta.DeletedProductIDs))
	for _, id := range delta.DeletedProductIDs {
		deleted = append(deleted, id.String())
	}
	return DeltaDTO{
		FromCatalogVersion: delta.FromCatalogVersion,
		ToCatalogVersion:   delta.ToCatalogVersion,
		GeneratedAt:        delta.GeneratedAt.UTC().Format(time.RFC3339Nano),
		Upserts:            mapBootstrapProductsFromList(delta.Upserts, delta.ToCatalogVersion),
		DeletedProductIDs:  deleted,
	}
}

func mapBootstrapProducts(snap *Snapshot) []BootstrapProductDTO {
	return mapBootstrapProductsFromList(snap.Products, snap.CatalogVersion)
}

func mapBootstrapProductsFromList(products []Product, catalogVersion int32) []BootstrapProductDTO {
	out := make([]BootstrapProductDTO, 0, len(products))
	for _, p := range products {
		dto := BootstrapProductDTO{
			ID:         p.ID.String(),
			SKU:        p.SKU,
			Name:       p.Name,
			IsActive:   p.Active,
			Currency:   strPtr(p.Currency),
			CatalogVersion: int32Ptr(catalogVersion),
		}
		if p.Barcode != "" {
			dto.Barcode = strPtr(p.Barcode)
		}
		if p.ShortName != "" {
			dto.ShortName = strPtr(p.ShortName)
		}
		if p.Description != "" {
			dto.Description = strPtr(p.Description)
		}
		if p.BasePriceMinor > 0 {
			dto.BasePriceMinor = int64Ptr(p.BasePriceMinor)
		}
		if p.ImageKey != "" {
			dto.ImageKey = strPtr(p.ImageKey)
		}
		if p.ImageHash != "" {
			dto.ImageHash = strPtr(p.ImageHash)
		}
		if p.ThumbURL != "" {
			dto.ThumbURL = strPtr(p.ThumbURL)
			dto.ImageURL = strPtr(p.ThumbURL)
		}
		if p.DisplayURL != "" {
			dto.DisplayURL = strPtr(p.DisplayURL)
		}
		if p.ImageContentRevision > 0 {
			dto.ImageContentRevision = int64Ptr(p.ImageContentRevision)
		}
		out = append(out, dto)
	}
	return out
}

func countWithImageKey(products []Product) int {
	n := 0
	for _, p := range products {
		if p.ImageKey != "" {
			n++
		}
	}
	return n
}

func trimSlash(s string) string {
	return stringsTrimRightSlash(stringsTrimSpace(s))
}

func stringsTrimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 {
		c := s[len(s)-1]
		if c != ' ' && c != '\t' {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

func stringsTrimRightSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func int32Ptr(v int32) *int32 { return &v }
func int64Ptr(v int64) *int64 { return &v }

// WriteBootstrapBundleResponse writes a zip bootstrap bundle HTTP response.
func WriteBootstrapBundleResponse(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"assignment-catalog-bootstrap.zip\"")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, bytes.NewReader(data))
}
