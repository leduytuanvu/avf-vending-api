# Postman v3 Local Mode import guide

Use this guide when Postman v12 shows the JSON unsupported warning in **Local Mode / Native Git**.

## Prerequisites

- Postman Desktop v12+ with Local Mode / Native Git enabled
- Repository checked out locally with `postman/v3/` generated (`make postman-generate-v3`)
- Postman CLI optional for smoke: `npm install -g postman-cli`

## Which files to use

| Use case | Path |
|----------|------|
| **Postman Local Mode / Native Git** | `postman/v3/` (YAML folders + `.environment.yaml`) |
| **Newman / existing CI** | `postman/collections/*.postman_collection.json` + `postman/environments/*.postman_environment.json` |

Do **not** point Local Mode at JSON files under `postman/collections/` — use `postman/v3/` instead.

## Import steps

1. Open Postman Desktop.
2. Enable **Local Mode** / connect **Native Git** to this repository (or open the repo folder).
3. Point Postman at the repo root or specifically `postman/v3/`.
4. Confirm collections appear as folders:
   - `postman/v3/collections/avf-vending-api`
   - `postman/v3/environments/avf-local.environment.yaml` (and staging/production as needed)
5. Select environment **AVF Local** (`avf-local.environment.yaml`).
6. Fill **local-only** secrets in the environment (never commit):
   - `adminPassword`
   - `accessToken` / `refreshToken` (leave blank before login)
   - `machineToken` (after activation flow)
7. Open folder **Public** → run **GET /health/live**.
8. Run **01 POST /v1/auth/login** (Integrated flow) or production auth login in the full suite.
9. Confirm tests capture `accessToken` and `refreshToken` into the environment.
10. Run a safe readonly request, e.g. **GET /v1/auth/me** or **GET /health/ready**.
11. **Do not** set `allowGatedWrites=true` or `confirmProductionWrites` unless intentionally running a controlled production canary.

## Expected results

- Local Mode shows **no JSON upgrade warning** for assets under `postman/v3/`.
- Health requests return **200**.
- Login returns **200** with tokens when credentials are correct.
- Production environment blocks mutating requests until unlock variables are set (collection pre-request scripts).

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| JSON upgrade warning | Use `postman/v3/`, not `postman/collections/*.json` |
| v3 folder missing | Run `make postman-generate-v3` (requires Postman CLI) |
| `EXDEV` on Windows migrate | Set `TEMP` and `TMP` to a folder on the same drive as the repo |
| 401 / 403 | Refresh login; check `auth_type` and token expiry |
| Gated write blocked | Expected — set `allowGatedWrites` + `confirmProductionWrites` only for intentional prod tests |
| Missing idempotency key | Collection pre-request sets `Idempotency-Key`; verify environment/collection scripts loaded |
| Newman fails on v3 | Newman requires JSON — use `postman/collections/` paths |

## Optional CLI smoke

```bash
postman collection run "postman/v3/collections/avf-vending-api" \
  -e "postman/v3/environments/avf-local.environment.yaml"
```

## Related docs

- [`postman/v3/README.md`](../../postman/v3/README.md)
- [`docs/runbooks/postman.md`](../runbooks/postman.md)
- [`docs/testing/AVF_POSTMAN_PRODUCTION.md`](../testing/AVF_POSTMAN_PRODUCTION.md)
