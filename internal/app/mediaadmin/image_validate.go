package mediaadmin

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/avf/avf-vending-api/internal/config"
	"golang.org/x/image/webp"
)

func validateProductImageFile(filename, contentType string, sizeBytes int64, peek []byte, cfg config.MediaUploadConfig) error {
	allowed := cfg.AllowedTypes
	if len(allowed) == 0 {
		allowed = []string{"image/jpeg", "image/png", "image/webp", "image/gif"}
	}
	maxMB := int(cfg.MaxBytes >> 20)
	if maxMB <= 0 {
		maxMB = 5
	}
	if sizeBytes <= 0 {
		return invalidImageFile("file is empty", allowed, maxMB)
	}
	if cfg.MaxBytes > 0 && sizeBytes > cfg.MaxBytes {
		return fileTooLarge(maxMB, allowed)
	}
	fn := strings.TrimSpace(filename)
	if fn == "" {
		return invalidImageFile("filename is required", allowed, maxMB)
	}
	ext := strings.ToLower(strings.TrimSpace(path.Ext(fn)))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
	default:
		return invalidImageFile("unsupported file extension", allowed, maxMB)
	}
	ct := normalizeMIMEHeader(contentType)
	if ct == "" {
		ct = inferProductImageMIME(fn)
	}
	if ct == "" {
		return invalidImageFile("content type is required", allowed, maxMB)
	}
	if !mimeAllowed(ct, allowed) {
		return invalidImageFile("unsupported content type", allowed, maxMB)
	}
	if len(peek) == 0 {
		return invalidImageFile("unable to read file header", allowed, maxMB)
	}
	detected := normalizeMIMEHeader(http.DetectContentType(peek))
	if detected != "" && !mimeAllowed(detected, allowed) {
		return invalidImageFile("file content does not match allowed image types", allowed, maxMB)
	}
	if err := decodeImageConfig(peek, ct); err != nil {
		return invalidImageFile(err.Error(), allowed, maxMB)
	}
	return nil
}

func inferProductImageMIME(name string) string {
	ext := strings.ToLower(strings.TrimSpace(path.Ext(name)))
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return inferMIMEFromFilename(name)
	}
}

func mimeAllowed(ct string, allowed []string) bool {
	ct = normalizeMIMEHeader(ct)
	for _, a := range allowed {
		if normalizeMIMEHeader(a) == ct {
			return true
		}
	}
	return false
}

func decodeImageConfig(peek []byte, expected string) error {
	exp := normalizeMIMEHeader(expected)
	if exp == "image/webp" {
		if _, err := webp.DecodeConfig(bytes.NewReader(peek)); err != nil {
			return fmt.Errorf("invalid webp image bytes")
		}
		return nil
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(peek))
	if err != nil {
		return fmt.Errorf("invalid image bytes")
	}
	switch strings.ToLower(format) {
	case "jpeg", "png", "gif":
	default:
		return fmt.Errorf("unsupported image format %q", format)
	}
	switch exp {
	case "image/jpeg":
		if format != "jpeg" {
			return fmt.Errorf("content type %s does not match detected %s", exp, format)
		}
	case "image/png":
		if format != "png" {
			return fmt.Errorf("content type %s does not match detected %s", exp, format)
		}
	case "image/gif":
		if format != "gif" {
			return fmt.Errorf("content type %s does not match detected %s", exp, format)
		}
	}
	_ = cfg
	return nil
}

func readImageHeader(r io.Reader, max int) ([]byte, error) {
	if max <= 0 {
		max = 512
	}
	buf := make([]byte, max)
	n, err := io.ReadAtLeast(r, buf, 1)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}
