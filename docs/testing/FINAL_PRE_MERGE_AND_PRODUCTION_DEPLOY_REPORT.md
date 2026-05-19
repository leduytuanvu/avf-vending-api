# Final Pre-Merge and Production Deploy Report

End-to-end verification and release tracking for the AVF Vending API full-system hardening effort.  
Phases 0–5 on feature branch; Phases 6–9 on `main` (2026-05-19 – 2026-05-20).

**Related reports:**

- [PRODUCTION_DEPLOY_INPUT_RESOLUTION.md](./PRODUCTION_DEPLOY_INPUT_RESOLUTION.md) — Phase 6
- [PRODUCTION_DEPLOY_RUN_REPORT.md](./PRODUCTION_DEPLOY_RUN_REPORT.md) — Phase 7
- [POST_PRODUCTION_SMOKE_TEST_REPORT.md](./POST_PRODUCTION_SMOKE_TEST_REPORT.md) — Phase 8

---

## Phase summary (Phases 0–5)

| Phase | Outcome |
|-------|---------|
| 0 Pre-flight | SAFE_TO_CONTINUE locally |
| 1–3 Verification | Local `go test`, UUID v7, migrations, full-system wrapper **PASS** on feature branch |
| 4 Push → develop | [PR #227](https://github.com/leduytuanvu/avf-vending-api/pull/227) open — **CI blocked**, not merged |
| 5 develop → main | **Skipped** — preconditions unmet; `origin/develop` already contained in `origin/main` (PR #226) |

Detailed Phase 0–4 notes remain on branch `chore/final-full-system-verification-uuidv7-postman-tests` (commit `7a6809f`).

---

## Phase 9 — Final production release section (2026-05-20)

### 1. Feature branch name and commit SHA

| Field | Value |
|-------|-------|
| Branch | `chore/final-full-system-verification-uuidv7-postman-tests` |
| Tip SHA | **`7a6809fb33c5d90815387ccfb0fc28457f6d2da0`** |
| Key commits | `891c5e2` (verification hardening), `fee0bca`, `7a6809f` (docs) |
| PR | [#227](https://github.com/leduytuanvu/avf-vending-api/pull/227) → `develop` — **OPEN**, mergeState **BLOCKED** |
| On `main`? | **No** |

### 2. Develop merge commit SHA

| Field | Value |
|-------|-------|
| Feature → develop merge | **Not performed** (PR #227 blocked) |
| `origin/develop` tip | `705eae60d80fc3893ffc89e9a4ad20442bba8686` (PR #225) |

### 3. Main merge commit SHA

| Field | Value |
|-------|-------|
| Phase 5 develop → main merge | **Not performed** (preconditions unmet; no new develop commits) |
| `origin/main` tip | **`6527d502437f5137fb05c56d4851043b258afbc1`** — Merge PR #226 from `develop` |
| Phase 5 docs PR | [#228](https://github.com/leduytuanvu/avf-vending-api/pull/228) — docs only |

### 4. Main CI workflow statuses (@ `6527d502`)

| Workflow | Run id | Conclusion |
|----------|--------|------------|
| CI | 26091978633 | **success** |
| Security | 26091978720 | **success** |
| Enterprise release verification | 26091978636 | **success** |
| CodeQL | 26091978643 | skipped |

### 5. Build workflow run id / status

| Field | Value |
|-------|-------|
| Workflow | Build and Push Images |
| Run id | **26092186016** |
| URL | https://github.com/leduytuanvu/avf-vending-api/actions/runs/26092186016 |
| Conclusion | **success** |
| headSha | `6527d502437f5137fb05c56d4851043b258afbc1` |

### 6. Security workflow run id / status

| Field | Value |
|-------|-------|
| Workflow | Security Release |
| Run id | **26092412966** |
| URL | https://github.com/leduytuanvu/avf-vending-api/actions/runs/26092412966 |
| Conclusion | **success** |
| security_verdict | pass |
| source_build_run_id | 26092186016 |

### 7. Production deploy workflow run id / status

| Field | Value |
|-------|-------|
| Phase 7 dispatch | **Not triggered** (Phase 6 `BLOCKED_INPUTS_MISSING`) |
| Last successful deploy (prior operator run) | **26093589896** |
| URL | https://github.com/leduytuanvu/avf-vending-api/actions/runs/26093589896 |
| Conclusion | **success** (2026-05-19) |
| Workflow headSha | `6527d502437f5137fb05c56d4851043b258afbc1` |
| Staging gate | Bypassed (`allow_missing_staging_evidence: true`) |

### 8. Production deployed git_sha / version (`/version`)

**Checked 2026-05-20** — `GET https://api.ldtv.dev/version`:

| Field | Value |
|-------|-------|
| `version` | `v1.0.01` |
| `git_sha` | **`52a076e340a15a69dad7787cad54d7e3000fcafe`** |
| `node_name` | `app-node-a` |
| Expected (`main` / deploy workflow) | `6527d502437f5137fb05c56d4851043b258afbc1` |
| Alignment | **MISMATCH** — production reports PR #99-era build, not current `main` |

### 9. DB migration deploy gate result

| Field | Value |
|-------|-------|
| Phase 7 `run_migration` | Would be `false` — not dispatched |
| Last deploy run `26093589896` | `run_migration: false` |
| Migration step | **Not executed** (by design) |
| Feature branch migration gates | Verified locally only; **not on production** |

### 10. DB backup result

| Field | Value |
|-------|-------|
| Phase 7 | **N/A** — no deploy triggered |
| Last deploy (`run_migration=false`) | **N/A** — backup not required |
| Production DB | **Not modified** in any phase |

### 11. Public smoke test result

See [POST_PRODUCTION_SMOKE_TEST_REPORT.md](./POST_PRODUCTION_SMOKE_TEST_REPORT.md).

| Check | Result |
|-------|--------|
| `/health/live` | **200** — PASS |
| `/health/ready` | **200** — PASS |
| `/version` git_sha vs `main` | **FAIL** — drift (`52a076e` vs `6527d502`) |
| Phase 8 verdict | **PRODUCTION_SMOKE_FAILED** |

### 12. Server container health result

| Check | Result |
|-------|--------|
| SSH `root@72.62.244.94` | **Permission denied** — no access from CI/agent environment |
| `docker compose ps` (api, caddy, worker, mqtt-ingest, reconciler) | **Not verified** |
| Log scan (10m, error/panic/fatal) | **Not run** |

### 13. UUID v7 final status

| Scope | Status |
|-------|--------|
| Feature branch | **PASS** — audits, migration `00005`, checks green locally |
| `main` / production | **Not deployed** — UUID v7 platform not merged to `main` |
| Production runtime | **Unknown / not applicable** — running pre-UUID-v7 build (`52a076e`) |

### 14. REST / gRPC / MQTT / Postman final status

| Scope | Status |
|-------|--------|
| Feature branch (local full-system verify) | **PASS** — 23 passed, 5 skipped (`verify-full-system.sh`) |
| Reports on feature branch | REST, gRPC, MQTT, Postman audit docs under `docs/testing/` (not on `main`) |
| Production | **Not re-verified** — stale `git_sha`; Phase 8 smoke incomplete |
| PR #227 CI | **FAIL** — workflow path regressions block merge to `develop` |

### 15. Final verdict

## **NOT_DEPLOYED_WITH_BLOCKERS**

| Blocker | Detail |
|---------|--------|
| Feature not on `main` | PR #227 open, CI blocked |
| Phase 6 inputs | `staging_evidence_id` unresolved (strict path) |
| Phase 7 | Deploy not triggered |
| Production version drift | `/version` reports `52a076e`, not `6527d502` |
| Smoke test | **PRODUCTION_SMOKE_FAILED** (SHA mismatch) |
| Container verification | SSH unavailable |
| UUID v7 / new migrations | Not in production |

**Operational note:** Production HTTP health is **green** (`live=200`, `ready=200`), but the release verification chain did **not** complete successfully for current `main` or the verification feature branch.

**Unblock path:**

1. Fix PR #227 CI → merge to `develop` → validate → merge to `main`.
2. Pass **Staging Deployment Contract** or document approved bypass → Phase 6 `READY_TO_DEPLOY`.
3. Dispatch **Deploy Production** with resolved inputs → Phase 7 success.
4. Confirm `/version` git_sha matches deploy headSha → Phase 8 **PRODUCTION_SMOKE_PASS**.
5. SSH verify containers on app-node(s).

---

*Phase 9 — final report committed 2026-05-20.*
