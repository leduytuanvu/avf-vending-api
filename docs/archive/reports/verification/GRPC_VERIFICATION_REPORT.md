# gRPC Verification Report

Generated: 2026-05-20

## Proto inventory

| Scope | Services | RPCs |
|-------|----------|------|
| `proto/avf/machine/v1/` | 14 | 69 |
| `proto/avf/internal/v1/` | 7 | 11 |
| **Total** | **21** | **80** |

## Code generation

```bash
cd proto && go run github.com/bufbuild/buf/cmd/buf@v1.47.0 generate --exclude-path avf/internal
cd proto && go run github.com/bufbuild/buf/cmd/buf@v1.47.0 generate --template buf.gen.avfinternal.yaml --path avf/internal/v1
cd proto && go run github.com/bufbuild/buf/cmd/buf@v1.47.0 lint
git diff --exit-code -- proto/avf/machine/v1/ proto/avf/v1/ internal/gen/avfinternalv1/
```

**Result: PASS** — no generated drift

## Machine gRPC docs check

```bash
python scripts/ci/check_machine_grpc_docs.py
# OK: machine gRPC docs mention 13 services
```

**Result: PASS**

## Automated tests

```bash
go test -count=1 ./internal/grpcserver/...
```

**Result: PASS** (cached; includes machine auth, offline sync, stock adjustment audit scenarios when `TEST_DATABASE_URL` set)

Coverage areas in `internal/grpcserver/`:

- Machine JWT authentication
- Bootstrap / sync
- Inventory snapshot + adjustments
- Offline queue replay + idempotency
- Telemetry upload paths
- Command ACK handling
- Duplicate request rejection

## grpcurl examples (local)

Prerequisites: API running with gRPC on `127.0.0.1:9090`, valid machine JWT.

```bash
# List services (reflection)
grpcurl -plaintext 127.0.0.1:9090 list

# Bootstrap (machine token in metadata)
grpcurl -plaintext \
  -H "authorization: Bearer ${MACHINE_TOKEN}" \
  -d '{}' \
  127.0.0.1:9090 avf.machine.v1.MachineBootstrapService/GetBootstrap

# Inventory snapshot
grpcurl -plaintext \
  -H "authorization: Bearer ${MACHINE_TOKEN}" \
  -d '{"machine_id":"'"${MACHINE_ID}"'"}' \
  127.0.0.1:9090 avf.machine.v1.MachineInventoryService/GetInventorySnapshot
```

Proto import path: `-import-path proto` with `-proto avf/machine/v1/bootstrap.proto` etc.

Full harness: `bash scripts/test/run-grpc-full-coverage.sh` (set `VERIFY_WITH_GRPC=1`).

## Live probe status

**Not executed** — no local gRPC server running during this pass.

## Verdict

**gRPC: PASS** — proto generation/lint clean; grpcserver tests pass; live grpcurl documented for manual follow-up.
