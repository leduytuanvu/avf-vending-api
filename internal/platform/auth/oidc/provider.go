// Package oidc defines provider-agnostic hooks for future SSO / OpenID Connect integration.
// No concrete IdP (Google, Microsoft, Auth0, Keycloak, etc.) is implemented in this repository.
//
// Expected future wiring (documented in docs/runbooks/oidc-sso-integration.md):
//   - OIDC_ISSUER_URL — issuer metadata (/.well-known/openid-configuration)
//   - OIDC_CLIENT_ID / OIDC_CLIENT_SECRET — confidential client (secret via vault, not committed)
//   - OIDC_REDIRECT_URI — callback URL registered with the IdP
//   - Optional: OIDC_SCOPES, OIDC_AUDIENCE, HTTP_AUTH_JWT_* for validating IdP-issued JWTs at the API edge
package oidc

import "context"

// Tokens holds opaque OAuth2/OIDC token handles returned from the token endpoint.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	IDToken      string
	TokenType    string
	ExpiresIn    int64
}

// Claims is a minimal normalized view of an ID token after signature verification.
type Claims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Issuer        string
	Audience      []string
	Raw           map[string]any
}

// Provider is implemented by future enterprise adapters (per-tenant or global).
type Provider interface {
	// AuthorizationURL returns the IdP authorize endpoint URL including state/nonce parameters.
	AuthorizationURL(state, nonce, redirectURI string) (string, error)

	// Exchange trades an authorization code for tokens at the token endpoint.
	Exchange(ctx context.Context, code, redirectURI string) (*Tokens, error)

	// VerifyIDToken validates the ID token (signature, iss, aud, exp) and returns claims.
	VerifyIDToken(ctx context.Context, rawIDToken string) (*Claims, error)
}
