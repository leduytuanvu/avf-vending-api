# Machine activation runbook

Machine activation is implemented over REST and issues a machine-scoped JWT plus MQTT/bootstrap hints. Protected machine runtime HTTP and machine gRPC calls require a valid machine principal after activation.

## Routes

- Admin create/list/revoke activation codes:
  - `POST /v1/admin/machines/{machineId}/activation-codes`
  - `GET /v1/admin/machines/{machineId}/activation-codes`
  - `DELETE /v1/admin/machines/{machineId}/activation-codes/{activationCodeId}`
- Public device claim:
  - `POST /v1/setup/activation-codes/claim`
- Authenticated bootstrap:
  - `GET /v1/setup/machines/{machineId}/bootstrap`

## Operator procedure

1. Confirm the machine exists in the expected organization and is not retired or compromised.
2. Create an activation code with a short expiry and minimal `maxUses`.
3. Deliver the plaintext code through the approved secure field channel. The API only returns plaintext on create.
4. The device calls `/v1/setup/activation-codes/claim` with its fingerprint.
5. Verify the response contains a machine access token, MQTT hints, and bootstrap URL.
6. Verify bootstrap with the machine token.
7. Revoke unused codes after the activation window.

## Git Bash example

```bash
BASE_URL="http://localhost:8080"
ORG_ID="11111111-1111-1111-1111-111111111111"
MACHINE_ID="55555555-5555-5555-5555-555555555555"
TOKEN="<admin bearer token>"

curl -sS -X POST "$BASE_URL/v1/admin/machines/$MACHINE_ID/activation-codes?organization_id=$ORG_ID" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: activation-$(date +%s)" \
  -d '{"expiresInMinutes":60,"maxUses":1,"notes":"field activation"}'
```

## PowerShell example

```powershell
$BaseUrl = "http://localhost:8080"
$OrgId = "11111111-1111-1111-1111-111111111111"
$MachineId = "55555555-5555-5555-5555-555555555555"
$Token = "<admin bearer token>"

Invoke-RestMethod -Method Post `
  -Uri "$BaseUrl/v1/admin/machines/$MachineId/activation-codes?organization_id=$OrgId" `
  -Headers @{ Authorization = "Bearer $Token"; "Idempotency-Key" = "activation-$(Get-Date -Format yyyyMMddHHmmss)" } `
  -ContentType "application/json" `
  -Body '{"expiresInMinutes":60,"maxUses":1,"notes":"field activation"}'
```

## Failure handling

- `400 activation_invalid`: invalid, expired, exhausted, or revoked code. Do not disclose which condition to the field user.
- `403/404` on admin create/list: wrong organization scope or missing machine.
- Bootstrap failure after claim: verify token audience/issuer, machine status, and `machine_id` URL match.

Related: `docs/api/machine-activation-implementation-handoff.md`, `docs/api/setup-machine.md`, `docs/api/machine-grpc.md`.
