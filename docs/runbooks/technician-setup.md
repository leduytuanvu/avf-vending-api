# Technician setup runbook

Technician setup uses Admin Web REST APIs for technician directory and assignments, plus machine-scoped operator sessions for field actions such as inventory adjustment and cash collection.

## Admin setup

Routes are under `/v1/admin` and require the appropriate fleet/technician permissions.

- `POST /v1/admin/technicians`
- `GET /v1/admin/technicians`
- `PATCH /v1/admin/technicians/{technicianId}`
- `POST /v1/admin/technicians/{technicianId}/disable`
- `POST /v1/admin/technician-assignments`
- `GET /v1/admin/technician-assignments`
- `DELETE /v1/admin/technician-assignments/{assignmentId}`
- Organization-scoped alternates also exist under `/v1/admin/organizations/{organizationId}/machines/{machineId}/technicians`.

## Field operator session

Machine operator session routes are mounted under:

- `POST /v1/machines/{machineId}/operator-sessions/login`
- `GET /v1/machines/{machineId}/operator-sessions/current`
- `POST /v1/machines/{machineId}/operator-sessions/logout`
- `POST /v1/machines/{machineId}/operator-sessions/{sessionId}/heartbeat`

Inventory and cash workflows require an active operator session ID where the route body asks for `operator_session_id`.

## Git Bash quick check

```bash
BASE_URL="http://localhost:8080"
TOKEN="<admin bearer token>"
ORG_ID="11111111-1111-1111-1111-111111111111"

curl -sS "$BASE_URL/v1/admin/technicians?organization_id=$ORG_ID&limit=10" \
  -H "Authorization: Bearer $TOKEN"
```

## PowerShell quick check

```powershell
$BaseUrl = "http://localhost:8080"
$Token = "<admin bearer token>"
$OrgId = "11111111-1111-1111-1111-111111111111"

Invoke-RestMethod -Method Get `
  -Uri "$BaseUrl/v1/admin/technicians?organization_id=$OrgId&limit=10" `
  -Headers @{ Authorization = "Bearer $Token" }
```

## Troubleshooting

- `403`: the account lacks fleet/technician permissions, or a machine principal tried to call admin routes.
- `404` on assignment: machine or technician is outside the caller organization.
- Field mutation rejected for missing operator session: create/login an operator session on the same machine and retry with the returned session ID.

Audit records are written for technician and assignment mutations. Do not bypass assignment APIs with direct SQL except in an approved incident procedure.
