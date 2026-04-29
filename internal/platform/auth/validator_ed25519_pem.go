package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type ed25519PEMValidator struct {
	pub       ed25519.PublicKey
	issuer    string
	audiences []string
	leeway    time.Duration
}

func newEd25519PEMValidator(pemBytes []byte, issuer string, audiences []string, leeway time.Duration) (*ed25519PEMValidator, error) {
	pub, err := parseEd25519PublicKeyPEM(pemBytes)
	if err != nil {
		return nil, err
	}
	if leeway <= 0 {
		leeway = DefaultClockLeeway
	}
	var auds []string
	for _, a := range audiences {
		if s := strings.TrimSpace(a); s != "" {
			auds = append(auds, s)
		}
	}
	return &ed25519PEMValidator{pub: pub, issuer: strings.TrimSpace(issuer), audiences: auds, leeway: leeway}, nil
}

func parseEd25519PublicKeyPEM(pemBytes []byte) (ed25519.PublicKey, error) {
	b, _ := pem.Decode(pemBytes)
	if b == nil {
		return nil, fmt.Errorf("auth: PEM decode failed for Ed25519 public key")
	}
	switch b.Type {
	case "PUBLIC KEY":
		k, err := x509.ParsePKIXPublicKey(b.Bytes)
		if err != nil {
			return nil, err
		}
		pub, ok := k.(ed25519.PublicKey)
		if !ok {
			return nil, fmt.Errorf("auth: PKIX public key is not Ed25519")
		}
		return pub, nil
	default:
		return nil, fmt.Errorf("auth: unsupported PEM type %q for Ed25519 (want PUBLIC KEY)", b.Type)
	}
}

func (v *ed25519PEMValidator) ValidateAccessToken(_ context.Context, raw string) (Principal, error) {
	opts := []jwt.ParserOption{jwt.WithLeeway(v.leeway), jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()})}
	if v.issuer != "" {
		opts = append(opts, jwt.WithIssuer(v.issuer))
	}
	parser := jwt.NewParser(opts...)

	token, err := parser.ParseWithClaims(strings.TrimSpace(raw), jwt.MapClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
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
	if len(v.audiences) > 0 && !jwtMapClaimsAudienceAllowed(claims, v.audiences) {
		return Principal{}, ErrUnauthenticated
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return Principal{}, ErrUnauthenticated
	}
	return PrincipalFromJWTPayloadJSON(payload, v.leeway)
}
