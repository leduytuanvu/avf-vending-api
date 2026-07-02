# Final Commit, Merge, and Deploy Verdict

**UTC:** 20260702T194000Z  
**Classification:** `RELEASE_COMMITTED_MERGED_DEPLOYED_PRODUCTION_OK`

---

## 1. API repository committed?

**Yes.** Enterprise flow release on `develop` via PR #392 (`aa8d41ee` + migration fix `271de1fa`), merged at `78d6ba89`.

## 2. App repository committed?

**Yes.** Docs/reports cleanup `944d3c9` on `main`; reconciliation merge `6f78a18`.

## 3. API develop/main tree parity?

**Yes.** `git diff origin/develop..origin/main` empty (trees identical; main has merge commit `156fc468`).

## 4. App develop/main tree parity?

**Yes.** Both at `6f78a18`.

## 5. Pre-commit tests passed (current run)?

**Yes (API).** `go test ./...`, enterprise-flow validators, migration verify. **App:** Gradle assemble production release passed; `./gradlew test` has pre-existing failures unrelated to docs-only commit.

## 6. API develop pushed?

**Yes** (via PR #392 merge to origin/develop).

## 7. API main promoted?

**Yes.** PR #393 merged → `156fc468`.

## 8. App main/develop synced?

**Yes.** Pushed and develop reset to main.

## 9. Main CI green?

**Yes.** Run [28615428060](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28615428060).

## 10. Build and Push Images green on main?

**Yes.** Run [28615687682](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28615687682).

## 11. Security Release verdict pass on main?

**Yes.** Run [28615969226](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28615969226), `verdict: pass`.

## 12. Staging evidence used?

**No.** Documented bypass — no successful Staging Deployment Contract for SHA `156fc468`.

## 13. Production deploy triggered?

**Yes.** Attempt 1 [28616100078](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28616100078) failed (pg_dump pool limit). Retry [28616475685](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28616475685) **success**.

## 14. Migration 00016 applied?

**Yes** (retry deploy with `run_migration=true` succeeded).

## 15. Production git_sha matches main?

**Yes.** `/version` reports `156fc468fa3c5fec7042e1f656f78b6ea94c2639`.

## 16. Health live/ready pass?

**Yes.** Both HTTP 200.

## 17. OpenAPI reachable?

**Yes.** HTTP 200 on `/swagger/doc.json`.

## 18. gRPC/MQTT smoke?

**Yes.** TLS reachability to `machine-api.ldtv.dev:443` and `mqtt.ldtv.dev:8883`.

## 19. Image digests deployed?

- App: `sha256:17785fabd82a7cfe8c4e9dcc7d8f2693a03ff186a5294d69a97c7b5e890fddae`
- Goose: `sha256:ab9abcb8cce2fe7d2136305da4603c5e2ec4c55eaa098aaf26bc11b31587aae6`

## 20. Key SHAs

| Repo | Branch | SHA |
|------|--------|-----|
| avf-vending-api | main | `156fc468fa3c5fec7042e1f656f78b6ea94c2639` |
| avf-vending-api | develop | `78d6ba89645e84c18dd22c536243d949c189c2ce` |
| avf-vending-app | main/develop | `6f78a18c32c922adb8f28e2e1ec7e2b07b04447b` |

## 21–25. Evidence paths

- API release: `reports/release/20260702T184910Z/`
- Enterprise flow: `reports/enterprise-flow-verification/20260703T013119Z/`
- App release: `avf-vending-app/reports/release/20260702T184910Z/`

## Final verdict

**RELEASE_COMMITTED_MERGED_DEPLOYED_PRODUCTION_OK**

Notes:
- First deploy attempt failed on Supabase pooler during pg_dump; automatic rollback preserved prior production. Retry succeeded.
- Staging gate bypass used (documented operator approval).
