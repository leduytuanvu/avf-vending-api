package catalogadmin

import "testing"

func TestValidateProductImageMIME(t *testing.T) {
	if err := ValidateProductImageMIME("image/webp"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProductImageMIME("IMAGE/JPEG"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProductImageMIME("application/octet-stream"); err == nil {
		t.Fatal("expected error")
	}
}
