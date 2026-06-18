# Duplicate surface cleanup plan

**Policy:** Mark legacy; do not remove unless proven unused + tested.

## REST

| Duplicate | Classification | Action |
|-----------|----------------|--------|
| `/v1/admin/users` vs `/v1/admin/auth/users` | alias | Keep both; document canonical auth path; Postman → Legacy folder |
| POST/PATCH vs PUT `.../roles` | verb alias | Keep; deprecation header on POST/PATCH when middleware present |
| `/v1/admin/media` vs `/media/assets` | alias | Keep; canonical assets |
| `/products/{id}/image` vs `/media` | overlapping | Keep; image=bind, media=attach |
| `/v1/machines/{id}/commands/dispatch` vs admin commands | legacy fleet | Keep; document MQTT enterprise canonical |
| Machine REST runtime (`/v1/commerce`, `/device`, sale-catalog) | legacy fallback | Off in production by default; gRPC canonical |

## gRPC

| Duplicate | Classification | Action |
|-----------|----------------|--------|
| `avf.v1` internal protos | dead registration | Keep protos; not mounted |
| `MachineAuthService` vs Activation+Token | facade | Keep; document |
| `MachineSaleService` vs `MachineCommerceService` | legacy_companion | Keep; prefer Commerce for vend |
| Catalog vs Media manifest RPCs | overlapping responsibility | Keep; document which RPC for which client version |

## MQTT

| Duplicate | Classification | Action |
|-----------|----------------|--------|
| legacy vs enterprise layout | compatibility | Subscribe both patterns in ingest |
| `commands/dispatch` vs `commands` | layout-specific | Document per `MQTT_TOPIC_LAYOUT` |
| `commands/receipt` vs `commands/ack` | alias | Normalize in router |
| generic `events` vs typed `events/vend` | compatibility | Require `event_type` on generic |

## Code

| Area | Action |
|------|--------|
| Pagination parse | Already shared helpers in httpserver — no change this pass |
| Idempotency | Centralized in commerce/grpc layers — add tests only |
| Media validation | Shared admin media service — document |

## Safe refactors completed / planned

- [x] Inventory generator (`tools/generate_market_readiness_inventory.py`)
- [x] Postman REST expected count aligned to OpenAPI **329** (was stale 327)
- [ ] Deprecation headers (on `perf/*` branch; merge to develop separately)
- [ ] Shared route metadata package (future, low priority)
