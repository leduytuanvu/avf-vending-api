# 01 — Implementation plan snapshot

See attached plan: **gRPC machine_code response** (Cursor plan file, not edited in-repo).

## Phases

1. Proto + codegen — additive `machine_code` on 3 messages
2. gRPC mapping — `ClaimActivation`, `RefreshMachineToken`, `mapBootstrapToProto`
3. Integration + JWT safety tests
4. Docs + contract checks
5. Production smoke script + reports
6. PR → develop → main parity → deploy → production verification

## Commit message

`fix(grpc): return machine code in machine activation responses`
