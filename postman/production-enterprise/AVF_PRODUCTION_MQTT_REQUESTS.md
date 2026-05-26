# AVF Production MQTT requests

Broker: `mqtts://{{mqttHost}}:{{mqttPort}}` TLS. Credentials: fill locally (never commit).

Topic layout (enterprise): `{{mqttTopicPrefix}}/machines/{{machineId}}/...` — see `internal/platform/mqtt/topics.go`.

## Canonical topic catalog (production enterprise layout)

| Rel topic | Direction | Actor | E2E | Pattern |
|-----------|-----------|-------|-----|---------|
| `presence` | publish | device | YES | `{{mqttTopicPrefix}}/machines/{{machineId}}/presence` |
| `state/heartbeat` | publish | device | YES | `{{mqttTopicPrefix}}/machines/{{machineId}}/state/heartbeat` |
| `telemetry/snapshot` | publish | device | YES | `{{mqttTopicPrefix}}/machines/{{machineId}}/telemetry/snapshot` |
| `telemetry/incident` | publish | device | YES | `{{mqttTopicPrefix}}/machines/{{machineId}}/telemetry/incident` |
| `events/vend` | publish | device | YES | `{{mqttTopicPrefix}}/machines/{{machineId}}/events/vend` |
| `events/cash` | publish | device | YES | `{{mqttTopicPrefix}}/machines/{{machineId}}/events/cash` |
| `events/inventory` | publish | device | YES | `{{mqttTopicPrefix}}/machines/{{machineId}}/events/inventory` |
| `commands/ack` | publish | device | YES | `{{mqttTopicPrefix}}/machines/{{machineId}}/commands/ack` |
| `commands/receipt` | publish | device | YES | `{{mqttTopicPrefix}}/machines/{{machineId}}/commands/receipt` |
| `shadow/reported` | publish | device | YES | `{{mqttTopicPrefix}}/machines/{{machineId}}/shadow/reported` |
| `shadow/desired` | publish | device | YES | `{{mqttTopicPrefix}}/machines/{{machineId}}/shadow/desired` |
| `commands` | subscribe | backend_outbound | YES | `{{mqttTopicPrefix}}/machines/{{machineId}}/commands` |
| `telemetry` | publish | device_legacy | YES | `{{mqttTopicPrefix}}/machines/{{machineId}}/telemetry` |

## E2E flows (mosquitto / Postman Desktop MQTT)

| Flow ID | Direction | Topic pattern | QoS |
|---------|-----------|---------------|-----|
| MQTT-CONN-001 | pub/sub | `{{mqttTopicPrefix}}/machines/{{machineId}}/telemetry` | 1 |
| MQTT-CONN-002 | pub/sub | `{{mqttTopicPrefix}}/machines/{{machineId}}/telemetry` | 1 |
| MQTT-CMD-001 | pub/sub | `{{mqttTopicPrefix}}/machines/{{machineId}}/commands + ack` | 1 |
| MQTT-TEL-001 | pub/sub | `{{mqttTopicPrefix}}/machines/{{machineId}}/state/heartbeat` | 1 |
| MQTT-TEL-002 | pub/sub | `{{mqttTopicPrefix}}/machines/{{machineId}}/presence` | 1 |
| MQTT-TEL-003 | pub/sub | `{{mqttTopicPrefix}}/machines/{{machineId}}/telemetry/snapshot` | 1 |
| MQTT-TEL-004 | pub/sub | `{{mqttTopicPrefix}}/machines/{{machineId}}/events/inventory` | 1 |
| MQTT-READ-001 | pub/sub | `{{mqttTopicPrefix}}/machines/{{machineId}}/telemetry` | 1 |
| MQTT-NEG-001 | pub/sub | `{{mqttTopicPrefix}}/machines/{{machineId}}/telemetry` | 1 |
| MQTT-NEG-002 | pub/sub | `{{mqttTopicPrefix}}/machines/{{machineId}}/telemetry` | 1 |
| MQTT-NEG-003 | pub/sub | `{{mqttTopicPrefix}}/machines/{{machineId}}/telemetry` | 1 |
| MQTT-NEG-004 | pub/sub | `{{mqttTopicPrefix}}/machines/{{machineId}}/telemetry` | 1 |

## mosquitto examples

```bash
mosquitto_pub -h mqtt.ldtv.dev -p 8883 --capath /etc/ssl/certs \
  -u "$E2E_PROD_MQTT_USERNAME" -P "$E2E_PROD_MQTT_PASSWORD" \
  -t 'avf/prod/machines/$MACHINE_ID/state/heartbeat' -m '{}' -q 1
```

```bash
mosquitto_pub -h mqtt.ldtv.dev -p 8883 --capath /etc/ssl/certs \
  -u "$E2E_PROD_MQTT_USERNAME" -P "$E2E_PROD_MQTT_PASSWORD" \
  -t 'avf/prod/machines/$MACHINE_ID/commands/ack' -m '{"command_id":"...","status":"completed"}' -q 1
```
- **presence** (publish, device): `{{mqttTopicPrefix}}/machines/{{machineId}}/presence`
- **state/heartbeat** (publish, device): `{{mqttTopicPrefix}}/machines/{{machineId}}/state/heartbeat`
- **telemetry/snapshot** (publish, device): `{{mqttTopicPrefix}}/machines/{{machineId}}/telemetry/snapshot`
- **telemetry/incident** (publish, device): `{{mqttTopicPrefix}}/machines/{{machineId}}/telemetry/incident`
- **events/vend** (publish, device): `{{mqttTopicPrefix}}/machines/{{machineId}}/events/vend`
- **events/cash** (publish, device): `{{mqttTopicPrefix}}/machines/{{machineId}}/events/cash`
- **events/inventory** (publish, device): `{{mqttTopicPrefix}}/machines/{{machineId}}/events/inventory`
- **commands/ack** (publish, device): `{{mqttTopicPrefix}}/machines/{{machineId}}/commands/ack`
- **commands/receipt** (publish, device): `{{mqttTopicPrefix}}/machines/{{machineId}}/commands/receipt`
- **shadow/reported** (publish, device): `{{mqttTopicPrefix}}/machines/{{machineId}}/shadow/reported`
- **shadow/desired** (publish, device): `{{mqttTopicPrefix}}/machines/{{machineId}}/shadow/desired`