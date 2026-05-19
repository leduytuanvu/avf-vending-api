# MQTT — AVF FULL 100 adjacent tests

MQTT is executed with **mosquitto_pub / mosquitto_sub** (or `tests/e2e/run-mqtt-local.sh`), not Newman.

## Runner

- `bash postman/full-production-suite/mqtt/run-mqtt-postman-adjacent.sh`

## Assets

- `AVF_MQTT_100_TOPIC_MATRIX.csv` — topic templates + sample mosquitto commands.
- `AVF_MQTT_100_PAYLOADS.json` — canonical payload JSON templates.

## Native Postman MQTT

Postman Desktop MQTT mode can subscribe/publish using the same topics — **no** importable MQTT collection JSON is generated here.
