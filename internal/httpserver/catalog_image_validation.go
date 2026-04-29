package httpserver

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

var allowedProductImageMimeTypes = map[string]struct{}{
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
	"image/gif":  {},
}

func validateHTTPSArtifactURL(label, raw string) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		return fmt.Errorf("%s is required", label)
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return fmt.Errorf("%s must be a URL with a host", label)
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return nil
	case "http":
		h := strings.ToLower(strings.TrimSpace(u.Hostname()))
		if h == "localhost" || h == "127.0.0.1" || h == "::1" {
			return nil
		}
	}
	return fmt.Errorf("%s must be https or http://localhost / http://127.0.0.1", label)
}

func validateProductImageBindInput(displayURL, thumbURL, contentHash, mimeType string) error {
	if err := validateHTTPSArtifactURL("displayUrl", displayURL); err != nil {
		return err
	}
	if t := strings.TrimSpace(thumbURL); t != "" {
		if err := validateHTTPSArtifactURL("thumbUrl", t); err != nil {
			return err
		}
	}
	ch := strings.TrimSpace(contentHash)
	if ch != "" {
		ch = strings.TrimPrefix(strings.ToLower(ch), "sha256:")
		ch = strings.TrimSpace(ch)
		if len(ch) != 64 {
			return fmt.Errorf("contentHash must be 64 hex chars (optionally prefixed sha256:)")
		}
		if _, err := hex.DecodeString(ch); err != nil {
			return fmt.Errorf("contentHash must be hexadecimal")
		}
	}
	if mt := strings.TrimSpace(mimeType); mt != "" {
		if _, ok := allowedProductImageMimeTypes[strings.ToLower(mt)]; !ok {
			return fmt.Errorf("mimeType must be one of image/jpeg, image/png, image/webp, image/gif")
		}
	}
	return nil
}
