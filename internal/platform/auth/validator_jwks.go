package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type rs256JWKSCachedValidator struct {
	url    string
	client *http.Client
	ttl    time.Duration
	issuer string
	aud    string
	leeway time.Duration

	mu      sync.RWMutex
	byKid   map[string]*rsa.PublicKey
	expires time.Time
}

func newRS256JWKSCachedValidator(url string, ttl time.Duration, issuer, audience string, leeway time.Duration) *rs256JWKSCachedValidator {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if leeway <= 0 {
		leeway = DefaultClockLeeway
	}
	return &rs256JWKSCachedValidator{
		url:    strings.TrimSpace(url),
		client: &http.Client{Timeout: 15 * time.Second},
		ttl:    ttl,
		issuer: strings.TrimSpace(issuer),
		aud:    strings.TrimSpace(audience),
		leeway: leeway,
	}
}

func jwkRSAPublicKey(nB64, eB64 string) (*rsa.PublicKey, error) {
	nb, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, err
	}
	eb, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, err
	}
	n := new(big.Int).SetBytes(nb)
	eInt := new(big.Int).SetBytes(eb).Int64()
	if eInt <= 0 || eInt > 1<<31-1 {
		return nil, fmt.Errorf("auth: invalid RSA exponent")
	}
	return &rsa.PublicKey{N: n, E: int(eInt)}, nil
}

func (v *rs256JWKSCachedValidator) refreshLocked(ctx context.Context) error {
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
			Kid string `json:"kid"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return err
	}
	next := make(map[string]*rsa.PublicKey)
	for _, k := range doc.Keys {
		if !strings.EqualFold(k.Kty, "RSA") || k.N == "" || k.E == "" {
			continue
		}
		if strings.TrimSpace(k.Use) != "" && !strings.EqualFold(k.Use, "sig") {
			continue
		}
		pub, err := jwkRSAPublicKey(k.N, k.E)
		if err != nil {
			continue
		}
		kid := strings.TrimSpace(k.Kid)
		if kid == "" {
			continue
		}
		next[kid] = pub
	}
	if len(next) == 0 {
		return fmt.Errorf("auth: JWKS contained no usable RSA signature keys")
	}
	v.byKid = next
	v.expires = time.Now().UTC().Add(v.ttl)
	return nil
}

func (v *rs256JWKSCachedValidator) publicKeyForKid(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.byKid == nil || time.Now().UTC().After(v.expires) || (kid != "" && v.byKid[kid] == nil) {
		if err := v.refreshLocked(ctx); err != nil {
			return nil, err
		}
		if kid != "" && v.byKid[kid] == nil {
			// One forced refresh in case of key rotation race.
			_ = v.refreshLocked(ctx)
		}
	}
	if kid == "" && len(v.byKid) == 1 {
		for _, pub := range v.byKid {
			return pub, nil
		}
	}
	pub := v.byKid[kid]
	if pub == nil {
		return nil, ErrUnauthenticated
	}
	return pub, nil
}

func (v *rs256JWKSCachedValidator) ValidateAccessToken(ctx context.Context, raw string) (Principal, error) {
	raw = strings.TrimSpace(raw)
	opts := []jwt.ParserOption{jwt.WithLeeway(v.leeway), jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()})}
	if v.issuer != "" {
		opts = append(opts, jwt.WithIssuer(v.issuer))
	}
	if v.aud != "" {
		opts = append(opts, jwt.WithAudience(v.aud))
	}
	parser := jwt.NewParser(opts...)

	token, err := parser.ParseWithClaims(raw, jwt.MapClaims{}, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return v.publicKeyForKid(ctx, strings.TrimSpace(kid))
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

// WarmJWKS fetches keys once (used at API startup for fail-fast when JWKS is unreachable).
func (v *rs256JWKSCachedValidator) WarmJWKS(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.refreshLocked(ctx)
}
