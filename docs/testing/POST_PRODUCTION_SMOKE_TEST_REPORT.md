# Post-Production Smoke Test Report

Phase 8 — **2026-05-20**. Public checks only; no production DB access or modification.

---

## Context

| Field | Value |
|-------|-------|
| Target URL | `https://api.ldtv.dev` |
| Last documented deploy workflow | Run `26093589896` — success, headSha `6527d502437f5137fb05c56d4851043b258afbc1` |
| Current `origin/main` | `6527d502437f5137fb05c56d4851043b258afbc1` |
| Phase 7 | Deploy **not** triggered (Phase 6 blocked) |

---

## 1. Public health result

Executed with `curl.exe` from operator workstation (2026-05-20):

| Endpoint | HTTP code | Expected | Result |
|----------|-----------|----------|--------|
| `/health/live` | **200** | 200 | **PASS** |
| `/health/ready` | **200** | 200 | **PASS** |

```
live=200
ready=200
```

---

## 2. Version response

**Request:** `GET https://api.ldtv.dev/version`

**Response (200):**

```json
{
  "name": "avf-vending-api",
  "version": "v1.0.01",
  "git_sha": "52a076e340a15a69dad7787cad54d7e3000fcafe",
  "app_env": "production",
  "process": "api",
  "runtime_role": "api",
  "region": "ap-southeast-1",
  "node_name": "app-node-a",
  "instance_id": "app-node-a",
  "public_base_url": "https://api.ldtv.dev",
  "machine_public_base_url": "https://api.ldtv.dev"
}
```

| Check | Expected | Actual | Result |
|-------|----------|--------|--------|
| Endpoint reachable | 200 JSON | 200 JSON | **PASS** |
| `git_sha` present | non-empty | `52a076e340a15a69dad7787cad54d7e3000fcafe` | **PASS** |
| `git_sha` matches `origin/main` | `6527d502…` | `52a076e3…` | **FAIL** |
| `git_sha` matches last deploy workflow headSha | `6527d502…` | `52a076e3…` | **FAIL** |

**Production commit identity:** `52a076e` — *Merge pull request #99 from leduytuanvu/develop* (ancestor of `main`, **not** current tip).

**Drift:** Public API reports a build **~127 merge commits behind** current `main` (`6527d502`). This contradicts the last successful **Deploy Production** workflow headSha unless rollout did not update running containers or traffic hits a stale node.

---

## 3. Container status

| Check | Result |
|-------|--------|
| SSH `root@72.62.244.94` | **Not available** — `Permission denied (publickey,password)` |
| `docker compose ps` | **Not run** — no SSH access from this environment |

Expected services (api, caddy, worker, mqtt-ingest, reconciler) — **unverified**.

---

## 4. Recent error log scan

| Check | Result |
|-------|--------|
| SSH log grep (`error\|panic\|fatal\|SQLSTATE\|migration\|audit\|uuid`) | **Not run** — no SSH access |

No secrets collected or printed.

---

## 5. Supplementary public checks (informational)

| Endpoint | Code |
|----------|------|
| `/swagger/index.html` | 200 |
| `/openapi.json` | 404 (not exposed publicly; not a health gate) |

---

## Final verdict

### **PRODUCTION_SMOKE_FAILED**

| Area | Status |
|------|--------|
| Liveness / readiness | **PASS** — both 200 |
| Version endpoint | **PASS** — valid JSON |
| Version `git_sha` vs expected deploy / `main` | **FAIL** — production reports `52a076e`, expected `6527d502` |
| Container health (SSH) | **SKIPPED** — no access |
| Error log scan (SSH) | **SKIPPED** — no access |

**Summary:** Production is **up and healthy** at the HTTP layer, but **does not report the git SHA** from the last successful deploy workflow or current `main`. Post-deploy verification is **incomplete** without SSH/container confirmation and version alignment.

**Recommended follow-up (operator, out of scope for this phase):**

1. SSH to app-node(s) and run `docker compose ps` + image digest inspection.
2. Confirm whether run `26093589896` rolled out to `app-node-a` or only partially completed.
3. Re-deploy with resolved inputs once Phase 6 is `READY_TO_DEPLOY`, or investigate stale `git_sha` on running containers.

No production DB reset, manual DB edits, or secret output performed.

---

*Phase 8 complete — smoke test stopped after public verification and SSH attempt.*
