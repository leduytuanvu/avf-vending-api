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

func TestMediaAssetKeys_shape(t *testing.T) {
	t.Parallel()
	org := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	mid := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	orig := MediaAssetOriginalKey(org, mid)
	th := MediaAssetThumbWebpKey(org, mid)
	disp := MediaAssetDisplayWebpKey(org, mid)
	prefix := MediaAssetPrefix(org, mid)
	if !strings.HasPrefix(orig, prefix) || !strings.HasPrefix(th, prefix) || !strings.HasPrefix(disp, prefix) {
		t.Fatalf("prefix mismatch:\n%s\n%s\n%s\nprefix=%s", orig, th, disp, prefix)
	}
	if !strings.HasSuffix(th, "thumb.webp") || !strings.HasSuffix(disp, "display.webp") {
		t.Fatalf("unexpected variants: %s %s", th, disp)
	}
	if strings.HasSuffix(orig, ".webp") {
		t.Fatalf("original should not be forced to webp suffix: %s", orig)
	}
}

func TestProductMediaDisplayWebpKey_roundTrip(t *testing.T) {
	org := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	pid := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	dk := ProductMediaDisplayWebpKey(org, pid)
	tk := ProductMediaThumbWebpKey(org, pid)
	o2, p2, ok := ParseProductMediaDisplayKey(dk)
	if !ok || o2 != org || p2 != pid {
		t.Fatalf("parse display key: ok=%v org=%v pid=%v", ok, o2, p2)
	}
	if !strings.HasPrefix(dk, "org/") || !strings.HasSuffix(dk, "/display.webp") {
		t.Fatalf("unexpected display key: %s", dk)
	}
	if !strings.HasSuffix(tk, "/thumb.webp") {
		t.Fatalf("unexpected thumb key: %s", tk)
	}
}

func TestProductMediaObjectKey_deterministicAndSafe(t *testing.T) {
	t.Parallel()
	org := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	pid := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	mid := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	key := ProductMediaObjectKey(org, pid, mid, "sha256:ABC/../../DEF", "../display", "../webp")
	want := ProductMediaObjectKey(org, pid, mid, "sha256:ABC/../../DEF", "../display", "../webp")
	if key != want {
		t.Fatalf("key should be deterministic: %q != %q", key, want)
	}
	if strings.Contains(key, "..") || strings.Contains(key, "//") {
		t.Fatalf("unsafe key: %s", key)
	}
	if !strings.HasPrefix(key, "org/"+org.String()+"/products/"+pid.String()+"/media/"+mid.String()+"/") {
		t.Fatalf("missing scoped prefix: %s", key)
	}
}

func TestValidateCanonicalMediaAssetKey_acceptsCanonicalPathsOnly(t *testing.T) {
	t.Parallel()
	org := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	mid := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	orig := MediaAssetOriginalKey(org, mid)
	if err := ValidateCanonicalMediaAssetKey(org, mid, orig, "original"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCanonicalMediaAssetKey(org, mid, "../../../"+orig, "original"); err == nil {
		t.Fatal("expected rejection for traversal key")
	}
	if err := ValidateCanonicalMediaAssetKey(org, uuid.New(), orig, "original"); err == nil {
		t.Fatal("expected rejection when asset id mismatched")
	}
}
