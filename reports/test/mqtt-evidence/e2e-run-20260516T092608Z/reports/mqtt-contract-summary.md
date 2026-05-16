# MQTT contract summary (Phase 7)

Generated: 2026-05-16T09:26:18Z

| Result | Count |
|--------|-------|
| pass | 6 |
| fail | 0 |
| skip | 4 |

## By flow

### MQTT-30

| step | topic | status | note |
|------|-------|--------|------|
| tcp | `tcp://127.0.0.1:1883` | **pass** | port_open |
| subscribe-command | `avf/devices/55555555-5555-5555-5555-555555555555/commands/dispatch` | **pass** | connected_or_timeout_ok |
| publish-telemetry | `avf/devices/55555555-5555-5555-5555-555555555555/telemetry` | **pass** | mosquitto_pub_ok |

### MQTT-31

| step | topic | status | note |
|------|-------|--------|------|
| publish-heartbeat | `avf/devices/55555555-5555-5555-5555-555555555555/telemetry` | **pass** | published event_id=e2e-tel-hb-9271 |
| verify-rest | `—` | **skip** | partial_no_per_event_mqtt_read_api_documented |
| hint-machine-health | `—` | **skip** | no_ADMIN_TOKEN |

### MQTT-32

| step | topic | status | note |
|------|-------|--------|------|
| admin-dispatch | `—` | **skip** | no_ADMIN_TOKEN |
| synthetic-command | `avf/devices/55555555-5555-5555-5555-555555555555/commands/dispatch` | **pass** | broker_only_noop |
| publish-ack | `avf/devices/55555555-5555-5555-5555-555555555555/commands/ack` | **pass** | commands/ack_sent |
| verify-command-get | `—` | **skip** | admin_full_flow_not_used_or_no_commandId |

