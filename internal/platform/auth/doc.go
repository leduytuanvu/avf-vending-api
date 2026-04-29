// Package auth provides HTTP-facing authentication primitives: bearer JWT validation,
// principal/actor/role extraction, and coarse scope hints (organization, site, machines).
//
// Validation mode is selected via config (HTTP_AUTH_MODE): hs256 (shared secret, dev-friendly),
// rs256_pem (RSA public key PEM), rs256_jwks (JWKS with RS256 keys and kid rotation),
// ed25519_pem (Ed25519 public key PEM for EdDSA access tokens), or jwt_jwks (JWKS with RS256 + Ed25519 keys).
// Optional HTTP_AUTH_JWT_ALG (HS256|RS256|EdDSA) cross-checks the configured mode for deployment guardrails.
// Optional HTTP_AUTH_JWT_SECRET_PREVIOUS accepts tokens signed with the prior HS256 secret during rotation.
//
// Scope enforcement is intentionally shallow at the edge: route middleware checks claims;
// data plane should still use scoped SQL (e.g. ListMachinesByOrganizationID) in repositories.
package auth
