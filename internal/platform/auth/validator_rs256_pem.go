package auth

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type rs256PEMValidator struct {
	pub    *rsa.PublicKey
	issuer string
	aud    string
	leeway time.Duration
}

func newRS256PEMValidator(pemBytes []byte, issuer, audience string, leeway time.Duration) (*rs256PEMValidator, error) {
	pub, err := parseRSAPublicKeyPEM(pemBytes)
	if err != nil {
		return nil, err
	}
	if leeway <= 0 {
		leeway = DefaultClockLeeway
	}
	return &rs256PEMValidator{pub: pub, issuer: strings.TrimSpace(issuer), aud: strings.TrimSpace(audience), leeway: leeway}, nil
}

func parseRSAPublicKeyPEM(pemBytes []byte) (*rsa.PublicKey, error) {
	b, _ := pem.Decode(pemBytes)
	if b == nil {
		return nil, fmt.Errorf("auth: PEM decode failed for RSA public key")
	}
	switch b.Type {
	case "PUBLIC KEY":
		k, err := x509.ParsePKIXPublicKey(b.Bytes)
		if err != nil {
			return nil, err
		}
		pub, ok := k.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("auth: PKIX public key is not RSA")
		}
		return pub, nil
	case "RSA PUBLIC KEY":
		return x509.ParsePKCS1PublicKey(b.Bytes)
	default:
		return nil, fmt.Errorf("auth: unsupported PEM type %q (want PUBLIC KEY or RSA PUBLIC KEY)", b.Type)
	}
}

func (v *rs256PEMValidator) ValidateAccessToken(_ context.Context, raw string) (Principal, error) {
	opts := []jwt.ParserOption{jwt.WithLeeway(v.leeway), jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()})}
	if v.issuer != "" {
		opts = append(opts, jwt.WithIssuer(v.issuer))
	}
	if v.aud != "" {
		opts = append(opts, jwt.WithAudience(v.aud))
	}
	parser := jwt.NewParser(opts...)

	token, err := parser.ParseWithClaims(strings.TrimSpace(raw), jwt.MapClaims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok || t.Method.Alg() != jwt.SigningMethodRS256.Alg() {
			return nil, ErrUnauthenticated
		}
		return v.pub, nil
	})
	if err != nil || token == nil {
		return Principal{}, ErrUnauthenticated
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return Principal{}, ErrUnauthenticated
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	return PrincipalFromJWTPayloadJSON(payload, v.leeway)
}
