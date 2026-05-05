# Local infrastructure — E2E blocker (agent run 2026-05-05)

## Symptom

Preflight expects HTTP **200** from:

- `GET ${BASE_URL}/health/live`
- `GET ${BASE_URL}/health/ready`
- `GET ${BASE_URL}/version`

On this workstation, `BASE_URL=http://127.0.0.1:8080` returned **404** for those paths, which means **no AVF API** (or a different app) is bound there.

## Remediation (from repo docs)

From the repository root:

1. **Dependencies (Postgres, optional broker profile):** see [`docs/runbooks/local-dev.md`](../runbooks/local-dev.md)

   ```bash
   make dev-up
   ```

2. **Database:** configure `.env` from [`.env.local.example`](../../.env.local.example) and run:

   ```bash
   make dev-migrate
   ```

3. **API + gRPC:** load env then:

   ```bash
   make run-api
   ```

   For gRPC on `127.0.0.1:9090`, set `GRPC_ENABLED=true` and `GRPC_ADDR=127.0.0.1:9090` before starting (see [`docs/local/grpc-local-test.md`](../local/grpc-local-test.md)).

4. **E2E env:** copy or symlink secrets into `tests/e2e/.env` (from [`tests/e2e/.env.example`](../../tests/e2e/.env.example)); or use **`E2E_ENV_FILE=tests/e2e/.env.local`** for non-secret defaults and add `ADMIN_TOKEN` / machine tokens as required by Web Admin and machine flows.

5. **Toolchain (Windows):**

   - **jq:** `winget install jqlang.jq` (Git Bash may need `export PATH="...:$PATH"` until the shell is restarted).
   - **Python 3:** use `py -3` (recommended) or install from python.org and disable Windows **App Execution Aliases** for `python.exe` / `python3.exe` if they open the Store stub.

## Rerun E2E

```bash
./scripts/ci/verify_e2e_assets.sh
E2E_ENV_FILE=tests/e2e/.env.local E2E_ENABLE_FLOW_REVIEW=true ./tests/e2e/run-flow-review.sh --static-only
E2E_ENV_FILE=tests/e2e/.env.local E2E_ENABLE_FLOW_REVIEW=true ./tests/e2e/run-rest-local.sh --readonly
E2E_ENV_FILE=tests/e2e/.env.local E2E_ENABLE_FLOW_REVIEW=true E2E_FAIL_ON_P0_FINDINGS=true ./tests/e2e/run-all-local.sh --fresh-data
```

After a successful fresh run:

```bash
latest_run="$(ls -dt .e2e-runs/run-* | head -n1)"
E2E_ENV_FILE=tests/e2e/.env.local ./tests/e2e/run-all-local.sh --reuse-data "$latest_run/test-data.json"
```
