# 00 — Current state audit (gRPC machine_code)

**Date:** 2026-07-06 UTC  
**Branch:** `fix/grpc-machine-code-response`

## Verified gap before change

| Layer | `machine_code` / `machineCode` |
|-------|--------------------------------|
| DB `machines.code` | Present |
| Activation service (`ClaimResult.MachineCode`) | Populated on claim (~649) and refresh (~930) |
| REST claim / refresh | Returns `machineCode` |
| gRPC proto + server mapping | **Missing** — Android could not read human code from gRPC |

## Identity constraints (unchanged)

- JWT `machine_id` claim = UUID
- MQTT username / topics = UUID
- No new request field accepts `machine_code` for authorization
- Additive proto fields only (17 / 13 / 11)

## Proto field numbers verified free

| Message | Field |
|---------|-------|
| `ClaimActivationResponse` | `machine_code = 17` |
| `RefreshMachineTokenResponse` | `machine_code = 13` |
| `BootstrapMachine` | `machine_code = 11` |
