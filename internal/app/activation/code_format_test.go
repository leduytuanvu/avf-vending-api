package activation

import (
	"regexp"
	"testing"
)

func TestRandomActivationCodeFormat(t *testing.T) {
	t.Parallel()
	re := regexp.MustCompile(`^[0-9]{6}$`)
	for i := 0; i < 100; i++ {
		code, err := randomActivationCode()
		if err != nil {
			t.Fatalf("randomActivationCode: %v", err)
		}
		if len(code) != ActivationCodeLength {
			t.Fatalf("len=%d want %d code=%q", len(code), ActivationCodeLength, code)
		}
		if !re.MatchString(code) {
			t.Fatalf("code %q does not match ^[0-9]{6}$", code)
		}
	}
}

func TestRandomActivationCodeAllowsLeadingZerosIfGeneratedOrMocked(t *testing.T) {
	t.Parallel()
	// Leading zeros are valid when the numeric value is small.
	if !validActivationCode("000001") {
		t.Fatal("000001 should be valid")
	}
	if !validActivationCode("000000") {
		t.Fatal("000000 should be valid")
	}
}

func TestNormalizeActivationCodeTrimOnly(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, want string
	}{
		{"342209", "342209"},
		{" 342209 ", "342209"},
		{"AVF-123456", "AVF-123456"},
		{"123 456", "123 456"},
	}
	for _, tc := range cases {
		if got := normalizeActivationCode(tc.in); got != tc.want {
			t.Fatalf("normalizeActivationCode(%q) = %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestValidActivationCode(t *testing.T) {
	t.Parallel()
	valid := []string{"342209", "000000", "000001", "999999", " 342209 "}
	for _, s := range valid {
		if !validActivationCode(s) {
			t.Fatalf("%q should be valid", s)
		}
	}
	invalid := []string{
		"12345", "1234567", "123 456", "123-456", "AVF123", "AVF-123456",
		"abcdef", "", "１２３４５６",
	}
	for _, s := range invalid {
		if validActivationCode(s) {
			t.Fatalf("%q should be invalid", s)
		}
	}
}

func TestHashActivationCodeUsesStrictSixDigitString(t *testing.T) {
	t.Parallel()
	pepper := []byte("test-pepper")
	a := hashActivationCode(pepper, "342209")
	b := hashActivationCode(pepper, " 342209 ")
	if string(a) != string(b) {
		t.Fatal("hash should use trimmed 6-digit string consistently")
	}
	upper := hashActivationCode(pepper, "000001")
	if len(upper) == 0 {
		t.Fatal("hash should not be empty")
	}
	legacy := hashActivationCode(pepper, "AVF-123456")
	if string(a) == string(legacy) {
		t.Fatal("6-digit hash must differ from legacy AVF-style hash")
	}
}
