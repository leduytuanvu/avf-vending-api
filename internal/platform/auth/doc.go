// Package auth provides HTTP-facing authentication primitives: bearer JWT validation,
// principal/actor/role extraction, and coarse scope hints (organization, site, machines).
//
// Validation mode is selected via config (HTTP_AUTH_MODE): hs256 (shared secret, dev-friendly),
// rs256_pem (RSA public key PEM), or rs256_jwks (OIDC-style JWKS URL with TTL cache for key rotation).
// Optional HTTP_AUTH_JWT_SECRET_PREVIOUS accepts tokens signed with the prior HS256 secret during rotation.
//
// Scope enforcement is intentionally shallow at the edge: route middleware checks claims;
// data plane should still use scoped SQL (e.g. ListMachinesByOrganizationID) in repositories.
package auth
