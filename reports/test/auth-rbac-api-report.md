# Auth / JWT / session / MFA / RBAC — verification report

**Scope:** REST auth surface (`/v1/auth/*`), JWT issuance for interactive admin sessions, RBAC middleware tests, and OpenAPI/Postman references to retired multi-company fields.

---

## Final verdict

| Gate | Result |
|------|--------|
| Focused Go tests (§2) | **PASS** |
| Manual HTTP exercise (§3) | **NOT RUN** — local `:8080` did not serve this API (see §6) |
| Contract checks (login/me/JWT/RBAC) via code + OpenAPI | **PASS** (§4–§5) |
| OpenAPI/Postman edits + regeneration | **Not needed** — no `organizationId` / legacy tenant strings found in `docs/swagger/swagger.json` or `docs/postman/*.json` |

**Overall:** **PASS** on automation + static contracts; **manual smoke blocked** until the real API process listens on a known base URL.

---

## 1. Local API

**Action:** Probed `GET http://127.0.0.1:8080/health/live` and `POST http://127.0.0.1:8080/v1/auth/login`.

**Observation:** Apache-style **HTML 404** for both — listener on **8080 is not** the chi router from this repo (matches earlier preflight notes).

**Recommended start (operator):**

```powershell
# From repo root, after Postgres/redis/etc. per docs:
go run ./cmd/api
# Or bind explicitly:
# $env:HTTP_ADDR=":18080"; go run ./cmd/api
```

Then re-hit `GET http://127.0.0.1:<port>/health/live` until **200** JSON (not HTML).

---

## 2. Focused Go tests — command & outcome

```powershell
go test ./internal/app/auth/... ./internal/httpserver/... `
  -run 'Auth|Login|Me|Session|RBAC|MFA|Password|Token|Principal|Permission' -count=1
```

| Package | Result |
|---------|--------|
| `./internal/app/auth/...` | `[no test files]` |
| `./internal/httpserver/...` | **ok** (~0.23s this run) |

**Representative tests executed (verbose sampling):**

- `TestOpenAPI_securitySchemesBearerAuthPresent`, `TestOpenAPI_bearerAuthOnProtectedV1Routes`
- `TestRBAC_*` — viewer/catalog/finance/fleet/technician/support/**orgAdmin**/machine/disabled matrices (`TestRBAC_orgAdmin_passesCatalogAndRefunds`, `TestRBAC_orgAdmin_canUserReadAndSessionsRevoke`, …)
- `TestAuthObservabilityMiddleware_PropagatesPrincipalFields`
- `TestAbuseProtection_LoginPOST_*`
- `TestMountV1_adminAuthUsersRoutesRegistered`, `TestMountV1_noDuplicateAuthRoute`

**Exit code:** **0**

---

## 3. Manual HTTP matrix — planned vs actual

| Endpoint | Intended check | Actual |
|----------|----------------|--------|
| `POST /v1/auth/login` | 401/200 JSON envelope; body lacks retired `organizationId` | **Skipped** — upstream 404 HTML |
| `GET /v1/auth/me` | 401 without Bearer; 200 shape | **Skipped** |
| `POST /v1/auth/refresh` | accepts refresh token envelope | **Skipped** |
| `POST /v1/auth/logout` | accepts refresh / revokeAll | **Skipped** |
| `GET /v1/auth/sessions` | listed in router (`auth_http.go`) | **Skipped** |

**Redacted evidence:** none captured (no successful JSON responses).

---

## 4. Contract verification (source + OpenAPI)

### 4.1 Login / `me` JSON (documented DTOs)

`internal/httpserver/openapi_types.go`:

- **`V1AuthLoginResponse`:** `accountId`, `scopeId`, `email`, `roles`, `tokens`, optional MFA fields — **no** `organizationId`.
- **`V1AuthMeResponse`:** `accountId`, `scopeId`, `email`, `roles` — **no** `organizationId`.

`scopeId` represents the **single-company context** (aligned with removal of multi-company org payloads).

### 4.2 Access JWT claims (interactive)

`internal/platform/auth/token_issuer.go` → `sessionAccessClaims`:

Embeds `sub`, `roles`, optional operator/machine fields, `account_status`, standard `iss`/`aud`/`iat`/`exp`, `token_use`, etc.

**There are no JSON tags for `organization_id`, `organizationId`, `tenant`, or `tenant_id`.**  
(`IssueAccessJWT` still accepts a `companyUUID` parameter for future/session plumbing; it is **not** written into these HS256 interactive claims in the shown path.)

### 4.3 `org_admin` vs company admin role string

- Go constant `RoleOrgAdmin` maps to JWT/database role string **`"admin"`** (`principal.go`).
- Repository grep for literal **`"org_admin"`** in `*.go`: **no matches**.

RBAC tests named `TestRBAC_orgAdmin_*` exercise the **`admin`** principal role, not a literal `org_admin` API string.

### 4.4 Permissions / RBAC still enforced

Covered by passing `TestRBAC_*` suite under `internal/httpserver` (catalog, refunds, fleet, user read, session revoke, machine bypass, disabled account, etc.).

### 4.5 OpenAPI / Postman

```powershell
Select-String -Path docs/swagger/swagger.json -Pattern 'organizationId','organization_id' -SimpleMatch
Select-String -Path docs/postman/*.json -Pattern 'organizationId','organization_id' -SimpleMatch -Recurse
```

**No matches** (same outcome as `git grep` on those artifacts in this session).

No regeneration required.

---

## 5. Files changed

**None** — verification only.

---

## 6. Blockers & next steps

1. **Wrong process on 8080** — free the port or set `HTTP_ADDR` to an unused port and rerun §3 with `curl`/Postman using redacted tokens only in notes.
2. **Integration confidence** — optional: `go test ./internal/httpserver/... -count=1 -v` without `-run` filter for broader coverage once DB-backed tests are desired (`TEST_DATABASE_URL`).

---

## 7. Pass / fail summary

| Area | Status |
|------|--------|
| Focused Go tests | **PASS** |
| Live HTTP smoke | **FAIL / SKIPPED** (infrastructure) |
| Login/me/JWT/OpenAPI policy | **PASS** (static) |
| Docs regeneration | **N/A** |
