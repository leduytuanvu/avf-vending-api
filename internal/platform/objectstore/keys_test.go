package objectstore

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestBackendArtifactObjectKey_roundTrip(t *testing.T) {
	org := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	art := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	key := BackendArtifactObjectKey(org, art)
	o2, a2, ok := ParseBackendArtifactKey(key)
	if !ok {
		t.Fatal("parse failed")
	}
	if o2 != org || a2 != art {
		t.Fatalf("got org=%v art=%v", o2, a2)
	}
	if !strings.HasPrefix(key, "artifacts/backend/") || !strings.HasSuffix(key, "/payload") {
		t.Fatalf("unexpected key: %s", key)
	}
}

func TestParseBackendArtifactKey_rejectsTraversal(t *testing.T) {
	_, _, ok := ParseBackendArtifactKey("artifacts/backend/../../../etc/passwd/payload")
	if ok {
		t.Fatal("expected reject")
	}
}
