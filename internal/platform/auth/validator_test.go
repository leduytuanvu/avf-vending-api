package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/avf/avf-vending-api/internal/config"
	"github.com/golang-jwt/jwt/v5"
)

func TestHS256Validator_previousSecretRotation(t *testing.T) {
	primary := []byte("primary-secret-32bytes-minimum!!")
	previous := []byte("old-secret-before-rotation-32b")
	v := newHS256Validator(primary, previous, time.Minute)

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "rot-user",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	raw, err := tok.SignedString(previous)
	if err != nil {
		t.Fatal(err)
	}
	p, err := v.ValidateAccessToken(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.Subject != "rot-user" {
		t.Fatalf("subject %q", p.Subject)
	}
}

func TestHS256Validator_rejectsBadSignature(t *testing.T) {
	secret := []byte("primary-secret-32bytes-minimum!!")
	v := newHS256Validator(secret, nil, time.Minute)

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "u",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	raw, err := tok.SignedString([]byte("other-secret-32bytes-minimum!!"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.ValidateAccessToken(context.Background(), raw)
	if err != ErrUnauthenticated {
		t.Fatalf("want ErrUnauthenticated, got %v", err)
	}
}

func TestNewAccessTokenValidator_rs256PEM_roundTrip(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	cfg := config.HTTPAuthConfig{
		Mode:            "rs256_pem",
		JWTLeeway:       time.Minute,
		RSAPublicKeyPEM: pubPEM,
	}
	v, err := NewAccessTokenValidator(cfg)
	if err != nil {
		t.Fatal(err)
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "rsa-user",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	raw, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	p, err := v.ValidateAccessToken(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}
	if p.Subject != "rsa-user" {
		t.Fatalf("subject %q", p.Subject)
	}
}

func TestNewAccessTokenValidator_unknownMode(t *testing.T) {
	_, err := NewAccessTokenValidator(config.HTTPAuthConfig{Mode: "eddsa"})
	if err == nil {
		t.Fatal("expected error")
	}
}
