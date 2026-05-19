# gRPC — AVF FULL 100 adjacent tests

Postman REST collection `AVF_FULL_100.postman_collection.json` imports directly into Postman for **HTTP**. Newman runs **HTTP only**; **grpcurl** carries machine/admin RPC coverage.

## Runner

- `bash postman/full-production-suite/grpc/run-grpc-postman-adjacent.sh` — delegates to `tests/e2e/run-grpc-local.sh`.

## Assets

- `AVF_GRPC_100_METHOD_MATRIX.csv` — one row per RPC + sample **grpcurl** column.
- `AVF_GRPC_100_REQUESTS.json` — protobuf JSON templates keyed like templates list.
- Proto bundle: `grpc/avf_all_services.proto` + tree under `grpc/proto/avf/`.

## Native Postman gRPC

Desktop Postman can open **gRPC** requests manually using the same protos/metadata as templates — there is **no** checked-in Postman gRPC JSON export in this repo.
