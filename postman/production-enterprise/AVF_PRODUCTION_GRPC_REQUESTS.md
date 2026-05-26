# AVF Production gRPC requests

Target: `{{grpcTarget}}` (TLS, ALPN h2). Machine JWT in metadata `authorization: Bearer <redacted>`.

Postman Desktop: New → gRPC → server URL → import proto from `proto/avf/machine/v1/` → invoke.

Verified with **grpcurl** in production E2E; Newman does not run gRPC.

| Flow ID | Service | RPC | Metadata | Request (redacted) |
|---------|---------|-----|----------|-------------------|
| GRPC-TOKEN-001 | MachineTokenService | RefreshMachineToken | (none) | `{
"refreshToken": "{{machineRefreshToken}}"
}...` |

### GRPC-TOKEN-001 — MachineTokenService/RefreshMachineToken

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{"refreshToken": "{{machineRefreshToken}}"}' {grpcTarget} MachineTokenService.RefreshMachineToken
```

| GRPC-BOOT-001 | MachineBootstrapService | GetBootstrap | authorization: Bearer <machineAccessToken> | `{}...` |

### GRPC-BOOT-001 — MachineBootstrapService/GetBootstrap

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {grpcTarget} MachineBootstrapService.GetBootstrap
```

| GRPC-BOOT-002 | MachineBootstrapService | CheckIn | authorization: Bearer <machineAccessToken>; idempotency-key | `{
"bootId": "{{run_prefix}}-boot",
"networkState": "online",
"attributes": {
"source": "e2e-prod-grpc"
}
}...` |

### GRPC-BOOT-002 — MachineBootstrapService/CheckIn

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{"bootId": "{{run_prefix}}-boot", "networkState": "online", "attributes": {"source": "e2e-prod-grpc"}}' {grpcTarget} MachineBootstrapService.CheckIn
```

| GRPC-CAT-001 |  |  | authorization: Bearer <machineAccessToken> | `{}...` |

### GRPC-CAT-001 — Catalog sync + published item assertions

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {grpcTarget} None.None
```

| GRPC-CAT-002 | MachineCatalogService | GetCatalogDelta | authorization: Bearer <machineAccessToken> | `{
"machineId": "{{machineId}}",
"basisCatalogVersion": "{{catalogVersion}}"
}...` |

### GRPC-CAT-002 — MachineCatalogService/GetCatalogDelta

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{"machineId": "{{machineId}}", "basisCatalogVersion": "{{catalogVersion}}"}' {grpcTarget} MachineCatalogService.GetCatalogDelta
```

| GRPC-CAT-003 | MachineCatalogService | AckCatalogVersion | authorization: Bearer <machineAccessToken>; idempotency-key | `{
"acknowledgedCatalogVersion": "{{catalogVersion}}"
}...` |

### GRPC-CAT-003 — MachineCatalogService/AckCatalogVersion

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{"acknowledgedCatalogVersion": "{{catalogVersion}}"}' {grpcTarget} MachineCatalogService.AckCatalogVersion
```

| GRPC-MED-001 |  |  | authorization: Bearer <machineAccessToken> | `{}...` |

### GRPC-MED-001 — Media manifest + download cache

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {grpcTarget} None.None
```

| GRPC-INV-001 | MachineInventoryService | GetInventorySnapshot | authorization: Bearer <machineAccessToken> | `{}...` |

### GRPC-INV-001 — MachineInventoryService/GetInventorySnapshot (baseline)

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {grpcTarget} MachineInventoryService.GetInventorySnapshot
```

| GRPC-INV-002 | MachineInventoryService | AckInventorySync | authorization: Bearer <machineAccessToken>; idempotency-key | `{
"syncCursor": "{{run_prefix}}-inv-cursor"
}...` |

### GRPC-INV-002 — MachineInventoryService/AckInventorySync

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{"syncCursor": "{{run_prefix}}-inv-cursor"}' {grpcTarget} MachineInventoryService.AckInventorySync
```

| GRPC-COMM-CASH-001 |  |  | authorization: Bearer <machineAccessToken> | `{}...` |

### GRPC-COMM-CASH-001 — Commerce cash path (CreateOrder → ConfirmCash → Vend)

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {grpcTarget} None.None
```

| GRPC-COMM-QR-001 |  |  | authorization: Bearer <machineAccessToken> | `{}...` |

### GRPC-COMM-QR-001 — Commerce QR path + REST webhook

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {grpcTarget} None.None
```

| GRPC-COMM-FAIL-001 |  |  | authorization: Bearer <machineAccessToken> | `{}...` |

### GRPC-COMM-FAIL-001 — Vend failure path

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {grpcTarget} None.None
```

| GRPC-COMM-CANCEL-001 |  |  | authorization: Bearer <machineAccessToken> | `{}...` |

### GRPC-COMM-CANCEL-001 — Cancel order idempotency

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {grpcTarget} None.None
```

| GRPC-IDEM-001 |  |  | authorization: Bearer <machineAccessToken> | `{}...` |

### GRPC-IDEM-001 — CreateOrder idempotency replay

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {grpcTarget} None.None
```

| GRPC-OFFLINE-001 |  |  | authorization: Bearer <machineAccessToken> | `{}...` |

### GRPC-OFFLINE-001 — Offline event replay

```bash
grpcurl -H 'authorization: Bearer $MACHINE_TOKEN' -d '{}' {grpcTarget} None.None
```
