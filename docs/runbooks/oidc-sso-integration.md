# OIDC / SSO integration (future hooks)

The API ships **interfaces only** in `internal/platform/auth/oidc` — no third-party IdP SDKs or hardcoded providers.

## Configuration sketch

When implementing a provider adapter (outside this repo or in a follow-on phase), you will typically need:

| Variable | Purpose |
|----------|---------|
| `OIDC_ISSUER_URL` | Issuer base URL; discovery at `/.well-known/openid-configuration`. |
| `OIDC_CLIENT_ID` | Registered OAuth2 client id. |
| `OIDC_CLIENT_SECRET` | Client secret (inject via secret manager; **never** commit). |
| `OIDC_REDIRECT_URI` | Backend or BFF callback URL whitelisted at the IdP. |
| `OIDC_SCOPES` | Optional; default `openid email profile`. |

## JWT validation at the API

After SSO, access to `/v1` may use:

- `HTTP_AUTH_MODE=jwt_jwks` or `rs256_jwks` with the IdP’s JWKS URL (`HTTP_AUTH_JWT_JWKS_URL`), **or**
- `HTTP_AUTH_MODE=ed25519_pem` / `jwt_jwks` if the IdP issues EdDSA tokens.

Set `HTTP_AUTH_JWT_ISSUER` and `HTTP_AUTH_JWT_AUDIENCE` to match the IdP.

Interactive **login** responses from this API remain **HS256**-signed session JWTs via `HTTP_AUTH_LOGIN_JWT_SECRET` when using asymmetric verification for inbound IdP tokens.

## References

- [production-readiness.md](./production-readiness.md) — TLS and auth posture.
- `internal/platform/auth/validator_factory.go` — supported `HTTP_AUTH_MODE` values.
