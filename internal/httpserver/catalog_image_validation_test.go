package httpserver

import "testing"

func TestValidateProductImageBindInput_ok(t *testing.T) {
	t.Parallel()
	err := validateProductImageBindInput(
		"https://cdn.example.com/a.webp",
		"https://cdn.example.com/b.webp",
		"sha256:"+repeatHex(64),
		"image/webp",
	)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateProductImageBindInput_displayNotHTTPS(t *testing.T) {
	t.Parallel()
	err := validateProductImageBindInput("http://evil.example/x", "", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateProductImageBindInput_badHash(t *testing.T) {
	t.Parallel()
	err := validateProductImageBindInput("https://a.example/x", "", "not-hex", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateProductImageBindInput_badMime(t *testing.T) {
	t.Parallel()
	err := validateProductImageBindInput("https://a.example/x", "", "", "application/octet-stream")
	if err == nil {
		t.Fatal("expected error")
	}
}

func repeatHex(n int) string {
	const hx = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hx[i%16]
	}
	return string(b)
}
