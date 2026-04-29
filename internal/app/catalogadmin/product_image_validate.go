package catalogadmin

import (
	"fmt"
	"strings"
)

var allowedProductImageMIMEs = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
	// image/gif permitted for legacy external URL binds; enterprise upload pipeline (mediaadmin InitUpload) accepts only jpeg/png/webp.
	"image/gif": {},
}

func normalizeProductImageMIME(mt string) string {
	return strings.TrimSpace(strings.ToLower(mt))
}

// ValidateProductImageMIME returns nil when mt is an allowed image/* MIME type.
func ValidateProductImageMIME(mt string) error {
	m := normalizeProductImageMIME(mt)
	if m == "" {
		return fmt.Errorf("mimeType is required: %w", ErrInvalidArgument)
	}
	if _, ok := allowedProductImageMIMEs[m]; !ok {
		return fmt.Errorf("mimeType must be one of image/jpeg, image/png, image/webp, image/gif: %w", ErrInvalidArgument)
	}
	return nil
}

func normalizeArtifactContentType(ct string) string {
	s := strings.TrimSpace(strings.ToLower(ct))
	if i := strings.IndexByte(s, ';'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}
