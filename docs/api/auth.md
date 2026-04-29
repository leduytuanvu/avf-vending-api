# Admin authentication and MFA (P1.1)

## Session JWT types

- **access** (`token_use=access`): normal interactive API token. Required for `/v1/auth/me`, password change, session list/revoke, and MFA disable.
- **mfa_pending** (`token_use=mfa_pending`): short-lived JWT after successful password verification when MFA enrollment or verification is required. Used for **POST /v1/auth/mfa/totp/enroll** (enrollment mode) and **POST /v1/auth/mfa/totp/verify** (login path).

## MFA (TOTP)

- **POST /v1/auth/mfa/totp/enroll** — creates a `pending` TOTP factor (secret encrypted with `ADMIN_MFA_ENCRYPTION_KEY`).
- **POST /v1/auth/mfa/totp/verify** — with **mfa_pending** JWT, validates TOTP for login or production-required enrollment; with a normal **access** JWT, completes optional enrollment when a pending factor exists.
- **POST /v1/auth/mfa/totp/disable** — interactive only; requires password + current TOTP; revokes refresh tokens.

Production: set `ADMIN_MFA_REQUIRED_IN_PRODUCTION=true` and a **32-byte** `ADMIN_MFA_ENCRYPTION_KEY` (standard base64).

## Sessions

- **GET /v1/auth/sessions** — list current user sessions (from `admin_sessions`).
- **DELETE /v1/auth/sessions/{sessionId}** — revoke one refresh chain.
- **DELETE /v1/auth/sessions** — JSON body `{ "exceptRefreshToken": "<current>" }` revokes all **other** sessions.

Admin:

- **GET /v1/admin/organizations/{organizationId}/users/{userId}/sessions** — `user:read`; subject must be scoped to the organization (`CanAccessOrganizationAdminData`).
- **POST .../revoke-sessions** — `user:sessions:revoke` (existing).

## Password reset

- **POST /v1/auth/password/reset/request** — always returns `202 { "accepted": true }`; does not reveal whether the email exists.
- **POST /v1/auth/password/reset/confirm** — one-time hashed token (`password_reset_tokens`).

## Lockout

- Redis-backed failure counter (`PeekFailureCount` / `IncrementFailure`) mirrors DB lockout policy (`ADMIN_LOGIN_MAX_FAILED_ATTEMPTS`, `ADMIN_LOGIN_LOCKOUT_TTL`).
- Failed/password and MFA failures are audited (`auth.login.failed` taxonomy for interactive path).

## RBAC

- Fine-grained permission strings: `internal/platform/auth/permissions.go`.
- Tenant guard for URL `organizationId`: `internal/platform/auth/admin_rbac.go` (`CanAccessOrganizationAdminData`).
