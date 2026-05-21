package mediaadmin

import "testing"

func TestNormalizeProductMediaSourceType(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"upload":        "upload",
		"external":      "external",
		"import":        "import",
		"cloudinary":    "external",
		"uploaded_file": "external",
		"external_url":  "external",
		"":              "upload",
		"unknown":       "upload",
	}
	for in, want := range cases {
		if got := normalizeProductMediaSourceType(in); got != want {
			t.Fatalf("normalizeProductMediaSourceType(%q) = %q, want %q", in, got, want)
		}
	}
}
