# AVF gRPC Test Package

## Postman setup (manual)

1. Postman Desktop → **New** → **gRPC Request**
2. Server: `{{GRPC_HOST}}:{{GRPC_PORT}}` (set TLS if required)
3. Import protos from `postman/generated/grpc/proto/` (root) or bundled `avf_all_services.proto` after running suite generator
4. For each method in `AVF_GRPC_INVENTORY.md`, paste JSON from `AVF_GRPC_EXAMPLES.json`
5. Metadata: `authorization: Bearer {{ACCESS_TOKEN}}` or `{{MACHINE_TOKEN}}` for machine package

## grpcurl smoke

```bash
export GRPC_HOST=localhost GRPC_PORT=50051 ACCESS_TOKEN=... MACHINE_TOKEN=...
bash postman/generated/grpc/AVF_GRPCURL_SMOKE.sh list
bash postman/generated/grpc/AVF_GRPCURL_SMOKE.sh dry-run
bash postman/generated/grpc/AVF_GRPCURL_SMOKE.sh
```

**Note:** `AVF_GRPC_POSTMAN_IMPORT.json` is a reference catalog, not verified native gRPC import.
