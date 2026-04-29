package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type jwksEntryKind int

const (
	jwksKindRS256 jwksEntryKind = iota
	jwksKindEd25519
)

type jwtJWKSEntry struct {
	kind jwksEntryKind
	rsa  *rsa.PublicKey
	ed   ed25519.PublicKey
}

// jwtJWKSCachedValidator fetches a JWKS document and validates RS256 and EdDSA (Ed25519) signatures by kid.
// Intended for HTTP_AUTH_MODE=jwt_jwks (multi-alg, rotation-friendly).
type jwtJWKSCachedValidator struct {
	url       string
	client    *http.Client
	ttl       time.Duration
	issuer    string
	audiences []string
	leeway    time.Duration

	mu      sync.RWMutex
	byKid   map[string]jwtJWKSEntry
	expires time.Time
}

func newJWTJWKSCachedValidator(url string, ttl time.Duration, issuer string, audiences []string, leeway time.Duration) *jwtJWKSCachedValidator {
	if ttl <= 0 {
		ttl = 5 * time.Minute
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
	return &jwtJWKSCachedValidator{
		url:       strings.TrimSpace(url),
		client:    &http.Client{Timeout: 15 * time.Second},
		ttl:       ttl,
		issuer:    strings.TrimSpace(issuer),
		audiences: auds,
		leeway:    leeway,
	}
}

func (v *jwtJWKSCachedValidator) refreshLocked(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.url, nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("auth: JWKS HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	var doc struct {
		Keys []struct {
			Kty string `json:"kty"`
			Crv string `json:"crv"`
			Kid string `json:"kid"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
			X   string `json:"x"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return err
	}
	next := make(map[string]jwtJWKSEntry)
	for _, k := range doc.Keys {
		if strings.TrimSpace(k.Use) != "" && !strings.EqualFold(k.Use, "sig") {
			continue
		}
		kid := strings.TrimSpace(k.Kid)
		if kid == "" {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(k.Kty)) {
		case "RSA":
			if k.N == "" || k.E == "" {
				continue
			}
			pub, err := jwkRSAPublicKey(k.N, k.E)
			if err != nil {
				continue
			}
			next[kid] = jwtJWKSEntry{kind: jwksKindRS256, rsa: pub}
		case "OKP":
			if !strings.EqualFold(strings.TrimSpace(k.Crv), "Ed25519") || k.X == "" {
				continue
			}
			xb, err := base64.RawURLEncoding.DecodeString(k.X)
			if err != nil || len(xb) != ed25519.PublicKeySize {
				continue
			}
			var pk ed25519.PublicKey = xb
			next[kid] = jwtJWKSEntry{kind: jwksKindEd25519, ed: pk}
		default:
			continue
		}
	}
	if len(next) == 0 {
		return fmt.Errorf("auth: JWKS contained no usable RS256 or Ed25519 signature keys")
	}
	v.byKid = next
	v.expires = time.Now().UTC().Add(v.ttl)
	return nil
}

func (v *jwtJWKSCachedValidator) keyForToken(ctx context.Context, t *jwt.Token) (any, error) {
	kid, _ := t.Header["kid"].(string)
	kid = strings.TrimSpace(kid)
	alg, _ := t.Header["alg"].(string)
	alg = strings.TrimSpace(alg)

	v.mu.Lock()
	defer v.mu.Unlock()
	if v.byKid == nil || time.Now().UTC().After(v.expires) {
		if err := v.refreshLocked(ctx); err != nil {
			return nil, err
		}
	}

	resolveEnt := func() (jwtJWKSEntry, bool) {
		if kid != "" {
			ent, ok := v.byKid[kid]
			if ok {
				return ent, true
			}
			_ = v.refreshLocked(ctx)
			ent, ok = v.byKid[kid]
			return ent, ok
		}
		if len(v.byKid) == 1 {
			for _, e := range v.byKid {
				return e, true
			}
		}
		return jwtJWKSEntry{}, false
	}

	ent, ok := resolveEnt()
	if !ok {
		return nil, ErrUnauthenticated
	}

	switch alg {
	case jwt.SigningMethodRS256.Alg():
		if ent.kind != jwksKindRS256 || ent.rsa == nil {
			return nil, ErrUnauthenticated
		}
		return ent.rsa, nil
	case jwt.SigningMethodEdDSA.Alg():
		if ent.kind != jwksKindEd25519 {
			return nil, ErrUnauthenticated
		}
		return ent.ed, nil
	default:
		return nil, ErrUnauthenticated
	}
}

func (v *jwtJWKSCachedValidator) ValidateAccessToken(ctx context.Context, raw string) (Principal, error) {
	raw = strings.TrimSpace(raw)
	opts := []jwt.ParserOption{
		jwt.WithLeeway(v.leeway),
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg(), jwt.SigningMethodEdDSA.Alg()}),
	}
	if v.issuer != "" {
		opts = append(opts, jwt.WithIssuer(v.issuer))
	}
	parser := jwt.NewParser(opts...)

	token, err := parser.ParseWithClaims(raw, jwt.MapClaims{}, func(t *jwt.Token) (any, error) {
		return v.keyForToken(ctx, t)
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

// WarmJWKS fetches keys once (API startup connectivity check).
func (v *jwtJWKSCachedValidator) WarmJWKS(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.refreshLocked(ctx)
}
