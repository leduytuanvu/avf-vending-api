# Production release readiness (static verification)

This runbook describes the **repository-local** gate to run **before** calling a release “enterprise-ready”. It validates tests, deployment shell scripts, example Compose files, and obvious secret leakage in template env files. It does **not** connect to production, require real secrets, or start containers.

## Command

From the repository root:

```bash
make verify-enterprise-release
```

Equivalent:

```bash
bash scripts/verify_enterprise_release.sh
```

The command exits **non-zero** on any failed phase.

## What it runs (phases)

1. **Shell syntax** — `bash -n` on every `*.sh` under `deployments/` and `scripts/`.
2. **Example / template safety** — scans `deployments/**` files named `*.example` or `.env.*.example` for obvious live-secret patterns (payment keys, AWS key IDs, Slack/GitHub tokens, PEM private keys, JWT-shaped blobs). Fails on empty `VAR=` assignments in those files (use explicit `CHANGE_ME_*` placeholders).
3. **YAML structure (optional)** — if `python3` and **PyYAML** are available (`apt install python3-yaml` / `pip install pyyaml`), parses all `*.yml` / `*.yaml` under `deployments/`. If PyYAML is missing, this phase is **skipped** with a message (install it for full coverage).
4. **Docker Compose config** — `docker compose … config` (offline validation only) using:
   - `deployments/prod/app-node/.env.app-node.example` + `docker-compose.app-node.yml` (default, `temporal`, and `migration` profiles),
   - `deployments/prod/data-node/.env.data-node.example` + `docker-compose.data-node.yml`,
   - when present: `deployments/prod/.env.production.example` + `docker-compose.prod.yml` (legacy single-host rollback path).
5. **Go tests** — `go test ./...` (set `TEST_DATABASE_URL` when you want Postgres-backed integration tests to run; without it, tests that require a DB should skip as today).

Phases print clear `===` headers so CI logs stay readable.

### Windows / local quirks

- **`make verify-enterprise-release`** requires **`bash`** on `PATH` (Git Bash or WSL is typical on Windows).
- Some enterprise Windows builds use **Application Control** (WDAC / AppLocker) that blocks `go test` binaries under `%TEMP%\go-build*`. If `go test ./...` fails with “policy has blocked this file”, run the same target in **Linux CI** (this workflow) or adjust policy for Go’s temp build directory — the repository gate is designed to pass on `ubuntu-latest`.

## Optional environment overrides

| Variable | Effect |
| -------- | ------ |
| `VERIFY_ENTERPRISE_SKIP_DOCKER=1` | Skip Compose config phases (e.g. laptop without Docker). **Not** sufficient for a full enterprise sign-off. |
| `VERIFY_ENTERPRISE_SKIP_YAML=1` | Skip YAML parse phase. |
| `VERIFY_ENTERPRISE_SKIP_GO=1` | Skip `go test` (debug only). |

## CI

Workflow: [`.github/workflows/enterprise-release-verify.yml`](../../.github/workflows/enterprise-release-verify.yml) runs the same script on pushes and pull requests to `main`, and on `workflow_dispatch`.

## Relationship to other gates

- **`make ci-gates`** / Go CI: formatting, vet, sqlc, swagger, placeholder scans — still the day-to-day merge gate.
- **This verification**: broader **release** posture (all shell under `deployments/` + `scripts/`, example Compose, full `go test ./...`).
- **Post-deploy**: health checks, smoke tests, and telemetry ramp procedures remain in [telemetry-production-rollout.md](./telemetry-production-rollout.md), [production-2-vps.md](./production-2-vps.md), and operator runbooks — not replaced by this script.
- **Staging telemetry storm evidence** (100×100 / 500×200 / 1000×500 in sequence): operator wrapper `deployments/prod/scripts/run_staging_telemetry_storm_suite.sh` — documented in [telemetry-production-rollout.md](./telemetry-production-rollout.md#staging-storm-evidence-suite-scale-100--500--1000); separate from this static repo gate. **GitHub:** manual workflow [`telemetry-storm-staging.yml`](../../.github/workflows/telemetry-storm-staging.yml) uploads artifacts for the production **scale storm gate** (`telemetry-storm-result`); see [telemetry-production-rollout.md — GitHub Actions manual staging storm](./telemetry-production-rollout.md#github-actions-manual-staging-storm). **Deploy Production** enforces **minimum scenario strength**, **fresh `completed_at_utc`**, and strict **`pass` / zero-loss** fields for `scale-*` targets; see [telemetry-production-rollout.md — Production CI fleet-scale gate](./telemetry-production-rollout.md#fleet-scale-storm-gate).
- **Live monitoring readiness** (metrics + health against a running cluster): `bash deployments/prod/scripts/check_monitoring_readiness.sh` with `API_METRICS_URL`, `MQTT_INGEST_METRICS_URL`, and `WORKER_METRICS_URL` set to real **`/metrics`** URLs (see script header). This is **not** part of `make verify-enterprise-release`; run it from a bastion or app node before scaling the fleet or claiming enterprise readiness. Details: [production-observability-alerts.md — Monitoring readiness check](./production-observability-alerts.md#monitoring-readiness-check).

## Enterprise release evidence pack

Before declaring production **enterprise-ready** for fleet tiers through **~1000 machines**, assemble one folder with static verify, monitoring readiness, storm evidence (when `fleet_scale_target` is not `pilot`), the **production deployment manifest**, and a **known risks** markdown file.

### 1) Static verify JSON

After a successful `make verify-enterprise-release` (or CI job), emit a machine-readable result:

```bash
bash deployments/prod/scripts/emit_verify_enterprise_result_json.sh ./evidence/verify-result.json
```

Schema: `final_result` must be `"pass"`, plus `completed_at_utc` and `tool` (see script output).

### 2) Monitoring and storm (scale targets above `pilot`)

- **Monitoring:** run `deployments/prod/scripts/check_monitoring_readiness.sh` → `monitoring-readiness-result.json`.
- **Storm:** use `telemetry-storm-result.json` from staging storm CI or `run_staging_telemetry_storm_suite.sh` (must show `final_result: pass` or suite `final_suite_pass: true`).

For **`fleet_scale_target=pilot`** in the deployment manifest, these files are **optional**; if you pass paths, they must still **`final_result: pass`**.

### 3) Deployment manifest

Download **`production-deployment-manifest.json`** from the successful **Deploy Production** workflow artifact (digest-pinned `app_image_ref` / `goose_image_ref`).

### 4) Known risks

Maintain a non-empty `known-risks.md` (org-specific items + pointers to repo P0 gaps, e.g. [mqtt-contract.md](../api/mqtt-contract.md#application-level-ack-durable-device-outbox-and-business-durability-p0-clarity)).

### 5) Build the pack

From repo root:

```bash
export RELEASE_TAG="v1.2.3"
export SOURCE_COMMIT_SHA="$(git rev-parse HEAD)"
export VERIFY_RESULT_PATH="./evidence/verify-result.json"
export MONITORING_RESULT_PATH="./evidence/monitoring-readiness-result.json"
export STORM_RESULT_PATH="./evidence/telemetry-storm-result.json"
export DEPLOYMENT_MANIFEST_PATH="./evidence/production-deployment-manifest.json"
export KNOWN_RISKS_PATH="./evidence/known-risks.md"
export OUTPUT_DIR="./dist/release-evidence-pack"
# If manifest has rollback_available_before_deploy=false:
# export ROLLBACK_UNAVAILABLE_EXPLANATION="First production deploy; no prior manifest retained."
# Optional: export EXPECTED_FLEET_SCALE_TARGET=scale-100
# Optional: export ALLOW_OUTPUT_DIR_OVERWRITE=1

bash deployments/prod/scripts/build_release_evidence_pack.sh
```

The script exits **non-zero** if any required gate fails, digest pins are missing, tag/SHA disagree with the manifest, or `KNOWN_RISKS_PATH` is empty. It never emits **`final_verdict: pass`** with incomplete evidence for the manifest’s fleet tier.

**Attach** the resulting directory (zip) to your internal release record, GitHub Release, or change ticket together with `release-evidence-pack.json` and `release-evidence-summary.md`.

## Do not

- Commit real credentials into `*.example` files; keep `CHANGE_ME_*` and similar placeholders.
- Rely on this script alone for production safety — it is **static** verification only.
