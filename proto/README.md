# AVF protobuf workspace

This directory is the **buf** workspace for machine-facing and internal gRPC protobuf sources.

## Layout

| Path | Role | Generated Go output |
|------|------|---------------------|
| `avf/machine/v1/*.proto` | Machine app gRPC (canonical production surface) | `proto/avf/machine/v1/*.pb.go` (source-relative, tracked) |
| `avf/internal/v1/*.proto` | Internal query gRPC (`avf.internal.v1`) | `internal/gen/avfinternalv1/*.pb.go` via `buf.gen.avfinternal.yaml` |
| `avf/v1/*.proto` | **Codegen smoke / compatibility stubs only** | `proto/avf/v1/*.pb.go` (source-relative, tracked) |

## Why internal stubs live under `internal/gen/avfinternalv1/`

Go cannot import packages whose directory path contains a segment named `internal` from normal application code. Proto sources remain in `avf/internal/v1/`, but generated Go stubs are emitted to `internal/gen/avfinternalv1/` using `paths=import` (see `buf.gen.avfinternal.yaml`).

**Do not** commit duplicate `*.pb.go` files under `proto/avf/internal/v1/` — they are not imported and are excluded from generation (`make proto-generate` uses `--exclude-path avf/internal` for the default template).

## `avf/v1` (legacy / smoke test)

- `skeleton.proto` — buf + Go protobuf codegen smoke test only; no gRPC service registered.
- `internal_queries.proto` — older `avf.v1` internal query surface; **not registered** on the API listener. Canonical internal queries are `avf.internal.v1` under `avf/internal/v1/`.

These files remain in the drift gate (`make proto-check` diffs `proto/avf/v1/`) to prove the default buf template still works.

## Commands

```bash
make proto-generate   # regenerate all tracked outputs
make proto-check      # lint + breaking + drift gate
```

See also `Makefile`, `buf.gen.yaml`, `buf.gen.avfinternal.yaml`, and `docs/api/grpc-canonical-surface.md`.
