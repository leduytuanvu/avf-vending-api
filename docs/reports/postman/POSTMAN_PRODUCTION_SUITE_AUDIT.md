> **Relocated from avf-vending-system** — canonical home: `avf-vending-api/docs/reports/postman/`. System copy removed 2026-06-16.

﻿# Postman Production Suite Audit

Generated: 2026-06-12T09:00:33.4352798Z

## Source files
- Collection: `D:\admin\development\avf\avf-vending-system\local-postman-runtime\source\postman_collection.json`
- Environment: `D:\admin\development\avf\avf-vending-system\local-postman-runtime\source\postman_environment.json`

## Resolved environment
| Key | Exported | Override for target |
|-----|----------|---------------------|
| baseUrl | https://api.ldtv.dev | no |
| grpcTarget | machine-api.ldtv.dev:443 | no |
| adminEmail | (empty in export) | AVF_ADMIN_EMAIL / prompt |
| adminPassword | (empty in export) | AVF_ADMIN_PASSWORD / prompt |
| machineId | 00000000-0000-4000-8000-000000000099 | 019eb4d7-f821-78f4-9b2c-48166006af73 |
| siteId | 00000000-0000-4000-8000-000000000099 | 019e550b-729d-7d30-9295-4d2bb8780203 |
| packageName | com.avf.vending | com.avf.vending.tcn |
| machineToken | (empty) | from activation; required for GetBootstrap |

## Auth login
- POST `{{baseUrl}}/v1/auth/login`
- Headers: Accept, Content-Type, X-Request-ID, X-Correlation-ID, Idempotency-Key
- Exported body uses hardcoded example-password (unsafe for automation)
- Token capture: `tokens.accessToken`, `tokens.refreshToken`

## gRPC GetBootstrap
- `grpcs://{{grpcTarget}}/avf.machine.v1.MachineBootstrapService/GetBootstrap`
- Authorization: Bearer `{{machineToken}}` (not admin accessToken)

## REST bootstrap auth policy (API source)
- `GET /v1/setup/machines/{machineId}/bootstrap`
- Middleware: RequireMachineCompanyAccess + RequireInteractivePermissionOrMachinePrincipal(PermSetupWrite)
- Accepts machine JWT (preferred) OR admin JWT with setup write permission

## Mandatory statements
1. Login body in exported collection must **not** use `example-password`.
2. Exported environment values are templates/placeholders and must be overridden for the target machine.
3. **machineToken is required for GetBootstrap**.

