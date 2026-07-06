# 04 — Production test plan

**Target:** `https://api.ldtv.dev`  
**Deploy SHA:** `98169070b234d2940d8aab767d2dd25e52a85d11`  
**Deploy run:** GitHub Actions `28760850147`  
**Prefix:** `GRPC-MCODE-20260706T005447Z`

## Matrix

| Suite | Command | Pass criteria |
|-------|---------|---------------|
| Full REST/gRPC/MQTT (×3) | `run_production_full_suite.py --passes 3` | REST 363, gRPC 75, MQTT 17 — fail=0 each pass |
| Machine-code activation | `run_machine_code_activation_prod.py` | All admin + REST claim steps pass |
| gRPC machine_code smoke | `run_grpc_machine_code_prod.py` | Claim + Refresh + Bootstrap return `machine_code`; MQTT username = UUID |
| Version gate | `GET /version` | `git_sha` == deploy commit |

## Identity checks (manual in smoke)

- `ClaimActivation.machine_code` matches `machines.code`
- `RefreshMachineToken.machine_code` matches
- `GetBootstrap.machine.machine_code` matches
- JWT / MQTT username remain UUID (not AVF code)
