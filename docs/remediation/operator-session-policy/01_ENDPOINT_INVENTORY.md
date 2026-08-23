# Operator session policy — endpoint inventory

Live HTTP handlers (not client-generated). Session = a human physically operating that machine. Remote Web Admin auth is not a session.

| Endpoint | File | Policy | Notes |
|---|---|---|---|
| `PUT /v1/admin/machines/{machineId}/topology` | `internal/httpserver/admin_inventory_http.go` | `SESSION_OPTIONAL_BY_ORIGIN` | Omit session → origin `api`. Invalid session rejected. No auto-start. |
| `PUT /v1/admin/machines/{machineId}/planograms/draft` | same | `SESSION_OPTIONAL_BY_ORIGIN` | Same as topology. |
| `POST /v1/admin/machines/{machineId}/planograms/publish` | same | `SESSION_OPTIONAL_BY_ORIGIN` | MQTT dispatch still required. Snapshot `operator_session_id` nullable. |
| `POST /v1/admin/machines/{machineId}/sync` | same | `SESSION_OPTIONAL_BY_ORIGIN` | MQTT dispatch still required. |
| `POST /v1/admin/machines/{machineId}/stock-adjustments` | same | `SESSION_OPTIONAL_BY_ORIGIN` | Repo writes session attribution when session present; HTTP writes `origin=api` when omitted. |
| `POST /v1/admin/machines/{machineId}/cash-collections` + `.../close` | `internal/httpserver/admin_cash_http.go` | `SESSION_REQUIRED` | Unchanged. |
| Technician fleet reattach when `role=technician` | `internal/httpserver/admin_machine_enterprise_http.go` | `SESSION_REQUIRED` | Admin reattach may omit session. |
| Operator login / logout / heartbeat / current | `internal/httpserver/operator_http.go` | lifecycle | Unchanged. Admin `POST .../operator-sessions/start` is explicit only. |
| gRPC kiosk operator / inventory / cash | machine proto | `SESSION_REQUIRED` | Physical APP. |

Canonical policy: [`docs/operator-session-policy.md`](../../operator-session-policy.md).
