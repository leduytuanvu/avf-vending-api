# AVF Production MQTT requests

Broker: `mqtts://{{mqttHost}}:{{mqttPort}}` TLS. Credentials: fill locally (never commit).

Topic layout (enterprise): `avf/prod/machines/{machineId}/...` — see `tests/e2e/production/lib/mqtt_common.sh`.

| Flow ID | Direction | Topic pattern | QoS |
|---------|-----------|---------------|-----|
| MQTT-CONN-001 | pub/sub | commands/ack/telemetry per mqtt_common.sh | 1 |
| MQTT-CONN-002 | pub/sub | commands/ack/telemetry per mqtt_common.sh | 1 |
| MQTT-CMD-001 | pub/sub | commands/ack/telemetry per mqtt_common.sh | 1 |
| MQTT-TEL-001 | pub/sub | `{prefix}/machines/{machineId}/state/heartbeat` | 1 |
| MQTT-TEL-002 | pub/sub | `{prefix}/machines/{machineId}/presence` | 1 |
| MQTT-TEL-003 | pub/sub | `{prefix}/machines/{machineId}/telemetry/snapshot` | 1 |
| MQTT-TEL-004 | pub/sub | `{prefix}/machines/{machineId}/events/inventory` | 1 |
| MQTT-READ-001 | pub/sub | commands/ack/telemetry per mqtt_common.sh | 1 |
| MQTT-NEG-001 | pub/sub | commands/ack/telemetry per mqtt_common.sh | 1 |
| MQTT-NEG-002 | pub/sub | commands/ack/telemetry per mqtt_common.sh | 1 |
| MQTT-NEG-003 | pub/sub | commands/ack/telemetry per mqtt_common.sh | 1 |
| MQTT-NEG-004 | pub/sub | commands/ack/telemetry per mqtt_common.sh | 1 |

```bash
mosquitto_pub -h mqtt.ldtv.dev -p 8883 --capath /etc/ssl/certs \
  -u "$E2E_PROD_MQTT_USERNAME" -P "$E2E_PROD_MQTT_PASSWORD" \
  -t 'avf/prod/machines/$MACHINE_ID/state/heartbeat' -m '{}' -q 1
```