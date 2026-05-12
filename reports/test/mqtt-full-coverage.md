# MQTT Full Coverage

- Broker status: **blocked-tooling**
- Reason: scripts/test/run-mqtt-full-coverage.sh was not executable on this host or broker/tools were unavailable
- Total flows: **6**

| Flow | Priority | Class | Status | Reason |
|---|---|---|---|---|
| `connect` | P0 | safe-read | **partial** | Full MQTT runner not executed yet |
| `telemetry` | P0 | local-write | **partial** | Full MQTT runner not executed yet |
| `command publish/subscribe` | P0 | canary-write | **partial** | Full MQTT runner not executed yet |
| `ACK duplicate/timeout` | P0 | hardware-required | **partial** | Full MQTT runner not executed yet |
| `invalid topic/payload` | P1 | local-write | **partial** | Full MQTT runner not executed yet |
| `reconnect/offline` | P1 | hardware-required | **partial** | Full MQTT runner not executed yet |
