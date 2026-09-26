package textutil

import "testing"

func TestSanitizeProtoString_keepsValidUtf8(t *testing.T) {
	if SanitizeProtoString("catalog-v1") != "catalog-v1" {
		t.Fatalf("expected valid utf8 to be unchanged")
	}
}

func TestSanitizeProtoString_dropsInvalidUtf8(t *testing.T) {
	invalid := "cat\x80alog"
	got := SanitizeProtoString(invalid)
	if got != "catalog" {
		t.Fatalf("expected invalid bytes to be stripped, got %q", got)
	}
}
