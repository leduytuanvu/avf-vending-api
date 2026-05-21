# Postman Production Full Suite OpenAPI Parity

Generated: 2026-05-21

## Summary

| Check | Result |
| --- | --- |
| OpenAPI operations | 328 |
| Collection REST requests | 328 |
| Missing routes | 0 |
| Extra routes | 0 |
| Method/path match | PASS (normalized `{param}` vs `{{param}}`) |

## Notable routes verified

| Route | In collection | Notes |
| --- | --- | --- |
| `POST /v1/admin/media/uploads/init` | yes | 200/503 examples |
| `POST /v1/admin/media/external-images` | yes | 201/503 examples |
| `POST /v1/auth/login` | yes | captures `tokens.accessToken` |
| `GET /v1/auth/me` | yes | Bearer `{{accessToken}}` |

## Excluded

None — full OpenAPI REST surface included.

## gRPC / MQTT

Documented as manual folders under each domain (not counted in REST parity).
