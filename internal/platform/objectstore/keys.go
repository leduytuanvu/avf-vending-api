package objectstore

import (
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
)

// ValidateCanonicalMediaAssetKey rejects keys that are not the exact deterministic path for the
// company + media asset (prevents path traversal / cross-company reads when a caller supplies a key).
func ValidateCanonicalMediaAssetKey(companyID, mediaAssetID uuid.UUID, actualKey, role string) error {
	want := strings.Trim(strings.TrimSpace(actualKey), "/")
	var exp string
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "original":
		exp = strings.Trim(MediaAssetOriginalKey(companyID, mediaAssetID), "/")
	case "thumb", "thumb.webp":
		exp = strings.Trim(MediaAssetThumbWebpKey(companyID, mediaAssetID), "/")
	case "display", "display.webp":
		exp = strings.Trim(MediaAssetDisplayWebpKey(companyID, mediaAssetID), "/")
	default:
		return fmt.Errorf("objectstore: invalid media asset key role %q", role)
	}
	if want != exp {
		return fmt.Errorf("objectstore: key does not match canonical media path")
	}
	return nil
}

func joinS3Key(parts ...string) string {
	return strings.Join(parts, "/")
}

// BackendArtifactOrgPrefix is the list prefix for all backend-managed artifacts in an company.
func BackendArtifactOrgPrefix(companyID uuid.UUID) string {
	return joinS3Key("artifacts", "backend", companyID.String()) + "/"
}

// BackendArtifactObjectKey returns the canonical object key for a single backend artifact payload.
// One object per artifact_id (OTA-ready: campaigns reference stable artifact UUIDs without inventing a second key space).
func BackendArtifactObjectKey(companyID, artifactID uuid.UUID) string {
	return joinS3Key("artifacts", "backend", companyID.String(), artifactID.String(), "payload")
}

// ParseBackendArtifactKey extracts company and artifact UUIDs from a full object key.
// Returns false if the key is not exactly the canonical backend artifact payload path.
func ParseBackendArtifactKey(fullKey string) (scopeID uuid.UUID, artifactID uuid.UUID, ok bool) {
	parts := strings.Split(strings.Trim(fullKey, "/"), "/")
	if len(parts) != 5 {
		return uuid.Nil, uuid.Nil, false
	}
	if parts[0] != "artifacts" || parts[1] != "backend" || parts[4] != "payload" {
		return uuid.Nil, uuid.Nil, false
	}
	org, err := uuid.Parse(parts[2])
	if err != nil {
		return uuid.Nil, uuid.Nil, false
	}
	art, err := uuid.Parse(parts[3])
	if err != nil {
		return uuid.Nil, uuid.Nil, false
	}
	return org, art, true
}

// OTAObjectKey returns the object key for an OTA artifact blob.
func OTAObjectKey(companyID, artifactID uuid.UUID, filename string) string {
	fn := strings.TrimSpace(path.Base(filename))
	if fn == "" || fn == "." {
		fn = "artifact.bin"
	}
	return joinS3Key("ota", companyID.String(), artifactID.String(), fn)
}

// ProductMediaDisplayWebpKey is the canonical display image object key for catalog production deployments.
// Format: org/{companyID}/products/{productID}/display.webp
func ProductMediaDisplayWebpKey(companyID, productID uuid.UUID) string {
	return joinS3Key("org", companyID.String(), "products", productID.String(), "display.webp")
}

// ProductMediaThumbWebpKey is the canonical thumbnail object key paired with ProductMediaDisplayWebpKey.
func ProductMediaThumbWebpKey(companyID, productID uuid.UUID) string {
	return joinS3Key("org", companyID.String(), "products", productID.String(), "thumb.webp")
}

// ParseProductMediaDisplayKey extracts company and product UUIDs from a deterministic display.webp key.
func ParseProductMediaDisplayKey(fullKey string) (companyID, productID uuid.UUID, ok bool) {
	parts := strings.Split(strings.Trim(fullKey, "/"), "/")
	if len(parts) != 5 {
		return uuid.Nil, uuid.Nil, false
	}
	if parts[0] != "org" || parts[2] != "products" || parts[4] != "display.webp" {
		return uuid.Nil, uuid.Nil, false
	}
	org, err := uuid.Parse(parts[1])
	if err != nil {
		return uuid.Nil, uuid.Nil, false
	}
	pid, err := uuid.Parse(parts[3])
	if err != nil {
		return uuid.Nil, uuid.Nil, false
	}
	return org, pid, true
}

// MediaAssetPrefix is the list prefix for one logical media asset (original + variants).
func MediaAssetPrefix(companyID, mediaAssetID uuid.UUID) string {
	return joinS3Key("media", companyID.String(), mediaAssetID.String()) + "/"
}

// MediaAssetOriginalKey is the uploaded source object (any image MIME; not necessarily WebP).
func MediaAssetOriginalKey(companyID, mediaAssetID uuid.UUID) string {
	return joinS3Key("media", companyID.String(), mediaAssetID.String(), "original")
}

// MediaAssetThumbWebpKey is the canonical thumbnail variant (WebP target once processing is wired).
func MediaAssetThumbWebpKey(companyID, mediaAssetID uuid.UUID) string {
	return joinS3Key("media", companyID.String(), mediaAssetID.String(), "thumb.webp")
}

// MediaAssetDisplayWebpKey is the canonical display variant (WebP target once processing is wired).
func MediaAssetDisplayWebpKey(companyID, mediaAssetID uuid.UUID) string {
	return joinS3Key("media", companyID.String(), mediaAssetID.String(), "display.webp")
}

// ProductMediaObjectKey returns a deterministic, role-scoped key for a product-bound media object.
// The hash is normalized to hex and included so app cache keys change when content changes.
func ProductMediaObjectKey(companyID, productID, mediaID uuid.UUID, contentHash, variant, extension string) string {
	v := sanitizeKeyPart(variant, "original")
	ext := sanitizeKeyPart(strings.TrimPrefix(extension, "."), "bin")
	hash := strings.ToLower(strings.TrimSpace(contentHash))
	hash = strings.TrimPrefix(hash, "sha256:")
	hash = sanitizeKeyPart(hash, "unhashed")
	return joinS3Key("org", companyID.String(), "products", productID.String(), "media", mediaID.String(), hash, v+"."+ext)
}

func sanitizeKeyPart(raw, fallback string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	if s == "" {
		return fallback
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), ".-_/")
	if out == "" || out == "." {
		return fallback
	}
	return out
}

// DiagnosticBundleKey returns the object key for a machine diagnostic / log bundle.
func DiagnosticBundleKey(companyID, machineID uuid.UUID, bundleID string) string {
	b := strings.TrimSpace(bundleID)
	if b == "" {
		b = "bundle"
	}
	return joinS3Key("diagnostics", companyID.String(), machineID.String(), fmt.Sprintf("%s.tgz", b))
}
