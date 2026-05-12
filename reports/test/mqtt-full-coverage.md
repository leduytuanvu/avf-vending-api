# MQTT Full Coverage

- Generated At: `2026-05-12T08:24:38.836871+00:00`
- Mqtt Host: `127.0.0.1`
- Mqtt Port: `1883`
- Mqtt Topic Prefix: `avf-dev/devices`
- Broker Status: `reachable`
- Total Flows: `12`
- Passed: `1`
- Failed: `0`
- Partial: `9`
- Blocked: `2`
- Broker reason: publish/subscribe round-trip evidence captured

| Flow | Priority | Class | Status | Reason |
|---|---|---|---|---|
| `connect` | P0 | safe-read | **pass** | publish/subscribe round-trip evidence captured |
| `telemetry_publish` | P0 | local-write | **partial** | covered by tests/e2e/run-mqtt-local.sh; this generic runner only proves broker round-trip |
| `command_subscribe` | P0 | safe-read | **partial** | broker reachable; flow needs scenario assertion |
| `command_dispatch` | P0 | canary-write | **partial** | covered by tests/e2e/run-mqtt-local.sh; this generic runner only proves broker round-trip |
| `ack_success` | P0 | canary-write | **partial** | covered by tests/e2e/run-mqtt-local.sh; this generic runner only proves broker round-trip |
| `ack_duplicate` | P1 | canary-write | **partial** | covered by tests/e2e/run-mqtt-local.sh; this generic runner only proves broker round-trip |
| `ack_timeout_no_ack` | P0 | hardware-required | **blocked-hardware** | requires real canary device reconnect/no-ACK evidence |
| `invalid_payload` | P1 | local-write | **partial** | covered by tests/e2e/run-mqtt-local.sh; this generic runner only proves broker round-trip |
| `topic_prefix_correctness` | P0 | safe-read | **partial** | broker reachable; flow needs scenario assertion |
| `crlf_windows_handling` | P1 | local-write | **partial** | covered by tests/e2e/run-mqtt-local.sh; this generic runner only proves broker round-trip |
| `reconnect_offline` | P1 | hardware-required | **blocked-hardware** | requires real canary device reconnect/no-ACK evidence |
| `acl_topic_isolation` | P1 | production-readonly | **partial** | covered by tests/e2e/run-mqtt-local.sh; this generic runner only proves broker round-trip |
