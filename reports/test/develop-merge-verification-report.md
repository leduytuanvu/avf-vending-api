# develop merge verification — single-company baseline cleanup

Date: 2026-05-18 (UTC)

## Source / target

- **Source branch:** `test/openapi-json-body-shape-proof` (tip merged into PR below)
- **Target branch:** `develop`
- **Pull request:** [#223](https://github.com/leduytuanvu/avf-vending-api/pull/223) — *fix: finalize single-company baseline cleanup*

## Intended merge method

- **Squash merge** with branch deletion (per team workflow); execution blocked until branch protection requirements are satisfied (see **Final decision**).

## CI status (GitHub)

All required checks on PR **223** reported **pass** at verification time, including:

- Go CI Gates  
- Linux race and contract gates  
- Migration Safety Check  
- Workflow and Script Quality  
- Docker Compose config / governance / secret scan / vulnerability scan / deployment scan  

*(Some optional checks showed “skipping” — treated as non-blocking.)*

## Local / workstation gates (pre-push)

Aligned with `reports/test/final-remove-scope-id-100-percent-report.md` on the same PR:

- Generator suite (OpenAPI, sqlc, Postman scripts): PASS  
- `go vet ./...`: PASS  
- `go test ./... -count=1`: PASS (including post-rebase rerun)  
- Fresh Postgres (`avf-postgres`): goose applied through **`00003`**; legacy-name schema probes: **0 rows** each  
- Forbidden-token `git grep` gates: PASS  
- E2E (Git Bash): gRPC, MQTT, and `run-all-local.sh --fresh-data` — PASS (failed **0**, skipped **0**)

### E2E run directories (gitignored)

Under `.e2e-runs/` (local only):

- `run-20260518T042444Z-35248-29269` — gRPC standalone  
- `run-20260518T042652Z-37326-15100` — MQTT standalone  
- `run-20260518T042756Z-38232-26502` — full `--fresh-data`

## Production reset warning

Squashed baseline migrations require a **planned `public` schema reset** and **`goose up` from zero** on production-style databases; coordinate backup, downtime, seed of platform admin, and smoke tests.

## Final decision

**BLOCKED** — GitHub reports the pull request is **not mergeable** under current **base branch policy** (`gh pr merge` rejected; auto-merge is disabled on the repository). **CI is green.**

After a permitted reviewer (or administrator merge per policy) squash-merges PR **223**, update this section to **MERGED_TO_DEVELOP** and record the **`develop` tip SHA** from:

`git fetch origin && git checkout develop && git pull --ff-only origin develop && git rev-parse HEAD`
