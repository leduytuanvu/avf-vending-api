# AVF MQTT Request/Response Catalog

## mqtt-1-Telemetry-Publish

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/+/telemetry`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "telemetry",
  "payload": {}
}
```
**Expected ack topic:** `HTTP reconcile / application ack (không dùng PUBACK làm business ack)`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-2-Machine-Heartbeat-Check-in

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/+/presence`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "telemetry",
  "payload": {}
}
```
**Expected ack topic:** `HTTP reconcile / application ack (không dùng PUBACK làm business ack)`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-3-Machine-Heartbeat-Check-in

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/+/state/heartbeat`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "telemetry",
  "payload": {}
}
```
**Expected ack topic:** `HTTP reconcile / application ack (không dùng PUBACK làm business ack)`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-4-Telemetry-Publish

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/+/telemetry/snapshot`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "telemetry",
  "payload": {}
}
```
**Expected ack topic:** `HTTP reconcile / application ack (không dùng PUBACK làm business ack)`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-5-Telemetry-Publish

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/+/telemetry/incident`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "telemetry",
  "payload": {}
}
```
**Expected ack topic:** `HTTP reconcile / application ack (không dùng PUBACK làm business ack)`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-6-Telemetry-Publish

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/+/events/vend`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "telemetry",
  "payload": {}
}
```
**Expected ack topic:** `HTTP reconcile / application ack (không dùng PUBACK làm business ack)`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-7-Telemetry-Publish

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/+/events/cash`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "telemetry",
  "payload": {}
}
```
**Expected ack topic:** `HTTP reconcile / application ack (không dùng PUBACK làm business ack)`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-8-Telemetry-Publish

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/+/events/inventory`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "telemetry",
  "payload": {}
}
```
**Expected ack topic:** `HTTP reconcile / application ack (không dùng PUBACK làm business ack)`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-9-Config-Shadow

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/+/shadow/reported`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "telemetry",
  "payload": {}
}
```
**Expected ack topic:** `HTTP reconcile / application ack (không dùng PUBACK làm business ack)`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-10-Config-Shadow

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/+/shadow/desired`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "telemetry",
  "payload": {}
}
```
**Expected ack topic:** `HTTP reconcile / application ack (không dùng PUBACK làm business ack)`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-11-Command-ACK

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/+/commands/receipt`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "telemetry",
  "payload": {}
}
```
**Expected ack topic:** `HTTP reconcile / application ack (không dùng PUBACK làm business ack)`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-12-Command-ACK

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/+/commands/ack`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "telemetry",
  "payload": {}
}
```
**Expected ack topic:** `HTTP reconcile / application ack (không dùng PUBACK làm business ack)`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-13-Telemetry-Publish

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/machines/+/telemetry`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "telemetry",
  "payload": {}
}
```
**Expected ack topic:** `same as legacy note`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-14-Machine-Heartbeat-Check-in

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/machines/+/presence`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "presence",
  "payload": {}
}
```
**Expected ack topic:** `same as legacy note`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-15-Machine-Heartbeat-Check-in

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/machines/+/state/heartbeat`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "heartbeat",
  "payload": {}
}
```
**Expected ack topic:** `same as legacy note`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-16-Telemetry-Publish

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/machines/+/telemetry/snapshot`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "snapshot",
  "payload": {}
}
```
**Expected ack topic:** `same as legacy note`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-17-Telemetry-Publish

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/machines/+/telemetry/incident`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "incident",
  "payload": {}
}
```
**Expected ack topic:** `same as legacy note`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-18-Telemetry-Publish

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/machines/+/events/vend`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "vend",
  "payload": {}
}
```
**Expected ack topic:** `same as legacy note`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-19-Telemetry-Publish

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/machines/+/events/cash`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "cash",
  "payload": {}
}
```
**Expected ack topic:** `same as legacy note`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-20-Telemetry-Publish

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/machines/+/events/inventory`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "inventory",
  "payload": {}
}
```
**Expected ack topic:** `same as legacy note`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-21-Config-Shadow

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/machines/+/shadow/reported`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "reported",
  "payload": {}
}
```
**Expected ack topic:** `same as legacy note`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-22-Config-Shadow

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/machines/+/shadow/desired`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "desired",
  "payload": {}
}
```
**Expected ack topic:** `same as legacy note`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-23-Command-ACK

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/machines/+/commands/receipt`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "receipt",
  "payload": {}
}
```
**Expected ack topic:** `same as legacy note`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-24-Command-ACK

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/machines/+/commands/ack`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "ack",
  "payload": {}
}
```
**Expected ack topic:** `same as legacy note`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-25-Telemetry-Publish

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/machines/+/events`
- **Direction:** publish (machine → backend)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "schema_version": 1,
  "event_id": "{{$guid}}",
  "machine_id": "{{machineId}}",
  "dedupe_key": "avf-postman-{{$guid}}",
  "event_type": "events",
  "payload": {}
}
```
**Expected ack topic:** `same as legacy note`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-26-Backend-Commands

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/{{MACHINE_ID}}/commands/dispatch`
- **Direction:** publish (backend → machine)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "command_id": "{{commandId}}",
  "machine_id": "{{machineId}}",
  "sequence": 1,
  "command_type": "NOOP_POSTMAN",
  "payload": {},
  "idempotency_key": "avf-postman-{{$guid}}",
  "correlation_id": "{{$guid}}"
}
```
**Expected ack topic:** `{{mqttTopicPrefix}}/machines/{{machineId}}/commands/ack (enterprise) hoặc .../{{machineId}}/commands/ack (legacy)`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-27-Backend-Commands

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/{{MACHINE_ID}}/commands/down`
- **Direction:** publish (backend → machine)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "command_id": "{{commandId}}",
  "machine_id": "{{machineId}}",
  "sequence": 1,
  "command_type": "NOOP_POSTMAN",
  "payload": {},
  "idempotency_key": "avf-postman-{{$guid}}",
  "correlation_id": "{{$guid}}"
}
```
**Expected ack topic:** `{{mqttTopicPrefix}}/machines/{{machineId}}/commands/ack (enterprise) hoặc .../{{machineId}}/commands/ack (legacy)`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```

## mqtt-28-Backend-Commands

- **Topic:** `{{MQTT_TOPIC_PREFIX}}/machines/{{MACHINE_ID}}/commands`
- **Direction:** publish (backend → machine)
- **QoS:** 1 | **Retain:** False

**Payload example:**
```json
{
  "command_id": "{{commandId}}",
  "machine_id": "{{machineId}}",
  "sequence": 1,
  "command_type": "NOOP_POSTMAN",
  "payload": {},
  "idempotency_key": "avf-postman-{{$guid}}",
  "correlation_id": "{{$guid}}"
}
```
**Expected ack topic:** `{{mqttTopicPrefix}}/machines/{{machineId}}/commands/ack (enterprise) hoặc .../{{machineId}}/commands/ack (legacy)`
```json
{
  "status": "ack",
  "command_id": "{{commandId}}"
}
```
