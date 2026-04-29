package auth_test

import (
	"bytes"
	"strings"
	"testing"

	plauth "github.com/avf/avf-vending-api/internal/platform/auth"
)

func TestEncryptMFASecretRoundTrip(t *testing.T) {
	t.Parallel()
	key := bytes.Repeat([]byte("k"), 32)
	plain := []byte(strings.Repeat("s", 20))
	ct, err := plauth.EncryptMFASecret(key, plain)
	if err != nil {
		t.Fatal(err)
	}
	out, err := plauth.DecryptMFASecret(key, ct)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(plain) {
		t.Fatalf("plain mismatch")
	}
}
