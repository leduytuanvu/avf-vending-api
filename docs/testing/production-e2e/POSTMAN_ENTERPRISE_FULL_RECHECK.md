# Postman enterprise full recheck

Generated: 2026-05-26T16:46:58Z

## Production targets
- REST: https://api.ldtv.dev
- gRPC: machine-api.ldtv.dev:443
- MQTT: mqtt.ldtv.dev:8883

## Structural counts
- REST collection requests: 264
- Market release flow stubs: 20 (folder 90)
- Classifications: {"RUNNABLE": 170, "OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT": 67, "CONFIG_REQUIRED": 17, "ONLINE_PAYMENT_EXCLUDED": 9, "REST_TOTAL": 264}

## Checker
```bash
python postman/production-enterprise/check_enterprise_postman_completeness.py
```
