# gRPC & MQTT manual collection guide

REST is importable as Postman Collection v2.1. **gRPC and MQTT are not faked in Newman JSON** — use:

- Postman Desktop native gRPC/MQTT request types, or
- `grpcurl` / `mosquitto_pub` / `mosquitto_sub` (commands in sibling markdown files)

## gRPC

1. Server: `machine-api.ldtv.dev:443` TLS + SNI
2. Import protos from `proto/avf/machine/v1/`
3. Metadata: `authorization: Bearer {{machineAccessToken}}`
4. Follow flow order in `AVF_PRODUCTION_GRPC_REQUESTS.md` (token refresh → bootstrap → catalog → media → inventory → cash commerce)

## MQTT

1. Broker: `mqtts://mqtt.ldtv.dev:8883`
2. Subscribe command topic before dispatching REST `catalog.refresh`
3. Publish ACK on `.../commands/ack`
4. See `AVF_PRODUCTION_MQTT_REQUESTS.md`
