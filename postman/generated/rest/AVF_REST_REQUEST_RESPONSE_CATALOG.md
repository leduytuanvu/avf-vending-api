# AVF REST Request/Response Catalog

Complete catalog derived from OpenAPI + inventory validation.

| Method | Path | Folder | Auth | Request Body | Success Response |
| --- | --- | --- | --- | --- | --- |
| GET | `/health/live` | 00_Health_System | none | no | `{}` |

### GET /health/live

- **Purpose:** Liveness probe
- **Auth:** none (required=False)
- **Response 200:** Liveness probe
```json
{}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /health/live

| GET | `/health/ready` | 00_Health_System | none | no | `{}` |

### GET /health/ready

- **Purpose:** Readiness probe
- **Auth:** none (required=False)
- **Response 200:** Readiness probe
```json
{}
```
- **Response 503:** error
```json
{}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /health/ready

| GET | `/metrics` | 00_Health_System | none | no | `{}` |

### GET /metrics

- **Purpose:** Prometheus metrics scrape (public listener; optional)
- **Auth:** none (required=False)
- **Response 200:** Prometheus metrics scrape (public listener; optional)
```json
{}
```
- **Response 401:** error
```json
{}
```
- **Response 404:** error
```json
{}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /metrics

| GET | `/swagger/doc.json` | 00_Health_System | none | no | `{"info": {"title": "AVF Vending HTTP API", "version": "1.0"}, "openapi": "3.0.3"}` |

### GET /swagger/doc.json

- **Purpose:** OpenAPI 3.0 document (embedded)
- **Auth:** none (required=False)
- **Response 200:** OpenAPI 3.0 document (embedded)
```json
{
  "info": {
    "title": "AVF Vending HTTP API",
    "version": "1.0"
  },
  "openapi": "3.0.3"
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /swagger/doc.json

| GET | `/swagger/index.html` | 00_Health_System | none | no | `{}` |

### GET /swagger/index.html

- **Purpose:** Swagger UI (HTML)
- **Auth:** none (required=False)
- **Response 200:** Swagger UI (HTML)
```json
{}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /swagger/index.html

| GET | `/v1/admin/activation-codes` | 07_Machines_Provisioning | bearer | no | `{"items": [{"activationCodeId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "createdAt": "2026-04-29T00:00:00Z", "expiresAt"` |

### GET /v1/admin/activation-codes

- **Purpose:** List activation codes across machines (admin catalog)
- **Auth:** bearer (required=True)
- **Response 200:** List activation codes across machines (admin catalog)
```json
{
  "items": [
    {
      "activationCodeId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
      "createdAt": "2026-04-29T00:00:00Z",
      "expiresAt": "2026-04-30T00:00:00Z",
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "maxUses": 1,
      "notes": "",
      "remainingUses": 1,
      "status": "active",
      "uses": 0
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "totalCount": 1
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/activation-codes

| POST | `/v1/admin/activation-codes` | 07_Machines_Provisioning | bearer | yes | `{"activationCode": "AVF-123456", "activationCodeId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "expiresAt": "2026-04-30T00` |

### POST /v1/admin/activation-codes

- **Purpose:** Create activation code (catalog path; targets machine in body)
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "expiresInMinutes": 1440,
  "machineId": "{{machineId}}",
  "maxUses": 1,
  "notes": "pilot"
}
```
- **Response 201:** Create activation code (catalog path; targets machine in body)
```json
{
  "activationCode": "AVF-123456",
  "activationCodeId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "expiresAt": "2026-04-30T00:00:00Z",
  "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "maxUses": 1,
  "remainingUses": 1,
  "status": "active"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/activation-codes

| POST | `/v1/admin/activation-codes/{codeId}/revoke` | 07_Machines_Provisioning | bearer | no | `{"status": "ok"}` |

### POST /v1/admin/activation-codes/{codeId}/revoke

- **Purpose:** Revoke activation code by id (catalog path)
- **Auth:** bearer (required=True)
- **Path params:** `codeId`
- **Response 200:** Revoke activation code by id (catalog path)
```json
{
  "status": "ok"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/activation-codes/{codeId}/revoke

| GET | `/v1/admin/anomalies` | 99_Utilities | bearer | no | `{"items": [{"anomalyType": "machine_offline_too_long", "createdAt": "2026-04-29T12:00:00.000000000Z", "detectedAt": "202` |

### GET /v1/admin/anomalies

- **Purpose:** List operational anomalies
- **Auth:** bearer (required=True)
- **Response 200:** List operational anomalies
```json
{
  "items": [
    {
      "anomalyType": "machine_offline_too_long",
      "createdAt": "2026-04-29T12:00:00.000000000Z",
      "detectedAt": "2026-04-29T12:00:00.000000000Z",
      "fingerprint": "offline_long|7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "machineName": "Lobby A",
      "machineSerialNumber": "SN-001",
      "payload": {
        "last_seen_at": "2026-04-29T08:00:00Z",
        "threshold": "2 hours"
      },
      "status": "open",
      "updatedAt": "2026-04-29T12:00:00.000000000Z"
    }
  ]
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/anomalies

| GET | `/v1/admin/anomalies/{anomalyId}` | 99_Utilities | bearer | no | `{"anomalyType": "repeated_vend_failure", "createdAt": "2026-04-29T12:00:00.000000000Z", "detectedAt": "2026-04-29T12:00:` |

### GET /v1/admin/anomalies/{anomalyId}

- **Purpose:** Get operational anomaly
- **Auth:** bearer (required=True)
- **Path params:** `anomalyId`
- **Response 200:** Get operational anomaly
```json
{
  "anomalyType": "repeated_vend_failure",
  "createdAt": "2026-04-29T12:00:00.000000000Z",
  "detectedAt": "2026-04-29T12:00:00.000000000Z",
  "fingerprint": "repeated_vend_failure|7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "machineName": "Lobby A",
  "machineSerialNumber": "SN-001",
  "payload": {
    "failed_vend_count_24h": 4
  },
  "status": "open",
  "updatedAt": "2026-04-29T12:00:00.000000000Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/anomalies/{anomalyId}

| POST | `/v1/admin/anomalies/{anomalyId}/ignore` | 99_Utilities | bearer | no | `{"anomalyId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "status": "ignored"}` |

### POST /v1/admin/anomalies/{anomalyId}/ignore

- **Purpose:** Ignore operational anomaly
- **Auth:** bearer (required=True)
- **Path params:** `anomalyId`
- **Response 200:** Ignore operational anomaly
```json
{
  "anomalyId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "status": "ignored"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/anomalies/{anomalyId}/ignore

| POST | `/v1/admin/anomalies/{anomalyId}/resolve` | 99_Utilities | bearer | no | `{"anomalyId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "status": "resolved"}` |

### POST /v1/admin/anomalies/{anomalyId}/resolve

- **Purpose:** Resolve operational anomaly
- **Auth:** bearer (required=True)
- **Path params:** `anomalyId`
- **Response 200:** Resolve operational anomaly
```json
{
  "anomalyId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "status": "resolved"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/anomalies/{anomalyId}/resolve

| GET | `/v1/admin/artifacts` | 99_Utilities | bearer | no | `{"items": [], "meta": {"limit": 50, "offset": 0, "returned": 0, "totalCount": 0}}` |

### GET /v1/admin/artifacts

- **Purpose:** List artifacts
- **Auth:** bearer (required=True)
- **Response 200:** List artifacts
```json
{
  "items": [],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 0,
    "totalCount": 0
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/artifacts

| POST | `/v1/admin/artifacts` | 99_Utilities | bearer | no | `{"artifact_id": "ffffffff-0000-1111-2222-333333333333", "upload_path": "org/acme/artifacts/ff/..."}` |

### POST /v1/admin/artifacts

- **Purpose:** Reserve artifact id
- **Auth:** bearer (required=True)
- **Response 201:** Reserve artifact id
```json
{
  "artifact_id": "ffffffff-0000-1111-2222-333333333333",
  "upload_path": "org/acme/artifacts/ff/..."
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/artifacts

| DELETE | `/v1/admin/artifacts/{artifactId}` | 99_Utilities | bearer | no | `{"artifact_id": "ffffffff-0000-1111-2222-333333333333", "status": "deleted"}` |

### DELETE /v1/admin/artifacts/{artifactId}

- **Purpose:** Delete artifact
- **Auth:** bearer (required=True)
- **Path params:** `artifactId`
- **Response 200:** Delete artifact
```json
{
  "artifact_id": "ffffffff-0000-1111-2222-333333333333",
  "status": "deleted"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/artifacts/{artifactId}

| GET | `/v1/admin/artifacts/{artifactId}` | 99_Utilities | bearer | no | `{"artifact_id": "ffffffff-0000-1111-2222-333333333333", "status": "uploaded"}` |

### GET /v1/admin/artifacts/{artifactId}

- **Purpose:** Get artifact metadata
- **Auth:** bearer (required=True)
- **Path params:** `artifactId`
- **Response 200:** Get artifact metadata
```json
{
  "artifact_id": "ffffffff-0000-1111-2222-333333333333",
  "status": "uploaded"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/artifacts/{artifactId}

| PUT | `/v1/admin/artifacts/{artifactId}/content` | 99_Utilities | bearer | no | `{"artifact_id": "11111111-2222-3333-4444-555555555555", "status": "uploaded"}` |

### PUT /v1/admin/artifacts/{artifactId}/content

- **Purpose:** Upload artifact bytes
- **Auth:** bearer (required=True)
- **Path params:** `artifactId`
- **Response 200:** Upload artifact bytes
```json
{
  "artifact_id": "11111111-2222-3333-4444-555555555555",
  "status": "uploaded"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:PUT /v1/admin/artifacts/{artifactId}/content

| GET | `/v1/admin/artifacts/{artifactId}/download` | 99_Utilities | bearer | no | `{"expires_at": "2026-04-19T13:00:00Z", "headers": {}, "method": "GET", "url": "https://storage.example/presigned-read"}` |

### GET /v1/admin/artifacts/{artifactId}/download

- **Purpose:** Presigned download URL
- **Auth:** bearer (required=True)
- **Path params:** `artifactId`
- **Response 200:** Presigned download URL
```json
{
  "expires_at": "2026-04-19T13:00:00Z",
  "headers": {},
  "method": "GET",
  "url": "https://storage.example/presigned-read"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/artifacts/{artifactId}/download

| GET | `/v1/admin/assignments` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"assignmentId": "dddddddd-eeee-ffff-0000-111111111111", "createdAt": "2026-04-01T00:00:00Z", "machineId": "7` |

### GET /v1/admin/assignments

- **Purpose:** List technician assignments (admin)
- **Auth:** bearer (required=True)
- **Query params:** `technician_id, machine_id, from, to, limit, offset`
- **Response 200:** List technician assignments (admin)
```json
{
  "items": [
    {
      "assignmentId": "dddddddd-eeee-ffff-0000-111111111111",
      "createdAt": "2026-04-01T00:00:00Z",
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "machineName": "Lobby A",
      "machineSerialNumber": "SN-001",
      "role": "maintainer",
      "technicianDisplayName": "Alex Tech",
      "technicianId": "eeeeeeee-ffff-0000-1111-222222222222",
      "validFrom": "2026-04-01T00:00:00Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/assignments

| POST | `/v1/admin/assignments` | 08_Machines_Runtime_Config | bearer | yes | `{"created_at": "2026-04-01T00:00:00.000000000Z", "id": "dddddddd-eeee-ffff-0000-111111111111", "machine_id": "7c9e6679-7` |

### POST /v1/admin/assignments

- **Purpose:** Create technician assignment
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "machine_id": "{{machineId}}",
  "role": "field_service",
  "technician_id": "{{$guid}}"
}
```
- **Response 201:** Create technician assignment
```json
{
  "created_at": "2026-04-01T00:00:00.000000000Z",
  "id": "dddddddd-eeee-ffff-0000-111111111111",
  "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "role": "maintainer",
  "status": "active",
  "technician_id": "eeeeeeee-ffff-0000-1111-222222222222",
  "updated_at": "2026-04-01T00:00:00.000000000Z",
  "valid_from": "2026-04-01T00:00:00.000000000Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/assignments

| DELETE | `/v1/admin/assignments/{assignmentId}` | 08_Machines_Runtime_Config | bearer | no | `{"created_at": "2026-04-01T00:00:00.000000000Z", "id": "dddddddd-eeee-ffff-0000-111111111111", "machine_id": "7c9e6679-7` |

### DELETE /v1/admin/assignments/{assignmentId}

- **Purpose:** Release assignment (delete)
- **Auth:** bearer (required=True)
- **Path params:** `assignmentId`
- **Response 200:** Release assignment (delete)
```json
{
  "created_at": "2026-04-01T00:00:00.000000000Z",
  "id": "dddddddd-eeee-ffff-0000-111111111111",
  "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "role": "maintainer",
  "status": "active",
  "technician_id": "eeeeeeee-ffff-0000-1111-222222222222",
  "updated_at": "2026-04-01T00:00:00.000000000Z",
  "valid_from": "2026-04-01T00:00:00.000000000Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/assignments/{assignmentId}

| GET | `/v1/admin/assignments/{assignmentId}` | 08_Machines_Runtime_Config | bearer | no | `{"created_at": "2026-04-01T00:00:00.000000000Z", "id": "dddddddd-eeee-ffff-0000-111111111111", "machine_id": "7c9e6679-7` |

### GET /v1/admin/assignments/{assignmentId}

- **Purpose:** Get technician–machine assignment by id
- **Auth:** bearer (required=True)
- **Path params:** `assignmentId`
- **Response 200:** Get technician–machine assignment by id
```json
{
  "created_at": "2026-04-01T00:00:00.000000000Z",
  "id": "dddddddd-eeee-ffff-0000-111111111111",
  "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "role": "maintainer",
  "status": "active",
  "technician_id": "eeeeeeee-ffff-0000-1111-222222222222",
  "updated_at": "2026-04-01T00:00:00.000000000Z",
  "valid_from": "2026-04-01T00:00:00.000000000Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/assignments/{assignmentId}

| GET | `/v1/admin/audit/events` | 19_Audit_Logs | bearer | no | `{"items": [{"action": "catalog.product.update", "actorId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "actorType": "user", ` |

### GET /v1/admin/audit/events

- **Purpose:** List enterprise audit events
- **Auth:** bearer (required=True)
- **Query params:** `action, actorId, actorType, outcome, resourceType, resourceId, machineId, from, to, limit, offset`
- **Response 200:** List enterprise audit events
```json
{
  "items": [
    {
      "action": "catalog.product.update",
      "actorId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
      "actorType": "user",
      "createdAt": "2026-04-19T12:00:00Z",
      "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "metadata": {},
      "occurredAt": "2026-04-19T12:00:00Z",
      "outcome": "success",
      "resourceId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "resourceType": "product",
      "siteId": null
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/audit/events

| GET | `/v1/admin/audit/events/{auditEventId}` | 19_Audit_Logs | bearer | no | `{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6"}` |

### GET /v1/admin/audit/events/{auditEventId}

- **Purpose:** Get one enterprise audit event by id
- **Auth:** bearer (required=True)
- **Path params:** `auditEventId`
- **Response 200:** Get one enterprise audit event by id
```json
{
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/audit/events/{auditEventId}

| GET | `/v1/admin/auth/users` | 99_Utilities | bearer | no | `{"items": [{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator` |

### GET /v1/admin/auth/users

- **Purpose:** List API accounts for an company (admin)
- **Auth:** bearer (required=True)
- **Query params:** `limit, offset`
- **Response 200:** List API accounts for an company (admin)
```json
{
  "items": [
    {
      "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "createdAt": "2026-01-01T00:00:00Z",
      "email": "operator@example.com",
      "roles": [
        "admin"
      ],
      "status": "active",
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/auth/users

| POST | `/v1/admin/auth/users` | 99_Utilities | bearer | yes | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### POST /v1/admin/auth/users

- **Purpose:** Create API account (admin)
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "email": "{{adminEmail}}",
  "password": "{{adminPassword}}",
  "roles": [
    "viewer"
  ],
  "status": "active"
}
```
- **Response 201:** Create API account (admin)
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/auth/users

| GET | `/v1/admin/auth/users/{accountId}` | 99_Utilities | bearer | no | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### GET /v1/admin/auth/users/{accountId}

- **Purpose:** Get API account by id (admin)
- **Auth:** bearer (required=True)
- **Path params:** `accountId`
- **Response 200:** Get API account by id (admin)
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/auth/users/{accountId}

| PATCH | `/v1/admin/auth/users/{accountId}` | 99_Utilities | bearer | yes | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### PATCH /v1/admin/auth/users/{accountId}

- **Purpose:** Patch API account (admin)
- **Auth:** bearer (required=True)
- **Path params:** `accountId`
- **Request body example:**
```json
{
  "status": "disabled"
}
```
- **Response 200:** Patch API account (admin)
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/auth/users/{accountId}

| POST | `/v1/admin/auth/users/{accountId}/activate` | 99_Utilities | bearer | no | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### POST /v1/admin/auth/users/{accountId}/activate

- **Purpose:** Activate API account (admin)
- **Auth:** bearer (required=True)
- **Path params:** `accountId`
- **Response 200:** Activate API account (admin)
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/auth/users/{accountId}/activate

| POST | `/v1/admin/auth/users/{accountId}/deactivate` | 99_Utilities | bearer | no | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### POST /v1/admin/auth/users/{accountId}/deactivate

- **Purpose:** Deactivate API account (admin)
- **Auth:** bearer (required=True)
- **Path params:** `accountId`
- **Response 200:** Deactivate API account (admin)
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/auth/users/{accountId}/deactivate

| POST | `/v1/admin/auth/users/{accountId}/reset-password` | 99_Utilities | bearer | yes | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### POST /v1/admin/auth/users/{accountId}/reset-password

- **Purpose:** Reset password (admin)
- **Auth:** bearer (required=True)
- **Path params:** `accountId`
- **Request body example:**
```json
{
  "password": "{{adminPassword}}"
}
```
- **Response 200:** Reset password (admin)
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/auth/users/{accountId}/reset-password

| POST | `/v1/admin/auth/users/{accountId}/revoke-sessions` | 99_Utilities | bearer | no | `{}` |

### POST /v1/admin/auth/users/{accountId}/revoke-sessions

- **Purpose:** Revoke API account sessions (admin)
- **Auth:** bearer (required=True)
- **Path params:** `accountId`
- **Response 204:** Revoke API account sessions (admin)
```json
{}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/auth/users/{accountId}/revoke-sessions

| PATCH | `/v1/admin/auth/users/{accountId}/roles` | 99_Utilities | bearer | yes | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### PATCH /v1/admin/auth/users/{accountId}/roles

- **Purpose:** Replace API account roles — PATCH alias
- **Auth:** bearer (required=True)
- **Path params:** `accountId`
- **Request body example:**
```json
{
  "roles": [
    "viewer"
  ]
}
```
- **Response 200:** Replace API account roles — PATCH alias
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/auth/users/{accountId}/roles

| POST | `/v1/admin/auth/users/{accountId}/roles` | 99_Utilities | bearer | yes | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### POST /v1/admin/auth/users/{accountId}/roles

- **Purpose:** Replace API account roles (admin)
- **Auth:** bearer (required=True)
- **Path params:** `accountId`
- **Request body example:**
```json
{
  "roles": [
    "viewer"
  ]
}
```
- **Response 200:** Replace API account roles (admin)
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/auth/users/{accountId}/roles

| PUT | `/v1/admin/auth/users/{accountId}/roles` | 99_Utilities | bearer | yes | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### PUT /v1/admin/auth/users/{accountId}/roles

- **Purpose:** Replace API account roles (admin)
- **Auth:** bearer (required=True)
- **Path params:** `accountId`
- **Request body example:**
```json
{
  "roles": [
    "viewer"
  ]
}
```
- **Response 200:** Replace API account roles (admin)
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:PUT /v1/admin/auth/users/{accountId}/roles

| GET | `/v1/admin/auth/users/{accountId}/sessions` | 99_Utilities | bearer | no | `{"sessions": [{"createdAt": "2026-04-19T10:00:00Z", "expiresAt": "2026-05-19T12:00:00Z", "sessionId": "bbbbbbbb-bbbb-bbb` |

### GET /v1/admin/auth/users/{accountId}/sessions

- **Purpose:** List sessions for an API account (admin)
- **Auth:** bearer (required=True)
- **Path params:** `accountId`
- **Response 200:** List sessions for an API account (admin)
```json
{
  "sessions": [
    {
      "createdAt": "2026-04-19T10:00:00Z",
      "expiresAt": "2026-05-19T12:00:00Z",
      "sessionId": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
      "status": "active"
    }
  ]
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/auth/users/{accountId}/sessions

| PATCH | `/v1/admin/auth/users/{accountId}/status` | 99_Utilities | bearer | yes | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### PATCH /v1/admin/auth/users/{accountId}/status

- **Purpose:** Patch API account status only
- **Auth:** bearer (required=True)
- **Path params:** `accountId`
- **Request body example:**
```json
{
  "status": "disabled"
}
```
- **Response 200:** Patch API account status only
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/auth/users/{accountId}/status

| GET | `/v1/admin/brands` | 03_Catalog_Categories_Brands_Tags | bearer | no | `{"items": [{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "name": "` |

### GET /v1/admin/brands

- **Purpose:** List brands
- **Auth:** bearer (required=True)
- **Query params:** `limit, offset`
- **Response 200:** List brands
```json
{
  "items": [
    {
      "active": true,
      "createdAt": "2026-01-01T00:00:00Z",
      "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "name": "Coca example",
      "slug": "coca-example",
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "totalCount": 1
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/brands

| POST | `/v1/admin/brands` | 03_Catalog_Categories_Brands_Tags | bearer | yes | `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "name": "Coca exampl` |

### POST /v1/admin/brands

- **Purpose:** Create brand
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "active": true,
  "name": "Coca {{$timestamp}}",
  "slug": "coca-{{$timestamp}}"
}
```
- **Response 200:** Create brand
```json
{
  "active": true,
  "createdAt": "2026-01-01T00:00:00Z",
  "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "name": "Coca example",
  "slug": "coca-example",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/brands

| DELETE | `/v1/admin/brands/{brandId}` | 03_Catalog_Categories_Brands_Tags | bearer | no | `{"active": false, "createdAt": "2026-01-01T00:00:00Z", "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "name": "Coca examp` |

### DELETE /v1/admin/brands/{brandId}

- **Purpose:** Deactivate brand
- **Auth:** bearer (required=True)
- **Path params:** `brandId`
- **Response 200:** Deactivate brand
```json
{
  "active": false,
  "createdAt": "2026-01-01T00:00:00Z",
  "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "name": "Coca example",
  "slug": "coca-example",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/brands/{brandId}

| PATCH | `/v1/admin/brands/{brandId}` | 03_Catalog_Categories_Brands_Tags | bearer | yes | `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "name": "Coca exampl` |

### PATCH /v1/admin/brands/{brandId}

- **Purpose:** Update brand (PATCH)
- **Auth:** bearer (required=True)
- **Path params:** `brandId`
- **Request body example:**
```json
{
  "active": true,
  "name": "Coca {{$timestamp}}",
  "slug": "coca-{{$timestamp}}"
}
```
- **Response 200:** Update brand (PATCH)
```json
{
  "active": true,
  "createdAt": "2026-01-01T00:00:00Z",
  "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "name": "Coca example",
  "slug": "coca-example",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/brands/{brandId}

| PUT | `/v1/admin/brands/{brandId}` | 03_Catalog_Categories_Brands_Tags | bearer | yes | `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "name": "Coca exampl` |

### PUT /v1/admin/brands/{brandId}

- **Purpose:** Update brand
- **Auth:** bearer (required=True)
- **Path params:** `brandId`
- **Request body example:**
```json
{
  "active": true,
  "name": "Coca {{$timestamp}}",
  "slug": "coca-{{$timestamp}}"
}
```
- **Response 200:** Update brand
```json
{
  "active": true,
  "createdAt": "2026-01-01T00:00:00Z",
  "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "name": "Coca example",
  "slug": "coca-example",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:PUT /v1/admin/brands/{brandId}

| GET | `/v1/admin/categories` | 03_Catalog_Categories_Brands_Tags | bearer | no | `{"items": [{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "name": "` |

### GET /v1/admin/categories

- **Purpose:** List categories
- **Auth:** bearer (required=True)
- **Query params:** `limit, offset`
- **Response 200:** List categories
```json
{
  "items": [
    {
      "active": true,
      "createdAt": "2026-01-01T00:00:00Z",
      "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
      "name": "Drinks example",
      "slug": "drinks-example",
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "totalCount": 1
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/categories

| POST | `/v1/admin/categories` | 03_Catalog_Categories_Brands_Tags | bearer | yes | `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "name": "Drinks exam` |

### POST /v1/admin/categories

- **Purpose:** Create category
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "active": true,
  "name": "Drinks {{$timestamp}}",
  "slug": "drinks-{{$timestamp}}"
}
```
- **Response 200:** Create category
```json
{
  "active": true,
  "createdAt": "2026-01-01T00:00:00Z",
  "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "name": "Drinks example",
  "slug": "drinks-example",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/categories

| DELETE | `/v1/admin/categories/{categoryId}` | 03_Catalog_Categories_Brands_Tags | bearer | no | `{"active": false, "createdAt": "2026-01-01T00:00:00Z", "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "name": "Drinks exa` |

### DELETE /v1/admin/categories/{categoryId}

- **Purpose:** Deactivate category
- **Auth:** bearer (required=True)
- **Path params:** `categoryId`
- **Response 200:** Deactivate category
```json
{
  "active": false,
  "createdAt": "2026-01-01T00:00:00Z",
  "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "name": "Drinks example",
  "slug": "drinks-example",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/categories/{categoryId}

| PATCH | `/v1/admin/categories/{categoryId}` | 03_Catalog_Categories_Brands_Tags | bearer | yes | `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "name": "Drinks exam` |

### PATCH /v1/admin/categories/{categoryId}

- **Purpose:** Update category (PATCH)
- **Auth:** bearer (required=True)
- **Path params:** `categoryId`
- **Request body example:**
```json
{
  "active": true,
  "name": "Drinks {{$timestamp}}",
  "slug": "drinks-{{$timestamp}}"
}
```
- **Response 200:** Update category (PATCH)
```json
{
  "active": true,
  "createdAt": "2026-01-01T00:00:00Z",
  "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "name": "Drinks example",
  "slug": "drinks-example",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/categories/{categoryId}

| PUT | `/v1/admin/categories/{categoryId}` | 03_Catalog_Categories_Brands_Tags | bearer | yes | `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "name": "Drinks exam` |

### PUT /v1/admin/categories/{categoryId}

- **Purpose:** Update category
- **Auth:** bearer (required=True)
- **Path params:** `categoryId`
- **Request body example:**
```json
{
  "active": true,
  "name": "Drinks {{$timestamp}}",
  "slug": "drinks-{{$timestamp}}"
}
```
- **Response 200:** Update category
```json
{
  "active": true,
  "createdAt": "2026-01-01T00:00:00Z",
  "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "name": "Drinks example",
  "slug": "drinks-example",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:PUT /v1/admin/categories/{categoryId}

| GET | `/v1/admin/commands` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"attemptCount": 1, "commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "commandType": "SET_TEMPERATURE", "c` |

### GET /v1/admin/commands

- **Purpose:** List machine commands (admin)
- **Auth:** bearer (required=True)
- **Query params:** `machine_id, status, from, to, limit, offset`
- **Response 200:** List machine commands (admin)
```json
{
  "items": [
    {
      "attemptCount": 1,
      "commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
      "commandType": "SET_TEMPERATURE",
      "createdAt": "2026-04-19T12:00:00Z",
      "latestAttemptStatus": "sent",
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "machineName": "Lobby A",
      "machineSerialNumber": "SN-001",
      "sequence": 42
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body command_id→commandId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/commands

| GET | `/v1/admin/commands/{commandId}` | 08_Machines_Runtime_Config | bearer | no | `{"attempts": [{"attemptNo": 1, "dispatchState": "failed", "id": "cccccccc-dddd-eeee-ffff-000000000001", "sentAt": "2026-` |

### GET /v1/admin/commands/{commandId}

- **Purpose:** Get command ledger row by id
- **Auth:** bearer (required=True)
- **Path params:** `commandId`
- **Response 200:** Get command ledger row by id
```json
{
  "attempts": [
    {
      "attemptNo": 1,
      "dispatchState": "failed",
      "id": "cccccccc-dddd-eeee-ffff-000000000001",
      "sentAt": "2026-04-29T12:00:10.000000000Z",
      "status": "failed",
      "timeoutReason": "mqtt_timeout"
    }
  ],
  "commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "commandType": "SET_TEMPERATURE",
  "createdAt": "2026-04-29T12:00:00.000000000Z",
  "idempotencyKey": "idem-retry-safe",
  "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "payload": {
    "celsius": 4
  },
  "sequence": 42
}
```
- **Captures:** response body command_id→commandId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/commands/{commandId}

| POST | `/v1/admin/commands/{commandId}/cancel` | 08_Machines_Runtime_Config | bearer | no | `{"attemptsCancelled": 1}` |

### POST /v1/admin/commands/{commandId}/cancel

- **Purpose:** Cancel pending command
- **Auth:** bearer (required=True)
- **Path params:** `commandId`
- **Response 200:** Cancel pending command
```json
{
  "attemptsCancelled": 1
}
```
- **Captures:** response body command_id→commandId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/commands/{commandId}/cancel

| POST | `/v1/admin/commands/{commandId}/retry` | 08_Machines_Runtime_Config | bearer | no | `{"attemptId": "dddddddd-eeee-ffff-0000-111111111111", "commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "dispatchStat` |

### POST /v1/admin/commands/{commandId}/retry

- **Purpose:** Retry failed command dispatch
- **Auth:** bearer (required=True)
- **Path params:** `commandId`
- **Response 200:** Retry failed command dispatch
```json
{
  "attemptId": "dddddddd-eeee-ffff-0000-111111111111",
  "commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "dispatchState": "published",
  "replay": false,
  "sequence": 42,
  "skippedRepublish": false
}
```
- **Captures:** response body command_id→commandId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/commands/{commandId}/retry

| GET | `/v1/admin/commerce/reconciliation` | 12_Orders | bearer | no | `{"items": [{"caseType": "payment_paid_vend_failed", "firstDetectedAt": "2026-04-19T12:10:00Z", "id": "99999999-8888-7777` |

### GET /v1/admin/commerce/reconciliation

- **Purpose:** List commerce reconciliation cases
- **Auth:** bearer (required=True)
- **Query params:** `status, case_type, limit, offset`
- **Response 200:** List commerce reconciliation cases
```json
{
  "items": [
    {
      "caseType": "payment_paid_vend_failed",
      "firstDetectedAt": "2026-04-19T12:10:00Z",
      "id": "99999999-8888-7777-6666-555555555555",
      "lastDetectedAt": "2026-04-19T12:10:00Z",
      "metadata": {
        "payment_state": "captured",
        "vend_state": "failed"
      },
      "orderId": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "paymentId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "reason": "captured payment is attached to a failed vend",
      "severity": "critical",
      "status": "open"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/commerce/reconciliation

| GET | `/v1/admin/commerce/reconciliation/{caseId}` | 12_Orders | bearer | no | `{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6"}` |

### GET /v1/admin/commerce/reconciliation/{caseId}

- **Purpose:** Get commerce reconciliation case
- **Auth:** bearer (required=True)
- **Path params:** `caseId`
- **Response 200:** Get commerce reconciliation case
```json
{
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/commerce/reconciliation/{caseId}

| POST | `/v1/admin/commerce/reconciliation/{caseId}/ignore` | 12_Orders | bearer | yes | `{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6"}` |

### POST /v1/admin/commerce/reconciliation/{caseId}/ignore

- **Purpose:** Ignore commerce reconciliation case
- **Auth:** bearer (required=True)
- **Path params:** `caseId`
- **Request body example:**
```json
{
  "id": "{{resource_uuid}}"
}
```
- **Response 200:** Ignore commerce reconciliation case
```json
{
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/commerce/reconciliation/{caseId}/ignore

| POST | `/v1/admin/commerce/reconciliation/{caseId}/request-refund` | 12_Orders | bearer | yes | `{"status": "ok"}` |

### POST /v1/admin/commerce/reconciliation/{caseId}/request-refund

- **Purpose:** Request refund from reconciliation case
- **Auth:** bearer (required=True)
- **Path params:** `caseId`
- **Request body example:**
```json
{
  "status": "ok"
}
```
- **Response 200:** Request refund from reconciliation case
```json
{
  "status": "ok"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/commerce/reconciliation/{caseId}/request-refund

| POST | `/v1/admin/commerce/reconciliation/{caseId}/resolve` | 12_Orders | bearer | yes | `{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6"}` |

### POST /v1/admin/commerce/reconciliation/{caseId}/resolve

- **Purpose:** Resolve commerce reconciliation case
- **Auth:** bearer (required=True)
- **Path params:** `caseId`
- **Request body example:**
```json
{
  "id": "{{resource_uuid}}"
}
```
- **Response 200:** Resolve commerce reconciliation case
```json
{
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/commerce/reconciliation/{caseId}/resolve

| GET | `/v1/admin/feature-flags` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"createdAt": "2026-04-01T00:00:00Z", "description": "Experimental UI", "displayName": "Beta UI", "enabled": ` |

### GET /v1/admin/feature-flags

- **Purpose:** List company feature flags
- **Auth:** bearer (required=True)
- **Query params:** `limit, offset`
- **Response 200:** List company feature flags
```json
{
  "items": [
    {
      "createdAt": "2026-04-01T00:00:00Z",
      "description": "Experimental UI",
      "displayName": "Beta UI",
      "enabled": false,
      "flagKey": "kiosk.beta_ui",
      "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "metadata": {},
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/feature-flags

| POST | `/v1/admin/feature-flags` | 08_Machines_Runtime_Config | bearer | yes | `{"createdAt": "2026-04-01T00:00:00Z", "description": "Experimental UI", "displayName": "Beta UI", "enabled": false, "fla` |

### POST /v1/admin/feature-flags

- **Purpose:** Create a feature flag
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "description": "Experimental UI",
  "displayName": "Beta UI",
  "enabled": false,
  "flagKey": "kiosk.beta_ui",
  "metadata": {}
}
```
- **Response 201:** Create a feature flag
```json
{
  "createdAt": "2026-04-01T00:00:00Z",
  "description": "Experimental UI",
  "displayName": "Beta UI",
  "enabled": false,
  "flagKey": "kiosk.beta_ui",
  "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "metadata": {},
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/feature-flags

| GET | `/v1/admin/feature-flags/{flagId}` | 08_Machines_Runtime_Config | bearer | no | `{"flag": {"createdAt": "2026-04-01T00:00:00Z", "description": "Experimental UI", "displayName": "Beta UI", "enabled": fa` |

### GET /v1/admin/feature-flags/{flagId}

- **Purpose:** Get feature flag and scoped targets
- **Auth:** bearer (required=True)
- **Path params:** `flagId`
- **Response 200:** Get feature flag and scoped targets
```json
{
  "flag": {
    "createdAt": "2026-04-01T00:00:00Z",
    "description": "Experimental UI",
    "displayName": "Beta UI",
    "enabled": false,
    "flagKey": "kiosk.beta_ui",
    "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
    "metadata": {},
    "updatedAt": "2026-04-19T10:00:00Z"
  },
  "targets": []
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/feature-flags/{flagId}

| PATCH | `/v1/admin/feature-flags/{flagId}` | 08_Machines_Runtime_Config | bearer | yes | `{"createdAt": "2026-04-01T00:00:00Z", "description": "Experimental UI", "displayName": "Beta UI", "enabled": false, "fla` |

### PATCH /v1/admin/feature-flags/{flagId}

- **Purpose:** Patch feature flag metadata / master enabled bit
- **Auth:** bearer (required=True)
- **Path params:** `flagId`
- **Request body example:**
```json
{
  "displayName": "Beta UI v2",
  "enabled": true
}
```
- **Response 200:** Patch feature flag metadata / master enabled bit
```json
{
  "createdAt": "2026-04-01T00:00:00Z",
  "description": "Experimental UI",
  "displayName": "Beta UI",
  "enabled": false,
  "flagKey": "kiosk.beta_ui",
  "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "metadata": {},
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/feature-flags/{flagId}

| POST | `/v1/admin/feature-flags/{flagId}/disable` | 08_Machines_Runtime_Config | bearer | no | `{"createdAt": "2026-04-01T00:00:00Z", "description": "Experimental UI", "displayName": "Beta UI", "enabled": false, "fla` |

### POST /v1/admin/feature-flags/{flagId}/disable

- **Purpose:** Disable feature flag (master switch)
- **Auth:** bearer (required=True)
- **Path params:** `flagId`
- **Response 200:** Disable feature flag (master switch)
```json
{
  "createdAt": "2026-04-01T00:00:00Z",
  "description": "Experimental UI",
  "displayName": "Beta UI",
  "enabled": false,
  "flagKey": "kiosk.beta_ui",
  "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "metadata": {},
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/feature-flags/{flagId}/disable

| POST | `/v1/admin/feature-flags/{flagId}/enable` | 08_Machines_Runtime_Config | bearer | no | `{"createdAt": "2026-04-01T00:00:00Z", "description": "Experimental UI", "displayName": "Beta UI", "enabled": false, "fla` |

### POST /v1/admin/feature-flags/{flagId}/enable

- **Purpose:** Enable feature flag (master switch)
- **Auth:** bearer (required=True)
- **Path params:** `flagId`
- **Response 200:** Enable feature flag (master switch)
```json
{
  "createdAt": "2026-04-01T00:00:00Z",
  "description": "Experimental UI",
  "displayName": "Beta UI",
  "enabled": false,
  "flagKey": "kiosk.beta_ui",
  "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "metadata": {},
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/feature-flags/{flagId}/enable

| PUT | `/v1/admin/feature-flags/{flagId}/targets` | 08_Machines_Runtime_Config | bearer | yes | `{"targets": []}` |

### PUT /v1/admin/feature-flags/{flagId}/targets

- **Purpose:** Replace scoped targets for a feature flag
- **Auth:** bearer (required=True)
- **Path params:** `flagId`
- **Request body example:**
```json
{
  "targets": [
    {
      "enabled": true,
      "machineId": "{{machineId}}",
      "metadata": {},
      "priority": 10,
      "targetType": "machine"
    }
  ]
}
```
- **Response 200:** Replace scoped targets for a feature flag
```json
{
  "targets": []
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:PUT /v1/admin/feature-flags/{flagId}/targets

| GET | `/v1/admin/finance/daily-close` | 16_Finance_Reconciliation | bearer | no | `{"items": [{"cashMinor": 60000, "closeDate": "2026-04-27", "createdAt": "2026-04-27T18:00:00.000000000Z", "discountMinor` |

### GET /v1/admin/finance/daily-close

- **Purpose:** List finance daily closes
- **Auth:** bearer (required=True)
- **Query params:** `limit, offset`
- **Response 200:** List finance daily closes
```json
{
  "items": [
    {
      "cashMinor": 60000,
      "closeDate": "2026-04-27",
      "createdAt": "2026-04-27T18:00:00.000000000Z",
      "discountMinor": 0,
      "failedMinor": 200,
      "grossSalesMinor": 100000,
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "idempotencyKey": "REPLACE_ME",
      "netMinor": 99500,
      "pendingMinor": 300,
      "qrWalletMinor": 40000,
      "refundMinor": 500,
      "timezone": "Asia/Bangkok"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/finance/daily-close

| POST | `/v1/admin/finance/daily-close` | 16_Finance_Reconciliation | bearer | yes | `{"cashMinor": 60000, "closeDate": "2026-04-27", "createdAt": "2026-04-27T18:00:00.000000000Z", "discountMinor": 0, "fail` |

### POST /v1/admin/finance/daily-close

- **Purpose:** Create immutable finance daily close (requires Idempotency-Key)
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "closeDate": "2026-04-27",
  "timezone": "Asia/Bangkok"
}
```
- **Response 201:** Create immutable finance daily close (requires Idempotency-Key)
```json
{
  "cashMinor": 60000,
  "closeDate": "2026-04-27",
  "createdAt": "2026-04-27T18:00:00.000000000Z",
  "discountMinor": 0,
  "failedMinor": 200,
  "grossSalesMinor": 100000,
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "idempotencyKey": "REPLACE_ME",
  "netMinor": 99500,
  "pendingMinor": 300,
  "qrWalletMinor": 40000,
  "refundMinor": 500,
  "timezone": "Asia/Bangkok"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/finance/daily-close

| GET | `/v1/admin/finance/daily-close/{closeId}` | 16_Finance_Reconciliation | bearer | no | `{"cashMinor": 60000, "closeDate": "2026-04-27", "createdAt": "2026-04-27T18:00:00.000000000Z", "discountMinor": 0, "fail` |

### GET /v1/admin/finance/daily-close/{closeId}

- **Purpose:** Get one finance daily close by id
- **Auth:** bearer (required=True)
- **Path params:** `closeId`
- **Response 200:** Get one finance daily close by id
```json
{
  "cashMinor": 60000,
  "closeDate": "2026-04-27",
  "createdAt": "2026-04-27T18:00:00.000000000Z",
  "discountMinor": 0,
  "failedMinor": 200,
  "grossSalesMinor": 100000,
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "idempotencyKey": "REPLACE_ME",
  "netMinor": 99500,
  "pendingMinor": 300,
  "qrWalletMinor": 40000,
  "refundMinor": 500,
  "timezone": "Asia/Bangkok"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/finance/daily-close/{closeId}

| GET | `/v1/admin/inventory/anomalies` | 10_Inventory | bearer | no | `{"items": [{"anomalyType": "negative_stock", "createdAt": "2026-04-29T12:00:00.000000000Z", "detectedAt": "2026-04-29T12` |

### GET /v1/admin/inventory/anomalies

- **Purpose:** List inventory anomalies (ledger-backed)
- **Auth:** bearer (required=True)
- **Response 200:** List inventory anomalies (ledger-backed)
```json
{
  "items": [
    {
      "anomalyType": "negative_stock",
      "createdAt": "2026-04-29T12:00:00.000000000Z",
      "detectedAt": "2026-04-29T12:00:00.000000000Z",
      "fingerprint": "negative-stock:A3",
      "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "machineName": "Lobby A",
      "machineSerialNumber": "SN-001",
      "payload": {
        "quantity": -1
      },
      "slotCode": "A3",
      "status": "open",
      "updatedAt": "2026-04-29T12:00:00.000000000Z"
    }
  ]
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/inventory/anomalies

| POST | `/v1/admin/inventory/anomalies/{anomalyId}/resolve` | 10_Inventory | bearer | no | `{"anomalyId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "status": "resolved"}` |

### POST /v1/admin/inventory/anomalies/{anomalyId}/resolve

- **Purpose:** Resolve inventory anomaly
- **Auth:** bearer (required=True)
- **Path params:** `anomalyId`
- **Response 200:** Resolve inventory anomaly
```json
{
  "anomalyId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "status": "resolved"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/inventory/anomalies/{anomalyId}/resolve

| GET | `/v1/admin/inventory/low-stock` | 10_Inventory | bearer | no | `{"items": [{"currentQuantity": 3, "dailyVelocity": 1.0, "daysToEmpty": 3.0, "fillRatio": 0.3, "machineId": "7c9e6679-742` |

### GET /v1/admin/inventory/low-stock

- **Purpose:** List slots estimated to need refill soon (low stock)
- **Auth:** bearer (required=True)
- **Query params:** `site_id, machine_id, product_id, velocity_days, urgency, days_threshold, limit, offset`
- **Response 200:** List slots estimated to need refill soon (low stock)
```json
{
  "items": [
    {
      "currentQuantity": 3,
      "dailyVelocity": 1.0,
      "daysToEmpty": 3.0,
      "fillRatio": 0.3,
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "machineName": "Lobby-01",
      "maxQuantity": 10,
      "planogramId": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
      "planogramName": "Lobby default",
      "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "productName": "Cola 12oz",
      "productSku": "COLA-12",
      "siteId": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
      "siteName": "HQ",
      "slotIndex": 0,
      "suggestedRefillQuantity": 7,
      "unitsSoldInWindow": 14,
      "urgency": "medium"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  },
  "velocityWindowDays": 14,
  "windowEnd": "2026-04-28T00:00:00.000000000Z",
  "windowStart": "2026-04-14T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/inventory/low-stock

| GET | `/v1/admin/inventory/refill-suggestions` | 10_Inventory | bearer | no | `{"items": [{"currentQuantity": 3, "dailyVelocity": 1.0, "daysToEmpty": 3.0, "fillRatio": 0.3, "machineId": "7c9e6679-742` |

### GET /v1/admin/inventory/refill-suggestions

- **Purpose:** List refill suggestions across machines (all slots)
- **Auth:** bearer (required=True)
- **Query params:** `site_id, machine_id, product_id, velocity_days, urgency, days_threshold, limit, offset`
- **Response 200:** List refill suggestions across machines (all slots)
```json
{
  "items": [
    {
      "currentQuantity": 3,
      "dailyVelocity": 1.0,
      "daysToEmpty": 3.0,
      "fillRatio": 0.3,
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "machineName": "Lobby-01",
      "maxQuantity": 10,
      "planogramId": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
      "planogramName": "Lobby default",
      "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "productName": "Cola 12oz",
      "productSku": "COLA-12",
      "siteId": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
      "siteName": "HQ",
      "slotIndex": 0,
      "suggestedRefillQuantity": 7,
      "unitsSoldInWindow": 14,
      "urgency": "medium"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  },
  "velocityWindowDays": 14,
  "windowEnd": "2026-04-28T00:00:00.000000000Z",
  "windowStart": "2026-04-14T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/inventory/refill-suggestions

| GET | `/v1/admin/machine-config/rollouts` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"createdAt": "2026-04-19T12:00:00.000000000Z", "id": "77777777-8888-9999-aaaa-bbbbbbbbbbbb", "scopeType": "c` |

### GET /v1/admin/machine-config/rollouts

- **Purpose:** List machine config rollouts
- **Auth:** bearer (required=True)
- **Query params:** `limit, offset`
- **Response 200:** List machine config rollouts
```json
{
  "items": [
    {
      "createdAt": "2026-04-19T12:00:00.000000000Z",
      "id": "77777777-8888-9999-aaaa-bbbbbbbbbbbb",
      "scopeType": "company",
      "status": "pending",
      "targetVersionId": "11111111-2222-3333-4444-555555555555"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/machine-config/rollouts

| POST | `/v1/admin/machine-config/rollouts` | 08_Machines_Runtime_Config | bearer | yes | `{"createdAt": "2026-04-19T12:00:00.000000000Z", "id": "77777777-8888-9999-aaaa-bbbbbbbbbbbb", "scopeType": "company", "s` |

### POST /v1/admin/machine-config/rollouts

- **Purpose:** Create machine config rollout (or rollback)
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "scopeType": "company",
  "targetVersionId": "11111111-2222-3333-4444-555555555555"
}
```
- **Response 201:** Create machine config rollout (or rollback)
```json
{
  "createdAt": "2026-04-19T12:00:00.000000000Z",
  "id": "77777777-8888-9999-aaaa-bbbbbbbbbbbb",
  "scopeType": "company",
  "status": "pending",
  "targetVersionId": "11111111-2222-3333-4444-555555555555"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machine-config/rollouts

| GET | `/v1/admin/machine-config/rollouts/{rolloutId}` | 08_Machines_Runtime_Config | bearer | no | `{"createdAt": "2026-04-19T12:00:00.000000000Z", "id": "77777777-8888-9999-aaaa-bbbbbbbbbbbb", "scopeType": "company", "s` |

### GET /v1/admin/machine-config/rollouts/{rolloutId}

- **Purpose:** Get one machine config rollout
- **Auth:** bearer (required=True)
- **Path params:** `rolloutId`
- **Response 200:** Get one machine config rollout
```json
{
  "createdAt": "2026-04-19T12:00:00.000000000Z",
  "id": "77777777-8888-9999-aaaa-bbbbbbbbbbbb",
  "scopeType": "company",
  "status": "pending",
  "targetVersionId": "11111111-2222-3333-4444-555555555555"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/machine-config/rollouts/{rolloutId}

| GET | `/v1/admin/machines` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"assignedTechnicians": [], "commandSequence": 12, "createdAt": "2026-01-01T00:00:00.000000000Z", "effectiveT` |

### GET /v1/admin/machines

- **Purpose:** List machines (admin)
- **Auth:** bearer (required=True)
- **Query params:** `site_id, machine_id, status, from, to, limit, offset`
- **Response 200:** List machines (admin)
```json
{
  "items": [
    {
      "assignedTechnicians": [],
      "commandSequence": 12,
      "createdAt": "2026-01-01T00:00:00.000000000Z",
      "effectiveTimezone": "America/Los_Angeles",
      "inventorySummary": {
        "lowStockSlots": 2,
        "occupiedSlots": 18,
        "outOfStockSlots": 0,
        "totalSlots": 24
      },
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "machineName": "Lobby A",
      "name": "Lobby A",
      "serialNumber": "SN-001",
      "siteId": "11111111-2222-3333-4444-555555555555",
      "siteName": "Main Campus",
      "status": "online",
      "updatedAt": "2026-04-19T10:00:00.000000000Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/machines

| POST | `/v1/admin/machines` | 08_Machines_Runtime_Config | bearer | yes | `{"code": "M001", "command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 0, "id": "7c9e6679-7` |

### POST /v1/admin/machines

- **Purpose:** Create machine (admin)
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "cabinetType": "ambient",
  "code": "canary-code-{{$guid}}",
  "model": "AVF-1",
  "name": "canary-name-{{$guid}}",
  "serialNumber": "SN-001",
  "siteId": "{{siteId}}",
  "status": "draft",
  "timezone": "UTC"
}
```
- **Response 201:** Create machine (admin)
```json
{
  "code": "M001",
  "command_sequence": 0,
  "created_at": "2026-04-29T00:00:00Z",
  "credential_version": 0,
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "name": "Lobby A",
  "serial_number": "SN-001",
  "site_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "status": "draft",
  "updated_at": "2026-04-29T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines

| GET | `/v1/admin/machines/{machineId}` | 08_Machines_Runtime_Config | bearer | no | `{"assignedTechnicians": [], "commandSequence": 12, "createdAt": "2026-01-01T00:00:00.000000000Z", "effectiveTimezone": "` |

### GET /v1/admin/machines/{machineId}

- **Purpose:** Get machine (admin)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Get machine (admin)
```json
{
  "assignedTechnicians": [],
  "commandSequence": 12,
  "createdAt": "2026-01-01T00:00:00.000000000Z",
  "effectiveTimezone": "America/Los_Angeles",
  "inventorySummary": {
    "lowStockSlots": 2,
    "occupiedSlots": 18,
    "outOfStockSlots": 0,
    "totalSlots": 24
  },
  "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "machineName": "Lobby A",
  "name": "Lobby A",
  "serialNumber": "SN-001",
  "siteId": "11111111-2222-3333-4444-555555555555",
  "siteName": "Main Campus",
  "status": "online",
  "updatedAt": "2026-04-19T10:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/machines/{machineId}

| PATCH | `/v1/admin/machines/{machineId}` | 08_Machines_Runtime_Config | bearer | yes | `{"code": "M001", "command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 0, "id": "7c9e6679-7` |

### PATCH /v1/admin/machines/{machineId}

- **Purpose:** Patch machine metadata (admin)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "name": "canary-name-{{$guid}}",
  "status": "active"
}
```
- **Response 200:** Patch machine metadata (admin)
```json
{
  "code": "M001",
  "command_sequence": 0,
  "created_at": "2026-04-29T00:00:00Z",
  "credential_version": 0,
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "name": "Lobby A1",
  "serial_number": "SN-001",
  "site_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "status": "active",
  "updated_at": "2026-04-29T00:05:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/machines/{machineId}

| GET | `/v1/admin/machines/{machineId}/activation-codes` | 07_Machines_Provisioning | bearer | no | `{"items": [{"activationCodeId": "11111111-2222-3333-4444-555555555555", "createdAt": "2026-04-23T00:00:00Z", "expiresAt"` |

### GET /v1/admin/machines/{machineId}/activation-codes

- **Purpose:** List activation codes for a machine
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** List activation codes for a machine
```json
{
  "items": [
    {
      "activationCodeId": "11111111-2222-3333-4444-555555555555",
      "createdAt": "2026-04-23T00:00:00Z",
      "expiresAt": "2026-04-24T00:00:00Z",
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "maxUses": 1,
      "notes": "Field install",
      "remainingUses": 1,
      "status": "active",
      "uses": 0
    }
  ]
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/machines/{machineId}/activation-codes

| POST | `/v1/admin/machines/{machineId}/activation-codes` | 07_Machines_Provisioning | bearer | yes | `{"activationCode": "AVF-123456-ABCDEF", "activationCodeId": "11111111-2222-3333-4444-555555555555", "expiresAt": "2026-0` |

### POST /v1/admin/machines/{machineId}/activation-codes

- **Purpose:** Create machine activation code
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "expiresInMinutes": 1440,
  "maxUses": 1,
  "notes": "Field install at site A"
}
```
- **Response 201:** Create machine activation code
```json
{
  "activationCode": "AVF-123456-ABCDEF",
  "activationCodeId": "11111111-2222-3333-4444-555555555555",
  "expiresAt": "2026-04-24T00:00:00Z",
  "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "maxUses": 1,
  "remainingUses": 1,
  "status": "active"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/activation-codes

| DELETE | `/v1/admin/machines/{machineId}/activation-codes/{activationCodeId}` | 07_Machines_Provisioning | bearer | no | `""` |

### DELETE /v1/admin/machines/{machineId}/activation-codes/{activationCodeId}

- **Purpose:** Revoke an activation code
- **Auth:** bearer (required=True)
- **Path params:** `machineId, activationCodeId`
- **Response 204:** Revoke an activation code
```json
""
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/machines/{machineId}/activation-codes/{activationCodeId}

| POST | `/v1/admin/machines/{machineId}/archive` | 08_Machines_Runtime_Config | bearer | no | `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 0, "id": "7c9e6679-7425-40de-944b-e0` |

### POST /v1/admin/machines/{machineId}/archive

- **Purpose:** Retire machine (archive alias)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Retire machine (archive alias)
```json
{
  "command_sequence": 0,
  "created_at": "2026-04-29T00:00:00Z",
  "credential_version": 0,
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "name": "Lobby A",
  "serial_number": "SN-001",
  "site_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "status": "decommissioned",
  "updated_at": "2026-04-29T00:05:00Z"
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/archive

| GET | `/v1/admin/machines/{machineId}/cash-collections` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"close_request_hash_hex": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", "closed_at": "` |

### GET /v1/admin/machines/{machineId}/cash-collections

- **Purpose:** List cash collections for machine
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Query params:** `limit, offset`
- **Response 200:** List cash collections for machine
```json
{
  "items": [
    {
      "close_request_hash_hex": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
      "closed_at": "2026-04-19T15:00:00.000000000Z",
      "collected_at": "2026-04-19T14:00:00.000000000Z",
      "countedPhysicalCashMinor": 1200,
      "counted_amount_minor": 1200,
      "currency": "USD",
      "disclosure": "Accounting-only: cloud ledger vs operator physical count; does not sense or command hardware.",
      "expectedCloudCashMinor": 1200,
      "expected_amount_minor": 1200,
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "lifecycle_status": "closed",
      "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "opened_at": "2026-04-19T14:00:00.000000000Z",
      "reconciliation_status": "matched",
      "requires_review": false,
      "reviewState": "matched",
      "varianceMinor": 0,
      "variance_amount_minor": 0
    },
    {
      "close_request_hash_hex": "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
      "closed_at": "2026-04-19T15:30:00.000000000Z",
      "collected_at": "2026-04-19T14:00:00.000000000Z",
      "countedPhysicalCashMinor": 2000,
      "counted_amount_minor": 2000,
      "currency": "USD",
      "disclosure": "Accounting-only: cloud ledger vs operator physical count; does not sense or command hardware.",
      "expectedCloudCashMinor": 1200,
      "expected_amount_minor": 1200,
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "lifecycle_status": "closed",
      "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "opened_at": "2026-04-19T14:00:00.000000000Z",
      "reconciliation_status": "pending",
      "requires_review": true,
      "reviewState": "pending_review",
      "varianceMinor": 800,
      "variance_amount_minor": 800
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/machines/{machineId}/cash-collections

| POST | `/v1/admin/machines/{machineId}/cash-collections` | 08_Machines_Runtime_Config | bearer | yes | `{"close_request_hash_hex": null, "closed_at": null, "collected_at": "2026-04-19T14:00:00.000000000Z", "countedPhysicalCa` |

### POST /v1/admin/machines/{machineId}/cash-collections

- **Purpose:** Start cash collection session
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "currency": "USD",
  "notes": "Field collection — tray A",
  "operator_session_id": "{{operatorSessionId}}",
  "startedAt": "2026-04-24T00:00:00Z"
}
```
- **Response 200:** Start cash collection session
```json
{
  "close_request_hash_hex": null,
  "closed_at": null,
  "collected_at": "2026-04-19T14:00:00.000000000Z",
  "countedPhysicalCashMinor": 0,
  "counted_amount_minor": 0,
  "currency": "USD",
  "disclosure": "Accounting-only: cloud ledger vs operator physical count; does not sense or command hardware.",
  "expectedCloudCashMinor": 0,
  "expected_amount_minor": 0,
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "lifecycle_status": "open",
  "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "opened_at": "2026-04-19T14:00:00.000000000Z",
  "reconciliation_status": "pending",
  "requires_review": false,
  "reviewState": "open",
  "varianceMinor": 0,
  "variance_amount_minor": 0
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/cash-collections

| GET | `/v1/admin/machines/{machineId}/cash-collections/{collectionId}` | 08_Machines_Runtime_Config | bearer | no | `{"close_request_hash_hex": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "closed_at": "2026-04-19T` |

### GET /v1/admin/machines/{machineId}/cash-collections/{collectionId}

- **Purpose:** Get one cash collection
- **Auth:** bearer (required=True)
- **Path params:** `machineId, collectionId`
- **Response 200:** Get one cash collection
```json
{
  "close_request_hash_hex": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "closed_at": "2026-04-19T14:30:00.000000000Z",
  "collected_at": "2026-04-19T14:00:00.000000000Z",
  "countedPhysicalCashMinor": 1250,
  "counted_amount_minor": 1250,
  "currency": "USD",
  "disclosure": "Accounting-only: cloud ledger vs operator physical count; does not sense or command hardware.",
  "expectedCloudCashMinor": 1200,
  "expected_amount_minor": 1200,
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "lifecycle_status": "closed",
  "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "opened_at": "2026-04-19T14:00:00.000000000Z",
  "reconciliation_status": "mismatch",
  "requires_review": false,
  "reviewState": "variance_recorded",
  "varianceMinor": 50,
  "variance_amount_minor": 50
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/machines/{machineId}/cash-collections/{collectionId}

| POST | `/v1/admin/machines/{machineId}/cash-collections/{collectionId}/close` | 08_Machines_Runtime_Config | bearer | yes | `{"close_request_hash_hex": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "closed_at": "2026-04-19T` |

### POST /v1/admin/machines/{machineId}/cash-collections/{collectionId}/close

- **Purpose:** Close cash collection with counted cash
- **Auth:** bearer (required=True)
- **Path params:** `machineId, collectionId`
- **Request body example:**
```json
{
  "closedAt": "2026-04-24T00:10:00Z",
  "countedCashboxMinor": 995000,
  "countedRecyclerMinor": 200000,
  "currency": "VND",
  "denominations": [
    {
      "count": 50,
      "denominationMinor": 10000
    }
  ],
  "evidence": {
    "photoArtifactId": "22222222-3333-4444-5555-666666666666"
  },
  "notes": "Monthly collection",
  "operator_session_id": "{{operatorSessionId}}"
}
```
- **Response 200:** Close cash collection with counted cash
```json
{
  "close_request_hash_hex": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "closed_at": "2026-04-19T14:30:00.000000000Z",
  "collected_at": "2026-04-19T14:00:00.000000000Z",
  "countedPhysicalCashMinor": 1250,
  "counted_amount_minor": 1250,
  "currency": "USD",
  "disclosure": "Accounting-only: cloud ledger vs operator physical count; does not sense or command hardware.",
  "expectedCloudCashMinor": 1200,
  "expected_amount_minor": 1200,
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "lifecycle_status": "closed",
  "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "opened_at": "2026-04-19T14:00:00.000000000Z",
  "reconciliation_status": "mismatch",
  "requires_review": false,
  "reviewState": "variance_recorded",
  "varianceMinor": 50,
  "variance_amount_minor": 50
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/cash-collections/{collectionId}/close

| GET | `/v1/admin/machines/{machineId}/cashbox` | 08_Machines_Runtime_Config | bearer | no | `{"currency": "VND", "denominations": [], "disclosure": "Accounting-only: cloud ledger expectation only; does not sense o` |

### GET /v1/admin/machines/{machineId}/cashbox

- **Purpose:** Cashbox summary (expected vault from commerce)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Query params:** `currency`
- **Response 200:** Cashbox summary (expected vault from commerce)
```json
{
  "currency": "VND",
  "denominations": [],
  "disclosure": "Accounting-only: cloud ledger expectation only; does not sense or command physical cash hardware.",
  "expectedCashboxMinor": 1000000,
  "expectedCloudCashMinor": 1000000,
  "expectedRecyclerMinor": 0,
  "lastCollectionAt": "2026-04-24T00:00:00Z",
  "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "openCollectionId": null,
  "varianceReviewThresholdMinor": 500
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/machines/{machineId}/cashbox

| POST | `/v1/admin/machines/{machineId}/commands` | 08_Machines_Runtime_Config | bearer | yes | `{"status": "ok"}` |

### POST /v1/admin/machines/{machineId}/commands

- **Purpose:** Dispatch new command to machine (MQTT/device pipeline)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "commandType": "REQUEST_DIAGNOSTICS",
  "payload": {
    "bundle": "logs"
  }
}
```
- **Response 200:** Dispatch new command to machine (MQTT/device pipeline)
```json
{
  "status": "ok"
}
```
- **Captures:** response body machine_id→machineId, response body command_id→commandId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/commands

| GET | `/v1/admin/machines/{machineId}/diagnostics/bundles` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"bundleId": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "commandId": "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "co` |

### GET /v1/admin/machines/{machineId}/diagnostics/bundles

- **Purpose:** List machine diagnostic bundles
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Query params:** `limit, offset`
- **Response 200:** List machine diagnostic bundles
```json
{
  "items": [
    {
      "bundleId": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "commandId": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
      "contentType": "application/gzip",
      "createdAt": "2026-04-29T00:00:00Z",
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "metadata": {
        "app_version": "1.2.3"
      },
      "requestId": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "sha256Hex": "abc123",
      "sizeBytes": 1024,
      "status": "available",
      "storageKey": "diagnostics/org/machine/bundle.tgz",
      "storageProvider": "s3"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/machines/{machineId}/diagnostics/bundles

| POST | `/v1/admin/machines/{machineId}/diagnostics/requests` | 08_Machines_Runtime_Config | bearer | no | `{"commandId": "6ba7b810-9dad-11d1-80b4-00c04fd430c8", "dispatchState": "published", "machineId": "7c9e6679-7425-40de-944` |

### POST /v1/admin/machines/{machineId}/diagnostics/requests

- **Purpose:** Request machine diagnostic bundle
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 202:** Request machine diagnostic bundle
```json
{
  "commandId": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
  "dispatchState": "published",
  "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "replay": false,
  "requestId": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "sequence": 42
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/diagnostics/requests

| POST | `/v1/admin/machines/{machineId}/disable` | 08_Machines_Runtime_Config | bearer | no | `{"command_sequence": 0, "created_at": "2026-04-01T00:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "n` |

### POST /v1/admin/machines/{machineId}/disable

- **Purpose:** Disable machine (admin)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Disable machine (admin)
```json
{
  "command_sequence": 0,
  "created_at": "2026-04-01T00:00:00.000000000Z",
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "name": "New unit",
  "serial_number": "SN-NEW",
  "site_id": "aaaaaaaa-bbbb-cccc-dddd-111111111111",
  "status": "provisioning",
  "updated_at": "2026-04-01T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/disable

| POST | `/v1/admin/machines/{machineId}/enable` | 08_Machines_Runtime_Config | bearer | no | `{"command_sequence": 0, "created_at": "2026-04-01T00:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "n` |

### POST /v1/admin/machines/{machineId}/enable

- **Purpose:** Enable machine (admin)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Enable machine (admin)
```json
{
  "command_sequence": 0,
  "created_at": "2026-04-01T00:00:00.000000000Z",
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "name": "New unit",
  "serial_number": "SN-NEW",
  "site_id": "aaaaaaaa-bbbb-cccc-dddd-111111111111",
  "status": "offline",
  "updated_at": "2026-04-01T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/enable

| GET | `/v1/admin/machines/{machineId}/health` | 08_Machines_Runtime_Config | bearer | no | `{"failedCommandCount": 0, "inventoryAnomalyCount": 0, "lastSeenAt": "2026-04-29T12:00:00.000000000Z", "machineId": "7c9e` |

### GET /v1/admin/machines/{machineId}/health

- **Purpose:** Machine health detail
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Machine health detail
```json
{
  "failedCommandCount": 0,
  "inventoryAnomalyCount": 0,
  "lastSeenAt": "2026-04-29T12:00:00.000000000Z",
  "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "pendingCommandCount": 1,
  "status": "online",
  "telemetryFreshnessSeconds": 95
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/machines/{machineId}/health

| GET | `/v1/admin/machines/{machineId}/inventory` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"cabinetCode": "CAB-A", "cabinetIndex": 0, "lowStock": false, "machineId": "7c9e6679-7425-40de-944b-e07fc1f9` |

### GET /v1/admin/machines/{machineId}/inventory

- **Purpose:** Aggregate inventory by product for a machine
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Aggregate inventory by product for a machine
```json
{
  "items": [
    {
      "cabinetCode": "CAB-A",
      "cabinetIndex": 0,
      "lowStock": false,
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "machineName": "Lobby-01",
      "machineStatus": "active",
      "maxCapacityAnySlot": 12,
      "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "productName": "Cola 12oz",
      "productSku": "COLA-12",
      "slotCount": 2,
      "totalQuantity": 24
    }
  ]
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/machines/{machineId}/inventory

| GET | `/v1/admin/machines/{machineId}/inventory-events` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"cabinetCode": "CAB-A", "currency": "USD", "eventType": "adjustment", "id": 1001, "machineId": "7c9e6679-742` |

### GET /v1/admin/machines/{machineId}/inventory-events

- **Purpose:** List append-only inventory ledger events for a machine
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Query params:** `from, to, limit, offset`
- **Response 200:** List append-only inventory ledger events for a machine
```json
{
  "items": [
    {
      "cabinetCode": "CAB-A",
      "currency": "USD",
      "eventType": "adjustment",
      "id": 1001,
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "occurredAt": "2026-04-19T12:34:56.123456789Z",
      "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "quantityAfter": 7,
      "quantityBefore": 5,
      "quantityDelta": 2,
      "reasonCode": "manual_adjustment",
      "recordedAt": "2026-04-19T12:34:57.000000000Z",
      "slotCode": "legacy-0",
      "unitPriceMinor": 199
    }
  ]
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/machines/{machineId}/inventory-events

| GET | `/v1/admin/machines/{machineId}/inventory/anomalies` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"anomalyType": "stale_inventory_sync", "createdAt": "2026-04-29T12:00:00.000000000Z", "detectedAt": "2026-04` |

### GET /v1/admin/machines/{machineId}/inventory/anomalies

- **Purpose:** List inventory anomalies for one machine
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** List inventory anomalies for one machine
```json
{
  "items": [
    {
      "anomalyType": "stale_inventory_sync",
      "createdAt": "2026-04-29T12:00:00.000000000Z",
      "detectedAt": "2026-04-29T12:00:00.000000000Z",
      "fingerprint": "stale-sync",
      "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "machineName": "Lobby A",
      "machineSerialNumber": "SN-001",
      "payload": {
        "publishedVersion": 3,
        "snapshotVersion": 2
      },
      "status": "open",
      "updatedAt": "2026-04-29T12:00:00.000000000Z"
    }
  ]
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/machines/{machineId}/inventory/anomalies

| POST | `/v1/admin/machines/{machineId}/inventory/reconcile` | 08_Machines_Runtime_Config | bearer | no | `{"status": "ok"}` |

### POST /v1/admin/machines/{machineId}/inventory/reconcile

- **Purpose:** Post machine inventory reconcile adjustment
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Post machine inventory reconcile adjustment
```json
{
  "status": "ok"
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/inventory/reconcile

| POST | `/v1/admin/machines/{machineId}/mark-compromised` | 08_Machines_Runtime_Config | bearer | no | `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 1, "id": "7c9e6679-7425-40de-944b-e0` |

### POST /v1/admin/machines/{machineId}/mark-compromised

- **Purpose:** Mark machine compromised (admin)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Mark machine compromised (admin)
```json
{
  "command_sequence": 0,
  "created_at": "2026-04-29T00:00:00Z",
  "credential_version": 1,
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "name": "Lobby A",
  "revoked_at": "2026-04-29T00:05:00Z",
  "serial_number": "SN-001",
  "site_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "status": "compromised",
  "updated_at": "2026-04-29T00:05:00Z"
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/mark-compromised

| PUT | `/v1/admin/machines/{machineId}/planograms/draft` | 08_Machines_Runtime_Config | bearer | yes | `{}` |

### PUT /v1/admin/machines/{machineId}/planograms/draft

- **Purpose:** Save draft cabinet slot planogram assignments
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "items": [
    {
      "cabinetCode": "A",
      "layoutKey": "grid-4x6",
      "layoutRevision": 1,
      "legacySlotIndex": 3,
      "maxQuantity": 12,
      "metadata": {},
      "priceMinor": 150,
      "productId": "{{productId}}",
      "slotCode": "A3"
    }
  ],
  "operator_session_id": "{{operatorSessionId}}",
  "planogramId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
  "planogramRevision": 3,
  "syncLegacyReadModel": true
}
```
- **Response 204:** Save draft cabinet slot planogram assignments
```json
{}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:PUT /v1/admin/machines/{machineId}/planograms/draft

| POST | `/v1/admin/machines/{machineId}/planograms/publish` | 08_Machines_Runtime_Config | bearer | yes | `{"command": {"commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "dispatchState": "published", "replay": false, "sequen` |

### POST /v1/admin/machines/{machineId}/planograms/publish

- **Purpose:** Publish draft planogram as current and dispatch device command
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "items": [
    {
      "cabinetCode": "A",
      "layoutKey": "grid-4x6",
      "layoutRevision": 1,
      "legacySlotIndex": 3,
      "maxQuantity": 12,
      "metadata": {},
      "priceMinor": 150,
      "productId": "{{productId}}",
      "slotCode": "A3"
    }
  ],
  "operator_session_id": "{{operatorSessionId}}",
  "planogramId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
  "planogramRevision": 3,
  "syncLegacyReadModel": true
}
```
- **Response 200:** Publish draft planogram as current and dispatch device command
```json
{
  "command": {
    "commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
    "dispatchState": "published",
    "replay": false,
    "sequence": 43
  },
  "desiredConfigVersion": 7,
  "planogramId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
  "planogramRevision": 3
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/planograms/publish

| GET | `/v1/admin/machines/{machineId}/refill-suggestions` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"currentQuantity": 3, "dailyVelocity": 1.0, "daysToEmpty": 3.0, "fillRatio": 0.3, "machineId": "7c9e6679-742` |

### GET /v1/admin/machines/{machineId}/refill-suggestions

- **Purpose:** Refill suggestions for one machine
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Query params:** `site_id, machine_id, product_id, velocity_days, urgency, days_threshold, limit, offset`
- **Response 200:** Refill suggestions for one machine
```json
{
  "items": [
    {
      "currentQuantity": 3,
      "dailyVelocity": 1.0,
      "daysToEmpty": 3.0,
      "fillRatio": 0.3,
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "machineName": "Lobby-01",
      "maxQuantity": 10,
      "planogramId": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
      "planogramName": "Lobby default",
      "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "productName": "Cola 12oz",
      "productSku": "COLA-12",
      "siteId": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
      "siteName": "HQ",
      "slotIndex": 0,
      "suggestedRefillQuantity": 7,
      "unitsSoldInWindow": 14,
      "urgency": "medium"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  },
  "velocityWindowDays": 14,
  "windowEnd": "2026-04-28T00:00:00.000000000Z",
  "windowStart": "2026-04-14T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/machines/{machineId}/refill-suggestions

| POST | `/v1/admin/machines/{machineId}/resume` | 08_Machines_Runtime_Config | bearer | no | `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 0, "id": "7c9e6679-7425-40de-944b-e0` |

### POST /v1/admin/machines/{machineId}/resume

- **Purpose:** Resume suspended machine (admin)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Resume suspended machine (admin)
```json
{
  "command_sequence": 0,
  "created_at": "2026-04-29T00:00:00Z",
  "credential_version": 0,
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "name": "Lobby A",
  "serial_number": "SN-001",
  "site_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "status": "active",
  "updated_at": "2026-04-29T00:05:00Z"
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/resume

| POST | `/v1/admin/machines/{machineId}/retire` | 08_Machines_Runtime_Config | bearer | no | `{"command_sequence": 0, "created_at": "2026-04-01T00:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "n` |

### POST /v1/admin/machines/{machineId}/retire

- **Purpose:** Retire machine (admin)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Retire machine (admin)
```json
{
  "command_sequence": 0,
  "created_at": "2026-04-01T00:00:00.000000000Z",
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "name": "New unit",
  "serial_number": "SN-NEW",
  "site_id": "aaaaaaaa-bbbb-cccc-dddd-111111111111",
  "status": "provisioning",
  "updated_at": "2026-04-01T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/retire

| POST | `/v1/admin/machines/{machineId}/revoke-credentials` | 08_Machines_Runtime_Config | bearer | no | `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 3, "id": "7c9e6679-7425-40de-944b-e0` |

### POST /v1/admin/machines/{machineId}/revoke-credentials

- **Purpose:** Revoke machine credential material (admin)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Revoke machine credential material (admin)
```json
{
  "command_sequence": 0,
  "created_at": "2026-04-29T00:00:00Z",
  "credential_version": 3,
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "name": "Lobby A",
  "revoked_at": "2026-04-29T00:05:00Z",
  "serial_number": "SN-001",
  "site_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "status": "active",
  "updated_at": "2026-04-29T00:05:00Z"
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/revoke-credentials

| POST | `/v1/admin/machines/{machineId}/revoke-sessions` | 08_Machines_Runtime_Config | bearer | no | `{}` |

### POST /v1/admin/machines/{machineId}/revoke-sessions

- **Purpose:** Revoke interactive sessions for machine technicians/operators (admin)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 204:** Revoke interactive sessions for machine technicians/operators (admin)
```json
{}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/revoke-sessions

| POST | `/v1/admin/machines/{machineId}/revoke-token` | 08_Machines_Runtime_Config | bearer | no | `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 3, "id": "7c9e6679-7425-40de-944b-e0` |

### POST /v1/admin/machines/{machineId}/revoke-token

- **Purpose:** Revoke machine tokens (alias path)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Revoke machine tokens (alias path)
```json
{
  "command_sequence": 0,
  "created_at": "2026-04-29T00:00:00Z",
  "credential_version": 3,
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "name": "Lobby A",
  "revoked_at": "2026-04-29T00:05:00Z",
  "serial_number": "SN-001",
  "site_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "status": "active",
  "updated_at": "2026-04-29T00:05:00Z"
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/revoke-token

| POST | `/v1/admin/machines/{machineId}/rotate-credential` | 08_Machines_Runtime_Config | bearer | no | `{"command_sequence": 0, "created_at": "2026-04-01T00:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "n` |

### POST /v1/admin/machines/{machineId}/rotate-credential

- **Purpose:** Rotate machine credential (admin)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Rotate machine credential (admin)
```json
{
  "command_sequence": 0,
  "created_at": "2026-04-01T00:00:00.000000000Z",
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "name": "New unit",
  "serial_number": "SN-NEW",
  "site_id": "aaaaaaaa-bbbb-cccc-dddd-111111111111",
  "status": "provisioning",
  "updated_at": "2026-04-01T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/rotate-credential

| POST | `/v1/admin/machines/{machineId}/rotate-credentials` | 08_Machines_Runtime_Config | bearer | no | `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 2, "id": "7c9e6679-7425-40de-944b-e0` |

### POST /v1/admin/machines/{machineId}/rotate-credentials

- **Purpose:** Rotate machine credential (plural alias)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Rotate machine credential (plural alias)
```json
{
  "command_sequence": 0,
  "created_at": "2026-04-29T00:00:00Z",
  "credential_version": 2,
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "name": "Lobby A",
  "rotated_at": "2026-04-29T00:05:00Z",
  "serial_number": "SN-001",
  "site_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "status": "active",
  "updated_at": "2026-04-29T00:05:00Z"
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/rotate-credentials

| POST | `/v1/admin/machines/{machineId}/rotate-token-version` | 08_Machines_Runtime_Config | bearer | no | `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 2, "id": "7c9e6679-7425-40de-944b-e0` |

### POST /v1/admin/machines/{machineId}/rotate-token-version

- **Purpose:** Bump credential version / rotate token (admin)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Bump credential version / rotate token (admin)
```json
{
  "command_sequence": 0,
  "created_at": "2026-04-29T00:00:00Z",
  "credential_version": 2,
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "name": "Lobby A",
  "rotated_at": "2026-04-29T00:05:00Z",
  "serial_number": "SN-001",
  "site_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "status": "active",
  "updated_at": "2026-04-29T00:05:00Z"
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/rotate-token-version

| GET | `/v1/admin/machines/{machineId}/slots` | 08_Machines_Runtime_Config | bearer | no | `{"cabinets": [], "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "slots": [{"cabinetCode": "A", "currentQuantity": ` |

### GET /v1/admin/machines/{machineId}/slots

- **Purpose:** List live slot inventory for a machine (restock / cycle-count UI)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** List live slot inventory for a machine (restock / cycle-count UI)
```json
{
  "cabinets": [],
  "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "slots": [
    {
      "cabinetCode": "A",
      "currentQuantity": 8,
      "maxQuantity": 12,
      "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "slotCode": "A3",
      "slotIndex": 3
    }
  ]
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/machines/{machineId}/slots

| POST | `/v1/admin/machines/{machineId}/stock-adjustments` | 08_Machines_Runtime_Config | bearer | yes | `{"eventIds": [1001, 1002], "replay": false}` |

### POST /v1/admin/machines/{machineId}/stock-adjustments

- **Purpose:** Apply stock adjustments (restock, cycle count, manual, reconcile)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "items": [
    {
      "cabinetCode": "A",
      "planogramId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "productId": "{{productId}}",
      "quantityAfter": 10,
      "quantityBefore": 2,
      "slotCode": "A3",
      "slotIndex": 3
    }
  ],
  "occurredAt": "2026-04-19T12:00:00.000000000Z",
  "operator_session_id": "{{operatorSessionId}}",
  "reason": "restock"
}
```
- **Response 200:** Apply stock adjustments (restock, cycle count, manual, reconcile)
```json
{
  "eventIds": [
    1001,
    1002
  ],
  "replay": false
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/stock-adjustments

| POST | `/v1/admin/machines/{machineId}/suspend` | 08_Machines_Runtime_Config | bearer | no | `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 0, "id": "7c9e6679-7425-40de-944b-e0` |

### POST /v1/admin/machines/{machineId}/suspend

- **Purpose:** Suspend machine (admin)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Suspend machine (admin)
```json
{
  "command_sequence": 0,
  "created_at": "2026-04-29T00:00:00Z",
  "credential_version": 0,
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "name": "Lobby A",
  "serial_number": "SN-001",
  "site_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "status": "suspended",
  "updated_at": "2026-04-29T00:05:00Z"
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/suspend

| POST | `/v1/admin/machines/{machineId}/sync` | 08_Machines_Runtime_Config | bearer | yes | `{"command": {"commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "dispatchState": "published", "replay": false, "sequen` |

### POST /v1/admin/machines/{machineId}/sync

- **Purpose:** Queue a machine setup / inventory sync command
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "operator_session_id": "{{operatorSessionId}}",
  "reason": "post_restock_verify"
}
```
- **Response 200:** Queue a machine setup / inventory sync command
```json
{
  "command": {
    "commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
    "dispatchState": "published",
    "replay": false,
    "sequence": 43
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/sync

| GET | `/v1/admin/machines/{machineId}/technicians` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"assignmentId": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "createdAt": "2026-04-29T00:00:00Z", "machineId": "7` |

### GET /v1/admin/machines/{machineId}/technicians

- **Purpose:** List technicians assigned to a machine
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** List technicians assigned to a machine
```json
{
  "items": [
    {
      "assignmentId": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "createdAt": "2026-04-29T00:00:00Z",
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "machineName": "Lobby A",
      "machineSerialNumber": "SN-001",
      "role": "field_service",
      "technicianDisplayName": "Field Tech",
      "technicianId": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
      "validFrom": "2026-04-29T00:00:00Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/machines/{machineId}/technicians

| POST | `/v1/admin/machines/{machineId}/technicians` | 08_Machines_Runtime_Config | bearer | yes | `{"created_at": "2026-04-29T00:00:00Z", "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "machine_id": "7c9e6679-7425-40de-9` |

### POST /v1/admin/machines/{machineId}/technicians

- **Purpose:** Assign technician user to machine
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "role": "field_service",
  "scope": "maintenance",
  "userId": "{{$guid}}"
}
```
- **Response 201:** Assign technician user to machine
```json
{
  "created_at": "2026-04-29T00:00:00Z",
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "role": "field_service",
  "scope": "maintenance",
  "status": "active",
  "technician_id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
  "updated_at": "2026-04-29T00:00:00Z",
  "valid_from": "2026-04-29T00:00:00Z"
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/technicians

| DELETE | `/v1/admin/machines/{machineId}/technicians/{userId}` | 08_Machines_Runtime_Config | bearer | no | `""` |

### DELETE /v1/admin/machines/{machineId}/technicians/{userId}

- **Purpose:** Remove technician assignment from machine by user id
- **Auth:** bearer (required=True)
- **Path params:** `machineId, userId`
- **Response 204:** Remove technician assignment from machine by user id
```json
""
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/machines/{machineId}/technicians/{userId}

| GET | `/v1/admin/machines/{machineId}/timeline` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"eventKind": "command_attempt", "occurredAt": "2026-04-29T12:00:00.000000000Z", "payload": {"status": "sent"` |

### GET /v1/admin/machines/{machineId}/timeline

- **Purpose:** Machine operational timeline
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Machine operational timeline
```json
{
  "items": [
    {
      "eventKind": "command_attempt",
      "occurredAt": "2026-04-29T12:00:00.000000000Z",
      "payload": {
        "status": "sent"
      },
      "refId": "cccccccc-dddd-eeee-ffff-000000000001",
      "title": "Attempt sent"
    }
  ]
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/machines/{machineId}/timeline

| PUT | `/v1/admin/machines/{machineId}/topology` | 08_Machines_Runtime_Config | bearer | yes | `{}` |

### PUT /v1/admin/machines/{machineId}/topology

- **Purpose:** Upsert machine cabinet topology and slot layouts
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "cabinets": [
    {
      "code": "canary-code-{{$guid}}",
      "metadata": {},
      "sortOrder": 1,
      "title": "canary-title-{{$guid}}"
    }
  ],
  "layouts": [
    {
      "cabinetCode": "A",
      "layoutKey": "grid-4x6",
      "layoutSpec": {
        "cols": 6,
        "rows": 4
      },
      "revision": 1,
      "status": "active"
    }
  ],
  "operator_session_id": "{{operatorSessionId}}"
}
```
- **Response 204:** Upsert machine cabinet topology and slot layouts
```json
{}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:PUT /v1/admin/machines/{machineId}/topology

| POST | `/v1/admin/machines/{machineId}/transfer-site` | 08_Machines_Runtime_Config | bearer | yes | `{"command_sequence": 0, "created_at": "2026-04-29T00:00:00Z", "credential_version": 0, "id": "7c9e6679-7425-40de-944b-e0` |

### POST /v1/admin/machines/{machineId}/transfer-site

- **Purpose:** Move machine to another site (admin)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "site_id": "{{siteId}}"
}
```
- **Response 200:** Move machine to another site (admin)
```json
{
  "command_sequence": 0,
  "created_at": "2026-04-29T00:00:00Z",
  "credential_version": 0,
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "name": "Lobby A",
  "serial_number": "SN-001",
  "site_id": "11111111-2222-3333-4444-555555555555",
  "status": "active",
  "updated_at": "2026-04-29T00:06:00Z"
}
```
- **Captures:** response body machine_id→machineId, response body site_id→siteId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/machines/{machineId}/transfer-site

| GET | `/v1/admin/media` | 04_Product_Media_Offline_Cache | bearer | no | `{"items": [{"created_at": "2026-01-01T00:00:00Z", "etag": "W/\"etag1\"", "id": "11111111-2222-3333-4444-555555555555", "` |

### GET /v1/admin/media

- **Purpose:** List media assets (alias path)
- **Auth:** bearer (required=True)
- **Response 200:** List media assets (alias path)
```json
{
  "items": [
    {
      "created_at": "2026-01-01T00:00:00Z",
      "etag": "W/\"etag1\"",
      "id": "11111111-2222-3333-4444-555555555555",
      "kind": "product_image",
      "mime_type": "image/webp",
      "object_version": 1,
      "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "size_bytes": 12000,
      "status": "ready",
      "updated_at": "2026-04-19T10:00:00Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "totalCount": 1
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/media

| GET | `/v1/admin/media/assets` | 04_Product_Media_Offline_Cache | bearer | no | `{"items": [{"created_at": "2026-01-01T00:00:00Z", "etag": "W/\"etag1\"", "id": "11111111-2222-3333-4444-555555555555", "` |

### GET /v1/admin/media/assets

- **Purpose:** List media assets (admin)
- **Auth:** bearer (required=True)
- **Query params:** `limit, offset`
- **Response 200:** List media assets (admin)
```json
{
  "items": [
    {
      "created_at": "2026-01-01T00:00:00Z",
      "etag": "W/\"etag1\"",
      "id": "11111111-2222-3333-4444-555555555555",
      "kind": "product_image",
      "mime_type": "image/webp",
      "object_version": 1,
      "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      "size_bytes": 12000,
      "status": "ready",
      "updated_at": "2026-04-19T10:00:00Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "totalCount": 1
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/media/assets

| POST | `/v1/admin/media/assets` | 04_Product_Media_Offline_Cache | bearer | yes | `{"complete_path": "/v1/admin/media/11111111-2222-3333-4444-555555555555/complete", "expires_at": "2026-04-19T13:00:00Z",` |

### POST /v1/admin/media/assets

- **Purpose:** Start enterprise media asset upload
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "content_type": "image/jpeg"
}
```
- **Response 200:** Start enterprise media asset upload
```json
{
  "complete_path": "/v1/admin/media/11111111-2222-3333-4444-555555555555/complete",
  "expires_at": "2026-04-19T13:00:00Z",
  "media_id": "11111111-2222-3333-4444-555555555555",
  "upload_headers": {
    "Content-Type": [
      "image/jpeg"
    ]
  },
  "upload_method": "PUT",
  "upload_url": "https://s3.example.com/presigned-put"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/media/assets

| DELETE | `/v1/admin/media/assets/{mediaId}` | 04_Product_Media_Offline_Cache | bearer | no | `{}` |

### DELETE /v1/admin/media/assets/{mediaId}

- **Purpose:** Delete media asset (admin)
- **Auth:** bearer (required=True)
- **Path params:** `mediaId`
- **Response 204:** Delete media asset (admin)
```json
{}
```
- **Response 404:** error
```json
{
  "error": {
    "code": "not_found",
    "details": {},
    "message": "resource was not found",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/media/assets/{mediaId}

| GET | `/v1/admin/media/assets/{mediaId}` | 04_Product_Media_Offline_Cache | bearer | no | `{"created_at": "2026-01-01T00:00:00Z", "etag": "W/\"etag1\"", "id": "11111111-2222-3333-4444-555555555555", "kind": "pro` |

### GET /v1/admin/media/assets/{mediaId}

- **Purpose:** Get one media asset by id (admin)
- **Auth:** bearer (required=True)
- **Path params:** `mediaId`
- **Response 200:** Get one media asset by id (admin)
```json
{
  "created_at": "2026-01-01T00:00:00Z",
  "etag": "W/\"etag1\"",
  "id": "11111111-2222-3333-4444-555555555555",
  "kind": "product_image",
  "mime_type": "image/webp",
  "object_version": 1,
  "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "size_bytes": 12000,
  "status": "ready",
  "updated_at": "2026-04-19T10:00:00Z"
}
```
- **Response 404:** error
```json
{
  "error": {
    "code": "not_found",
    "details": {},
    "message": "resource was not found",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/media/assets/{mediaId}

| POST | `/v1/admin/media/uploads` | 04_Product_Media_Offline_Cache | bearer | yes | `{"complete_path": "/v1/admin/media/11111111-2222-3333-4444-555555555555/complete", "expires_at": "2026-04-19T13:00:00Z",` |

### POST /v1/admin/media/uploads

- **Purpose:** Start enterprise media upload (presigned PUT)
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "content_type": "image/jpeg"
}
```
- **Response 200:** Start enterprise media upload (presigned PUT)
```json
{
  "complete_path": "/v1/admin/media/11111111-2222-3333-4444-555555555555/complete",
  "expires_at": "2026-04-19T13:00:00Z",
  "media_id": "11111111-2222-3333-4444-555555555555",
  "upload_headers": {
    "Content-Type": [
      "image/jpeg"
    ]
  },
  "upload_method": "PUT",
  "upload_url": "https://s3.example.com/presigned-put"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/media/uploads

| POST | `/v1/admin/media/uploads/init` | 04_Product_Media_Offline_Cache | bearer | yes | `{"completePath": "/v1/admin/media/uploads/11111111-2222-3333-4444-555555555555/complete", "mediaId": "11111111-2222-3333` |

### POST /v1/admin/media/uploads/init

- **Purpose:** Start enterprise media upload (camelCase contract)
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "contentType": "image/png",
  "filename": "coca-330ml.png",
  "purpose": "product_image"
}
```
- **Response 200:** Start enterprise media upload (camelCase contract)
```json
{
  "completePath": "/v1/admin/media/uploads/11111111-2222-3333-4444-555555555555/complete",
  "mediaId": "11111111-2222-3333-4444-555555555555",
  "objectKey": "org/11111111-2222-3333-4444-555555555555/original",
  "status": "pending",
  "uploadUrl": "https://s3.example.com/presigned-put"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/media/uploads/init

| POST | `/v1/admin/media/uploads/{mediaId}/complete` | 04_Product_Media_Offline_Cache | bearer | yes | `{"id": "11111111-2222-3333-4444-555555555555", "status": "ready", "variants": [{"downloadUrl": "https://cdn.example.com/` |

### POST /v1/admin/media/uploads/{mediaId}/complete

- **Purpose:** Finalize media upload (uploads/{mediaId}/complete alias)
- **Auth:** bearer (required=True)
- **Path params:** `mediaId`
- **Request body example:**
```json
{
  "contentType": "image/png",
  "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
  "sizeBytes": 12345
}
```
- **Response 200:** Finalize media upload (uploads/{mediaId}/complete alias)
```json
{
  "id": "11111111-2222-3333-4444-555555555555",
  "status": "ready",
  "variants": [
    {
      "downloadUrl": "https://cdn.example.com/products/cola-thumb.webp",
      "height": 160,
      "mimeType": "image/webp",
      "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "sizeBytes": 8000,
      "variant": "thumb",
      "version": 1,
      "width": 160
    },
    {
      "downloadUrl": "https://cdn.example.com/products/cola-display.webp",
      "height": 512,
      "mimeType": "image/webp",
      "sha256": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
      "sizeBytes": 24000,
      "variant": "display",
      "version": 2,
      "width": 512
    }
  ]
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/media/uploads/{mediaId}/complete

| DELETE | `/v1/admin/media/{mediaId}` | 04_Product_Media_Offline_Cache | bearer | no | `""` |

### DELETE /v1/admin/media/{mediaId}

- **Purpose:** Delete media asset (alias path)
- **Auth:** bearer (required=True)
- **Path params:** `mediaId`
- **Response 204:** Delete media asset (alias path)
```json
""
```
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/media/{mediaId}

| GET | `/v1/admin/media/{mediaId}` | 04_Product_Media_Offline_Cache | bearer | no | `{"created_at": "2026-01-01T00:00:00Z", "etag": "W/\"etag1\"", "id": "11111111-2222-3333-4444-555555555555", "kind": "pro` |

### GET /v1/admin/media/{mediaId}

- **Purpose:** Get media asset (alias path)
- **Auth:** bearer (required=True)
- **Path params:** `mediaId`
- **Response 200:** Get media asset (alias path)
```json
{
  "created_at": "2026-01-01T00:00:00Z",
  "etag": "W/\"etag1\"",
  "id": "11111111-2222-3333-4444-555555555555",
  "kind": "product_image",
  "mime_type": "image/webp",
  "object_version": 1,
  "sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "size_bytes": 12000,
  "status": "ready",
  "updated_at": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/media/{mediaId}

| POST | `/v1/admin/media/{mediaId}/complete` | 04_Product_Media_Offline_Cache | bearer | yes | `{"id": "11111111-2222-3333-4444-555555555555", "status": "ready", "variants": [{"downloadUrl": "https://cdn.example.com/` |

### POST /v1/admin/media/{mediaId}/complete

- **Purpose:** Finalize media upload (variants + ready)
- **Auth:** bearer (required=True)
- **Path params:** `mediaId`
- **Request body example:**
```json
{
  "contentType": "{{$guid}}",
  "sha256": "{{$guid}}",
  "sizeBytes": 0
}
```
- **Response 200:** Finalize media upload (variants + ready)
```json
{
  "id": "11111111-2222-3333-4444-555555555555",
  "status": "ready",
  "variants": [
    {
      "downloadUrl": "https://cdn.example.com/products/cola-thumb.webp",
      "height": 160,
      "mimeType": "image/webp",
      "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
      "sizeBytes": 8000,
      "variant": "thumb",
      "version": 1,
      "width": 160
    },
    {
      "downloadUrl": "https://cdn.example.com/products/cola-display.webp",
      "height": 512,
      "mimeType": "image/webp",
      "sha256": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
      "sizeBytes": 24000,
      "variant": "display",
      "version": 2,
      "width": 512
    }
  ]
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/media/{mediaId}/complete

| GET | `/v1/admin/operations/machines/health` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"appVersion": "1.4.2", "catalogVersion": "2026-04-29T00:00:00Z", "configVersion": "7", "failedCommandCount":` |

### GET /v1/admin/operations/machines/health

- **Purpose:** Fleet machine health snapshot list
- **Auth:** bearer (required=True)
- **Response 200:** Fleet machine health snapshot list
```json
{
  "items": [
    {
      "appVersion": "1.4.2",
      "catalogVersion": "2026-04-29T00:00:00Z",
      "configVersion": "7",
      "failedCommandCount": 0,
      "inventoryAnomalyCount": 0,
      "lastCheckInAt": "2026-04-29T11:58:00.000000000Z",
      "lastErrorCode": "TEMP_SENSOR_DEGRADED",
      "lastSeenAt": "2026-04-29T12:00:00.000000000Z",
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "mediaVersion": "sha256:abcd0000",
      "mqttConnected": true,
      "pendingCommandCount": 1,
      "status": "online",
      "telemetryFreshnessSeconds": 95
    }
  ]
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/operations/machines/health

| GET | `/v1/admin/ops/outbox` | 99_Utilities | bearer | no | `{"meta": {"limit": 50, "offset": 0, "returned": 1, "total": 42}, "rows": [{"aggregateId": "7c9e6679-7425-40de-944b-e07fc` |

### GET /v1/admin/ops/outbox

- **Purpose:** List transactional outbox rows and pipeline stats
- **Auth:** bearer (required=True)
- **Query params:** `limit, offset`
- **Response 200:** List transactional outbox rows and pipeline stats
```json
{
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  },
  "rows": [
    {
      "aggregateId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "aggregateType": "payment",
      "createdAt": "2026-04-19T12:00:00.000000000Z",
      "eventType": "payment.session_started",
      "id": 101,
      "payload": {},
      "publishAttemptCount": 0,
      "status": "pending",
      "topic": "commerce.payments"
    }
  ],
  "stats": {
    "deadLetteredTotal": 1,
    "maxPendingAttempts": 5,
    "oldestPendingCreatedAt": "2026-04-19T12:00:00.000000000Z",
    "pendingDueNow": 2,
    "pendingTotal": 3,
    "publishingLeasedTotal": 0
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 403:** error
```json
{
  "error": {
    "code": "forbidden",
    "details": {},
    "message": "caller lacks permission for this resource",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/ops/outbox

| POST | `/v1/admin/ops/outbox/{outboxId}/retry` | 99_Utilities | bearer | no | `{"retried": true}` |

### POST /v1/admin/ops/outbox/{outboxId}/retry

- **Purpose:** Reset a dead-lettered outbox row for retry
- **Auth:** bearer (required=True)
- **Path params:** `outboxId`
- **Response 200:** Reset a dead-lettered outbox row for retry
```json
{
  "retried": true
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/ops/outbox/{outboxId}/retry

| GET | `/v1/admin/ops/retention` | 99_Utilities | bearer | no | `{"tables": [{"oldestRecordAgeDays": 28, "oldestRecordAt": "2026-04-01T00:00:00.000000000Z", "tableName": "outbox_events"` |

### GET /v1/admin/ops/retention

- **Purpose:** Show retention table visibility
- **Auth:** bearer (required=True)
- **Response 200:** Show retention table visibility
```json
{
  "tables": [
    {
      "oldestRecordAgeDays": 28,
      "oldestRecordAt": "2026-04-01T00:00:00.000000000Z",
      "tableName": "outbox_events",
      "totalRows": 120
    },
    {
      "oldestRecordAgeDays": 45,
      "oldestRecordAt": "2026-03-15T00:00:00.000000000Z",
      "tableName": "payment_provider_events",
      "totalRows": 240
    }
  ]
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 403:** error
```json
{
  "error": {
    "code": "forbidden",
    "details": {},
    "message": "caller lacks permission for this resource",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/ops/retention

| POST | `/v1/admin/orders/{orderId}/refunds` | 14_Refunds_Disputes | bearer | yes | `{"ledgerAmountMinor": 100, "ledgerCurrency": "USD", "ledgerRefundID": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "ledgerSta` |

### POST /v1/admin/orders/{orderId}/refunds

- **Purpose:** Create refund request + ledger refund (admin scoped)
- **Auth:** bearer (required=True)
- **Path params:** `orderId`
- **Request body example:**
```json
{
  "amountMinor": 100,
  "reason": "customer courtesy"
}
```
- **Response 200:** Create refund request + ledger refund (admin scoped)
```json
{
  "ledgerAmountMinor": 100,
  "ledgerCurrency": "USD",
  "ledgerRefundID": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "ledgerState": "requested",
  "refundRequest": {}
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body order_id→orderId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/orders/{orderId}/refunds

| GET | `/v1/admin/orders/{orderId}/timeline` | 12_Orders | bearer | no | `{"items": [], "meta": {"limit": 50, "offset": 0, "returned": 1, "total": 42}}` |

### GET /v1/admin/orders/{orderId}/timeline

- **Purpose:** List commerce order timeline events
- **Auth:** bearer (required=True)
- **Path params:** `orderId`
- **Query params:** `limit, offset`
- **Response 200:** List commerce order timeline events
```json
{
  "items": [],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body order_id→orderId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/orders/{orderId}/timeline

| GET | `/v1/admin/ota` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactStorageKey": "org/acme/ota/fw.bin", "campaign` |

### GET /v1/admin/ota

- **Purpose:** List OTA campaigns (admin)
- **Auth:** bearer (required=True)
- **Query params:** `status, from, to, limit, offset`
- **Response 200:** List OTA campaigns (admin)
```json
{
  "items": [
    {
      "artifactId": "dddddddd-eeee-ffff-0000-333333333333",
      "artifactStorageKey": "org/acme/ota/fw.bin",
      "campaignId": "cccccccc-dddd-eeee-ffff-000000000002",
      "campaignName": "April bundle",
      "campaignStatus": "active",
      "createdAt": "2026-04-10T00:00:00Z",
      "strategy": "rolling"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/ota

| GET | `/v1/admin/ota/campaigns` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver"` |

### GET /v1/admin/ota/campaigns

- **Purpose:** List OTA campaigns (lifecycle admin)
- **Auth:** bearer (required=True)
- **Query params:** `status, from, to, limit, offset`
- **Response 200:** List OTA campaigns (lifecycle admin)
```json
{
  "items": [
    {
      "approvedAt": "2026-04-10T00:00:00Z",
      "artifactId": "dddddddd-eeee-ffff-0000-333333333333",
      "artifactSemver": "1.2.3",
      "artifactStorageKey": "org/acme/ota/fw.bin",
      "artifactVersion": "1.2.3",
      "campaignId": "cccccccc-dddd-eeee-ffff-000000000002",
      "campaignType": "firmware",
      "canaryPercent": 10,
      "createdAt": "2026-04-10T00:00:00Z",
      "name": "April firmware",
      "rollbackArtifactId": "11111111-2222-3333-4444-555555555555",
      "rolloutNextOffset": 0,
      "rolloutStrategy": "canary",
      "status": "draft",
      "updatedAt": "2026-04-10T00:00:00Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/ota/campaigns

| POST | `/v1/admin/ota/campaigns` | 08_Machines_Runtime_Config | bearer | yes | `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", ` |

### POST /v1/admin/ota/campaigns

- **Purpose:** Create OTA campaign (draft)
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "artifactId": "dddddddd-eeee-ffff-0000-333333333333",
  "artifactVersion": "1.2.3",
  "campaignType": "firmware",
  "canaryPercent": 10,
  "name": "canary-name-{{$guid}}",
  "rolloutStrategy": "canary"
}
```
- **Response 201:** Create OTA campaign (draft)
```json
{
  "approvedAt": "2026-04-10T00:00:00Z",
  "artifactId": "dddddddd-eeee-ffff-0000-333333333333",
  "artifactSemver": "1.2.3",
  "artifactStorageKey": "org/acme/ota/fw.bin",
  "artifactVersion": "1.2.3",
  "campaignId": "cccccccc-dddd-eeee-ffff-000000000002",
  "campaignType": "firmware",
  "canaryPercent": 10,
  "createdAt": "2026-04-10T00:00:00Z",
  "name": "April firmware",
  "rollbackArtifactId": "11111111-2222-3333-4444-555555555555",
  "rolloutNextOffset": 0,
  "rolloutStrategy": "canary",
  "status": "draft",
  "updatedAt": "2026-04-10T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/ota/campaigns

| GET | `/v1/admin/ota/campaigns/{campaignId}` | 08_Machines_Runtime_Config | bearer | no | `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", ` |

### GET /v1/admin/ota/campaigns/{campaignId}

- **Purpose:** Get OTA campaign detail
- **Auth:** bearer (required=True)
- **Path params:** `campaignId`
- **Response 200:** Get OTA campaign detail
```json
{
  "approvedAt": "2026-04-10T00:00:00Z",
  "artifactId": "dddddddd-eeee-ffff-0000-333333333333",
  "artifactSemver": "1.2.3",
  "artifactStorageKey": "org/acme/ota/fw.bin",
  "artifactVersion": "1.2.3",
  "campaignId": "cccccccc-dddd-eeee-ffff-000000000002",
  "campaignType": "firmware",
  "canaryPercent": 10,
  "createdAt": "2026-04-10T00:00:00Z",
  "name": "April firmware",
  "rollbackArtifactId": "11111111-2222-3333-4444-555555555555",
  "rolloutNextOffset": 0,
  "rolloutStrategy": "canary",
  "status": "draft",
  "updatedAt": "2026-04-10T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/ota/campaigns/{campaignId}

| PATCH | `/v1/admin/ota/campaigns/{campaignId}` | 08_Machines_Runtime_Config | bearer | yes | `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", ` |

### PATCH /v1/admin/ota/campaigns/{campaignId}

- **Purpose:** Patch draft/approved OTA campaign
- **Auth:** bearer (required=True)
- **Path params:** `campaignId`
- **Request body example:**
```json
{
  "name": "canary-name-{{$guid}}"
}
```
- **Response 200:** Patch draft/approved OTA campaign
```json
{
  "approvedAt": "2026-04-10T00:00:00Z",
  "artifactId": "dddddddd-eeee-ffff-0000-333333333333",
  "artifactSemver": "1.2.3",
  "artifactStorageKey": "org/acme/ota/fw.bin",
  "artifactVersion": "1.2.3",
  "campaignId": "cccccccc-dddd-eeee-ffff-000000000002",
  "campaignType": "firmware",
  "canaryPercent": 10,
  "createdAt": "2026-04-10T00:00:00Z",
  "name": "April firmware",
  "rollbackArtifactId": "11111111-2222-3333-4444-555555555555",
  "rolloutNextOffset": 0,
  "rolloutStrategy": "canary",
  "status": "draft",
  "updatedAt": "2026-04-10T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/ota/campaigns/{campaignId}

| POST | `/v1/admin/ota/campaigns/{campaignId}/approve` | 08_Machines_Runtime_Config | bearer | no | `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", ` |

### POST /v1/admin/ota/campaigns/{campaignId}/approve

- **Purpose:** Approve OTA campaign
- **Auth:** bearer (required=True)
- **Path params:** `campaignId`
- **Response 200:** Approve OTA campaign
```json
{
  "approvedAt": "2026-04-10T00:00:00Z",
  "artifactId": "dddddddd-eeee-ffff-0000-333333333333",
  "artifactSemver": "1.2.3",
  "artifactStorageKey": "org/acme/ota/fw.bin",
  "artifactVersion": "1.2.3",
  "campaignId": "cccccccc-dddd-eeee-ffff-000000000002",
  "campaignType": "firmware",
  "canaryPercent": 10,
  "createdAt": "2026-04-10T00:00:00Z",
  "name": "April firmware",
  "rollbackArtifactId": "11111111-2222-3333-4444-555555555555",
  "rolloutNextOffset": 0,
  "rolloutStrategy": "canary",
  "status": "draft",
  "updatedAt": "2026-04-10T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/ota/campaigns/{campaignId}/approve

| POST | `/v1/admin/ota/campaigns/{campaignId}/cancel` | 08_Machines_Runtime_Config | bearer | no | `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", ` |

### POST /v1/admin/ota/campaigns/{campaignId}/cancel

- **Purpose:** Cancel OTA campaign
- **Auth:** bearer (required=True)
- **Path params:** `campaignId`
- **Response 200:** Cancel OTA campaign
```json
{
  "approvedAt": "2026-04-10T00:00:00Z",
  "artifactId": "dddddddd-eeee-ffff-0000-333333333333",
  "artifactSemver": "1.2.3",
  "artifactStorageKey": "org/acme/ota/fw.bin",
  "artifactVersion": "1.2.3",
  "campaignId": "cccccccc-dddd-eeee-ffff-000000000002",
  "campaignType": "firmware",
  "canaryPercent": 10,
  "createdAt": "2026-04-10T00:00:00Z",
  "name": "April firmware",
  "rollbackArtifactId": "11111111-2222-3333-4444-555555555555",
  "rolloutNextOffset": 0,
  "rolloutStrategy": "canary",
  "status": "draft",
  "updatedAt": "2026-04-10T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/ota/campaigns/{campaignId}/cancel

| POST | `/v1/admin/ota/campaigns/{campaignId}/pause` | 08_Machines_Runtime_Config | bearer | no | `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", ` |

### POST /v1/admin/ota/campaigns/{campaignId}/pause

- **Purpose:** Pause active rollout
- **Auth:** bearer (required=True)
- **Path params:** `campaignId`
- **Response 200:** Pause active rollout
```json
{
  "approvedAt": "2026-04-10T00:00:00Z",
  "artifactId": "dddddddd-eeee-ffff-0000-333333333333",
  "artifactSemver": "1.2.3",
  "artifactStorageKey": "org/acme/ota/fw.bin",
  "artifactVersion": "1.2.3",
  "campaignId": "cccccccc-dddd-eeee-ffff-000000000002",
  "campaignType": "firmware",
  "canaryPercent": 10,
  "createdAt": "2026-04-10T00:00:00Z",
  "name": "April firmware",
  "rollbackArtifactId": "11111111-2222-3333-4444-555555555555",
  "rolloutNextOffset": 0,
  "rolloutStrategy": "canary",
  "status": "draft",
  "updatedAt": "2026-04-10T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/ota/campaigns/{campaignId}/pause

| POST | `/v1/admin/ota/campaigns/{campaignId}/publish` | 08_Machines_Runtime_Config | bearer | no | `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", ` |

### POST /v1/admin/ota/campaigns/{campaignId}/publish

- **Purpose:** Publish OTA campaign (approve + start when needed)
- **Auth:** bearer (required=True)
- **Path params:** `campaignId`
- **Response 200:** Publish OTA campaign (approve + start when needed)
```json
{
  "approvedAt": "2026-04-10T00:00:00Z",
  "artifactId": "dddddddd-eeee-ffff-0000-333333333333",
  "artifactSemver": "1.2.3",
  "artifactStorageKey": "org/acme/ota/fw.bin",
  "artifactVersion": "1.2.3",
  "campaignId": "cccccccc-dddd-eeee-ffff-000000000002",
  "campaignType": "firmware",
  "canaryPercent": 10,
  "createdAt": "2026-04-10T00:00:00Z",
  "name": "April firmware",
  "rollbackArtifactId": "11111111-2222-3333-4444-555555555555",
  "rolloutNextOffset": 0,
  "rolloutStrategy": "canary",
  "status": "draft",
  "updatedAt": "2026-04-10T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/ota/campaigns/{campaignId}/publish

| GET | `/v1/admin/ota/campaigns/{campaignId}/results` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "createdAt": "2026-04-19T12:00:00.000000000Z", "machine` |

### GET /v1/admin/ota/campaigns/{campaignId}/results

- **Purpose:** List campaign machine rollout results
- **Auth:** bearer (required=True)
- **Path params:** `campaignId`
- **Response 200:** List campaign machine rollout results
```json
{
  "items": [
    {
      "commandId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
      "createdAt": "2026-04-19T12:00:00.000000000Z",
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "status": "dispatched",
      "updatedAt": "2026-04-19T12:00:00.000000000Z",
      "wave": "canary"
    }
  ]
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/ota/campaigns/{campaignId}/results

| POST | `/v1/admin/ota/campaigns/{campaignId}/resume` | 08_Machines_Runtime_Config | bearer | no | `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", ` |

### POST /v1/admin/ota/campaigns/{campaignId}/resume

- **Purpose:** Resume paused rollout (remaining machines)
- **Auth:** bearer (required=True)
- **Path params:** `campaignId`
- **Response 200:** Resume paused rollout (remaining machines)
```json
{
  "approvedAt": "2026-04-10T00:00:00Z",
  "artifactId": "dddddddd-eeee-ffff-0000-333333333333",
  "artifactSemver": "1.2.3",
  "artifactStorageKey": "org/acme/ota/fw.bin",
  "artifactVersion": "1.2.3",
  "campaignId": "cccccccc-dddd-eeee-ffff-000000000002",
  "campaignType": "firmware",
  "canaryPercent": 10,
  "createdAt": "2026-04-10T00:00:00Z",
  "name": "April firmware",
  "rollbackArtifactId": "11111111-2222-3333-4444-555555555555",
  "rolloutNextOffset": 0,
  "rolloutStrategy": "canary",
  "status": "draft",
  "updatedAt": "2026-04-10T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/ota/campaigns/{campaignId}/resume

| POST | `/v1/admin/ota/campaigns/{campaignId}/rollback` | 08_Machines_Runtime_Config | bearer | yes | `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", ` |

### POST /v1/admin/ota/campaigns/{campaignId}/rollback

- **Purpose:** Rollback OTA campaign (dispatch rollback commands)
- **Auth:** bearer (required=True)
- **Path params:** `campaignId`
- **Request body example:**
```json
{
  "rollbackArtifactId": "dddddddd-eeee-ffff-0000-333333333333"
}
```
- **Response 200:** Rollback OTA campaign (dispatch rollback commands)
```json
{
  "approvedAt": "2026-04-10T00:00:00Z",
  "artifactId": "dddddddd-eeee-ffff-0000-333333333333",
  "artifactSemver": "1.2.3",
  "artifactStorageKey": "org/acme/ota/fw.bin",
  "artifactVersion": "1.2.3",
  "campaignId": "cccccccc-dddd-eeee-ffff-000000000002",
  "campaignType": "firmware",
  "canaryPercent": 10,
  "createdAt": "2026-04-10T00:00:00Z",
  "name": "April firmware",
  "rollbackArtifactId": "11111111-2222-3333-4444-555555555555",
  "rolloutNextOffset": 0,
  "rolloutStrategy": "canary",
  "status": "draft",
  "updatedAt": "2026-04-10T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/ota/campaigns/{campaignId}/rollback

| POST | `/v1/admin/ota/campaigns/{campaignId}/start` | 08_Machines_Runtime_Config | bearer | no | `{"approvedAt": "2026-04-10T00:00:00Z", "artifactId": "dddddddd-eeee-ffff-0000-333333333333", "artifactSemver": "1.2.3", ` |

### POST /v1/admin/ota/campaigns/{campaignId}/start

- **Purpose:** Start OTA rollout (canary first wave)
- **Auth:** bearer (required=True)
- **Path params:** `campaignId`
- **Response 200:** Start OTA rollout (canary first wave)
```json
{
  "approvedAt": "2026-04-10T00:00:00Z",
  "artifactId": "dddddddd-eeee-ffff-0000-333333333333",
  "artifactSemver": "1.2.3",
  "artifactStorageKey": "org/acme/ota/fw.bin",
  "artifactVersion": "1.2.3",
  "campaignId": "cccccccc-dddd-eeee-ffff-000000000002",
  "campaignType": "firmware",
  "canaryPercent": 10,
  "createdAt": "2026-04-10T00:00:00Z",
  "name": "April firmware",
  "rollbackArtifactId": "11111111-2222-3333-4444-555555555555",
  "rolloutNextOffset": 0,
  "rolloutStrategy": "canary",
  "status": "draft",
  "updatedAt": "2026-04-10T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/ota/campaigns/{campaignId}/start

| GET | `/v1/admin/ota/campaigns/{campaignId}/targets` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "state": "pending", "updatedAt": "2026-04-19T12:00:00.0` |

### GET /v1/admin/ota/campaigns/{campaignId}/targets

- **Purpose:** List campaign machine targets
- **Auth:** bearer (required=True)
- **Path params:** `campaignId`
- **Response 200:** List campaign machine targets
```json
{
  "items": [
    {
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "state": "pending",
      "updatedAt": "2026-04-19T12:00:00.000000000Z"
    }
  ]
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/ota/campaigns/{campaignId}/targets

| PUT | `/v1/admin/ota/campaigns/{campaignId}/targets` | 08_Machines_Runtime_Config | bearer | yes | `{}` |

### PUT /v1/admin/ota/campaigns/{campaignId}/targets

- **Purpose:** Replace campaign machine targets (draft/approved only)
- **Auth:** bearer (required=True)
- **Path params:** `campaignId`
- **Request body example:**
```json
{
  "machineIds": [
    "{{$guid}}"
  ]
}
```
- **Response 204:** Replace campaign machine targets (draft/approved only)
```json
{}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:PUT /v1/admin/ota/campaigns/{campaignId}/targets

| GET | `/v1/admin/planograms` | 11_Planogram_Assortment | bearer | no | `{"items": [{"createdAt": "2026-04-01T00:00:00Z", "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff", "name": "Lobby spring", "` |

### GET /v1/admin/planograms

- **Purpose:** List planograms (admin catalog)
- **Auth:** bearer (required=True)
- **Query params:** `limit, offset`
- **Response 200:** List planograms (admin catalog)
```json
{
  "items": [
    {
      "createdAt": "2026-04-01T00:00:00Z",
      "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "name": "Lobby spring",
      "revision": 3,
      "status": "published"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "totalCount": 1
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/planograms

| GET | `/v1/admin/planograms/{planogramId}` | 11_Planogram_Assortment | bearer | no | `{"planogram": {"createdAt": "2026-04-01T00:00:00Z", "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff", "name": "Lobby spring"` |

### GET /v1/admin/planograms/{planogramId}

- **Purpose:** Get planogram detail with slots
- **Auth:** bearer (required=True)
- **Path params:** `planogramId`
- **Response 200:** Get planogram detail with slots
```json
{
  "planogram": {
    "createdAt": "2026-04-01T00:00:00Z",
    "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
    "name": "Lobby spring",
    "revision": 3,
    "status": "published"
  },
  "slots": [
    {
      "createdAt": "2026-04-01T00:00:00Z",
      "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
      "maxQuantity": 10,
      "planogramId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "productName": "Cola 12oz",
      "productSku": "COLA-12",
      "slotIndex": 1
    }
  ]
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/planograms/{planogramId}

| GET | `/v1/admin/price-books` | 15_Promotions_PriceBooks | bearer | no | `{"items": [{"active": true, "createdAt": "2026-01-01T00:00:00Z", "currency": "USD", "effectiveFrom": "2026-01-01T00:00:0` |

### GET /v1/admin/price-books

- **Purpose:** List price books (admin catalog)
- **Auth:** bearer (required=True)
- **Query params:** `limit, offset, include_inactive`
- **Response 200:** List price books (admin catalog)
```json
{
  "items": [
    {
      "active": true,
      "createdAt": "2026-01-01T00:00:00Z",
      "currency": "USD",
      "effectiveFrom": "2026-01-01T00:00:00Z",
      "id": "11111111-2222-3333-4444-555555555555",
      "isDefault": true,
      "name": "Default USD",
      "priority": 0,
      "scopeType": "company",
      "updatedAt": "2026-01-01T00:00:00Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "totalCount": 1
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/price-books

| POST | `/v1/admin/price-books` | 15_Promotions_PriceBooks | bearer | yes | `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "currency": "USD", "effectiveFrom": "2026-01-01T00:00:00Z", "id": ` |

### POST /v1/admin/price-books

- **Purpose:** Create price book
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "currency": "USD",
  "effectiveFrom": "2026-04-01T00:00:00Z",
  "isDefault": false,
  "name": "canary-name-{{$guid}}",
  "priority": 10,
  "scopeType": "company"
}
```
- **Response 200:** Create price book
```json
{
  "active": true,
  "createdAt": "2026-01-01T00:00:00Z",
  "currency": "USD",
  "effectiveFrom": "2026-01-01T00:00:00Z",
  "id": "11111111-2222-3333-4444-555555555555",
  "isDefault": true,
  "name": "Default USD",
  "priority": 0,
  "scopeType": "company",
  "updatedAt": "2026-01-01T00:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/price-books

| GET | `/v1/admin/price-books/{priceBookId}` | 15_Promotions_PriceBooks | bearer | no | `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "currency": "USD", "effectiveFrom": "2026-01-01T00:00:00Z", "id": ` |

### GET /v1/admin/price-books/{priceBookId}

- **Purpose:** Get price book by ID
- **Auth:** bearer (required=True)
- **Path params:** `priceBookId`
- **Response 200:** Get price book by ID
```json
{
  "active": true,
  "createdAt": "2026-01-01T00:00:00Z",
  "currency": "USD",
  "effectiveFrom": "2026-01-01T00:00:00Z",
  "id": "11111111-2222-3333-4444-555555555555",
  "isDefault": true,
  "name": "Default USD",
  "priority": 0,
  "scopeType": "company",
  "updatedAt": "2026-01-01T00:00:00Z"
}
```
- **Response 404:** error
```json
{
  "error": {
    "code": "not_found",
    "details": {},
    "message": "resource was not found",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/price-books/{priceBookId}

| PATCH | `/v1/admin/price-books/{priceBookId}` | 15_Promotions_PriceBooks | bearer | yes | `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "currency": "USD", "effectiveFrom": "2026-01-01T00:00:00Z", "id": ` |

### PATCH /v1/admin/price-books/{priceBookId}

- **Purpose:** Patch price book
- **Auth:** bearer (required=True)
- **Path params:** `priceBookId`
- **Request body example:**
```json
{
  "priority": 20
}
```
- **Response 200:** Patch price book
```json
{
  "active": true,
  "createdAt": "2026-01-01T00:00:00Z",
  "currency": "USD",
  "effectiveFrom": "2026-01-01T00:00:00Z",
  "id": "11111111-2222-3333-4444-555555555555",
  "isDefault": true,
  "name": "Default USD",
  "priority": 0,
  "scopeType": "company",
  "updatedAt": "2026-01-01T00:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/price-books/{priceBookId}

| POST | `/v1/admin/price-books/{priceBookId}/activate` | 15_Promotions_PriceBooks | bearer | no | `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "currency": "USD", "effectiveFrom": "2026-01-01T00:00:00Z", "id": ` |

### POST /v1/admin/price-books/{priceBookId}/activate

- **Purpose:** Activate price book
- **Auth:** bearer (required=True)
- **Path params:** `priceBookId`
- **Response 200:** Activate price book
```json
{
  "active": true,
  "createdAt": "2026-01-01T00:00:00Z",
  "currency": "USD",
  "effectiveFrom": "2026-01-01T00:00:00Z",
  "id": "11111111-2222-3333-4444-555555555555",
  "isDefault": true,
  "name": "Default USD",
  "priority": 0,
  "scopeType": "company",
  "updatedAt": "2026-01-01T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/price-books/{priceBookId}/activate

| POST | `/v1/admin/price-books/{priceBookId}/archive` | 15_Promotions_PriceBooks | bearer | no | `{"active": false, "createdAt": "2026-01-01T00:00:00Z", "currency": "USD", "effectiveFrom": "2026-01-01T00:00:00Z", "id":` |

### POST /v1/admin/price-books/{priceBookId}/archive

- **Purpose:** Archive price book (deactivate)
- **Auth:** bearer (required=True)
- **Path params:** `priceBookId`
- **Response 200:** Archive price book (deactivate)
```json
{
  "active": false,
  "createdAt": "2026-01-01T00:00:00Z",
  "currency": "USD",
  "effectiveFrom": "2026-01-01T00:00:00Z",
  "id": "11111111-2222-3333-4444-555555555555",
  "isDefault": true,
  "name": "Default USD",
  "priority": 0,
  "scopeType": "company",
  "updatedAt": "2026-01-01T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/price-books/{priceBookId}/archive

| POST | `/v1/admin/price-books/{priceBookId}/assign-target` | 15_Promotions_PriceBooks | bearer | yes | `{"createdAt": "2026-04-24T12:00:00Z", "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "machineId": "7c9e6679-7425-40de-944` |

### POST /v1/admin/price-books/{priceBookId}/assign-target

- **Purpose:** Assign company price book to machine or site
- **Auth:** bearer (required=True)
- **Path params:** `priceBookId`
- **Request body example:**
```json
{
  "machineId": "{{machineId}}"
}
```
- **Response 200:** Assign company price book to machine or site
```json
{
  "createdAt": "2026-04-24T12:00:00Z",
  "id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "priceBookId": "11111111-2222-3333-4444-555555555555",
  "siteId": null
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/price-books/{priceBookId}/assign-target

| POST | `/v1/admin/price-books/{priceBookId}/deactivate` | 15_Promotions_PriceBooks | bearer | no | `{"active": false, "createdAt": "2026-01-01T00:00:00Z", "currency": "USD", "effectiveFrom": "2026-01-01T00:00:00Z", "id":` |

### POST /v1/admin/price-books/{priceBookId}/deactivate

- **Purpose:** Deactivate price book
- **Auth:** bearer (required=True)
- **Path params:** `priceBookId`
- **Response 200:** Deactivate price book
```json
{
  "active": false,
  "createdAt": "2026-01-01T00:00:00Z",
  "currency": "USD",
  "effectiveFrom": "2026-01-01T00:00:00Z",
  "id": "11111111-2222-3333-4444-555555555555",
  "isDefault": true,
  "name": "Default USD",
  "priority": 0,
  "scopeType": "company",
  "updatedAt": "2026-01-01T00:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/price-books/{priceBookId}/deactivate

| GET | `/v1/admin/price-books/{priceBookId}/items` | 15_Promotions_PriceBooks | bearer | no | `{"items": [{"priceBookId": "11111111-2222-3333-4444-555555555555", "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff", ` |

### GET /v1/admin/price-books/{priceBookId}/items

- **Purpose:** List price book items
- **Auth:** bearer (required=True)
- **Path params:** `priceBookId`
- **Response 200:** List price book items
```json
{
  "items": [
    {
      "priceBookId": "11111111-2222-3333-4444-555555555555",
      "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "unitPriceMinor": 150
    }
  ]
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/price-books/{priceBookId}/items

| PUT | `/v1/admin/price-books/{priceBookId}/items` | 15_Promotions_PriceBooks | bearer | yes | `{}` |

### PUT /v1/admin/price-books/{priceBookId}/items

- **Purpose:** Replace price book items
- **Auth:** bearer (required=True)
- **Path params:** `priceBookId`
- **Request body example:**
```json
{
  "items": [
    {
      "productId": "{{productId}}",
      "unitPriceMinor": 150
    }
  ]
}
```
- **Response 204:** Replace price book items
```json
{}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:PUT /v1/admin/price-books/{priceBookId}/items

| DELETE | `/v1/admin/price-books/{priceBookId}/items/{productId}` | 15_Promotions_PriceBooks | bearer | no | `""` |

### DELETE /v1/admin/price-books/{priceBookId}/items/{productId}

- **Purpose:** Delete price book item
- **Auth:** bearer (required=True)
- **Path params:** `priceBookId, productId`
- **Response 204:** Delete price book item
```json
""
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body product_id→productId
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/price-books/{priceBookId}/items/{productId}

| PATCH | `/v1/admin/price-books/{priceBookId}/items/{productId}` | 15_Promotions_PriceBooks | bearer | yes | `{"priceBookId": "11111111-2222-3333-4444-555555555555", "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff", "unitPriceM` |

### PATCH /v1/admin/price-books/{priceBookId}/items/{productId}

- **Purpose:** Upsert one price book item
- **Auth:** bearer (required=True)
- **Path params:** `priceBookId, productId`
- **Request body example:**
```json
{
  "unitPriceMinor": 175
}
```
- **Response 200:** Upsert one price book item
```json
{
  "priceBookId": "11111111-2222-3333-4444-555555555555",
  "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
  "unitPriceMinor": 175
}
```
- **Captures:** response body product_id→productId
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/price-books/{priceBookId}/items/{productId}

| DELETE | `/v1/admin/price-books/{priceBookId}/targets/{targetId}` | 15_Promotions_PriceBooks | bearer | no | `""` |

### DELETE /v1/admin/price-books/{priceBookId}/targets/{targetId}

- **Purpose:** Remove price book target assignment
- **Auth:** bearer (required=True)
- **Path params:** `priceBookId, targetId`
- **Response 204:** Remove price book target assignment
```json
""
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 403:** error
```json
{
  "error": {
    "code": "forbidden",
    "details": {},
    "message": "caller lacks permission for this resource",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/price-books/{priceBookId}/targets/{targetId}

| POST | `/v1/admin/pricing/preview` | 99_Utilities | bearer | yes | `{"at": "2026-04-24T12:00:00.000000000Z", "currency": "USD", "lines": [{"appliedRuleIds": ["price_book:11111111-2222-3333` |

### POST /v1/admin/pricing/preview

- **Purpose:** Preview effective prices for products
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "machineId": "{{machineId}}",
  "productIds": [
    "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff"
  ]
}
```
- **Response 200:** Preview effective prices for products
```json
{
  "at": "2026-04-24T12:00:00.000000000Z",
  "currency": "USD",
  "lines": [
    {
      "appliedRuleIds": [
        "price_book:11111111-2222-3333-4444-555555555555"
      ],
      "basePrice": 150,
      "currency": "USD",
      "effectivePrice": 175,
      "priceBookId": "11111111-2222-3333-4444-555555555555",
      "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "reasons": [
        "tier_3",
        "priority_10"
      ]
    }
  ]
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/pricing/preview

| GET | `/v1/admin/products` | 05_Products | bearer | no | `{"items": [{"active": true, "barcode": "8850123456789", "brandId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "categoryId":` |

### GET /v1/admin/products

- **Purpose:** List products (admin catalog)
- **Auth:** bearer (required=True)
- **Query params:** `q, active_only, limit, offset`
- **Response 200:** List products (admin catalog)
```json
{
  "items": [
    {
      "active": true,
      "barcode": "8850123456789",
      "brandId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "categoryId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
      "createdAt": "2026-01-01T00:00:00Z",
      "description": "Example product",
      "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "media": {
        "primary": {
          "id": "11111111-2222-3333-4444-555555555555",
          "status": "ready",
          "variants": [
            {
              "downloadUrl": "https://cdn.example.com/products/cola-thumb.webp",
              "height": 160,
              "mimeType": "image/webp",
              "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
              "sizeBytes": 8000,
              "variant": "thumb",
              "version": 1,
              "width": 160
            },
            {
              "downloadUrl": "https://cdn.example.com/products/cola-display.webp",
              "height": 512,
              "mimeType": "image/webp",
              "sha256": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
              "sizeBytes": 24000,
              "variant": "display",
              "version": 2,
              "width": 512
            }
          ],
          "version": 1
        }
      },
      "name": "Cola 12oz",
      "primaryMediaId": "11111111-2222-3333-4444-555555555555",
      "sku": "COLA-12",
      "status": "active",
      "tags": [],
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "totalCount": 1
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body product_id→productId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/products

| POST | `/v1/admin/products` | 05_Products | bearer | yes | `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa` |

### POST /v1/admin/products

- **Purpose:** Create product (admin catalog)
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "active": true,
  "ageRestricted": false,
  "allergenCodes": [],
  "barcode": "8850123456789",
  "brandId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "categoryId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "description": "Example product",
  "name": "canary-name-{{$guid}}",
  "primaryMediaId": "11111111-2222-3333-4444-555555555555",
  "sku": "{{sku}}"
}
```
- **Response 200:** Create product (admin catalog)
```json
{
  "active": true,
  "ageRestricted": false,
  "allergenCodes": [],
  "attrs": {},
  "barcode": "8850123456789",
  "brandId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "categoryId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "createdAt": "2026-01-01T00:00:00Z",
  "description": "Example product",
  "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
  "media": {
    "primary": {
      "id": "11111111-2222-3333-4444-555555555555",
      "status": "ready",
      "variants": [
        {
          "downloadUrl": "https://cdn.example.com/products/cola-thumb.webp",
          "height": 160,
          "mimeType": "image/webp",
          "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
          "sizeBytes": 8000,
          "variant": "thumb",
          "version": 1,
          "width": 160
        },
        {
          "downloadUrl": "https://cdn.example.com/products/cola-display.webp",
          "height": 512,
          "mimeType": "image/webp",
          "sha256": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
          "sizeBytes": 24000,
          "variant": "display",
          "version": 2,
          "width": 512
        }
      ],
      "version": 1
    }
  },
  "name": "Cola 12oz",
  "primaryMediaId": "11111111-2222-3333-4444-555555555555",
  "sku": "COLA-12",
  "status": "active",
  "tags": [
    {
      "active": true,
      "createdAt": "2026-01-01T00:00:00Z",
      "id": "cccccccc-dddd-eeee-ffff-000000000000",
      "name": "Cold drink example",
      "slug": "cold-drink-example",
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body product_id→productId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/products

| DELETE | `/v1/admin/products/{productId}` | 05_Products | bearer | no | `{"active": false, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaa` |

### DELETE /v1/admin/products/{productId}

- **Purpose:** Deactivate product
- **Auth:** bearer (required=True)
- **Path params:** `productId`
- **Response 200:** Deactivate product
```json
{
  "active": false,
  "ageRestricted": false,
  "allergenCodes": [],
  "attrs": {},
  "barcode": "8850123456789",
  "brandId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "categoryId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "createdAt": "2026-01-01T00:00:00Z",
  "description": "Example product",
  "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
  "media": {
    "primary": {
      "id": "11111111-2222-3333-4444-555555555555",
      "status": "ready",
      "variants": [
        {
          "downloadUrl": "https://cdn.example.com/products/cola-thumb.webp",
          "height": 160,
          "mimeType": "image/webp",
          "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
          "sizeBytes": 8000,
          "variant": "thumb",
          "version": 1,
          "width": 160
        },
        {
          "downloadUrl": "https://cdn.example.com/products/cola-display.webp",
          "height": 512,
          "mimeType": "image/webp",
          "sha256": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
          "sizeBytes": 24000,
          "variant": "display",
          "version": 2,
          "width": 512
        }
      ],
      "version": 1
    }
  },
  "name": "Cola 12oz",
  "primaryMediaId": "11111111-2222-3333-4444-555555555555",
  "sku": "COLA-12",
  "status": "active",
  "tags": [
    {
      "active": true,
      "createdAt": "2026-01-01T00:00:00Z",
      "id": "cccccccc-dddd-eeee-ffff-000000000000",
      "name": "Cold drink example",
      "slug": "cold-drink-example",
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body product_id→productId
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/products/{productId}

| GET | `/v1/admin/products/{productId}` | 05_Products | bearer | no | `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa` |

### GET /v1/admin/products/{productId}

- **Purpose:** Get product by id (admin catalog)
- **Auth:** bearer (required=True)
- **Path params:** `productId`
- **Response 200:** Get product by id (admin catalog)
```json
{
  "active": true,
  "ageRestricted": false,
  "allergenCodes": [],
  "attrs": {},
  "barcode": "8850123456789",
  "brandId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "categoryId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "createdAt": "2026-01-01T00:00:00Z",
  "description": "Example product",
  "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
  "media": {
    "primary": {
      "id": "11111111-2222-3333-4444-555555555555",
      "status": "ready",
      "variants": [
        {
          "downloadUrl": "https://cdn.example.com/products/cola-thumb.webp",
          "height": 160,
          "mimeType": "image/webp",
          "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
          "sizeBytes": 8000,
          "variant": "thumb",
          "version": 1,
          "width": 160
        },
        {
          "downloadUrl": "https://cdn.example.com/products/cola-display.webp",
          "height": 512,
          "mimeType": "image/webp",
          "sha256": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
          "sizeBytes": 24000,
          "variant": "display",
          "version": 2,
          "width": 512
        }
      ],
      "version": 1
    }
  },
  "name": "Cola 12oz",
  "primaryMediaId": "11111111-2222-3333-4444-555555555555",
  "sku": "COLA-12",
  "status": "active",
  "tags": [
    {
      "active": true,
      "createdAt": "2026-01-01T00:00:00Z",
      "id": "cccccccc-dddd-eeee-ffff-000000000000",
      "name": "Cold drink example",
      "slug": "cold-drink-example",
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body product_id→productId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/products/{productId}

| PATCH | `/v1/admin/products/{productId}` | 05_Products | bearer | yes | `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa` |

### PATCH /v1/admin/products/{productId}

- **Purpose:** Update product (PATCH)
- **Auth:** bearer (required=True)
- **Path params:** `productId`
- **Request body example:**
```json
{
  "active": true,
  "ageRestricted": false,
  "allergenCodes": [],
  "barcode": "8850123456789",
  "brandId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "categoryId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "description": "Example product",
  "name": "canary-name-{{$guid}}",
  "primaryMediaId": "11111111-2222-3333-4444-555555555555",
  "sku": "{{sku}}"
}
```
- **Response 200:** Update product (PATCH)
```json
{
  "active": true,
  "ageRestricted": false,
  "allergenCodes": [],
  "attrs": {},
  "barcode": "8850123456789",
  "brandId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "categoryId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "createdAt": "2026-01-01T00:00:00Z",
  "description": "Example product",
  "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
  "media": {
    "primary": {
      "id": "11111111-2222-3333-4444-555555555555",
      "status": "ready",
      "variants": [
        {
          "downloadUrl": "https://cdn.example.com/products/cola-thumb.webp",
          "height": 160,
          "mimeType": "image/webp",
          "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
          "sizeBytes": 8000,
          "variant": "thumb",
          "version": 1,
          "width": 160
        },
        {
          "downloadUrl": "https://cdn.example.com/products/cola-display.webp",
          "height": 512,
          "mimeType": "image/webp",
          "sha256": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
          "sizeBytes": 24000,
          "variant": "display",
          "version": 2,
          "width": 512
        }
      ],
      "version": 1
    }
  },
  "name": "Cola 12oz",
  "primaryMediaId": "11111111-2222-3333-4444-555555555555",
  "sku": "COLA-12",
  "status": "active",
  "tags": [
    {
      "active": true,
      "createdAt": "2026-01-01T00:00:00Z",
      "id": "cccccccc-dddd-eeee-ffff-000000000000",
      "name": "Cold drink example",
      "slug": "cold-drink-example",
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body product_id→productId
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/products/{productId}

| PUT | `/v1/admin/products/{productId}` | 05_Products | bearer | yes | `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa` |

### PUT /v1/admin/products/{productId}

- **Purpose:** Update product (PUT/PATCH)
- **Auth:** bearer (required=True)
- **Path params:** `productId`
- **Request body example:**
```json
{
  "active": true,
  "ageRestricted": false,
  "allergenCodes": [],
  "barcode": "8850123456789",
  "brandId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "categoryId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "description": "Example product",
  "name": "canary-name-{{$guid}}",
  "primaryMediaId": "11111111-2222-3333-4444-555555555555",
  "sku": "{{sku}}",
  "tagIds": [
    "cccccccc-dddd-eeee-ffff-000000000000"
  ]
}
```
- **Response 200:** Update product (PUT/PATCH)
```json
{
  "active": true,
  "ageRestricted": false,
  "allergenCodes": [],
  "attrs": {},
  "barcode": "8850123456789",
  "brandId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "categoryId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "createdAt": "2026-01-01T00:00:00Z",
  "description": "Example product",
  "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
  "media": {
    "primary": {
      "id": "11111111-2222-3333-4444-555555555555",
      "status": "ready",
      "variants": [
        {
          "downloadUrl": "https://cdn.example.com/products/cola-thumb.webp",
          "height": 160,
          "mimeType": "image/webp",
          "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
          "sizeBytes": 8000,
          "variant": "thumb",
          "version": 1,
          "width": 160
        },
        {
          "downloadUrl": "https://cdn.example.com/products/cola-display.webp",
          "height": 512,
          "mimeType": "image/webp",
          "sha256": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
          "sizeBytes": 24000,
          "variant": "display",
          "version": 2,
          "width": 512
        }
      ],
      "version": 1
    }
  },
  "name": "Cola 12oz",
  "primaryMediaId": "11111111-2222-3333-4444-555555555555",
  "sku": "COLA-12",
  "status": "active",
  "tags": [
    {
      "active": true,
      "createdAt": "2026-01-01T00:00:00Z",
      "id": "cccccccc-dddd-eeee-ffff-000000000000",
      "name": "Cold drink example",
      "slug": "cold-drink-example",
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body product_id→productId
- **Source:** docs/swagger/swagger.json, openapi:PUT /v1/admin/products/{productId}

| DELETE | `/v1/admin/products/{productId}/image` | 05_Products | bearer | no | `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa` |

### DELETE /v1/admin/products/{productId}/image

- **Purpose:** Remove primary product image
- **Auth:** bearer (required=True)
- **Path params:** `productId`
- **Response 200:** Remove primary product image
```json
{
  "active": true,
  "ageRestricted": false,
  "allergenCodes": [],
  "attrs": {},
  "barcode": "8850123456789",
  "brandId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "categoryId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "createdAt": "2026-01-01T00:00:00Z",
  "description": "Example product",
  "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
  "media": {
    "primary": {
      "id": "11111111-2222-3333-4444-555555555555",
      "status": "ready",
      "variants": [
        {
          "downloadUrl": "https://cdn.example.com/products/cola-thumb.webp",
          "height": 160,
          "mimeType": "image/webp",
          "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
          "sizeBytes": 8000,
          "variant": "thumb",
          "version": 1,
          "width": 160
        },
        {
          "downloadUrl": "https://cdn.example.com/products/cola-display.webp",
          "height": 512,
          "mimeType": "image/webp",
          "sha256": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
          "sizeBytes": 24000,
          "variant": "display",
          "version": 2,
          "width": 512
        }
      ],
      "version": 1
    }
  },
  "name": "Cola 12oz",
  "primaryMediaId": "11111111-2222-3333-4444-555555555555",
  "sku": "COLA-12",
  "status": "active",
  "tags": [
    {
      "active": true,
      "createdAt": "2026-01-01T00:00:00Z",
      "id": "cccccccc-dddd-eeee-ffff-000000000000",
      "name": "Cold drink example",
      "slug": "cold-drink-example",
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body product_id→productId
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/products/{productId}/image

| POST | `/v1/admin/products/{productId}/image` | 05_Products | bearer | yes | `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa` |

### POST /v1/admin/products/{productId}/image

- **Purpose:** Bind primary product image (alias)
- **Auth:** bearer (required=True)
- **Path params:** `productId`
- **Request body example:**
```json
{
  "artifactId": "11111111-2222-3333-4444-555555555555",
  "contentHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "displayUrl": "https://cdn.example.com/products/coca330-display.webp",
  "height": 800,
  "mimeType": "image/webp",
  "thumbUrl": "https://cdn.example.com/products/coca330-thumb.webp",
  "width": 800
}
```
- **Response 200:** Bind primary product image (alias)
```json
{
  "active": true,
  "ageRestricted": false,
  "allergenCodes": [],
  "attrs": {},
  "barcode": "8850123456789",
  "brandId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "categoryId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "createdAt": "2026-01-01T00:00:00Z",
  "description": "Example product",
  "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
  "media": {
    "primary": {
      "id": "11111111-2222-3333-4444-555555555555",
      "status": "ready",
      "variants": [
        {
          "downloadUrl": "https://cdn.example.com/products/cola-thumb.webp",
          "height": 160,
          "mimeType": "image/webp",
          "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
          "sizeBytes": 8000,
          "variant": "thumb",
          "version": 1,
          "width": 160
        },
        {
          "downloadUrl": "https://cdn.example.com/products/cola-display.webp",
          "height": 512,
          "mimeType": "image/webp",
          "sha256": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
          "sizeBytes": 24000,
          "variant": "display",
          "version": 2,
          "width": 512
        }
      ],
      "version": 1
    }
  },
  "name": "Cola 12oz",
  "primaryMediaId": "11111111-2222-3333-4444-555555555555",
  "sku": "COLA-12",
  "status": "active",
  "tags": [
    {
      "active": true,
      "createdAt": "2026-01-01T00:00:00Z",
      "id": "cccccccc-dddd-eeee-ffff-000000000000",
      "name": "Cold drink example",
      "slug": "cold-drink-example",
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body product_id→productId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/products/{productId}/image

| PUT | `/v1/admin/products/{productId}/image` | 05_Products | bearer | yes | `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa` |

### PUT /v1/admin/products/{productId}/image

- **Purpose:** Bind primary product image
- **Auth:** bearer (required=True)
- **Path params:** `productId`
- **Request body example:**
```json
{
  "artifactId": "11111111-2222-3333-4444-555555555555",
  "contentHash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
  "displayUrl": "https://cdn.example.com/products/coca330-display.webp",
  "height": 800,
  "mimeType": "image/webp",
  "thumbUrl": "https://cdn.example.com/products/coca330-thumb.webp",
  "width": 800
}
```
- **Response 200:** Bind primary product image
```json
{
  "active": true,
  "ageRestricted": false,
  "allergenCodes": [],
  "attrs": {},
  "barcode": "8850123456789",
  "brandId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "categoryId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "createdAt": "2026-01-01T00:00:00Z",
  "description": "Example product",
  "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
  "media": {
    "primary": {
      "id": "11111111-2222-3333-4444-555555555555",
      "status": "ready",
      "variants": [
        {
          "downloadUrl": "https://cdn.example.com/products/cola-thumb.webp",
          "height": 160,
          "mimeType": "image/webp",
          "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
          "sizeBytes": 8000,
          "variant": "thumb",
          "version": 1,
          "width": 160
        },
        {
          "downloadUrl": "https://cdn.example.com/products/cola-display.webp",
          "height": 512,
          "mimeType": "image/webp",
          "sha256": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
          "sizeBytes": 24000,
          "variant": "display",
          "version": 2,
          "width": 512
        }
      ],
      "version": 1
    }
  },
  "name": "Cola 12oz",
  "primaryMediaId": "11111111-2222-3333-4444-555555555555",
  "sku": "COLA-12",
  "status": "active",
  "tags": [
    {
      "active": true,
      "createdAt": "2026-01-01T00:00:00Z",
      "id": "cccccccc-dddd-eeee-ffff-000000000000",
      "name": "Cold drink example",
      "slug": "cold-drink-example",
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body product_id→productId
- **Source:** docs/swagger/swagger.json, openapi:PUT /v1/admin/products/{productId}/image

| POST | `/v1/admin/products/{productId}/media` | 04_Product_Media_Offline_Cache | bearer | yes | `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa` |

### POST /v1/admin/products/{productId}/media

- **Purpose:** Bind media to product (POST)
- **Auth:** bearer (required=True)
- **Path params:** `productId`
- **Request body example:**
```json
{
  "media_id": "11111111-2222-3333-4444-555555555555"
}
```
- **Response 200:** Bind media to product (POST)
```json
{
  "active": true,
  "ageRestricted": false,
  "allergenCodes": [],
  "attrs": {},
  "barcode": "8850123456789",
  "brandId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "categoryId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "createdAt": "2026-01-01T00:00:00Z",
  "description": "Example product",
  "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
  "media": {
    "primary": {
      "id": "11111111-2222-3333-4444-555555555555",
      "status": "ready",
      "variants": [
        {
          "downloadUrl": "https://cdn.example.com/products/cola-thumb.webp",
          "height": 160,
          "mimeType": "image/webp",
          "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
          "sizeBytes": 8000,
          "variant": "thumb",
          "version": 1,
          "width": 160
        },
        {
          "downloadUrl": "https://cdn.example.com/products/cola-display.webp",
          "height": 512,
          "mimeType": "image/webp",
          "sha256": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
          "sizeBytes": 24000,
          "variant": "display",
          "version": 2,
          "width": 512
        }
      ],
      "version": 1
    }
  },
  "name": "Cola 12oz",
  "primaryMediaId": "11111111-2222-3333-4444-555555555555",
  "sku": "COLA-12",
  "status": "active",
  "tags": [
    {
      "active": true,
      "createdAt": "2026-01-01T00:00:00Z",
      "id": "cccccccc-dddd-eeee-ffff-000000000000",
      "name": "Cold drink example",
      "slug": "cold-drink-example",
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Captures:** response body product_id→productId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/products/{productId}/media

| PUT | `/v1/admin/products/{productId}/media` | 04_Product_Media_Offline_Cache | bearer | yes | `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa` |

### PUT /v1/admin/products/{productId}/media

- **Purpose:** Bind or replace product media (PUT)
- **Auth:** bearer (required=True)
- **Path params:** `productId`
- **Request body example:**
```json
{
  "media_id": "11111111-2222-3333-4444-555555555555"
}
```
- **Response 200:** Bind or replace product media (PUT)
```json
{
  "active": true,
  "ageRestricted": false,
  "allergenCodes": [],
  "attrs": {},
  "barcode": "8850123456789",
  "brandId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "categoryId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "createdAt": "2026-01-01T00:00:00Z",
  "description": "Example product",
  "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
  "media": {
    "primary": {
      "id": "11111111-2222-3333-4444-555555555555",
      "status": "ready",
      "variants": [
        {
          "downloadUrl": "https://cdn.example.com/products/cola-thumb.webp",
          "height": 160,
          "mimeType": "image/webp",
          "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
          "sizeBytes": 8000,
          "variant": "thumb",
          "version": 1,
          "width": 160
        },
        {
          "downloadUrl": "https://cdn.example.com/products/cola-display.webp",
          "height": 512,
          "mimeType": "image/webp",
          "sha256": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
          "sizeBytes": 24000,
          "variant": "display",
          "version": 2,
          "width": 512
        }
      ],
      "version": 1
    }
  },
  "name": "Cola 12oz",
  "primaryMediaId": "11111111-2222-3333-4444-555555555555",
  "sku": "COLA-12",
  "status": "active",
  "tags": [
    {
      "active": true,
      "createdAt": "2026-01-01T00:00:00Z",
      "id": "cccccccc-dddd-eeee-ffff-000000000000",
      "name": "Cold drink example",
      "slug": "cold-drink-example",
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Captures:** response body product_id→productId
- **Source:** docs/swagger/swagger.json, openapi:PUT /v1/admin/products/{productId}/media

| DELETE | `/v1/admin/products/{productId}/media/{mediaId}` | 04_Product_Media_Offline_Cache | bearer | no | `{"active": true, "ageRestricted": false, "allergenCodes": [], "attrs": {}, "barcode": "8850123456789", "brandId": "aaaaa` |

### DELETE /v1/admin/products/{productId}/media/{mediaId}

- **Purpose:** Remove bound media from product
- **Auth:** bearer (required=True)
- **Path params:** `productId, mediaId`
- **Response 200:** Remove bound media from product
```json
{
  "active": true,
  "ageRestricted": false,
  "allergenCodes": [],
  "attrs": {},
  "barcode": "8850123456789",
  "brandId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "categoryId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "createdAt": "2026-01-01T00:00:00Z",
  "description": "Example product",
  "id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
  "media": {
    "primary": {
      "id": "11111111-2222-3333-4444-555555555555",
      "status": "ready",
      "variants": [
        {
          "downloadUrl": "https://cdn.example.com/products/cola-thumb.webp",
          "height": 160,
          "mimeType": "image/webp",
          "sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
          "sizeBytes": 8000,
          "variant": "thumb",
          "version": 1,
          "width": 160
        },
        {
          "downloadUrl": "https://cdn.example.com/products/cola-display.webp",
          "height": 512,
          "mimeType": "image/webp",
          "sha256": "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
          "sizeBytes": 24000,
          "variant": "display",
          "version": 2,
          "width": 512
        }
      ],
      "version": 1
    }
  },
  "name": "Cola 12oz",
  "primaryMediaId": "11111111-2222-3333-4444-555555555555",
  "sku": "COLA-12",
  "status": "active",
  "tags": [
    {
      "active": true,
      "createdAt": "2026-01-01T00:00:00Z",
      "id": "cccccccc-dddd-eeee-ffff-000000000000",
      "name": "Cold drink example",
      "slug": "cold-drink-example",
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Captures:** response body product_id→productId
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/products/{productId}/media/{mediaId}

| GET | `/v1/admin/promotions` | 15_Promotions_PriceBooks | bearer | no | `{"items": [{"approvalStatus": "approved", "createdAt": "2026-04-01T00:00:00Z", "endsAt": "2026-09-01T00:00:00Z", "id": "` |

### GET /v1/admin/promotions

- **Purpose:** List promotions (admin catalog)
- **Auth:** bearer (required=True)
- **Query params:** `limit, offset, include_deactivated`
- **Response 200:** List promotions (admin catalog)
```json
{
  "items": [
    {
      "approvalStatus": "approved",
      "createdAt": "2026-04-01T00:00:00Z",
      "endsAt": "2026-09-01T00:00:00Z",
      "id": "10101010-1010-1010-1010-101010101010",
      "lifecycleStatus": "draft",
      "name": "Summer 10%",
      "priority": 10,
      "stackable": false,
      "startsAt": "2026-06-01T00:00:00Z",
      "updatedAt": "2026-04-01T00:00:00Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "totalCount": 1
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/promotions

| POST | `/v1/admin/promotions` | 15_Promotions_PriceBooks | bearer | yes | `{"approvalStatus": "approved", "createdAt": "2026-04-01T00:00:00Z", "endsAt": "2026-09-01T00:00:00Z", "id": "10101010-10` |

### POST /v1/admin/promotions

- **Purpose:** Create promotion (draft lifecycle)
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "endsAt": "2026-09-01T00:00:00Z",
  "name": "canary-name-{{$guid}}",
  "priority": 10,
  "rules": [
    {
      "payload": {
        "percent": 10
      },
      "priority": 0,
      "ruleType": "percentage_discount"
    }
  ],
  "stackable": false,
  "startsAt": "2026-06-01T00:00:00Z"
}
```
- **Response 200:** Create promotion (draft lifecycle)
```json
{
  "approvalStatus": "approved",
  "createdAt": "2026-04-01T00:00:00Z",
  "endsAt": "2026-09-01T00:00:00Z",
  "id": "10101010-1010-1010-1010-101010101010",
  "lifecycleStatus": "draft",
  "name": "Summer 10%",
  "priority": 10,
  "stackable": false,
  "startsAt": "2026-06-01T00:00:00Z",
  "updatedAt": "2026-04-01T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/promotions

| POST | `/v1/admin/promotions/preview` | 15_Promotions_PriceBooks | bearer | yes | `{"at": "2026-04-24T12:00:00.000000000Z", "lines": [{"appliedPromotionIds": ["10101010-1010-1010-1010-101010101010"], "ap` |

### POST /v1/admin/promotions/preview

- **Purpose:** Preview promotion discounts on top of catalog pricing
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "machineId": "{{machineId}}",
  "productIds": [
    "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff"
  ]
}
```
- **Response 200:** Preview promotion discounts on top of catalog pricing
```json
{
  "at": "2026-04-24T12:00:00.000000000Z",
  "lines": [
    {
      "appliedPromotionIds": [
        "10101010-1010-1010-1010-101010101010"
      ],
      "appliedRuleIds": [
        "promotion_rule:20202020-2020-2020-2020-202020202020"
      ],
      "basePriceMinor": 150,
      "currency": "USD",
      "discountMinor": 15,
      "finalPriceMinor": 135,
      "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "skippedRules": []
    }
  ]
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/promotions/preview

| GET | `/v1/admin/promotions/{promotionId}` | 15_Promotions_PriceBooks | bearer | no | `{"promotion": {"approvalStatus": "approved", "createdAt": "2026-04-01T00:00:00Z", "endsAt": "2026-09-01T00:00:00Z", "id"` |

### GET /v1/admin/promotions/{promotionId}

- **Purpose:** Get promotion detail with rules and targets
- **Auth:** bearer (required=True)
- **Path params:** `promotionId`
- **Response 200:** Get promotion detail with rules and targets
```json
{
  "promotion": {
    "approvalStatus": "approved",
    "createdAt": "2026-04-01T00:00:00Z",
    "endsAt": "2026-09-01T00:00:00Z",
    "id": "10101010-1010-1010-1010-101010101010",
    "lifecycleStatus": "draft",
    "name": "Summer 10%",
    "priority": 10,
    "stackable": false,
    "startsAt": "2026-06-01T00:00:00Z",
    "updatedAt": "2026-04-01T00:00:00Z"
  },
  "rules": [
    {
      "id": "20202020-2020-2020-2020-202020202020",
      "payload": {
        "percent": 10
      },
      "priority": 0,
      "promotionId": "10101010-1010-1010-1010-101010101010",
      "ruleType": "percentage_discount"
    }
  ],
  "targets": []
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 404:** error
```json
{
  "error": {
    "code": "not_found",
    "details": {},
    "message": "resource was not found",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/promotions/{promotionId}

| PATCH | `/v1/admin/promotions/{promotionId}` | 15_Promotions_PriceBooks | bearer | yes | `{"approvalStatus": "approved", "createdAt": "2026-04-01T00:00:00Z", "endsAt": "2026-09-01T00:00:00Z", "id": "10101010-10` |

### PATCH /v1/admin/promotions/{promotionId}

- **Purpose:** Patch promotion fields or replace rules
- **Auth:** bearer (required=True)
- **Path params:** `promotionId`
- **Request body example:**
```json
{
  "name": "canary-name-{{$guid}}",
  "priority": 11
}
```
- **Response 200:** Patch promotion fields or replace rules
```json
{
  "approvalStatus": "approved",
  "createdAt": "2026-04-01T00:00:00Z",
  "endsAt": "2026-09-01T00:00:00Z",
  "id": "10101010-1010-1010-1010-101010101010",
  "lifecycleStatus": "draft",
  "name": "Summer 12%",
  "priority": 11,
  "stackable": false,
  "startsAt": "2026-06-01T00:00:00Z",
  "updatedAt": "2026-04-01T00:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/promotions/{promotionId}

| POST | `/v1/admin/promotions/{promotionId}/activate` | 15_Promotions_PriceBooks | bearer | no | `{"approvalStatus": "approved", "createdAt": "2026-04-01T00:00:00Z", "endsAt": "2026-09-01T00:00:00Z", "id": "10101010-10` |

### POST /v1/admin/promotions/{promotionId}/activate

- **Purpose:** Activate promotion (lifecycle active)
- **Auth:** bearer (required=True)
- **Path params:** `promotionId`
- **Response 200:** Activate promotion (lifecycle active)
```json
{
  "approvalStatus": "approved",
  "createdAt": "2026-04-01T00:00:00Z",
  "endsAt": "2026-09-01T00:00:00Z",
  "id": "10101010-1010-1010-1010-101010101010",
  "lifecycleStatus": "active",
  "name": "Summer 10%",
  "priority": 10,
  "stackable": false,
  "startsAt": "2026-06-01T00:00:00Z",
  "updatedAt": "2026-04-01T00:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/promotions/{promotionId}/activate

| POST | `/v1/admin/promotions/{promotionId}/archive` | 15_Promotions_PriceBooks | bearer | no | `{"approvalStatus": "approved", "createdAt": "2026-04-01T00:00:00Z", "endsAt": "2026-09-01T00:00:00Z", "id": "10101010-10` |

### POST /v1/admin/promotions/{promotionId}/archive

- **Purpose:** Archive promotion (deactivate with audit trail)
- **Auth:** bearer (required=True)
- **Path params:** `promotionId`
- **Response 200:** Archive promotion (deactivate with audit trail)
```json
{
  "approvalStatus": "approved",
  "createdAt": "2026-04-01T00:00:00Z",
  "endsAt": "2026-09-01T00:00:00Z",
  "id": "10101010-1010-1010-1010-101010101010",
  "lifecycleStatus": "deactivated",
  "name": "Summer 10%",
  "priority": 10,
  "stackable": false,
  "startsAt": "2026-06-01T00:00:00Z",
  "updatedAt": "2026-04-01T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 404:** error
```json
{
  "error": {
    "code": "not_found",
    "details": {},
    "message": "resource was not found",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/promotions/{promotionId}/archive

| POST | `/v1/admin/promotions/{promotionId}/assign-target` | 15_Promotions_PriceBooks | bearer | yes | `{"createdAt": "2026-04-01T00:00:00Z", "id": "30303030-3030-3030-3030-303030303030", "productId": "9f1e2d3c-aaaa-bbbb-ccc` |

### POST /v1/admin/promotions/{promotionId}/assign-target

- **Purpose:** Assign a promotion target (company, site, machine, product, category, tag)
- **Auth:** bearer (required=True)
- **Path params:** `promotionId`
- **Request body example:**
```json
{
  "productId": "{{productId}}",
  "targetType": "product"
}
```
- **Response 200:** Assign a promotion target (company, site, machine, product, category, tag)
```json
{
  "createdAt": "2026-04-01T00:00:00Z",
  "id": "30303030-3030-3030-3030-303030303030",
  "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
  "promotionId": "10101010-1010-1010-1010-101010101010",
  "targetType": "product"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/promotions/{promotionId}/assign-target

| POST | `/v1/admin/promotions/{promotionId}/deactivate` | 15_Promotions_PriceBooks | bearer | no | `{"approvalStatus": "approved", "createdAt": "2026-04-01T00:00:00Z", "endsAt": "2026-09-01T00:00:00Z", "id": "10101010-10` |

### POST /v1/admin/promotions/{promotionId}/deactivate

- **Purpose:** Deactivate promotion
- **Auth:** bearer (required=True)
- **Path params:** `promotionId`
- **Response 200:** Deactivate promotion
```json
{
  "approvalStatus": "approved",
  "createdAt": "2026-04-01T00:00:00Z",
  "endsAt": "2026-09-01T00:00:00Z",
  "id": "10101010-1010-1010-1010-101010101010",
  "lifecycleStatus": "deactivated",
  "name": "Summer 10%",
  "priority": 10,
  "stackable": false,
  "startsAt": "2026-06-01T00:00:00Z",
  "updatedAt": "2026-04-01T00:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/promotions/{promotionId}/deactivate

| POST | `/v1/admin/promotions/{promotionId}/pause` | 15_Promotions_PriceBooks | bearer | no | `{"approvalStatus": "approved", "createdAt": "2026-04-01T00:00:00Z", "endsAt": "2026-09-01T00:00:00Z", "id": "10101010-10` |

### POST /v1/admin/promotions/{promotionId}/pause

- **Purpose:** Pause promotion
- **Auth:** bearer (required=True)
- **Path params:** `promotionId`
- **Response 200:** Pause promotion
```json
{
  "approvalStatus": "approved",
  "createdAt": "2026-04-01T00:00:00Z",
  "endsAt": "2026-09-01T00:00:00Z",
  "id": "10101010-1010-1010-1010-101010101010",
  "lifecycleStatus": "paused",
  "name": "Summer 10%",
  "priority": 10,
  "stackable": false,
  "startsAt": "2026-06-01T00:00:00Z",
  "updatedAt": "2026-04-01T00:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/promotions/{promotionId}/pause

| DELETE | `/v1/admin/promotions/{promotionId}/targets/{targetId}` | 15_Promotions_PriceBooks | bearer | no | `""` |

### DELETE /v1/admin/promotions/{promotionId}/targets/{targetId}

- **Purpose:** Remove a promotion target assignment
- **Auth:** bearer (required=True)
- **Path params:** `promotionId, targetId`
- **Response 204:** Remove a promotion target assignment
```json
""
```
- **Response 404:** error
```json
{
  "error": {
    "code": "not_found",
    "details": {},
    "message": "resource was not found",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/promotions/{promotionId}/targets/{targetId}

| GET | `/v1/admin/provisioning/batches/{batchId}` | 08_Machines_Runtime_Config | bearer | no | `{"batch": {"cabinetType": "ambient", "createdAt": "2026-04-29T12:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc` |

### GET /v1/admin/provisioning/batches/{batchId}

- **Purpose:** Get provisioning batch status
- **Auth:** bearer (required=True)
- **Path params:** `batchId`
- **Response 200:** Get provisioning batch status
```json
{
  "batch": {
    "cabinetType": "ambient",
    "createdAt": "2026-04-29T12:00:00.000000000Z",
    "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "machineCount": 1,
    "metadata": {},
    "siteId": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "status": "completed",
    "updatedAt": "2026-04-29T12:00:00.000000000Z"
  },
  "machines": []
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/provisioning/batches/{batchId}

| POST | `/v1/admin/provisioning/machines/bulk` | 08_Machines_Runtime_Config | bearer | yes | `{"status": "ok"}` |

### POST /v1/admin/provisioning/machines/bulk

- **Purpose:** Bulk provision machines at a site
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "cabinetType": "ambient",
  "generateActivationCodes": false,
  "machines": [
    {
      "model": "AVF-1",
      "name": "canary-name-{{$guid}}",
      "serialNumber": "SN-BULK-001"
    }
  ],
  "siteId": "{{siteId}}"
}
```
- **Response 200:** Bulk provision machines at a site
```json
{
  "status": "ok"
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/provisioning/machines/bulk

| GET | `/v1/admin/refunds` | 14_Refunds_Disputes | bearer | no | `{"items": [], "meta": {"limit": 50, "offset": 0, "returned": 1, "total": 42}}` |

### GET /v1/admin/refunds

- **Purpose:** List refund requests for company
- **Auth:** bearer (required=True)
- **Query params:** `status, limit, offset`
- **Response 200:** List refund requests for company
```json
{
  "items": [],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/refunds

| GET | `/v1/admin/refunds/{refundId}` | 14_Refunds_Disputes | bearer | no | `{}` |

### GET /v1/admin/refunds/{refundId}

- **Purpose:** Get refund request by id
- **Auth:** bearer (required=True)
- **Path params:** `refundId`
- **Response 200:** Get refund request by id
```json
{}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/refunds/{refundId}

| GET | `/v1/admin/reports/cash` | 13_Payments | bearer | no | `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"` |

### GET /v1/admin/reports/cash

- **Purpose:** Cash collection report
- **Auth:** bearer (required=True)
- **Query params:** `from, to, site_id, machine_id, limit, offset, format`
- **Response 200:** Cash collection report
```json
{
  "from": "2026-04-01T00:00:00Z",
  "items": [
    {
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "status": "open"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  },
  "to": "2026-04-20T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/cash

| GET | `/v1/admin/reports/cash-collections/export.csv` | 13_Payments | bearer | no | `{}` |

### GET /v1/admin/reports/cash-collections/export.csv

- **Purpose:** Export cash collection sessions as CSV (UTF-8)
- **Auth:** bearer (required=True)
- **Query params:** `from, to, site_id, machine_id`
- **Response 200:** Export cash collection sessions as CSV (UTF-8)
```json
{}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/cash-collections/export.csv

| GET | `/v1/admin/reports/commands` | 16_Finance_Reconciliation | bearer | no | `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"` |

### GET /v1/admin/reports/commands

- **Purpose:** Machine command failure report (terminal attempts only)
- **Auth:** bearer (required=True)
- **Query params:** `from, to, site_id, machine_id, limit, offset, format`
- **Response 200:** Machine command failure report (terminal attempts only)
```json
{
  "from": "2026-04-01T00:00:00Z",
  "items": [
    {
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "status": "open"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  },
  "to": "2026-04-20T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body command_id→commandId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/commands

| GET | `/v1/admin/reports/export` | 16_Finance_Reconciliation | bearer | no | `{}` |

### GET /v1/admin/reports/export

- **Purpose:** Unified CSV export dispatcher
- **Auth:** bearer (required=True)
- **Query params:** `from, to, report`
- **Response 200:** Unified CSV export dispatcher
```json
{}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/export

| GET | `/v1/admin/reports/failed-vends` | 16_Finance_Reconciliation | bearer | no | `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"` |

### GET /v1/admin/reports/failed-vends

- **Purpose:** Failed vend report
- **Auth:** bearer (required=True)
- **Query params:** `from, to, limit, offset`
- **Response 200:** Failed vend report
```json
{
  "from": "2026-04-01T00:00:00Z",
  "items": [
    {
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "status": "open"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  },
  "to": "2026-04-20T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/failed-vends

| GET | `/v1/admin/reports/fills` | 10_Inventory | bearer | no | `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"` |

### GET /v1/admin/reports/fills

- **Purpose:** Technician and fill / restock inventory operations
- **Auth:** bearer (required=True)
- **Query params:** `from, to, site_id, machine_id, product_id, limit, offset, format`
- **Response 200:** Technician and fill / restock inventory operations
```json
{
  "from": "2026-04-01T00:00:00Z",
  "items": [
    {
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "status": "open"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  },
  "to": "2026-04-20T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/fills

| GET | `/v1/admin/reports/inventory` | 10_Inventory | bearer | no | `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"` |

### GET /v1/admin/reports/inventory

- **Purpose:** Inventory BI (low stock or movement ledger)
- **Auth:** bearer (required=True)
- **Query params:** `from, to, kind, exception_kind, site_id, machine_id, product_id, limit, offset, format`
- **Response 200:** Inventory BI (low stock or movement ledger)
```json
{
  "from": "2026-04-01T00:00:00Z",
  "items": [
    {
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "status": "open"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  },
  "to": "2026-04-20T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/inventory

| GET | `/v1/admin/reports/inventory-low-stock` | 10_Inventory | bearer | no | `{"exceptionKind": "low_stock", "from": "2026-04-01T00:00:00.000000000Z", "items": [], "meta": {"limit": 50, "offset": 0,` |

### GET /v1/admin/reports/inventory-low-stock

- **Purpose:** Inventory low-stock report
- **Auth:** bearer (required=True)
- **Query params:** `from, to, limit, offset`
- **Response 200:** Inventory low-stock report
```json
{
  "exceptionKind": "low_stock",
  "from": "2026-04-01T00:00:00.000000000Z",
  "items": [],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 0,
    "total": 0
  },
  "to": "2026-04-20T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/inventory-low-stock

| GET | `/v1/admin/reports/machine-health` | 16_Finance_Reconciliation | bearer | no | `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"` |

### GET /v1/admin/reports/machine-health

- **Purpose:** Machine health and offline report
- **Auth:** bearer (required=True)
- **Query params:** `from, to, limit, offset`
- **Response 200:** Machine health and offline report
```json
{
  "from": "2026-04-01T00:00:00Z",
  "items": [
    {
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "status": "open"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  },
  "to": "2026-04-20T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/machine-health

| GET | `/v1/admin/reports/machines` | 16_Finance_Reconciliation | bearer | no | `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"` |

### GET /v1/admin/reports/machines

- **Purpose:** Machine uptime / last-seen report (alias naming)
- **Auth:** bearer (required=True)
- **Query params:** `from, to, site_id, machine_id, product_id, limit, offset, format`
- **Response 200:** Machine uptime / last-seen report (alias naming)
```json
{
  "from": "2026-04-01T00:00:00Z",
  "items": [
    {
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "status": "open"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  },
  "to": "2026-04-20T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/machines

| GET | `/v1/admin/reports/payments` | 13_Payments | bearer | no | `{"from": "2026-04-01T00:00:00Z", "items": [{"amountMinor": 12500, "bucketStart": "2026-04-01T00:00:00Z", "paymentCount":` |

### GET /v1/admin/reports/payments

- **Purpose:** Payment settlement report
- **Auth:** bearer (required=True)
- **Query params:** `from, to, timezone, site_id, machine_id, product_id, format`
- **Response 200:** Payment settlement report
```json
{
  "from": "2026-04-01T00:00:00Z",
  "items": [
    {
      "amountMinor": 12500,
      "bucketStart": "2026-04-01T00:00:00Z",
      "paymentCount": 5,
      "provider": "cash",
      "reconciliationStatus": "matched",
      "settlementStatus": "settled",
      "state": "captured"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  },
  "timezone": "UTC",
  "to": "2026-04-20T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/payments

| GET | `/v1/admin/reports/payments-summary/export.csv` | 13_Payments | bearer | no | `{}` |

### GET /v1/admin/reports/payments-summary/export.csv

- **Purpose:** Export payments summary as CSV (UTF-8)
- **Auth:** bearer (required=True)
- **Query params:** `from, to, group_by`
- **Response 200:** Export payments summary as CSV (UTF-8)
```json
{}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/payments-summary/export.csv

| GET | `/v1/admin/reports/products` | 16_Finance_Reconciliation | bearer | no | `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"` |

### GET /v1/admin/reports/products

- **Purpose:** Product performance report
- **Auth:** bearer (required=True)
- **Query params:** `from, to, site_id, machine_id, product_id, format`
- **Response 200:** Product performance report
```json
{
  "from": "2026-04-01T00:00:00Z",
  "items": [
    {
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "status": "open"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  },
  "to": "2026-04-20T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body product_id→productId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/products

| GET | `/v1/admin/reports/reconciliation` | 16_Finance_Reconciliation | bearer | no | `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"` |

### GET /v1/admin/reports/reconciliation

- **Purpose:** Reconciliation BI (open/closed summaries + scoped cases)
- **Auth:** bearer (required=True)
- **Query params:** `from, to, reconciliation_scope, site_id, machine_id, product_id, limit, offset, format`
- **Response 200:** Reconciliation BI (open/closed summaries + scoped cases)
```json
{
  "from": "2026-04-01T00:00:00Z",
  "items": [
    {
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "status": "open"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  },
  "to": "2026-04-20T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/reconciliation

| GET | `/v1/admin/reports/reconciliation-queue` | 16_Finance_Reconciliation | bearer | no | `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"` |

### GET /v1/admin/reports/reconciliation-queue

- **Purpose:** Reconciliation queue report
- **Auth:** bearer (required=True)
- **Query params:** `from, to, limit, offset`
- **Response 200:** Reconciliation queue report
```json
{
  "from": "2026-04-01T00:00:00Z",
  "items": [
    {
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "status": "open"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  },
  "to": "2026-04-20T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/reconciliation-queue

| GET | `/v1/admin/reports/refunds` | 14_Refunds_Disputes | bearer | no | `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"` |

### GET /v1/admin/reports/refunds

- **Purpose:** Refund report
- **Auth:** bearer (required=True)
- **Query params:** `from, to, limit, offset, format`
- **Response 200:** Refund report
```json
{
  "from": "2026-04-01T00:00:00Z",
  "items": [
    {
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "status": "open"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  },
  "to": "2026-04-20T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/refunds

| GET | `/v1/admin/reports/sales` | 16_Finance_Reconciliation | bearer | no | `{"breakdown": [], "from": "2026-04-01T00:00:00Z", "groupBy": "day", "summary": {"avgOrderValueMinor": 200, "grossTotalMi` |

### GET /v1/admin/reports/sales

- **Purpose:** Company sales report
- **Auth:** bearer (required=True)
- **Query params:** `from, to, timezone, site_id, machine_id, product_id, group_by, format`
- **Response 200:** Company sales report
```json
{
  "breakdown": [],
  "from": "2026-04-01T00:00:00Z",
  "groupBy": "day",
  "summary": {
    "avgOrderValueMinor": 200,
    "grossTotalMinor": 10000,
    "orderCount": 50,
    "subtotalMinor": 9000,
    "taxMinor": 1000
  },
  "to": "2026-04-20T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/sales

| GET | `/v1/admin/reports/sales-summary/export.csv` | 16_Finance_Reconciliation | bearer | no | `{}` |

### GET /v1/admin/reports/sales-summary/export.csv

- **Purpose:** Export sales summary as CSV (UTF-8)
- **Auth:** bearer (required=True)
- **Query params:** `from, to, group_by`
- **Response 200:** Export sales summary as CSV (UTF-8)
```json
{}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/sales-summary/export.csv

| GET | `/v1/admin/reports/vends` | 16_Finance_Reconciliation | bearer | no | `{"from": "2026-04-01T00:00:00Z", "items": [{"id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "status": "open"}], "meta": {"` |

### GET /v1/admin/reports/vends

- **Purpose:** Company vend lifecycle summary
- **Auth:** bearer (required=True)
- **Query params:** `from, to, site_id, machine_id, product_id, limit, offset, format`
- **Response 200:** Company vend lifecycle summary
```json
{
  "from": "2026-04-01T00:00:00Z",
  "items": [
    {
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "status": "open"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  },
  "to": "2026-04-20T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/reports/vends

| GET | `/v1/admin/restock/suggestions` | 10_Inventory | bearer | no | `{"items": [{"currentQuantity": 3, "dailyVelocity": 1.0, "daysToEmpty": 3.0, "fillRatio": 0.3, "machineId": "7c9e6679-742` |

### GET /v1/admin/restock/suggestions

- **Purpose:** Restock suggestions (admin)
- **Auth:** bearer (required=True)
- **Response 200:** Restock suggestions (admin)
```json
{
  "items": [
    {
      "currentQuantity": 3,
      "dailyVelocity": 1.0,
      "daysToEmpty": 3.0,
      "fillRatio": 0.3,
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "machineName": "Lobby-01",
      "maxQuantity": 10,
      "planogramId": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
      "planogramName": "Lobby default",
      "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "productName": "Cola 12oz",
      "productSku": "COLA-12",
      "siteId": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
      "siteName": "HQ",
      "slotIndex": 0,
      "suggestedRefillQuantity": 7,
      "unitsSoldInWindow": 14,
      "urgency": "medium"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 1
  },
  "velocityWindowDays": 14,
  "windowEnd": "2026-04-28T00:00:00.000000000Z",
  "windowStart": "2026-04-14T00:00:00.000000000Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/restock/suggestions

| GET | `/v1/admin/rollouts` | 08_Machines_Runtime_Config | bearer | no | `{"items": [], "meta": {"limit": 50, "offset": 0, "returned": 0}}` |

### GET /v1/admin/rollouts

- **Purpose:** List rollout campaigns
- **Auth:** bearer (required=True)
- **Response 200:** List rollout campaigns
```json
{
  "items": [],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 0
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/rollouts

| POST | `/v1/admin/rollouts` | 08_Machines_Runtime_Config | bearer | yes | `{"createdAt": "2026-04-29T12:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "rolloutType": "config_ver` |

### POST /v1/admin/rollouts

- **Purpose:** Create rollout campaign
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "rolloutType": "config_version",
  "strategy": {
    "canary_percent": 10,
    "confirm_full_rollout": false
  },
  "targetVersion": "2026-04-29T00:00:00Z"
}
```
- **Response 201:** Create rollout campaign
```json
{
  "createdAt": "2026-04-29T12:00:00.000000000Z",
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "rolloutType": "config_version",
  "status": "pending",
  "strategy": {
    "canary_percent": 10
  },
  "targetVersion": "2026-04-29T00:00:00Z",
  "updatedAt": "2026-04-29T12:00:00.000000000Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/rollouts

| GET | `/v1/admin/rollouts/{rolloutId}` | 08_Machines_Runtime_Config | bearer | no | `{"campaign": {"createdAt": "2026-04-29T12:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "rolloutType"` |

### GET /v1/admin/rollouts/{rolloutId}

- **Purpose:** Get rollout campaign
- **Auth:** bearer (required=True)
- **Path params:** `rolloutId`
- **Response 200:** Get rollout campaign
```json
{
  "campaign": {
    "createdAt": "2026-04-29T12:00:00.000000000Z",
    "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "rolloutType": "config_version",
    "status": "running",
    "strategy": {},
    "targetVersion": "2026-04-29T00:00:00Z",
    "updatedAt": "2026-04-29T12:00:00.000000000Z"
  },
  "targets": []
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/rollouts/{rolloutId}

| POST | `/v1/admin/rollouts/{rolloutId}/cancel` | 08_Machines_Runtime_Config | bearer | no | `{"campaign": {"cancelledAt": "2026-04-29T12:01:00.000000000Z", "createdAt": "2026-04-29T12:00:00.000000000Z", "id": "7c9` |

### POST /v1/admin/rollouts/{rolloutId}/cancel

- **Purpose:** Cancel rollout
- **Auth:** bearer (required=True)
- **Path params:** `rolloutId`
- **Response 200:** Cancel rollout
```json
{
  "campaign": {
    "cancelledAt": "2026-04-29T12:01:00.000000000Z",
    "createdAt": "2026-04-29T12:00:00.000000000Z",
    "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "rolloutType": "config_version",
    "status": "cancelled",
    "strategy": {},
    "targetVersion": "2026-04-29T00:00:00Z",
    "updatedAt": "2026-04-29T12:00:00.000000000Z"
  },
  "targets": []
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/rollouts/{rolloutId}/cancel

| POST | `/v1/admin/rollouts/{rolloutId}/pause` | 08_Machines_Runtime_Config | bearer | no | `{"campaign": {"createdAt": "2026-04-29T12:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "rolloutType"` |

### POST /v1/admin/rollouts/{rolloutId}/pause

- **Purpose:** Pause rollout
- **Auth:** bearer (required=True)
- **Path params:** `rolloutId`
- **Response 200:** Pause rollout
```json
{
  "campaign": {
    "createdAt": "2026-04-29T12:00:00.000000000Z",
    "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "rolloutType": "config_version",
    "status": "paused",
    "strategy": {},
    "targetVersion": "2026-04-29T00:00:00Z",
    "updatedAt": "2026-04-29T12:00:00.000000000Z"
  },
  "targets": []
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/rollouts/{rolloutId}/pause

| POST | `/v1/admin/rollouts/{rolloutId}/resume` | 08_Machines_Runtime_Config | bearer | no | `{"campaign": {"createdAt": "2026-04-29T12:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "rolloutType"` |

### POST /v1/admin/rollouts/{rolloutId}/resume

- **Purpose:** Resume rollout
- **Auth:** bearer (required=True)
- **Path params:** `rolloutId`
- **Response 200:** Resume rollout
```json
{
  "campaign": {
    "createdAt": "2026-04-29T12:00:00.000000000Z",
    "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "rolloutType": "config_version",
    "status": "running",
    "strategy": {},
    "targetVersion": "2026-04-29T00:00:00Z",
    "updatedAt": "2026-04-29T12:00:00.000000000Z"
  },
  "targets": []
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/rollouts/{rolloutId}/resume

| POST | `/v1/admin/rollouts/{rolloutId}/rollback` | 08_Machines_Runtime_Config | bearer | no | `{"campaign": {"createdAt": "2026-04-29T12:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "rolloutType"` |

### POST /v1/admin/rollouts/{rolloutId}/rollback

- **Purpose:** Roll back rollout
- **Auth:** bearer (required=True)
- **Path params:** `rolloutId`
- **Response 200:** Roll back rollout
```json
{
  "campaign": {
    "createdAt": "2026-04-29T12:00:00.000000000Z",
    "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "rolloutType": "config_version",
    "status": "rolled_back",
    "strategy": {
      "rollback_version": "2026-04-28T00:00:00Z"
    },
    "targetVersion": "2026-04-29T00:00:00Z",
    "updatedAt": "2026-04-29T12:00:00.000000000Z"
  },
  "targets": []
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/rollouts/{rolloutId}/rollback

| POST | `/v1/admin/rollouts/{rolloutId}/start` | 08_Machines_Runtime_Config | bearer | no | `{"campaign": {"createdAt": "2026-04-29T12:00:00.000000000Z", "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "rolloutType"` |

### POST /v1/admin/rollouts/{rolloutId}/start

- **Purpose:** Start rollout
- **Auth:** bearer (required=True)
- **Path params:** `rolloutId`
- **Response 200:** Start rollout
```json
{
  "campaign": {
    "createdAt": "2026-04-29T12:00:00.000000000Z",
    "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "rolloutType": "config_version",
    "status": "completed",
    "strategy": {},
    "targetVersion": "2026-04-29T00:00:00Z",
    "updatedAt": "2026-04-29T12:00:00.000000000Z"
  },
  "targets": []
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/rollouts/{rolloutId}/start

| GET | `/v1/admin/sites` | 06_Sites_Regions | bearer | no | `{"items": [{"address": {}, "code": "LOBBY", "created_at": "2026-04-29T00:00:00Z", "id": "3fa85f64-5717-4562-b3fc-2c963f6` |

### GET /v1/admin/sites

- **Purpose:** List sites (admin)
- **Auth:** bearer (required=True)
- **Query params:** `status, limit, offset`
- **Response 200:** List sites (admin)
```json
{
  "items": [
    {
      "address": {},
      "code": "LOBBY",
      "created_at": "2026-04-29T00:00:00Z",
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "name": "Lobby",
      "status": "active",
      "timezone": "UTC",
      "updated_at": "2026-04-29T00:00:00Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body site_id→siteId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/sites

| POST | `/v1/admin/sites` | 06_Sites_Regions | bearer | yes | `{"address": {}, "code": "LOBBY", "created_at": "2026-04-29T00:00:00Z", "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "na` |

### POST /v1/admin/sites

- **Purpose:** Create site (admin)
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "address": {
    "line1": "1 Main St"
  },
  "code": "canary-code-{{$guid}}",
  "name": "canary-name-{{$guid}}",
  "timezone": "UTC"
}
```
- **Response 201:** Create site (admin)
```json
{
  "address": {},
  "code": "LOBBY",
  "created_at": "2026-04-29T00:00:00Z",
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "name": "Lobby",
  "status": "active",
  "timezone": "UTC",
  "updated_at": "2026-04-29T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body site_id→siteId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/sites

| DELETE | `/v1/admin/sites/{siteId}` | 06_Sites_Regions | bearer | no | `{"address": {}, "code": "LOBBY", "created_at": "2026-04-29T00:00:00Z", "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "na` |

### DELETE /v1/admin/sites/{siteId}

- **Purpose:** Deactivate site (admin)
- **Auth:** bearer (required=True)
- **Path params:** `siteId`
- **Response 200:** Deactivate site (admin)
```json
{
  "address": {},
  "code": "LOBBY",
  "created_at": "2026-04-29T00:00:00Z",
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "name": "Lobby",
  "status": "archived",
  "timezone": "UTC",
  "updated_at": "2026-04-29T00:10:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body site_id→siteId
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/sites/{siteId}

| GET | `/v1/admin/sites/{siteId}` | 06_Sites_Regions | bearer | no | `{"address": {}, "code": "LOBBY", "created_at": "2026-04-29T00:00:00Z", "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "na` |

### GET /v1/admin/sites/{siteId}

- **Purpose:** Get site by ID (admin)
- **Auth:** bearer (required=True)
- **Path params:** `siteId`
- **Response 200:** Get site by ID (admin)
```json
{
  "address": {},
  "code": "LOBBY",
  "created_at": "2026-04-29T00:00:00Z",
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "name": "Lobby",
  "status": "active",
  "timezone": "UTC",
  "updated_at": "2026-04-29T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body site_id→siteId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/sites/{siteId}

| PATCH | `/v1/admin/sites/{siteId}` | 06_Sites_Regions | bearer | yes | `{"address": {}, "code": "LOBBY", "created_at": "2026-04-29T00:00:00Z", "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "na` |

### PATCH /v1/admin/sites/{siteId}

- **Purpose:** Patch site (admin)
- **Auth:** bearer (required=True)
- **Path params:** `siteId`
- **Request body example:**
```json
{
  "name": "canary-name-{{$guid}}",
  "status": "active"
}
```
- **Response 200:** Patch site (admin)
```json
{
  "address": {},
  "code": "LOBBY",
  "created_at": "2026-04-29T00:00:00Z",
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "name": "Lobby North",
  "status": "active",
  "timezone": "UTC",
  "updated_at": "2026-04-29T00:05:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body site_id→siteId
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/sites/{siteId}

| POST | `/v1/admin/sites/{siteId}/archive` | 06_Sites_Regions | bearer | no | `{"address": {}, "code": "LOBBY", "created_at": "2026-04-29T00:00:00Z", "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "na` |

### POST /v1/admin/sites/{siteId}/archive

- **Purpose:** Archive site (admin alias)
- **Auth:** bearer (required=True)
- **Path params:** `siteId`
- **Response 200:** Archive site (admin alias)
```json
{
  "address": {},
  "code": "LOBBY",
  "created_at": "2026-04-29T00:00:00Z",
  "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "name": "Lobby",
  "status": "inactive",
  "timezone": "UTC",
  "updated_at": "2026-04-29T00:05:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body site_id→siteId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/sites/{siteId}/archive

| POST | `/v1/admin/sites/{siteId}/disable` | 06_Sites_Regions | bearer | no | `{"address": {}, "code": "HQ-01", "created_at": "2026-04-01T00:00:00.000000000Z", "id": "aaaaaaaa-bbbb-cccc-dddd-11111111` |

### POST /v1/admin/sites/{siteId}/disable

- **Purpose:** Disable site (admin)
- **Auth:** bearer (required=True)
- **Path params:** `siteId`
- **Response 200:** Disable site (admin)
```json
{
  "address": {},
  "code": "HQ-01",
  "created_at": "2026-04-01T00:00:00.000000000Z",
  "id": "aaaaaaaa-bbbb-cccc-dddd-111111111111",
  "name": "HQ Lobby",
  "status": "inactive",
  "timezone": "America/New_York",
  "updated_at": "2026-04-01T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body site_id→siteId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/sites/{siteId}/disable

| GET | `/v1/admin/system/outbox` | 99_Utilities | bearer | no | `{"meta": {"limit": 50, "offset": 0, "returned": 1, "total": 42}, "rows": [{"aggregateId": "7c9e6679-7425-40de-944b-e07fc` |

### GET /v1/admin/system/outbox

- **Purpose:** List transactional outbox rows (system alias)
- **Auth:** bearer (required=True)
- **Query params:** `limit, offset`
- **Response 200:** List transactional outbox rows (system alias)
```json
{
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  },
  "rows": [
    {
      "aggregateId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "aggregateType": "payment",
      "createdAt": "2026-04-19T12:00:00.000000000Z",
      "eventType": "payment.session_started",
      "id": 101,
      "payload": {},
      "publishAttemptCount": 0,
      "status": "pending",
      "topic": "commerce.payments"
    }
  ],
  "stats": {
    "deadLetteredTotal": 1,
    "maxPendingAttempts": 5,
    "oldestPendingCreatedAt": "2026-04-19T12:00:00.000000000Z",
    "pendingDueNow": 2,
    "pendingTotal": 3,
    "publishingLeasedTotal": 0
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 403:** error
```json
{
  "error": {
    "code": "forbidden",
    "details": {},
    "message": "caller lacks permission for this resource",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/system/outbox

| GET | `/v1/admin/system/outbox/stats` | 99_Utilities | bearer | no | `{"stats": {"deadLetteredTotal": 1, "maxPendingAttempts": 5, "oldestPendingCreatedAt": "2026-04-19T12:00:00.000000000Z", ` |

### GET /v1/admin/system/outbox/stats

- **Purpose:** Outbox pipeline statistics (system alias)
- **Auth:** bearer (required=True)
- **Response 200:** Outbox pipeline statistics (system alias)
```json
{
  "stats": {
    "deadLetteredTotal": 1,
    "maxPendingAttempts": 5,
    "oldestPendingCreatedAt": "2026-04-19T12:00:00.000000000Z",
    "pendingDueNow": 2,
    "pendingTotal": 3,
    "publishingLeasedTotal": 0
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 403:** error
```json
{
  "error": {
    "code": "forbidden",
    "details": {},
    "message": "caller lacks permission for this resource",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/system/outbox/stats

| GET | `/v1/admin/system/outbox/{eventId}` | 99_Utilities | bearer | no | `{"aggregateId": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "aggregateType": "payment", "attempts": 0, "createdAt": "2026-04` |

### GET /v1/admin/system/outbox/{eventId}

- **Purpose:** Get one outbox row by id
- **Auth:** bearer (required=True)
- **Path params:** `eventId`
- **Response 200:** Get one outbox row by id
```json
{
  "aggregateId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "aggregateType": "payment",
  "attempts": 0,
  "createdAt": "2026-04-19T12:00:00.000000000Z",
  "deadLetteredAt": null,
  "eventType": "payment.session_started",
  "id": 101,
  "idempotencyKey": "idem-pay-001",
  "lastPublishAttemptAt": null,
  "lastPublishError": null,
  "lockedBy": null,
  "lockedUntil": null,
  "maxAttempts": 24,
  "nextAttemptAt": null,
  "nextPublishAfter": null,
  "payload": {},
  "publishAttemptCount": 0,
  "status": "pending",
  "topic": "commerce.payments",
  "updatedAt": "2026-04-19T12:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/system/outbox/{eventId}

| POST | `/v1/admin/system/outbox/{eventId}/mark-dlq` | 99_Utilities | bearer | yes | `{"marked": true}` |

### POST /v1/admin/system/outbox/{eventId}/mark-dlq

- **Purpose:** Manually move an outbox row to Postgres DLQ
- **Auth:** bearer (required=True)
- **Path params:** `eventId`
- **Request body example:**
```json
{
  "note": "Operator confirmed upstream outage before manual DLQ"
}
```
- **Response 200:** Manually move an outbox row to Postgres DLQ
```json
{
  "marked": true
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/system/outbox/{eventId}/mark-dlq

| POST | `/v1/admin/system/outbox/{eventId}/replay` | 99_Utilities | bearer | no | `{"retried": true}` |

### POST /v1/admin/system/outbox/{eventId}/replay

- **Purpose:** Replay a dead-lettered outbox row
- **Auth:** bearer (required=True)
- **Path params:** `eventId`
- **Response 200:** Replay a dead-lettered outbox row
```json
{
  "retried": true
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/system/outbox/{eventId}/replay

| POST | `/v1/admin/system/retention/dry-run` | 99_Utilities | bearer | no | `{"enterprise": {"candidates": {"inventory_events": 9000, "outbox_events_published": 12}, "dryRun": true, "enabled": true` |

### POST /v1/admin/system/retention/dry-run

- **Purpose:** Preview retention candidates (dry-run)
- **Auth:** bearer (required=True)
- **Response 200:** Preview retention candidates (dry-run)
```json
{
  "enterprise": {
    "candidates": {
      "inventory_events": 9000,
      "outbox_events_published": 12
    },
    "dryRun": true,
    "enabled": true
  },
  "overallDryRun": true,
  "telemetry": {
    "dryRun": true,
    "enabled": true,
    "stages": {
      "device_telemetry_events_non_critical": 1200
    }
  },
  "wouldModifyDatabase": false
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 403:** error
```json
{
  "error": {
    "code": "forbidden",
    "details": {},
    "message": "caller lacks permission for this resource",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/system/retention/dry-run

| POST | `/v1/admin/system/retention/run` | 99_Utilities | bearer | no | `{"enterprise": {"candidates": {"inventory_events": 9000, "outbox_events_published": 12}, "dryRun": true, "enabled": true` |

### POST /v1/admin/system/retention/run

- **Purpose:** Run bounded Postgres retention
- **Auth:** bearer (required=True)
- **Response 200:** Run bounded Postgres retention
```json
{
  "enterprise": {
    "candidates": {
      "inventory_events": 9000,
      "outbox_events_published": 12
    },
    "dryRun": true,
    "enabled": true
  },
  "overallDryRun": true,
  "telemetry": {
    "dryRun": true,
    "enabled": true,
    "stages": {
      "device_telemetry_events_non_critical": 1200
    }
  },
  "wouldModifyDatabase": false
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 403:** error
```json
{
  "error": {
    "code": "forbidden",
    "details": {},
    "message": "caller lacks permission for this resource",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/system/retention/run

| GET | `/v1/admin/system/retention/stats` | 99_Utilities | bearer | no | `{"policy": {"auditRetentionDays": 2555, "commandReceiptRetentionDays": 180, "commandRetentionDays": 180, "inventoryEvent` |

### GET /v1/admin/system/retention/stats

- **Purpose:** Data retention policy + table footprints (system)
- **Auth:** bearer (required=True)
- **Response 200:** Data retention policy + table footprints (system)
```json
{
  "policy": {
    "auditRetentionDays": 2555,
    "commandReceiptRetentionDays": 180,
    "commandRetentionDays": 180,
    "inventoryEventRetentionDays": 730,
    "offlineEventRetentionDays": 180,
    "outboxPublishedRetentionDays": 30,
    "paymentWebhookEventRetentionDays": 365,
    "processedMessageRetentionDays": 30,
    "telemetryCriticalRetentionDays": 365,
    "telemetryRetentionDays": 30
  },
  "runtime": {
    "destructiveRetentionAllowed": true,
    "enableRetentionWorker": true,
    "enterpriseCleanupEnabled": true,
    "globalDryRun": false,
    "telemetryCleanupEnabled": true
  },
  "tables": [
    {
      "oldestRecordAt": "2025-01-01T00:00:00.000000000Z",
      "tableName": "audit_events",
      "totalRows": 1000
    },
    {
      "oldestRecordAt": "2025-06-01T00:00:00.000000000Z",
      "tableName": "inventory_events",
      "totalRows": 50000
    }
  ]
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 403:** error
```json
{
  "error": {
    "code": "forbidden",
    "details": {},
    "message": "caller lacks permission for this resource",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/system/retention/stats

| GET | `/v1/admin/tags` | 03_Catalog_Categories_Brands_Tags | bearer | no | `{"items": [{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "cccccccc-dddd-eeee-ffff-000000000000", "name": "` |

### GET /v1/admin/tags

- **Purpose:** List tags
- **Auth:** bearer (required=True)
- **Query params:** `limit, offset`
- **Response 200:** List tags
```json
{
  "items": [
    {
      "active": true,
      "createdAt": "2026-01-01T00:00:00Z",
      "id": "cccccccc-dddd-eeee-ffff-000000000000",
      "name": "Cold drink example",
      "slug": "cold-drink-example",
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "totalCount": 1
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/tags

| POST | `/v1/admin/tags` | 03_Catalog_Categories_Brands_Tags | bearer | yes | `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "cccccccc-dddd-eeee-ffff-000000000000", "name": "Cold drink ` |

### POST /v1/admin/tags

- **Purpose:** Create tag
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "active": true,
  "name": "Cold Drink {{$timestamp}}",
  "slug": "cold-drink-{{$timestamp}}"
}
```
- **Response 200:** Create tag
```json
{
  "active": true,
  "createdAt": "2026-01-01T00:00:00Z",
  "id": "cccccccc-dddd-eeee-ffff-000000000000",
  "name": "Cold drink example",
  "slug": "cold-drink-example",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/tags

| DELETE | `/v1/admin/tags/{tagId}` | 03_Catalog_Categories_Brands_Tags | bearer | no | `{"active": false, "createdAt": "2026-01-01T00:00:00Z", "id": "cccccccc-dddd-eeee-ffff-000000000000", "name": "Cold drink` |

### DELETE /v1/admin/tags/{tagId}

- **Purpose:** Deactivate tag
- **Auth:** bearer (required=True)
- **Path params:** `tagId`
- **Response 200:** Deactivate tag
```json
{
  "active": false,
  "createdAt": "2026-01-01T00:00:00Z",
  "id": "cccccccc-dddd-eeee-ffff-000000000000",
  "name": "Cold drink example",
  "slug": "cold-drink-example",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/tags/{tagId}

| PATCH | `/v1/admin/tags/{tagId}` | 03_Catalog_Categories_Brands_Tags | bearer | yes | `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "cccccccc-dddd-eeee-ffff-000000000000", "name": "Cold drink ` |

### PATCH /v1/admin/tags/{tagId}

- **Purpose:** Update tag (PATCH)
- **Auth:** bearer (required=True)
- **Path params:** `tagId`
- **Request body example:**
```json
{
  "active": true,
  "name": "Cold Drink {{$timestamp}}",
  "slug": "cold-drink-{{$timestamp}}"
}
```
- **Response 200:** Update tag (PATCH)
```json
{
  "active": true,
  "createdAt": "2026-01-01T00:00:00Z",
  "id": "cccccccc-dddd-eeee-ffff-000000000000",
  "name": "Cold drink example",
  "slug": "cold-drink-example",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/tags/{tagId}

| PUT | `/v1/admin/tags/{tagId}` | 03_Catalog_Categories_Brands_Tags | bearer | yes | `{"active": true, "createdAt": "2026-01-01T00:00:00Z", "id": "cccccccc-dddd-eeee-ffff-000000000000", "name": "Cold drink ` |

### PUT /v1/admin/tags/{tagId}

- **Purpose:** Update tag
- **Auth:** bearer (required=True)
- **Path params:** `tagId`
- **Request body example:**
```json
{
  "active": true,
  "name": "Cold Drink {{$timestamp}}",
  "slug": "cold-drink-{{$timestamp}}"
}
```
- **Response 200:** Update tag
```json
{
  "active": true,
  "createdAt": "2026-01-01T00:00:00Z",
  "id": "cccccccc-dddd-eeee-ffff-000000000000",
  "name": "Cold drink example",
  "slug": "cold-drink-example",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:PUT /v1/admin/tags/{tagId}

| GET | `/v1/admin/technician-assignments` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"assignmentId": "dddddddd-eeee-ffff-0000-111111111111", "createdAt": "2026-04-01T00:00:00Z", "machineId": "7` |

### GET /v1/admin/technician-assignments

- **Purpose:** List technician assignments (alternate path)
- **Auth:** bearer (required=True)
- **Query params:** `technician_id, machine_id, from, to, limit, offset`
- **Response 200:** List technician assignments (alternate path)
```json
{
  "items": [
    {
      "assignmentId": "dddddddd-eeee-ffff-0000-111111111111",
      "createdAt": "2026-04-01T00:00:00Z",
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "machineName": "Lobby A",
      "machineSerialNumber": "SN-001",
      "role": "maintainer",
      "technicianDisplayName": "Alex Tech",
      "technicianId": "eeeeeeee-ffff-0000-1111-222222222222",
      "validFrom": "2026-04-01T00:00:00Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/technician-assignments

| POST | `/v1/admin/technician-assignments` | 08_Machines_Runtime_Config | bearer | yes | `{"created_at": "2026-04-01T00:00:00.000000000Z", "id": "dddddddd-eeee-ffff-0000-111111111111", "machine_id": "7c9e6679-7` |

### POST /v1/admin/technician-assignments

- **Purpose:** Create technician–machine assignment (admin)
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "machine_id": "{{machineId}}",
  "role": "maintainer",
  "technician_id": "eeeeeeee-ffff-0000-1111-222222222222"
}
```
- **Response 201:** Create technician–machine assignment (admin)
```json
{
  "created_at": "2026-04-01T00:00:00.000000000Z",
  "id": "dddddddd-eeee-ffff-0000-111111111111",
  "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "role": "maintainer",
  "status": "active",
  "technician_id": "eeeeeeee-ffff-0000-1111-222222222222",
  "updated_at": "2026-04-01T00:00:00.000000000Z",
  "valid_from": "2026-04-01T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/technician-assignments

| DELETE | `/v1/admin/technician-assignments/{assignmentId}` | 08_Machines_Runtime_Config | bearer | no | `{"created_at": "2026-04-01T00:00:00.000000000Z", "id": "dddddddd-eeee-ffff-0000-111111111111", "machine_id": "7c9e6679-7` |

### DELETE /v1/admin/technician-assignments/{assignmentId}

- **Purpose:** Release technician assignment (admin)
- **Auth:** bearer (required=True)
- **Path params:** `assignmentId`
- **Response 200:** Release technician assignment (admin)
```json
{
  "created_at": "2026-04-01T00:00:00.000000000Z",
  "id": "dddddddd-eeee-ffff-0000-111111111111",
  "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "role": "maintainer",
  "status": "active",
  "technician_id": "eeeeeeee-ffff-0000-1111-222222222222",
  "updated_at": "2026-04-01T00:00:00.000000000Z",
  "valid_from": "2026-04-01T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/technician-assignments/{assignmentId}

| GET | `/v1/admin/technician-assignments/{assignmentId}` | 08_Machines_Runtime_Config | bearer | no | `{"created_at": "2026-04-01T00:00:00.000000000Z", "id": "dddddddd-eeee-ffff-0000-111111111111", "machine_id": "7c9e6679-7` |

### GET /v1/admin/technician-assignments/{assignmentId}

- **Purpose:** Get technician assignment by ID (admin)
- **Auth:** bearer (required=True)
- **Path params:** `assignmentId`
- **Response 200:** Get technician assignment by ID (admin)
```json
{
  "created_at": "2026-04-01T00:00:00.000000000Z",
  "id": "dddddddd-eeee-ffff-0000-111111111111",
  "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "role": "maintainer",
  "status": "active",
  "technician_id": "eeeeeeee-ffff-0000-1111-222222222222",
  "updated_at": "2026-04-01T00:00:00.000000000Z",
  "valid_from": "2026-04-01T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/technician-assignments/{assignmentId}

| PATCH | `/v1/admin/technician-assignments/{assignmentId}` | 08_Machines_Runtime_Config | bearer | yes | `{"created_at": "2026-04-01T00:00:00.000000000Z", "id": "dddddddd-eeee-ffff-0000-111111111111", "machine_id": "7c9e6679-7` |

### PATCH /v1/admin/technician-assignments/{assignmentId}

- **Purpose:** Patch technician assignment (admin)
- **Auth:** bearer (required=True)
- **Path params:** `assignmentId`
- **Request body example:**
```json
{
  "role": "lead"
}
```
- **Response 200:** Patch technician assignment (admin)
```json
{
  "created_at": "2026-04-01T00:00:00.000000000Z",
  "id": "dddddddd-eeee-ffff-0000-111111111111",
  "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "role": "maintainer",
  "status": "active",
  "technician_id": "eeeeeeee-ffff-0000-1111-222222222222",
  "updated_at": "2026-04-01T00:00:00.000000000Z",
  "valid_from": "2026-04-01T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/technician-assignments/{assignmentId}

| POST | `/v1/admin/technician-assignments/{assignmentId}/cancel` | 08_Machines_Runtime_Config | bearer | no | `{"created_at": "2026-04-01T00:00:00.000000000Z", "id": "dddddddd-eeee-ffff-0000-111111111111", "machine_id": "7c9e6679-7` |

### POST /v1/admin/technician-assignments/{assignmentId}/cancel

- **Purpose:** Cancel technician assignment (admin)
- **Auth:** bearer (required=True)
- **Path params:** `assignmentId`
- **Response 200:** Cancel technician assignment (admin)
```json
{
  "created_at": "2026-04-01T00:00:00.000000000Z",
  "id": "dddddddd-eeee-ffff-0000-111111111111",
  "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "role": "maintainer",
  "status": "active",
  "technician_id": "eeeeeeee-ffff-0000-1111-222222222222",
  "updated_at": "2026-04-01T00:00:00.000000000Z",
  "valid_from": "2026-04-01T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/technician-assignments/{assignmentId}/cancel

| GET | `/v1/admin/technicians` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"createdAt": "2026-03-01T00:00:00Z", "displayName": "Alex Tech", "technicianId": "eeeeeeee-ffff-0000-1111-22` |

### GET /v1/admin/technicians

- **Purpose:** List technicians (admin)
- **Auth:** bearer (required=True)
- **Query params:** `technician_id, search, from, to, limit, offset`
- **Response 200:** List technicians (admin)
```json
{
  "items": [
    {
      "createdAt": "2026-03-01T00:00:00Z",
      "displayName": "Alex Tech",
      "technicianId": "eeeeeeee-ffff-0000-1111-222222222222"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/technicians

| POST | `/v1/admin/technicians` | 08_Machines_Runtime_Config | bearer | yes | `{"created_at": "2026-03-01T00:00:00.000000000Z", "display_name": "Alex Tech", "id": "eeeeeeee-ffff-0000-1111-22222222222` |

### POST /v1/admin/technicians

- **Purpose:** Create technician (admin)
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "display_name": "canary-display-name-{{$guid}}",
  "email": "{{adminEmail}}"
}
```
- **Response 201:** Create technician (admin)
```json
{
  "created_at": "2026-03-01T00:00:00.000000000Z",
  "display_name": "Alex Tech",
  "id": "eeeeeeee-ffff-0000-1111-222222222222",
  "status": "active",
  "updated_at": "2026-03-01T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/technicians

| GET | `/v1/admin/technicians/{technicianId}` | 08_Machines_Runtime_Config | bearer | no | `{"created_at": "2026-03-01T00:00:00.000000000Z", "display_name": "Alex Tech", "id": "eeeeeeee-ffff-0000-1111-22222222222` |

### GET /v1/admin/technicians/{technicianId}

- **Purpose:** Get technician by ID (admin)
- **Auth:** bearer (required=True)
- **Path params:** `technicianId`
- **Response 200:** Get technician by ID (admin)
```json
{
  "created_at": "2026-03-01T00:00:00.000000000Z",
  "display_name": "Alex Tech",
  "id": "eeeeeeee-ffff-0000-1111-222222222222",
  "status": "active",
  "updated_at": "2026-03-01T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/technicians/{technicianId}

| PATCH | `/v1/admin/technicians/{technicianId}` | 08_Machines_Runtime_Config | bearer | yes | `{"created_at": "2026-03-01T00:00:00.000000000Z", "display_name": "Alex Field", "id": "eeeeeeee-ffff-0000-1111-2222222222` |

### PATCH /v1/admin/technicians/{technicianId}

- **Purpose:** Patch technician (admin)
- **Auth:** bearer (required=True)
- **Path params:** `technicianId`
- **Request body example:**
```json
{
  "display_name": "canary-display-name-{{$guid}}"
}
```
- **Response 200:** Patch technician (admin)
```json
{
  "created_at": "2026-03-01T00:00:00.000000000Z",
  "display_name": "Alex Field",
  "id": "eeeeeeee-ffff-0000-1111-222222222222",
  "status": "active",
  "updated_at": "2026-03-01T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/technicians/{technicianId}

| POST | `/v1/admin/technicians/{technicianId}/disable` | 08_Machines_Runtime_Config | bearer | no | `{"created_at": "2026-03-01T00:00:00.000000000Z", "display_name": "Alex Tech", "id": "eeeeeeee-ffff-0000-1111-22222222222` |

### POST /v1/admin/technicians/{technicianId}/disable

- **Purpose:** Disable technician (admin)
- **Auth:** bearer (required=True)
- **Path params:** `technicianId`
- **Response 200:** Disable technician (admin)
```json
{
  "created_at": "2026-03-01T00:00:00.000000000Z",
  "display_name": "Alex Tech",
  "id": "eeeeeeee-ffff-0000-1111-222222222222",
  "status": "inactive",
  "updated_at": "2026-03-01T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/technicians/{technicianId}/disable

| POST | `/v1/admin/technicians/{technicianId}/enable` | 08_Machines_Runtime_Config | bearer | no | `{"created_at": "2026-03-01T00:00:00.000000000Z", "display_name": "Alex Tech", "id": "eeeeeeee-ffff-0000-1111-22222222222` |

### POST /v1/admin/technicians/{technicianId}/enable

- **Purpose:** Enable technician (admin)
- **Auth:** bearer (required=True)
- **Path params:** `technicianId`
- **Response 200:** Enable technician (admin)
```json
{
  "created_at": "2026-03-01T00:00:00.000000000Z",
  "display_name": "Alex Tech",
  "id": "eeeeeeee-ffff-0000-1111-222222222222",
  "status": "active",
  "updated_at": "2026-03-01T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/technicians/{technicianId}/enable

| GET | `/v1/admin/users` | 02_Admin_Accounts_RBAC | bearer | no | `{"items": [{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator` |

### GET /v1/admin/users

- **Purpose:** List API accounts (admin) — alternate path
- **Auth:** bearer (required=True)
- **Query params:** `limit, offset`
- **Response 200:** List API accounts (admin) — alternate path
```json
{
  "items": [
    {
      "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "createdAt": "2026-01-01T00:00:00Z",
      "email": "operator@example.com",
      "roles": [
        "admin"
      ],
      "status": "active",
      "updatedAt": "2026-04-19T10:00:00Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/users

| POST | `/v1/admin/users` | 02_Admin_Accounts_RBAC | bearer | yes | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### POST /v1/admin/users

- **Purpose:** Create API account — alternate path
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "email": "{{adminEmail}}",
  "password": "{{adminPassword}}",
  "roles": [
    "support"
  ],
  "status": "active"
}
```
- **Response 201:** Create API account — alternate path
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/users

| GET | `/v1/admin/users/{userId}` | 02_Admin_Accounts_RBAC | bearer | no | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### GET /v1/admin/users/{userId}

- **Purpose:** Get API account — alternate path
- **Auth:** bearer (required=True)
- **Path params:** `userId`
- **Response 200:** Get API account — alternate path
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Response 404:** error
```json
{
  "error": {
    "code": "not_found",
    "details": {},
    "message": "resource was not found",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/users/{userId}

| PATCH | `/v1/admin/users/{userId}` | 02_Admin_Accounts_RBAC | bearer | yes | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### PATCH /v1/admin/users/{userId}

- **Purpose:** Patch API account — alternate path
- **Auth:** bearer (required=True)
- **Path params:** `userId`
- **Request body example:**
```json
{
  "roles": [
    "support"
  ]
}
```
- **Response 200:** Patch API account — alternate path
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/users/{userId}

| POST | `/v1/admin/users/{userId}/disable` | 02_Admin_Accounts_RBAC | bearer | no | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### POST /v1/admin/users/{userId}/disable

- **Purpose:** Disable API account — alternate path
- **Auth:** bearer (required=True)
- **Path params:** `userId`
- **Response 200:** Disable API account — alternate path
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/users/{userId}/disable

| POST | `/v1/admin/users/{userId}/enable` | 02_Admin_Accounts_RBAC | bearer | no | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### POST /v1/admin/users/{userId}/enable

- **Purpose:** Enable API account — alternate path
- **Auth:** bearer (required=True)
- **Path params:** `userId`
- **Response 200:** Enable API account — alternate path
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/users/{userId}/enable

| POST | `/v1/admin/users/{userId}/reset-password` | 02_Admin_Accounts_RBAC | bearer | yes | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### POST /v1/admin/users/{userId}/reset-password

- **Purpose:** Reset password — alternate path
- **Auth:** bearer (required=True)
- **Path params:** `userId`
- **Request body example:**
```json
{
  "password": "{{adminPassword}}"
}
```
- **Response 200:** Reset password — alternate path
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/users/{userId}/reset-password

| POST | `/v1/admin/users/{userId}/revoke-sessions` | 02_Admin_Accounts_RBAC | bearer | no | `{}` |

### POST /v1/admin/users/{userId}/revoke-sessions

- **Purpose:** Revoke user sessions — alternate path
- **Auth:** bearer (required=True)
- **Path params:** `userId`
- **Response 204:** Revoke user sessions — alternate path
```json
{}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/users/{userId}/revoke-sessions

| PATCH | `/v1/admin/users/{userId}/roles` | 02_Admin_Accounts_RBAC | bearer | yes | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### PATCH /v1/admin/users/{userId}/roles

- **Purpose:** Replace roles — PATCH alias
- **Auth:** bearer (required=True)
- **Path params:** `userId`
- **Request body example:**
```json
{
  "roles": [
    "support"
  ]
}
```
- **Response 200:** Replace roles — PATCH alias
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/users/{userId}/roles

| POST | `/v1/admin/users/{userId}/roles` | 02_Admin_Accounts_RBAC | bearer | yes | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### POST /v1/admin/users/{userId}/roles

- **Purpose:** Replace roles — alternate path
- **Auth:** bearer (required=True)
- **Path params:** `userId`
- **Request body example:**
```json
{
  "roles": [
    "support"
  ]
}
```
- **Response 200:** Replace roles — alternate path
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/admin/users/{userId}/roles

| PUT | `/v1/admin/users/{userId}/roles` | 02_Admin_Accounts_RBAC | bearer | yes | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### PUT /v1/admin/users/{userId}/roles

- **Purpose:** Replace roles — alternate path
- **Auth:** bearer (required=True)
- **Path params:** `userId`
- **Request body example:**
```json
{
  "roles": [
    "catalog_manager"
  ]
}
```
- **Response 200:** Replace roles — alternate path
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:PUT /v1/admin/users/{userId}/roles

| DELETE | `/v1/admin/users/{userId}/roles/{role}` | 02_Admin_Accounts_RBAC | bearer | no | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### DELETE /v1/admin/users/{userId}/roles/{role}

- **Purpose:** Remove one role from user
- **Auth:** bearer (required=True)
- **Path params:** `userId, role`
- **Response 200:** Remove one role from user
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "viewer"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/admin/users/{userId}/roles/{role}

| GET | `/v1/admin/users/{userId}/sessions` | 02_Admin_Accounts_RBAC | bearer | no | `{"sessions": [{"createdAt": "2026-04-19T10:00:00Z", "expiresAt": "2026-05-19T12:00:00Z", "sessionId": "bbbbbbbb-bbbb-bbb` |

### GET /v1/admin/users/{userId}/sessions

- **Purpose:** List user sessions — alternate path
- **Auth:** bearer (required=True)
- **Path params:** `userId`
- **Response 200:** List user sessions — alternate path
```json
{
  "sessions": [
    {
      "createdAt": "2026-04-19T10:00:00Z",
      "expiresAt": "2026-05-19T12:00:00Z",
      "sessionId": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
      "status": "active"
    }
  ]
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/admin/users/{userId}/sessions

| PATCH | `/v1/admin/users/{userId}/status` | 02_Admin_Accounts_RBAC | bearer | yes | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "createdAt": "2026-01-01T00:00:00Z", "email": "operator@example.co` |

### PATCH /v1/admin/users/{userId}/status

- **Purpose:** Patch account status only — alternate path
- **Auth:** bearer (required=True)
- **Path params:** `userId`
- **Request body example:**
```json
{
  "status": "disabled"
}
```
- **Response 200:** Patch account status only — alternate path
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "createdAt": "2026-01-01T00:00:00Z",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "status": "active",
  "updatedAt": "2026-04-19T10:00:00Z"
}
```
- **Source:** docs/swagger/swagger.json, openapi:PATCH /v1/admin/users/{userId}/status

| POST | `/v1/auth/change-password` | 01_Auth | bearer | yes | `{}` |

### POST /v1/auth/change-password

- **Purpose:** Change password (self-service)
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "currentPassword": "{{adminPassword}}",
  "newPassword": "{{adminPassword}}"
}
```
- **Response 204:** Change password (self-service)
```json
{}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/auth/change-password

| POST | `/v1/auth/login` | 01_Auth | none | yes | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "email": "operator@example.com", "roles": ["admin"], "tokens": {"a` |

### POST /v1/auth/login

- **Purpose:** Exchange email/password for JWT session tokens
- **Auth:** none (required=False)
- **Request body example:**
```json
{
  "email": "{{adminEmail}}",
  "password": "{{adminPassword}}"
}
```
- **Response 200:** Exchange email/password for JWT session tokens
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "tokens": {
    "accessExpiresAt": "2026-04-19T13:00:00Z",
    "accessToken": "stub-access-token",
    "refreshExpiresAt": "2026-05-19T12:00:00Z",
    "refreshToken": "stub-refresh-token",
    "tokenType": "Bearer"
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "invalid credentials",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body access_token→accessToken, response body refresh_token→refreshToken
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/auth/login

| POST | `/v1/auth/logout` | 01_Auth | bearer | yes | `{}` |

### POST /v1/auth/logout

- **Purpose:** Revoke refresh token(s)
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "revokeAll": false
}
```
- **Response 204:** Revoke refresh token(s)
```json
{}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/auth/logout

| GET | `/v1/auth/me` | 01_Auth | bearer | no | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "email": "operator@example.com", "roles": ["admin"]}` |

### GET /v1/auth/me

- **Purpose:** Current authenticated principal
- **Auth:** bearer (required=True)
- **Response 200:** Current authenticated principal
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ]
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 500:** error
```json
{
  "error": {
    "code": "internal",
    "details": {},
    "message": "unexpected server error",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/auth/me

| POST | `/v1/auth/mfa/totp/disable` | 01_Auth | bearer | yes | `{}` |

### POST /v1/auth/mfa/totp/disable

- **Purpose:** Disable TOTP for the current user
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "currentPassword": "{{adminPassword}}",
  "totpCode": "123456"
}
```
- **Response 204:** Disable TOTP for the current user
```json
{}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/auth/mfa/totp/disable

| POST | `/v1/auth/mfa/totp/enroll` | 01_Auth | bearer | no | `{"otpauthUri": "otpauth://totp/AVF%20Admin:operator%40example.com?secret=ABCDABCDABCDABCD&issuer=AVF%20Admin", "secret":` |

### POST /v1/auth/mfa/totp/enroll

- **Purpose:** Start TOTP MFA enrollment
- **Auth:** bearer (required=True)
- **Response 200:** Start TOTP MFA enrollment
```json
{
  "otpauthUri": "otpauth://totp/AVF%20Admin:operator%40example.com?secret=ABCDABCDABCDABCD&issuer=AVF%20Admin",
  "secret": "ABCDABCDABCDABCDABCDABCDABCDABCD"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/auth/mfa/totp/enroll

| POST | `/v1/auth/mfa/totp/verify` | 01_Auth | bearer | yes | `{"accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "email": "operator@example.com", "roles": ["admin"], "tokens": {"a` |

### POST /v1/auth/mfa/totp/verify

- **Purpose:** Verify TOTP (enrollment or login)
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "code": "canary-code-{{$guid}}"
}
```
- **Response 200:** Verify TOTP (enrollment or login)
```json
{
  "accountId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "email": "operator@example.com",
  "roles": [
    "admin"
  ],
  "tokens": {
    "accessExpiresAt": "2026-04-19T13:00:00Z",
    "accessToken": "stub-access-token",
    "refreshExpiresAt": "2026-05-19T12:00:00Z",
    "refreshToken": "stub-refresh-token",
    "tokenType": "Bearer"
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/auth/mfa/totp/verify

| POST | `/v1/auth/password/change` | 01_Auth | bearer | yes | `{}` |

### POST /v1/auth/password/change

- **Purpose:** Change password (self-service)
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "currentPassword": "{{adminPassword}}",
  "newPassword": "{{adminPassword}}"
}
```
- **Response 204:** Change password (self-service)
```json
{}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/auth/password/change

| POST | `/v1/auth/password/reset/confirm` | 01_Auth | none | yes | `{}` |

### POST /v1/auth/password/reset/confirm

- **Purpose:** Confirm password reset
- **Auth:** none (required=False)
- **Request body example:**
```json
{
  "newPassword": "{{adminPassword}}",
  "token": "opaque-reset-token"
}
```
- **Response 204:** Confirm password reset
```json
{}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/auth/password/reset/confirm

| POST | `/v1/auth/password/reset/request` | 01_Auth | none | yes | `{"accepted": true}` |

### POST /v1/auth/password/reset/request

- **Purpose:** Request password reset
- **Auth:** none (required=False)
- **Request body example:**
```json
{
  "email": "{{adminEmail}}"
}
```
- **Response 202:** Request password reset
```json
{
  "accepted": true
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/auth/password/reset/request

| POST | `/v1/auth/refresh` | 01_Auth | none | yes | `{"tokens": {"accessExpiresAt": "2026-04-19T13:00:00Z", "accessToken": "stub-access-token", "refreshExpiresAt": "2026-05-` |

### POST /v1/auth/refresh

- **Purpose:** Rotate access token using a refresh token
- **Auth:** none (required=False)
- **Request body example:**
```json
{
  "refreshToken": "{{refreshToken}}"
}
```
- **Response 200:** Rotate access token using a refresh token
```json
{
  "tokens": {
    "accessExpiresAt": "2026-04-19T13:00:00Z",
    "accessToken": "stub-access-token",
    "refreshExpiresAt": "2026-05-19T12:00:00Z",
    "refreshToken": "stub-refresh-token",
    "tokenType": "Bearer"
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/auth/refresh

| DELETE | `/v1/auth/sessions` | 01_Auth | bearer | yes | `{}` |

### DELETE /v1/auth/sessions

- **Purpose:** Revoke other sessions
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "exceptRefreshToken": "stub-refresh-token"
}
```
- **Response 204:** Revoke other sessions
```json
{}
```
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/auth/sessions

| GET | `/v1/auth/sessions` | 01_Auth | bearer | no | `{"sessions": [{"createdAt": "2026-04-19T10:00:00Z", "expiresAt": "2026-05-19T12:00:00Z", "sessionId": "bbbbbbbb-bbbb-bbb` |

### GET /v1/auth/sessions

- **Purpose:** List current admin sessions
- **Auth:** bearer (required=True)
- **Response 200:** List current admin sessions
```json
{
  "sessions": [
    {
      "createdAt": "2026-04-19T10:00:00Z",
      "expiresAt": "2026-05-19T12:00:00Z",
      "sessionId": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
      "status": "active"
    }
  ]
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/auth/sessions

| DELETE | `/v1/auth/sessions/{sessionId}` | 01_Auth | bearer | no | `{}` |

### DELETE /v1/auth/sessions/{sessionId}

- **Purpose:** Revoke one session
- **Auth:** bearer (required=True)
- **Path params:** `sessionId`
- **Response 204:** Revoke one session
```json
{}
```
- **Source:** docs/swagger/swagger.json, openapi:DELETE /v1/auth/sessions/{sessionId}

| POST | `/v1/commerce/cash-checkout` | 12_Orders | bearer | yes | `{"order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "order_status": "paid", "payment_id": "aaaaaaaa-bbbb-cccc-dddd-eeee` |

### POST /v1/commerce/cash-checkout

- **Purpose:** Create order, record captured cash payment, mark paid
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "currency": "USD",
  "machine_id": "{{machineId}}",
  "product_id": "{{productId}}",
  "slot_index": 3,
  "subtotal_minor": 125,
  "tax_minor": 10,
  "total_minor": 135
}
```
- **Response 200:** Create order, record captured cash payment, mark paid
```json
{
  "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "order_status": "paid",
  "payment_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "payment_state": "captured",
  "replay": false,
  "vend_session_id": "8d3e2f10-1111-2222-3333-444455556666"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/commerce/cash-checkout

| POST | `/v1/commerce/orders` | 12_Orders | bearer | yes | `{"order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "order_status": "created", "replay": false, "vend_session_id": "8d3` |

### POST /v1/commerce/orders

- **Purpose:** Create order and initial vend session
- **Auth:** bearer (required=True)
- **Request body example:**
```json
{
  "currency": "USD",
  "machine_id": "{{machineId}}",
  "product_id": "{{productId}}",
  "slot_index": 3,
  "subtotal_minor": 125,
  "tax_minor": 10,
  "total_minor": 135
}
```
- **Response 201:** Create order and initial vend session
```json
{
  "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "order_status": "created",
  "replay": false,
  "vend_session_id": "8d3e2f10-1111-2222-3333-444455556666",
  "vend_state": "pending"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "missing_idempotency_key",
    "details": {},
    "message": "missing idempotency key header (Idempotency-Key or X-Idempotency-Key)",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body order_id→orderId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/commerce/orders

| GET | `/v1/commerce/orders/{orderId}` | 12_Orders | bearer | no | `{"order": {"created_at": "2026-04-19T12:00:00Z", "currency": "USD", "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "machi` |

### GET /v1/commerce/orders/{orderId}

- **Purpose:** Checkout status for order
- **Auth:** bearer (required=True)
- **Path params:** `orderId`
- **Query params:** `slot_index`
- **Response 200:** Checkout status for order
```json
{
  "order": {
    "created_at": "2026-04-19T12:00:00Z",
    "currency": "USD",
    "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "status": "paid",
    "subtotal_minor": 125,
    "tax_minor": 10,
    "total_minor": 135,
    "updated_at": "2026-04-19T12:05:00Z"
  },
  "payment": {
    "amount_minor": 135,
    "created_at": "2026-04-19T12:04:00Z",
    "currency": "USD",
    "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
    "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "provider": "stripe",
    "state": "captured"
  },
  "vend": {
    "created_at": "2026-04-19T12:00:01Z",
    "id": "8d3e2f10-1111-2222-3333-444455556666",
    "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
    "product_id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
    "slot_index": 3,
    "state": "in_progress"
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body order_id→orderId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/commerce/orders/{orderId}

| POST | `/v1/commerce/orders/{orderId}/cancel` | 12_Orders | bearer | yes | `{"order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "order_status": "cancelled", "payment_state": "none", "refund_state` |

### POST /v1/commerce/orders/{orderId}/cancel

- **Purpose:** Cancel order before payment capture
- **Auth:** bearer (required=True)
- **Path params:** `orderId`
- **Request body example:**
```json
{
  "reason": "user_cancelled",
  "slot_index": 3
}
```
- **Response 200:** Cancel order before payment capture
```json
{
  "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "order_status": "cancelled",
  "payment_state": "none",
  "refund_state": "not_required",
  "replay": false
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body order_id→orderId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/commerce/orders/{orderId}/cancel

| POST | `/v1/commerce/orders/{orderId}/payment-session` | 12_Orders | bearer | yes | `{"outbox_event_id": 9001, "payment_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "payment_state": "created", "replay": fa` |

### POST /v1/commerce/orders/{orderId}/payment-session

- **Purpose:** Start payment with outbox row
- **Auth:** bearer (required=True)
- **Path params:** `orderId`
- **Request body example:**
```json
{
  "amount_minor": 135,
  "currency": "USD",
  "outbox_payload_json": {
    "source": "http_api"
  },
  "payment_state": "created",
  "provider": "stripe"
}
```
- **Response 200:** Start payment with outbox row
```json
{
  "outbox_event_id": 9001,
  "payment_id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "payment_state": "created",
  "replay": false
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body order_id→orderId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/commerce/orders/{orderId}/payment-session

| POST | `/v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks` | 12_Orders | none | yes | `{"order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "payment_state": "captured", "replay": false}` |

### POST /v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks

- **Purpose:** Apply provider webhook
- **Auth:** none (required=False)
- **Path params:** `orderId, paymentId`
- **Request body example:**
```json
{
  "event_type": "payment_intent.succeeded",
  "normalized_payment_state": "captured",
  "payload_json": {
    "id": "pi_example_123",
    "status": "succeeded"
  },
  "provider": "stripe",
  "provider_reference": "pi_example_123"
}
```
- **Response 200:** Apply provider webhook
```json
{
  "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "payment_state": "captured",
  "replay": false
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "webhook_timestamp_skew",
    "details": {},
    "message": "webhook timestamp outside allowed skew",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "webhook_auth_failed",
    "details": {},
    "message": "invalid webhook signature",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body order_id→orderId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/commerce/orders/{orderId}/payments/{paymentId}/webhooks

| GET | `/v1/commerce/orders/{orderId}/reconciliation` | 12_Orders | bearer | no | `{"kind": "commerce.reconciliation_snapshot", "status": {"order": {"created_at": "2026-04-19T12:00:00Z", "currency": "USD` |

### GET /v1/commerce/orders/{orderId}/reconciliation

- **Purpose:** Reconciliation snapshot wrapper
- **Auth:** bearer (required=True)
- **Path params:** `orderId`
- **Query params:** `slot_index`
- **Response 200:** Reconciliation snapshot wrapper
```json
{
  "kind": "commerce.reconciliation_snapshot",
  "status": {
    "order": {
      "created_at": "2026-04-19T12:00:00Z",
      "currency": "USD",
      "id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "status": "paid",
      "subtotal_minor": 125,
      "tax_minor": 10,
      "total_minor": 135,
      "updated_at": "2026-04-19T12:05:00Z"
    },
    "payment": {
      "amount_minor": 135,
      "created_at": "2026-04-19T12:04:00Z",
      "currency": "USD",
      "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "provider": "stripe",
      "state": "captured"
    },
    "vend": {
      "created_at": "2026-04-19T12:00:01Z",
      "id": "8d3e2f10-1111-2222-3333-444455556666",
      "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "product_id": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
      "slot_index": 3,
      "state": "in_progress"
    }
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body order_id→orderId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/commerce/orders/{orderId}/reconciliation

| GET | `/v1/commerce/orders/{orderId}/refunds` | 14_Refunds_Disputes | bearer | no | `{"items": [{"amount_minor": 15000, "created_at": "2026-04-24T00:00:00Z", "currency": "VND", "order_id": "3fa85f64-5717-4` |

### GET /v1/commerce/orders/{orderId}/refunds

- **Purpose:** List refunds for an order
- **Auth:** bearer (required=True)
- **Path params:** `orderId`
- **Response 200:** List refunds for an order
```json
{
  "items": [
    {
      "amount_minor": 15000,
      "created_at": "2026-04-24T00:00:00Z",
      "currency": "VND",
      "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "payment_id": "11111111-2222-3333-4444-555555555555",
      "refund_id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
      "refund_state": "pending"
    }
  ]
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body order_id→orderId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/commerce/orders/{orderId}/refunds

| POST | `/v1/commerce/orders/{orderId}/refunds` | 14_Refunds_Disputes | bearer | yes | `{"amount_minor": 15000, "currency": "VND", "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "payment_id": "11111111-2` |

### POST /v1/commerce/orders/{orderId}/refunds

- **Purpose:** Create or replay a refund (idempotent)
- **Auth:** bearer (required=True)
- **Path params:** `orderId`
- **Request body example:**
```json
{
  "amount_minor": 15000,
  "currency": "VND",
  "metadata": {
    "slot_index": 3,
    "vend_failure_reason": "motor_timeout"
  },
  "reason": "vend_failed"
}
```
- **Response 200:** Create or replay a refund (idempotent)
```json
{
  "amount_minor": 15000,
  "currency": "VND",
  "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "payment_id": "11111111-2222-3333-4444-555555555555",
  "refund_id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "refund_state": "pending",
  "replay": false
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "refund_not_allowed",
    "details": {},
    "message": "refund exceeds captured amount or order unpaid",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body order_id→orderId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/commerce/orders/{orderId}/refunds

| GET | `/v1/commerce/orders/{orderId}/refunds/{refundId}` | 14_Refunds_Disputes | bearer | no | `{"amount_minor": 15000, "created_at": "2026-04-24T00:00:00Z", "currency": "VND", "order_id": "3fa85f64-5717-4562-b3fc-2c` |

### GET /v1/commerce/orders/{orderId}/refunds/{refundId}

- **Purpose:** Get one refund on an order
- **Auth:** bearer (required=True)
- **Path params:** `orderId, refundId`
- **Response 200:** Get one refund on an order
```json
{
  "amount_minor": 15000,
  "created_at": "2026-04-24T00:00:00Z",
  "currency": "VND",
  "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "payment_id": "11111111-2222-3333-4444-555555555555",
  "refund_id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "refund_state": "pending"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body order_id→orderId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/commerce/orders/{orderId}/refunds/{refundId}

| POST | `/v1/commerce/orders/{orderId}/vend/failure` | 14_Refunds_Disputes | bearer | yes | `{"local_cash_refund_required": false, "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "order_status": "failed", "ref` |

### POST /v1/commerce/orders/{orderId}/vend/failure

- **Purpose:** Finalize vend failure
- **Auth:** bearer (required=True)
- **Path params:** `orderId`
- **Request body example:**
```json
{
  "failure_reason": "motor_timeout",
  "slot_index": 3
}
```
- **Response 200:** Finalize vend failure
```json
{
  "local_cash_refund_required": false,
  "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "order_status": "failed",
  "refund_required": true,
  "vend_state": "failed"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body order_id→orderId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/commerce/orders/{orderId}/vend/failure

| POST | `/v1/commerce/orders/{orderId}/vend/start` | 12_Orders | bearer | yes | `{"slot_index": 3, "vend_state": "in_progress"}` |

### POST /v1/commerce/orders/{orderId}/vend/start

- **Purpose:** Advance vend to in_progress
- **Auth:** bearer (required=True)
- **Path params:** `orderId`
- **Request body example:**
```json
{
  "slot_index": 3
}
```
- **Response 200:** Advance vend to in_progress
```json
{
  "slot_index": 3,
  "vend_state": "in_progress"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body order_id→orderId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/commerce/orders/{orderId}/vend/start

| POST | `/v1/commerce/orders/{orderId}/vend/success` | 12_Orders | bearer | yes | `{"order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "order_status": "completed", "vend_state": "success"}` |

### POST /v1/commerce/orders/{orderId}/vend/success

- **Purpose:** Finalize vend success
- **Auth:** bearer (required=True)
- **Path params:** `orderId`
- **Request body example:**
```json
{
  "slot_index": 3
}
```
- **Response 200:** Finalize vend success
```json
{
  "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "order_status": "completed",
  "vend_state": "success"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body order_id→orderId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/commerce/orders/{orderId}/vend/success

| POST | `/v1/device/machines/{machineId}/commands/poll` | 09_Machines_Telemetry | bearer | yes | `{"items": [{"command_type": "machine_planogram_publish", "correlation_id": "11111111-2222-3333-4444-555555555555", "idem` |

### POST /v1/device/machines/{machineId}/commands/poll

- **Purpose:** Poll pending remote commands over HTTP (MQTT fallback)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "limit": 10
}
```
- **Response 200:** Poll pending remote commands over HTTP (MQTT fallback)
```json
{
  "items": [
    {
      "command_type": "machine_planogram_publish",
      "correlation_id": "11111111-2222-3333-4444-555555555555",
      "idempotency_key": "example",
      "payload": {
        "desiredConfigVersion": 7,
        "planogramId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
        "planogramRevision": 3
      },
      "sequence": 42
    }
  ],
  "meta": {
    "returned": 1
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId, response body command_id→commandId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/device/machines/{machineId}/commands/poll

| POST | `/v1/device/machines/{machineId}/events/reconcile` | 09_Machines_Telemetry | bearer | yes | `{"items": [{"acceptedAt": "2026-04-24T00:00:00Z", "eventType": "events.vend", "idempotencyKey": "machine-001:boot-202604` |

### POST /v1/device/machines/{machineId}/events/reconcile

- **Purpose:** Batch reconcile critical telemetry idempotency keys
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "idempotencyKeys": [
    "machine-001:boot-20260424:seq-100:events.vend"
  ]
}
```
- **Response 200:** Batch reconcile critical telemetry idempotency keys
```json
{
  "items": [
    {
      "acceptedAt": "2026-04-24T00:00:00Z",
      "eventType": "events.vend",
      "idempotencyKey": "machine-001:boot-20260424:seq-100:events.vend",
      "processedAt": "2026-04-24T00:00:10Z",
      "retryable": false,
      "status": "processed"
    }
  ],
  "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_batch_size",
    "details": {},
    "message": "idempotencyKeys must contain 1 to 500 entries",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/device/machines/{machineId}/events/reconcile

| GET | `/v1/device/machines/{machineId}/events/{idempotencyKey}/status` | 09_Machines_Telemetry | bearer | no | `{"acceptedAt": null, "eventType": null, "idempotencyKey": "machine-001:boot-20260424:seq-100:events.vend", "processedAt"` |

### GET /v1/device/machines/{machineId}/events/{idempotencyKey}/status

- **Purpose:** Single critical telemetry idempotency status
- **Auth:** bearer (required=True)
- **Path params:** `machineId, idempotencyKey`
- **Response 200:** Single critical telemetry idempotency status
```json
{
  "acceptedAt": null,
  "eventType": null,
  "idempotencyKey": "machine-001:boot-20260424:seq-100:events.vend",
  "processedAt": null,
  "retryable": true,
  "status": "not_found"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/device/machines/{machineId}/events/{idempotencyKey}/status

| POST | `/v1/device/machines/{machineId}/vend-results` | 09_Machines_Telemetry | bearer | yes | `{"order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6", "order_status": "completed", "replay": false, "vend_state": "succes` |

### POST /v1/device/machines/{machineId}/vend-results

- **Purpose:** Report vend outcome for an order (HTTP bridge)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "correlation_id": "11111111-2222-3333-4444-555555555555",
  "order_id": "{{orderId}}",
  "outcome": "success",
  "slot_index": 3
}
```
- **Response 200:** Report vend outcome for an order (HTTP bridge)
```json
{
  "order_id": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
  "order_status": "completed",
  "replay": false,
  "vend_state": "success"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/device/machines/{machineId}/vend-results

| POST | `/v1/machines/{machineId}/check-ins` | 08_Machines_Runtime_Config | bearer | yes | `{"id": "12001", "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "occurred_at": "2026-04-19T12:00:00.000000000Z"}` |

### POST /v1/machines/{machineId}/check-ins

- **Purpose:** Record Android check-in
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "android_release": "14",
  "boot_id": "boot-session-1",
  "manufacturer": "Example",
  "metadata": {},
  "model": "Kiosk-1",
  "network_state": "wifi",
  "occurred_at": "2026-04-19T12:00:00Z",
  "package_name": "com.example.kiosk",
  "sdk_int": 34,
  "timezone": "America/Los_Angeles",
  "version_code": 100,
  "version_name": "1.0.0"
}
```
- **Response 201:** Record Android check-in
```json
{
  "id": "12001",
  "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "occurred_at": "2026-04-19T12:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/machines/{machineId}/check-ins

| POST | `/v1/machines/{machineId}/commands/dispatch` | 08_Machines_Runtime_Config | bearer | yes | `{"attempt_id": "cccccccc-dddd-eeee-ffff-000000000001", "command_id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff", "dispatch_s` |

### POST /v1/machines/{machineId}/commands/dispatch

- **Purpose:** Dispatch remote MQTT command
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "command_type": "SET_TEMPERATURE",
  "desired_state": {},
  "payload": {
    "celsius": 4
  }
}
```
- **Response 200:** Dispatch remote MQTT command
```json
{
  "attempt_id": "cccccccc-dddd-eeee-ffff-000000000001",
  "command_id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "dispatch_state": "published",
  "replay": false,
  "sequence": 42
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId, response body command_id→commandId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/machines/{machineId}/commands/dispatch

| GET | `/v1/machines/{machineId}/commands/receipts` | 08_Machines_Runtime_Config | bearer | no | `{"items": [], "meta": {"limit": 50, "returned": 0}}` |

### GET /v1/machines/{machineId}/commands/receipts

- **Purpose:** List recent command receipts
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Query params:** `limit`
- **Response 200:** List recent command receipts
```json
{
  "items": [],
  "meta": {
    "limit": 50,
    "returned": 0
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId, response body command_id→commandId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/machines/{machineId}/commands/receipts

| GET | `/v1/machines/{machineId}/commands/{sequence}/status` | 08_Machines_Runtime_Config | bearer | no | `{"attempt": {"ack_deadline_at": "2026-04-19T12:00:40Z", "attempt_no": 1, "id": "cccccccc-dddd-eeee-ffff-000000000001", "` |

### GET /v1/machines/{machineId}/commands/{sequence}/status

- **Purpose:** Get command dispatch status by sequence
- **Auth:** bearer (required=True)
- **Path params:** `machineId, sequence`
- **Response 200:** Get command dispatch status by sequence
```json
{
  "attempt": {
    "ack_deadline_at": "2026-04-19T12:00:40Z",
    "attempt_no": 1,
    "id": "cccccccc-dddd-eeee-ffff-000000000001",
    "sent_at": "2026-04-19T12:00:10Z",
    "status": "sent"
  },
  "command_id": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
  "command_type": "SET_TEMPERATURE",
  "dispatch_state": "published",
  "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "sequence": 42
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId, response body command_id→commandId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/machines/{machineId}/commands/{sequence}/status

| POST | `/v1/machines/{machineId}/config-applies` | 08_Machines_Runtime_Config | bearer | yes | `{"applied_at": "2026-04-19T12:05:00.000000000Z", "config_revision": 7, "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "ma` |

### POST /v1/machines/{machineId}/config-applies

- **Purpose:** Acknowledge config applied on device
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "android_id": "device-android-1",
  "app_version": "1.0.0",
  "applied_at": "2026-04-19T12:05:00Z",
  "config_payload": {
    "applied_revision": 7
  },
  "config_version": 7
}
```
- **Response 201:** Acknowledge config applied on device
```json
{
  "applied_at": "2026-04-19T12:05:00.000000000Z",
  "config_revision": 7,
  "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
  "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/machines/{machineId}/config-applies

| GET | `/v1/machines/{machineId}/operator-sessions/action-attributions` | 08_Machines_Runtime_Config | bearer | no | `{"items": [], "meta": {"limit": 50, "returned": 0}}` |

### GET /v1/machines/{machineId}/operator-sessions/action-attributions

- **Purpose:** List action attributions for machine
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Query params:** `limit`
- **Response 200:** List action attributions for machine
```json
{
  "items": [],
  "meta": {
    "limit": 50,
    "returned": 0
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/machines/{machineId}/operator-sessions/action-attributions

| GET | `/v1/machines/{machineId}/operator-sessions/auth-events` | 08_Machines_Runtime_Config | bearer | no | `{"items": [], "meta": {"limit": 50, "returned": 0}}` |

### GET /v1/machines/{machineId}/operator-sessions/auth-events

- **Purpose:** List auth events
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Query params:** `limit`
- **Response 200:** List auth events
```json
{
  "items": [],
  "meta": {
    "limit": 50,
    "returned": 0
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/machines/{machineId}/operator-sessions/auth-events

| GET | `/v1/machines/{machineId}/operator-sessions/current` | 08_Machines_Runtime_Config | bearer | no | `{"active_session": null, "technician_display_name": ""}` |

### GET /v1/machines/{machineId}/operator-sessions/current

- **Purpose:** Get current operator session
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Get current operator session
```json
{
  "active_session": null,
  "technician_display_name": ""
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/machines/{machineId}/operator-sessions/current

| GET | `/v1/machines/{machineId}/operator-sessions/history` | 08_Machines_Runtime_Config | bearer | no | `{"items": [], "meta": {"limit": 50, "returned": 0}}` |

### GET /v1/machines/{machineId}/operator-sessions/history

- **Purpose:** List session history
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Query params:** `limit`
- **Response 200:** List session history
```json
{
  "items": [],
  "meta": {
    "limit": 50,
    "returned": 0
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/machines/{machineId}/operator-sessions/history

| POST | `/v1/machines/{machineId}/operator-sessions/login` | 08_Machines_Runtime_Config | bearer | yes | `{"session": {"actor_type": "TECHNICIAN", "client_metadata": {}, "created_at": "2026-04-19T12:10:00Z", "id": "dddddddd-ee` |

### POST /v1/machines/{machineId}/operator-sessions/login

- **Purpose:** Start or resume operator session
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "auth_method": "oidc",
  "client_metadata": {
    "kiosk": "A12"
  }
}
```
- **Response 200:** Start or resume operator session
```json
{
  "session": {
    "actor_type": "TECHNICIAN",
    "client_metadata": {},
    "created_at": "2026-04-19T12:10:00Z",
    "id": "dddddddd-eeee-ffff-0000-111111111111",
    "last_activity_at": "2026-04-19T12:10:05Z",
    "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "started_at": "2026-04-19T12:10:00Z",
    "status": "ACTIVE",
    "technician_id": "eeeeeeee-ffff-0000-1111-222222222222",
    "updated_at": "2026-04-19T12:10:05Z"
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/machines/{machineId}/operator-sessions/login

| POST | `/v1/machines/{machineId}/operator-sessions/logout` | 08_Machines_Runtime_Config | bearer | yes | `{"session": {"actor_type": "TECHNICIAN", "client_metadata": {}, "created_at": "2026-04-19T12:10:00Z", "id": "dddddddd-ee` |

### POST /v1/machines/{machineId}/operator-sessions/logout

- **Purpose:** End operator session
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Request body example:**
```json
{
  "auth_method": "oidc",
  "ended_reason": "user_logout",
  "session_id": "{{operatorSessionId}}"
}
```
- **Response 200:** End operator session
```json
{
  "session": {
    "actor_type": "TECHNICIAN",
    "client_metadata": {},
    "created_at": "2026-04-19T12:10:00Z",
    "id": "dddddddd-eeee-ffff-0000-111111111111",
    "last_activity_at": "2026-04-19T12:10:05Z",
    "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "started_at": "2026-04-19T12:10:00Z",
    "status": "ACTIVE",
    "technician_id": "eeeeeeee-ffff-0000-1111-222222222222",
    "updated_at": "2026-04-19T12:10:05Z"
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/machines/{machineId}/operator-sessions/logout

| GET | `/v1/machines/{machineId}/operator-sessions/timeline` | 08_Machines_Runtime_Config | bearer | no | `{"items": [], "meta": {"limit": 50, "returned": 0}}` |

### GET /v1/machines/{machineId}/operator-sessions/timeline

- **Purpose:** Combined operator timeline
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Query params:** `limit`
- **Response 200:** Combined operator timeline
```json
{
  "items": [],
  "meta": {
    "limit": 50,
    "returned": 0
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/machines/{machineId}/operator-sessions/timeline

| POST | `/v1/machines/{machineId}/operator-sessions/{sessionId}/heartbeat` | 08_Machines_Runtime_Config | bearer | no | `{"session": {"session": {"actor_type": "TECHNICIAN", "client_metadata": {}, "created_at": "2026-04-19T12:10:00Z", "id": ` |

### POST /v1/machines/{machineId}/operator-sessions/{sessionId}/heartbeat

- **Purpose:** Session activity heartbeat
- **Auth:** bearer (required=True)
- **Path params:** `machineId, sessionId`
- **Response 200:** Session activity heartbeat
```json
{
  "session": {
    "session": {
      "actor_type": "TECHNICIAN",
      "client_metadata": {},
      "created_at": "2026-04-19T12:10:00Z",
      "id": "dddddddd-eeee-ffff-0000-111111111111",
      "last_activity_at": "2026-04-19T12:10:05Z",
      "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "started_at": "2026-04-19T12:10:00Z",
      "status": "ACTIVE",
      "technician_id": "eeeeeeee-ffff-0000-1111-222222222222",
      "updated_at": "2026-04-19T12:10:05Z"
    }
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/machines/{machineId}/operator-sessions/{sessionId}/heartbeat

| GET | `/v1/machines/{machineId}/sale-catalog` | 08_Machines_Runtime_Config | bearer | no | `{"configVersion": 7, "currency": "VND", "generatedAt": "2026-04-24T00:00:00Z", "items": [{"availableQuantity": 8, "cabin` |

### GET /v1/machines/{machineId}/sale-catalog

- **Purpose:** Runtime sale catalog (planogram, price, stock, images)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Query params:** `if_none_match_config_version, include_unavailable, include_images`
- **Response 200:** Runtime sale catalog (planogram, price, stock, images)
```json
{
  "configVersion": 7,
  "currency": "VND",
  "generatedAt": "2026-04-24T00:00:00Z",
  "items": [
    {
      "availableQuantity": 8,
      "cabinetCode": "A",
      "image": {
        "contentHash": "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
        "displayUrl": "https://cdn.example.com/products/coca330-display.webp",
        "thumbUrl": "https://cdn.example.com/products/coca330-thumb.webp",
        "updatedAt": "2026-04-24T00:00:00Z"
      },
      "isAvailable": true,
      "maxQuantity": 12,
      "name": "Coca Cola 330ml",
      "priceMinor": 15000,
      "productId": "22222222-3333-4444-5555-666666666666",
      "shortName": "Coca 330",
      "sku": "COCA330",
      "slotCode": "A3",
      "slotIndex": 3,
      "sortOrder": 10,
      "unavailableReason": null
    }
  ],
  "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "siteId": "11111111-2222-3333-4444-555555555555"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/machines/{machineId}/sale-catalog

| GET | `/v1/machines/{machineId}/shadow` | 08_Machines_Runtime_Config | bearer | no | `{"desired": {"temperature_c": 4.0}, "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7", "metadata": {"version": 12}, "` |

### GET /v1/machines/{machineId}/shadow

- **Purpose:** Get machine shadow JSON
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Get machine shadow JSON
```json
{
  "desired": {
    "temperature_c": 4.0
  },
  "machine_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "metadata": {
    "version": 12
  },
  "reported": {
    "temperature_c": 4.5
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/machines/{machineId}/shadow

| GET | `/v1/machines/{machineId}/telemetry/incidents` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"code": "TEMP_HIGH", "dedupeKey": "TEMP_HIGH:slot3", "detail": {"threshold_c": 8}, "id": "aaaaaaaa-bbbb-cccc` |

### GET /v1/machines/{machineId}/telemetry/incidents

- **Purpose:** Recent persisted machine incidents
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Query params:** `limit`
- **Response 200:** Recent persisted machine incidents
```json
{
  "items": [
    {
      "code": "TEMP_HIGH",
      "dedupeKey": "TEMP_HIGH:slot3",
      "detail": {
        "threshold_c": 8
      },
      "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "openedAt": "2026-04-19T12:00:00.000000000Z",
      "severity": "warning",
      "title": "Cabinet warm",
      "updatedAt": "2026-04-19T12:05:00.000000000Z"
    }
  ],
  "meta": {
    "limit": 50,
    "returned": 1
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/machines/{machineId}/telemetry/incidents

| GET | `/v1/machines/{machineId}/telemetry/rollups` | 08_Machines_Runtime_Config | bearer | no | `{"items": [{"bucketStart": "2026-04-19T12:00:00.000000000Z", "extra": {}, "granularity": "1m", "last": 7.1, "max": 8.2, ` |

### GET /v1/machines/{machineId}/telemetry/rollups

- **Purpose:** Telemetry rollup buckets (1m / 1h)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Query params:** `from, to, granularity`
- **Response 200:** Telemetry rollup buckets (1m / 1h)
```json
{
  "items": [
    {
      "bucketStart": "2026-04-19T12:00:00.000000000Z",
      "extra": {},
      "granularity": "1m",
      "last": 7.1,
      "max": 8.2,
      "metricKey": "temperature_c",
      "min": 6.5,
      "sampleCount": 60,
      "sum": 420.5
    }
  ],
  "meta": {
    "from": "2026-04-18T12:00:00.000000000Z",
    "granularity": "1m",
    "note": "Rollup buckets only — not raw MQTT telemetry history.",
    "returned": 1,
    "to": "2026-04-19T12:00:00.000000000Z"
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/machines/{machineId}/telemetry/rollups

| GET | `/v1/machines/{machineId}/telemetry/snapshot` | 08_Machines_Runtime_Config | bearer | no | `{"androidId": "dev123", "appVersion": "1.2.3", "deviceModel": "Pixel", "effectiveTimezone": "America/Los_Angeles", "firm` |

### GET /v1/machines/{machineId}/telemetry/snapshot

- **Purpose:** Current machine telemetry snapshot (projected)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Current machine telemetry snapshot (projected)
```json
{
  "androidId": "dev123",
  "appVersion": "1.2.3",
  "deviceModel": "Pixel",
  "effectiveTimezone": "America/Los_Angeles",
  "firmwareVersion": "fw-9",
  "lastHeartbeatAt": "2026-04-19T12:34:56.789012345Z",
  "lastIdentityAt": "2026-04-19T12:30:00.111111111Z",
  "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "metricsState": {
    "cpu_pct": 12.3
  },
  "osVersion": "14",
  "reportedState": {
    "temperature_c": 4.5
  },
  "simIccid": "89012601234567890123",
  "simSerial": "89012601234567890123",
  "siteId": "11111111-2222-3333-4444-555555555555",
  "updatedAt": "2026-04-19T12:35:00.000000001Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/machines/{machineId}/telemetry/snapshot

| GET | `/v1/operator-insights/technicians/{technicianId}/action-attributions` | 99_Utilities | bearer | no | `{"items": [], "meta": {"limit": 50, "returned": 0}}` |

### GET /v1/operator-insights/technicians/{technicianId}/action-attributions

- **Purpose:** List action attributions for a technician
- **Auth:** bearer (required=True)
- **Path params:** `technicianId`
- **Query params:** `limit`
- **Response 200:** List action attributions for a technician
```json
{
  "items": [],
  "meta": {
    "limit": 50,
    "returned": 0
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/operator-insights/technicians/{technicianId}/action-attributions

| GET | `/v1/operator-insights/users/action-attributions` | 99_Utilities | bearer | no | `{"items": [], "meta": {"limit": 50, "returned": 0}}` |

### GET /v1/operator-insights/users/action-attributions

- **Purpose:** List action attributions for a user principal
- **Auth:** bearer (required=True)
- **Query params:** `user_principal, limit`
- **Response 200:** List action attributions for a user principal
```json
{
  "items": [],
  "meta": {
    "limit": 50,
    "returned": 0
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/operator-insights/users/action-attributions

| GET | `/v1/orders` | 12_Orders | bearer | no | `{"items": [{"createdAt": "2026-04-19T12:00:00Z", "currency": "USD", "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",` |

### GET /v1/orders

- **Purpose:** List orders for company
- **Auth:** bearer (required=True)
- **Query params:** `status, machine_id, search, from, to, limit, offset`
- **Response 200:** List orders for company
```json
{
  "items": [
    {
      "createdAt": "2026-04-19T12:00:00Z",
      "currency": "USD",
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "orderId": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "status": "paid",
      "subtotalMinor": 100,
      "taxMinor": 0,
      "totalMinor": 100,
      "updatedAt": "2026-04-19T12:05:00Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body order_id→orderId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/orders

| GET | `/v1/payments` | 12_Orders | bearer | no | `{"items": [{"amountMinor": 100, "createdAt": "2026-04-19T12:04:00Z", "currency": "USD", "machineId": "7c9e6679-7425-40de` |

### GET /v1/payments

- **Purpose:** List payments for company
- **Auth:** bearer (required=True)
- **Query params:** `status, payment_method, machine_id, search, from, to, limit, offset`
- **Response 200:** List payments for company
```json
{
  "items": [
    {
      "amountMinor": 100,
      "createdAt": "2026-04-19T12:04:00Z",
      "currency": "USD",
      "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
      "orderId": "3fa85f64-5717-4562-b3fc-2c963f66afa6",
      "orderStatus": "paid",
      "paymentId": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
      "paymentState": "captured",
      "provider": "stripe",
      "reconciliationStatus": "pending",
      "settlementStatus": "unsettled",
      "updatedAt": "2026-04-19T12:04:01Z"
    }
  ],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 1,
    "total": 42
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/payments

| GET | `/v1/reports/fleet-health` | 16_Finance_Reconciliation | bearer | no | `{"from": "2026-04-01T00:00:00Z", "incidentsByStatus": [], "machineIncidentsBySeverity": [], "machineSummary": {"fault": ` |

### GET /v1/reports/fleet-health

- **Purpose:** Machine posture and incident rollups
- **Auth:** bearer (required=True)
- **Query params:** `from, to`
- **Response 200:** Machine posture and incident rollups
```json
{
  "from": "2026-04-01T00:00:00Z",
  "incidentsByStatus": [],
  "machineIncidentsBySeverity": [],
  "machineSummary": {
    "fault": 1,
    "offline": 2,
    "online": 22,
    "retired": 0,
    "total": 25,
    "warn": 0
  },
  "machinesByStatus": [
    {
      "count": 22,
      "status": "online"
    }
  ],
  "to": "2026-04-20T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/reports/fleet-health

| GET | `/v1/reports/inventory-exceptions` | 10_Inventory | bearer | no | `{"exceptionKind": "low_stock", "from": "2026-04-01T00:00:00.000000000Z", "items": [], "meta": {"limit": 50, "offset": 0,` |

### GET /v1/reports/inventory-exceptions

- **Purpose:** Slots needing refill or restock attention
- **Auth:** bearer (required=True)
- **Query params:** `from, to, exception_kind, limit, offset`
- **Response 200:** Slots needing refill or restock attention
```json
{
  "exceptionKind": "low_stock",
  "from": "2026-04-01T00:00:00.000000000Z",
  "items": [],
  "meta": {
    "limit": 50,
    "offset": 0,
    "returned": 0,
    "total": 0
  },
  "to": "2026-04-20T00:00:00.000000000Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/reports/inventory-exceptions

| GET | `/v1/reports/payments-summary` | 13_Payments | bearer | no | `{"breakdown": [], "from": "2026-04-01T00:00:00Z", "groupBy": "day", "summary": {"authorizedAmountMinor": 10200, "authori` |

### GET /v1/reports/payments-summary

- **Purpose:** Payment outcomes and method/status breakdown
- **Auth:** bearer (required=True)
- **Query params:** `from, to, group_by`
- **Response 200:** Payment outcomes and method/status breakdown
```json
{
  "breakdown": [],
  "from": "2026-04-01T00:00:00Z",
  "groupBy": "day",
  "summary": {
    "authorizedAmountMinor": 10200,
    "authorizedCount": 10,
    "capturedAmountMinor": 10000,
    "capturedCount": 48,
    "failedAmountMinor": 400,
    "failedCount": 2,
    "refundedAmountMinor": 0,
    "refundedCount": 0
  },
  "to": "2026-04-20T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/reports/payments-summary

| GET | `/v1/reports/sales-summary` | 16_Finance_Reconciliation | bearer | no | `{"breakdown": [], "from": "2026-04-01T00:00:00Z", "groupBy": "day", "summary": {"avgOrderValueMinor": 200, "grossTotalMi` |

### GET /v1/reports/sales-summary

- **Purpose:** Sales rollup and trend breakdown
- **Auth:** bearer (required=True)
- **Query params:** `from, to, group_by`
- **Response 200:** Sales rollup and trend breakdown
```json
{
  "breakdown": [],
  "from": "2026-04-01T00:00:00Z",
  "groupBy": "day",
  "summary": {
    "avgOrderValueMinor": 200,
    "grossTotalMinor": 10000,
    "orderCount": 50,
    "subtotalMinor": 9000,
    "taxMinor": 1000
  },
  "to": "2026-04-20T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/reports/sales-summary

| POST | `/v1/setup/activation-codes/claim` | 07_Machines_Provisioning | none | yes | `{"bootstrapUrl": "/v1/setup/machines/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee/bootstrap", "machineId": "7c9e6679-7425-40de-9` |

### POST /v1/setup/activation-codes/claim

- **Purpose:** Claim an activation code (public pre-auth)
- **Auth:** none (required=False)
- **Request body example:**
```json
{
  "activationCode": "{{activationCode}}",
  "deviceFingerprint": {
    "androidId": "android-123",
    "manufacturer": "SUNMI",
    "model": "K2",
    "packageName": "com.avf.vending",
    "serialNumber": "SN-001",
    "versionCode": 100,
    "versionName": "1.0.0"
  }
}
```
- **Response 200:** Claim an activation code (public pre-auth)
```json
{
  "bootstrapUrl": "/v1/setup/machines/aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee/bootstrap",
  "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "machineName": "Lobby A",
  "machineToken": "<jwt>",
  "mqtt": {
    "brokerUrl": "ssl://mqtt.example.com:8883",
    "topicPrefix": "avf/devices"
  },
  "siteId": "11111111-2222-3333-4444-555555555555",
  "tokenExpiresAt": "2026-04-24T00:00:00Z"
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "activation_invalid",
    "details": {},
    "message": "activation code is not valid",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 500:** error
```json
{
  "error": {
    "code": "internal",
    "details": {},
    "message": "unexpected server error",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Source:** docs/swagger/swagger.json, openapi:POST /v1/setup/activation-codes/claim

| GET | `/v1/setup/machines/{machineId}/bootstrap` | 07_Machines_Provisioning | bearer | no | `{"catalog": {"products": [{"assortmentId": "dddddddd-eeee-ffff-0000-111111111111", "assortmentName": "Standard", "name":` |

### GET /v1/setup/machines/{machineId}/bootstrap

- **Purpose:** Machine setup bootstrap (topology + catalog)
- **Auth:** bearer (required=True)
- **Path params:** `machineId`
- **Response 200:** Machine setup bootstrap (topology + catalog)
```json
{
  "catalog": {
    "products": [
      {
        "assortmentId": "dddddddd-eeee-ffff-0000-111111111111",
        "assortmentName": "Standard",
        "name": "Cola 12oz",
        "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
        "sku": "COLA-12",
        "sortOrder": 1
      }
    ]
  },
  "machine": {
    "commandSequence": 42,
    "createdAt": "2026-01-01T00:00:00.000000000Z",
    "machineId": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "name": "Lobby A",
    "serialNumber": "SN-LOBBY-001",
    "siteId": "11111111-2222-3333-4444-555555555555",
    "status": "online",
    "updatedAt": "2026-04-19T12:00:00.000000000Z"
  },
  "topology": {
    "cabinets": [
      {
        "code": "A",
        "id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
        "slots": [
          {
            "configId": "bbbbbbbb-cccc-dddd-eeee-ffffffffffff",
            "effectiveFrom": "2026-04-01T00:00:00.000000000Z",
            "isCurrent": true,
            "machineSlotLayout": "cccccccc-dddd-eeee-ffff-000000000001",
            "maxQuantity": 10,
            "metadata": {},
            "priceMinor": 150,
            "productId": "9f1e2d3c-aaaa-bbbb-cccc-ddddeeeeffff",
            "productName": "Cola 12oz",
            "productSku": "COLA-12",
            "slotCode": "A1",
            "slotIndex": 1
          }
        ],
        "sortOrder": 1,
        "title": "Main"
      }
    ]
  }
}
```
- **Response 400:** error
```json
{
  "error": {
    "code": "invalid_request",
    "details": {},
    "message": "request could not be validated",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Response 401:** error
```json
{
  "error": {
    "code": "unauthenticated",
    "details": {},
    "message": "missing or invalid bearer token",
    "requestId": "01ARZ3NDEKTSV4RRFFQ69G5FAV"
  }
}
```
- **Captures:** response body machine_id→machineId
- **Source:** docs/swagger/swagger.json, openapi:GET /v1/setup/machines/{machineId}/bootstrap

| GET | `/version` | 00_Health_System | none | no | `{"app_env": "development", "build_time": "2026-04-19T12:00:00Z", "git_sha": "abc123", "name": "avf-vending-api", "proces` |

### GET /version

- **Purpose:** Build and runtime version
- **Auth:** none (required=False)
- **Response 200:** Build and runtime version
```json
{
  "app_env": "development",
  "build_time": "2026-04-19T12:00:00Z",
  "git_sha": "abc123",
  "name": "avf-vending-api",
  "process": "api",
  "version": "0.0.0-dev"
}
```
- **Source:** docs/swagger/swagger.json, openapi:GET /version
