package layoutassignment

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// AssignmentFingerprint computes a deterministic SHA-256 over layout identity and slot tuples.
func AssignmentFingerprint(source string, layoutVersionID string, revision, rows, cols int32, slotParts []string) string {
	parts := []string{
		fmt.Sprintf("source:%s", strings.TrimSpace(source)),
		fmt.Sprintf("version:%s", strings.TrimSpace(layoutVersionID)),
		fmt.Sprintf("revision:%d", revision),
		fmt.Sprintf("rows:%d", rows),
		fmt.Sprintf("cols:%d", cols),
	}
	parts = append(parts, slotParts...)
	sort.Strings(parts)
	h := sha256.New()
	h.Write([]byte("layout_assignment:"))
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// DeriveSyncStatus compares desired and reported layout state.
func DeriveSyncStatus(desiredSource, reportedSource, desiredFP, reportedFP *string, applyFailure *string) string {
	if applyFailure != nil && strings.TrimSpace(*applyFailure) != "" {
		return SyncStatusApplyFailed
	}
	if desiredSource == nil || reportedSource == nil {
		return SyncStatusOfflineUnknown
	}
	if strings.TrimSpace(*desiredSource) != strings.TrimSpace(*reportedSource) {
		return SyncStatusDrift
	}
	if desiredFP != nil && reportedFP != nil && strings.TrimSpace(*desiredFP) != strings.TrimSpace(*reportedFP) {
		return SyncStatusDrift
	}
	if desiredFP != nil && reportedFP != nil && strings.TrimSpace(*desiredFP) == strings.TrimSpace(*reportedFP) {
		return SyncStatusInSync
	}
	return SyncStatusPending
}
