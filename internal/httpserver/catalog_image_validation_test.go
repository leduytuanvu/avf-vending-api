package httpserver

import "testing"

func Test_validateHTTPSArtifactURL_localhostHTTP(t *testing.T) {
	if err := validateHTTPSArtifactURL("displayUrl", "http://127.0.0.1:9000/x"); err != nil {
		t.Fatal(err)
	}
	if err := validateHTTPSArtifactURL("displayUrl", "http://localhost/foo"); err != nil {
		t.Fatal(err)
	}
	if err := validateHTTPSArtifactURL("displayUrl", "http://evil.example.com/x"); err == nil {
		t.Fatal("expected reject non-localhost http")
	}
}

func Test_validateProductImageBindInput_invalidMIME(t *testing.T) {
	err := validateProductImageBindInput("https://cdn.example.com/a.jpg", "", "", "image/x-unknown")
	if err == nil {
		t.Fatal("expected error")
	}
}
