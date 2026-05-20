# AVF MQTT Test Package

## Postman setup (manual)

1. Postman Desktop → **New** → **MQTT**
2. Host `{{MQTT_HOST}}`, port `{{MQTT_PORT}}`, credentials from environment (placeholders only in repo)
3. Use topics/payloads from `AVF_MQTT_EXAMPLES.json`

## mosquitto smoke

```bash
export MQTT_HOST=localhost MQTT_PORT=1883 MQTT_USERNAME=... MQTT_PASSWORD=... MQTT_TOPIC_PREFIX=avf MACHINE_ID=...
bash postman/generated/mqtt/AVF_MQTT_SMOKE.sh list
bash postman/generated/mqtt/AVF_MQTT_SMOKE.sh dry-run
bash postman/generated/mqtt/AVF_MQTT_SMOKE.sh
```
