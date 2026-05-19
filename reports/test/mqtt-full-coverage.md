# MQTT Full Coverage

- Generated At: `2026-05-19T18:40:54.807667+00:00`
- Mqtt Host: `127.0.0.1`
- Mqtt Port: `1883`
- Mqtt Topic Prefix: `avf/local`
- Broker Status: `blocked-tooling`
- Total Flows: `12`
- Passed: `0`
- Failed: `0`
- Partial: `0`
- Blocked: `12`
- Broker reason: mosquitto_pub/mosquitto_sub not installed

| Flow | Priority | Class | Status | Reason |
|---|---|---|---|---|
| `connect` | P0 | safe-read | **blocked-tooling** | mosquitto_pub/mosquitto_sub not installed |
| `telemetry_publish` | P0 | local-write | **blocked-tooling** | mosquitto_pub/mosquitto_sub not installed |
| `command_subscribe` | P0 | safe-read | **blocked-tooling** | mosquitto_pub/mosquitto_sub not installed |
| `command_dispatch` | P0 | canary-write | **blocked-tooling** | mosquitto_pub/mosquitto_sub not installed |
| `ack_success` | P0 | canary-write | **blocked-tooling** | mosquitto_pub/mosquitto_sub not installed |
| `ack_duplicate` | P1 | canary-write | **blocked-tooling** | mosquitto_pub/mosquitto_sub not installed |
| `ack_timeout_no_ack` | P0 | hardware-required | **blocked-tooling** | mosquitto_pub/mosquitto_sub not installed |
| `invalid_payload` | P1 | local-write | **blocked-tooling** | mosquitto_pub/mosquitto_sub not installed |
| `topic_prefix_correctness` | P0 | safe-read | **blocked-tooling** | mosquitto_pub/mosquitto_sub not installed |
| `crlf_windows_handling` | P1 | local-write | **blocked-tooling** | mosquitto_pub/mosquitto_sub not installed |
| `reconnect_offline` | P1 | hardware-required | **blocked-tooling** | mosquitto_pub/mosquitto_sub not installed |
| `acl_topic_isolation` | P1 | production-readonly | **blocked-tooling** | mosquitto_pub/mosquitto_sub not installed |
