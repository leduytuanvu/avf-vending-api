# Canonical REST API surface

AVF keeps legacy aliases mounted for compatibility. **Do not remove** routes until usage is zero and sunset is announced.

| Domain | Canonical API | Legacy / alias | Audience | Reason | Status | Removal policy |
|--------|---------------|----------------|----------|--------|--------|----------------|
| Admin users | `/v1/admin/auth/users/*` (`accountId`) | `/v1/admin/users/*` (`userId`) | Web admin | Historical path param name | legacy | No removal until metrics show zero |
| User roles | `PUT .../roles` | `POST`/`PATCH .../roles` | Web admin | HTTP verb aliases | legacy verbs | Same |
| Media assets | `/v1/admin/media/assets/*` | `/v1/admin/media`, `/v1/admin/media/{id}` | Web admin | Flat media tree | legacy | Same |
| Media upload | `/v1/admin/media/uploads/init`, `POST /media/assets` | `/v1/admin/media/uploads`, legacy complete paths | Web admin | Upload pipeline versions | legacy | Same |
| Product media | `/v1/admin/products/{productId}/media` | `/v1/admin/products/{productId}/image` | Web admin | Image bind vs media link | legacy | Same |
| Machine runtime | gRPC `avf.machine.v1` + MQTT enterprise | `/v1/setup`, `/v1/commerce`, `/v1/device`, sale-catalog, shadow, telemetry REST | Vending app | Transport boundary | legacy (off in prod by default) | Enable only with explicit legacy flag |
| Command dispatch | `POST /v1/admin/machines/{machineId}/commands` + MQTT `.../machines/{id}/commands` | `POST /v1/machines/{machineId}/commands/dispatch`, MQTT `commands/dispatch` | Admin + machine | Fleet vs device path | legacy | Same |

## Response headers

Legacy aliases may return:

- `Deprecation: true`
- `Link: </v1/admin/auth/users>; rel="successor-version"` (path template)

## Postman

Production enterprise collection places legacy flows under **11 - Legacy and Compatibility**.

## OpenAPI

Legacy operations remain documented; prefer canonical paths in new integrations.
