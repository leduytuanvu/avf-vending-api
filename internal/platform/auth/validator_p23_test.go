package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestEd25519PEMValidator_AcceptsEdDSA(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pkix, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pkix})

	v, err := newEd25519PEMValidator(pemBytes, "", nil, DefaultClockLeeway)
	if err != nil {
		t.Fatal(err)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"sub": "user:test-subject",
		"exp": time.Now().Add(10 * time.Minute).Unix(),
	})
	raw, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	p, err := v.ValidateAccessToken(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.Subject != "user:test-subject" {
		t.Fatalf("subject: %q", p.Subject)
	}
}

func TestJWTJWKSCachedValidator_InvalidKidRejected(t *testing.T) {
	privRSA, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	n := base64.RawURLEncoding.EncodeToString(privRSA.N.Bytes())
	eb := big.NewInt(int64(privRSA.E)).Bytes()
	e := base64.RawURLEncoding.EncodeToString(eb)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []any{map[string]any{
				"kty": "RSA",
				"kid": "good",
				"use": "sig",
				"n":   n,
				"e":   e,
			}},
		})
	}))
	defer srv.Close()

	v := newJWTJWKSCachedValidator(srv.URL, time.Minute, "", nil, DefaultClockLeeway)
	if err := v.WarmJWKS(context.Background()); err != nil {
		t.Fatal(err)
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "u:1",
		"exp": time.Now().Add(10 * time.Minute).Unix(),
	})
	tok.Header["kid"] = "bad"
	raw, err := tok.SignedString(privRSA)
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.ValidateAccessToken(context.Background(), raw)
	if err != ErrUnauthenticated {
		t.Fatalf("want ErrUnauthenticated, got %v", err)
	}
}

func TestMaybeWarmJWKS_ChainedValidator(t *testing.T) {
	privRSA, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	n := base64.RawURLEncoding.EncodeToString(privRSA.N.Bytes())
	eb := big.NewInt(int64(privRSA.E)).Bytes()
	e := base64.RawURLEncoding.EncodeToString(eb)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []any{map[string]any{
				"kty": "RSA",
				"kid": "k1",
				"use": "sig",
				"n":   n,
				"e":   e,
			}},
		})
	}))
	defer srv.Close()

	jwks := newJWTJWKSCachedValidator(srv.URL, time.Minute, "", nil, DefaultClockLeeway)
	secondary := newHS256Validator([]byte("secondary-secret-not-used-here"), nil, DefaultClockLeeway)
	ch := ChainAccessTokenValidators(jwks, secondary)
	if err := MaybeWarmJWKS(context.Background(), ch); err != nil {
		t.Fatal(err)
	}
}
