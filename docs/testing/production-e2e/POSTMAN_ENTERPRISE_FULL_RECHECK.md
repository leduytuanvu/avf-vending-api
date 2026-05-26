# Postman enterprise full recheck

Generated: 2026-05-26T20:49:39Z

## Production targets
- REST: https://api.ldtv.dev
- gRPC: machine-api.ldtv.dev:443
- MQTT: mqtt.ldtv.dev:8883

## Structural counts
- REST collection requests: 115
- Market release flow stubs: 20 (folder 90)
- Classifications: {"RUNNABLE": 41, "OPTIONAL_SKIPPED_BY_PRODUCTION_CONTRACT": 67, "ONLINE_PAYMENT_EXCLUDED": 6, "REST_TOTAL": 115, "EXCLUDED_UNHAPPY": 152}

## Checker
```bash
python postman/production-enterprise/check_enterprise_postman_completeness.py
```
