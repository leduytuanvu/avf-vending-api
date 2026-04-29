package mediaadmin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/avf/avf-vending-api/internal/platform/objectstore"
	"github.com/deepteams/webp"
	"github.com/google/uuid"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const webpContentType = "image/webp"

// metaSHA256Hex is the S3 user-metadata key (x-amz-meta-sha256hex) used for integrity on PUT.
const metaSHA256Hex = "sha256hex"

// VariantArtifacts captures stored object metadata after thumb/display generation.
type VariantArtifacts struct {
	ThumbSHA256Hex   string
	DisplaySHA256Hex string
	ThumbWidth       int
	ThumbHeight      int
	DisplayWidth     int
	DisplayHeight    int
	ThumbBytes       int64
	DisplayBytes     int64
}

// VariantGenerator produces thumb.webp and display.webp objects from the uploaded original.
type VariantGenerator interface {
	GenerateWebPVariants(ctx context.Context, store objectstore.Store, organizationID, mediaAssetID uuid.UUID, originalObjectKey string, maxOriginalBytes int64) (VariantArtifacts, error)
}

// WebPVariantGenerator resizes and encodes lossy WebP variants bounded by configured max edge lengths.
type WebPVariantGenerator struct {
	ThumbMax   int
	DisplayMax int
}

func (g WebPVariantGenerator) limits() (thumbMax, displayMax int) {
	thumbMax, displayMax = g.ThumbMax, g.DisplayMax
	if thumbMax <= 0 {
		thumbMax = 256
	}
	if displayMax <= 0 {
		displayMax = 1024
	}
	return thumbMax, displayMax
}

func (g WebPVariantGenerator) GenerateWebPVariants(ctx context.Context, store objectstore.Store, organizationID, mediaAssetID uuid.UUID, originalObjectKey string, maxOriginalBytes int64) (VariantArtifacts, error) {
	var out VariantArtifacts
	if store == nil {
		return out, fmt.Errorf("variant: nil store")
	}
	if err := objectstore.ValidateCanonicalMediaAssetKey(organizationID, mediaAssetID, originalObjectKey, "original"); err != nil {
		return out, fmt.Errorf("variant: %w", err)
	}
	if maxOriginalBytes <= 0 {
		maxOriginalBytes = 10 << 20
	}
	rc, _, err := store.Get(ctx, originalObjectKey)
	if err != nil {
		return out, err
	}
	defer rc.Close()
	raw, err := io.ReadAll(io.LimitReader(rc, maxOriginalBytes+1))
	if err != nil {
		return out, err
	}
	if int64(len(raw)) > maxOriginalBytes {
		return out, fmt.Errorf("variant: original exceeds max bytes")
	}
	if err := validateSniffedImageMIME(raw); err != nil {
		return out, err
	}
	if len(raw) == 0 {
		return out, fmt.Errorf("variant: empty original")
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return out, fmt.Errorf("variant: decode original: %w", err)
	}
	bounds := src.Bounds()
	ow, oh := bounds.Dx(), bounds.Dy()
	if ow < 1 || oh < 1 {
		return out, fmt.Errorf("variant: invalid image dimensions")
	}
	maxThumb, maxDisp := g.limits()
	tw, th := fitWithin(ow, oh, maxThumb, maxThumb)
	dw, dh := fitWithin(ow, oh, maxDisp, maxDisp)

	timg := resizeRGBA(src, tw, th)
	dimg := resizeRGBA(src, dw, dh)

	var tbuf, dbuf bytes.Buffer
	if err := webp.Encode(&tbuf, timg, &webp.EncoderOptions{Lossless: false, Quality: 82}); err != nil {
		return out, fmt.Errorf("variant: encode thumb webp: %w", err)
	}
	if err := webp.Encode(&dbuf, dimg, &webp.EncoderOptions{Lossless: false, Quality: 85}); err != nil {
		return out, fmt.Errorf("variant: encode display webp: %w", err)
	}

	tk := objectstore.MediaAssetThumbWebpKey(organizationID, mediaAssetID)
	dk := objectstore.MediaAssetDisplayWebpKey(organizationID, mediaAssetID)
	thash := sha256.Sum256(tbuf.Bytes())
	dhash := sha256.Sum256(dbuf.Bytes())
	out.ThumbSHA256Hex = hex.EncodeToString(thash[:])
	out.DisplaySHA256Hex = hex.EncodeToString(dhash[:])
	out.ThumbWidth, out.ThumbHeight = tw, th
	out.DisplayWidth, out.DisplayHeight = dw, dh
	out.ThumbBytes = int64(tbuf.Len())
	out.DisplayBytes = int64(dbuf.Len())

	tmeta := map[string]string{metaSHA256Hex: out.ThumbSHA256Hex}
	dmeta := map[string]string{metaSHA256Hex: out.DisplaySHA256Hex}
	if err := store.PutWithUserMetadata(ctx, tk, bytes.NewReader(tbuf.Bytes()), out.ThumbBytes, webpContentType, tmeta); err != nil {
		return out, err
	}
	if err := store.PutWithUserMetadata(ctx, dk, bytes.NewReader(dbuf.Bytes()), out.DisplayBytes, webpContentType, dmeta); err != nil {
		_ = store.Delete(ctx, tk)
		return out, err
	}
	return out, nil
}

func fitWithin(w, h, maxW, maxH int) (int, int) {
	if maxW <= 0 {
		maxW = w
	}
	if maxH <= 0 {
		maxH = h
	}
	if w <= maxW && h <= maxH {
		return w, h
	}
	rw := float64(maxW) / float64(w)
	rh := float64(maxH) / float64(h)
	r := math.Min(rw, rh)
	nw := int(math.Round(float64(w) * r))
	nh := int(math.Round(float64(h) * r))
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	return nw, nh
}

func resizeRGBA(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	return dst
}

// PassthroughVariantGenerator stores thumb/display using the original bytes and MIME type (tests / fallback).
type PassthroughVariantGenerator struct{}

func (PassthroughVariantGenerator) GenerateWebPVariants(ctx context.Context, store objectstore.Store, organizationID, mediaAssetID uuid.UUID, originalObjectKey string, maxOriginalBytes int64) (VariantArtifacts, error) {
	var out VariantArtifacts
	if store == nil {
		return out, fmt.Errorf("variant: nil store")
	}
	if err := objectstore.ValidateCanonicalMediaAssetKey(organizationID, mediaAssetID, originalObjectKey, "original"); err != nil {
		return out, fmt.Errorf("variant: %w", err)
	}
	if maxOriginalBytes <= 0 {
		maxOriginalBytes = 10 << 20
	}
	rc, contentType, err := store.Get(ctx, originalObjectKey)
	if err != nil {
		return out, err
	}
	defer rc.Close()
	body, err := io.ReadAll(io.LimitReader(rc, maxOriginalBytes+1))
	if err != nil {
		return out, err
	}
	if int64(len(body)) > maxOriginalBytes {
		return out, fmt.Errorf("variant: original exceeds max bytes")
	}
	if err := validateSniffedImageMIME(body); err != nil {
		return out, err
	}
	if len(body) == 0 {
		return out, fmt.Errorf("variant: empty original")
	}
	if src, _, derr := image.Decode(bytes.NewReader(body)); derr == nil {
		b := src.Bounds()
		out.DisplayWidth = b.Dx()
		out.DisplayHeight = b.Dy()
		out.ThumbWidth = out.DisplayWidth
		out.ThumbHeight = out.DisplayHeight
	}
	tk := objectstore.MediaAssetThumbWebpKey(organizationID, mediaAssetID)
	dk := objectstore.MediaAssetDisplayWebpKey(organizationID, mediaAssetID)
	ct := strings.TrimSpace(strings.ToLower(contentType))
	if ct == "" {
		ct = "application/octet-stream"
	}
	h := sha256.Sum256(body)
	hexh := hex.EncodeToString(h[:])
	meta := map[string]string{metaSHA256Hex: hexh}
	out.ThumbSHA256Hex = hexh
	out.DisplaySHA256Hex = hexh
	out.ThumbBytes = int64(len(body))
	out.DisplayBytes = int64(len(body))
	r1 := bytes.NewReader(body)
	if err := store.PutWithUserMetadata(ctx, tk, r1, int64(len(body)), ct, meta); err != nil {
		return out, err
	}
	r2 := bytes.NewReader(body)
	if err := store.PutWithUserMetadata(ctx, dk, r2, int64(len(body)), ct, meta); err != nil {
		_ = store.Delete(ctx, tk)
		return out, err
	}
	return out, nil
}

func normalizeSHA256Hex(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimPrefix(s, "sha256:")
	return s
}

// validateSniffedImageMIME enforces jpeg/png/webp only (aligned with upload Init presign MIME allowlist).
func validateSniffedImageMIME(data []byte) error {
	if len(data) < 12 {
		return fmt.Errorf("variant: truncated image payload")
	}
	sniff := strings.ToLower(strings.TrimSpace(http.DetectContentType(data)))
	if i := strings.IndexByte(sniff, ';'); i >= 0 {
		sniff = strings.TrimSpace(sniff[:i])
	}
	switch sniff {
	case "image/jpeg", "image/png", "image/webp":
		return nil
	default:
		return fmt.Errorf("variant: disallowed payload content (expected image/jpeg, image/png, or image/webp, got %q)", sniff)
	}
}
