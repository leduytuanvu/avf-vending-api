# Admin account recovery

## Locked account (failed logins)

1. Confirm Redis and Postgres state: repeated failures increment Redis key and `platform_auth_accounts.failed_login_count` / `locked_until`.
2. Operators with **user:write** may reset password via admin API **POST .../reset-password** or wait for lock TTL to expire (and clear via successful policy/login if configured).
3. Use **POST .../revoke-sessions** if compromise is suspected.

## Lost TOTP

1. Require identity verification out-of-band per org policy.
2. **platform_admin** or **org_admin** may use **POST .../reset-password** after audit approval.
3. After reset, user should re-enroll MFA (**POST /v1/auth/mfa/totp/enroll**) before next production login if `ADMIN_MFA_REQUIRED_IN_PRODUCTION=true`.

## Password reset email (self-service)

1. User calls **POST /v1/auth/password/reset/request** (never indicates if email exists).
2. Delivery of the raw reset token is via your org’s mailer/integration (API does not return the token in HTTP responses in production builds).
3. User completes **POST /v1/auth/password/reset/confirm** once; token is single-use.

## Session cleanup

- User: **DELETE /v1/auth/sessions** with `exceptRefreshToken` to sign out other devices.
- Admin: **POST .../revoke-sessions** for full refresh revocation and `admin_sessions` revocation.
