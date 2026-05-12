# Production Readiness

- **Target commit:** `3536e9e42536bcbf92e70052325b74c41a106ffc` on `security/goose-otel-fix`
- **CI (required gates on exact commit):** **NOT_VERIFIED** — no GitHub Actions runs list `3536e9e`; workflows trigger on PR/push to `develop`/`main` only. Open a PR or merge to obtain fresh CI, Production Proof, and Security runs.
- **Security Release / image Trivy:** **NOT_VERIFIED** for this tip — last green reference runs were for commit `3369b51`.
- **Production read-only smoke:** **NOT_RUN** — `STAGING_BASE_URL`, `PRODUCTION_BASE_URL`, `PROD_BASE_URL`, `BASE_URL_PROD` unset.
- **Production canary:** **BLOCKED** — canary env not provided.
- **PSP provider proof:** **BLOCKED**
- **Hardware proof:** **BLOCKED**
- **REST OpenAPI live coverage:** **PARTIAL** (6 pass / 365 ops in `rest-full-live-coverage.json`) — **no full live coverage claim**

## Final claim

**BLOCKED:** CI gates were not executed for commit `3536e9e`; production/staging smoke was not run; canary/PSP/hardware remain unproven. See `PRODUCTION_PROOF_REPORT.md` for required next actions.
