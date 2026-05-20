# AVF MQTT Inventory

| Topic | Direction | QoS | Folder |
| --- | --- | --- | --- |
| `{{MQTT_TOPIC_PREFIX}}/+/telemetry` | publish | 1 | Telemetry Publish |
| `{{MQTT_TOPIC_PREFIX}}/+/presence` | publish | 1 | Machine Heartbeat/Check-in |
| `{{MQTT_TOPIC_PREFIX}}/+/state/heartbeat` | publish | 1 | Machine Heartbeat/Check-in |
| `{{MQTT_TOPIC_PREFIX}}/+/telemetry/snapshot` | publish | 1 | Telemetry Publish |
| `{{MQTT_TOPIC_PREFIX}}/+/telemetry/incident` | publish | 1 | Telemetry Publish |
| `{{MQTT_TOPIC_PREFIX}}/+/events/vend` | publish | 1 | Telemetry Publish |
| `{{MQTT_TOPIC_PREFIX}}/+/events/cash` | publish | 1 | Telemetry Publish |
| `{{MQTT_TOPIC_PREFIX}}/+/events/inventory` | publish | 1 | Telemetry Publish |
| `{{MQTT_TOPIC_PREFIX}}/+/shadow/reported` | publish | 1 | Config/Shadow |
| `{{MQTT_TOPIC_PREFIX}}/+/shadow/desired` | publish | 1 | Config/Shadow |
| `{{MQTT_TOPIC_PREFIX}}/+/commands/receipt` | publish | 1 | Command ACK |
| `{{MQTT_TOPIC_PREFIX}}/+/commands/ack` | publish | 1 | Command ACK |
| `{{MQTT_TOPIC_PREFIX}}/machines/+/telemetry` | publish | 1 | Telemetry Publish |
| `{{MQTT_TOPIC_PREFIX}}/machines/+/presence` | publish | 1 | Machine Heartbeat/Check-in |
| `{{MQTT_TOPIC_PREFIX}}/machines/+/state/heartbeat` | publish | 1 | Machine Heartbeat/Check-in |
| `{{MQTT_TOPIC_PREFIX}}/machines/+/telemetry/snapshot` | publish | 1 | Telemetry Publish |
| `{{MQTT_TOPIC_PREFIX}}/machines/+/telemetry/incident` | publish | 1 | Telemetry Publish |
| `{{MQTT_TOPIC_PREFIX}}/machines/+/events/vend` | publish | 1 | Telemetry Publish |
| `{{MQTT_TOPIC_PREFIX}}/machines/+/events/cash` | publish | 1 | Telemetry Publish |
| `{{MQTT_TOPIC_PREFIX}}/machines/+/events/inventory` | publish | 1 | Telemetry Publish |
| `{{MQTT_TOPIC_PREFIX}}/machines/+/shadow/reported` | publish | 1 | Config/Shadow |
| `{{MQTT_TOPIC_PREFIX}}/machines/+/shadow/desired` | publish | 1 | Config/Shadow |
| `{{MQTT_TOPIC_PREFIX}}/machines/+/commands/receipt` | publish | 1 | Command ACK |
| `{{MQTT_TOPIC_PREFIX}}/machines/+/commands/ack` | publish | 1 | Command ACK |
| `{{MQTT_TOPIC_PREFIX}}/machines/+/events` | publish | 1 | Telemetry Publish |
| `{{MQTT_TOPIC_PREFIX}}/{{MACHINE_ID}}/commands/dispatch` | publish | 1 | Backend Commands |
| `{{MQTT_TOPIC_PREFIX}}/{{MACHINE_ID}}/commands/down` | publish | 1 | Backend Commands |
| `{{MQTT_TOPIC_PREFIX}}/machines/{{MACHINE_ID}}/commands` | publish | 1 | Backend Commands |
