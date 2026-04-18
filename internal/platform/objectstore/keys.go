package objectstore

import (
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
)

func joinS3Key(parts ...string) string {
	return strings.Join(parts, "/")
}

// BackendArtifactOrgPrefix is the list prefix for all backend-managed artifacts in an organization.
func BackendArtifactOrgPrefix(organizationID uuid.UUID) string {
	return joinS3Key("artifacts", "backend", organizationID.String()) + "/"
}

// BackendArtifactObjectKey returns the canonical object key for a single backend artifact payload.
// One object per artifact_id (OTA-ready: campaigns reference stable artifact UUIDs without inventing a second key space).
func BackendArtifactObjectKey(organizationID, artifactID uuid.UUID) string {
	return joinS3Key("artifacts", "backend", organizationID.String(), artifactID.String(), "payload")
}

// ParseBackendArtifactKey extracts organization and artifact UUIDs from a full object key.
// Returns false if the key is not exactly the canonical backend artifact payload path.
func ParseBackendArtifactKey(fullKey string) (orgID uuid.UUID, artifactID uuid.UUID, ok bool) {
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
func OTAObjectKey(organizationID, artifactID uuid.UUID, filename string) string {
	fn := strings.TrimSpace(path.Base(filename))
	if fn == "" || fn == "." {
		fn = "artifact.bin"
	}
	return joinS3Key("ota", organizationID.String(), artifactID.String(), fn)
}

// DiagnosticBundleKey returns the object key for a machine diagnostic / log bundle.
func DiagnosticBundleKey(organizationID, machineID uuid.UUID, bundleID string) string {
	b := strings.TrimSpace(bundleID)
	if b == "" {
		b = "bundle"
	}
	return joinS3Key("diagnostics", organizationID.String(), machineID.String(), fmt.Sprintf("%s.tgz", b))
}
