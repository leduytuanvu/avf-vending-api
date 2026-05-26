# AVF Production Enterprise Postman

## Import

1. Import `AVF_PRODUCTION_ENTERPRISE_REST.postman_collection.json`
2. Import `AVF_PRODUCTION_ENTERPRISE.postman_environment.json`
3. Copy `AVF_PRODUCTION_ENTERPRISE_PRIVATE.template.postman_environment.json` locally; fill secrets
4. Set `allowGatedWrites=true` and `confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION` for CRUD

## Scope

- Default gate: **no online payment** (MoMo/ZaloPay/VietQR/webhooks excluded)
- REST: Newman-runnable collection from E2E manifests
- gRPC/MQTT: see `AVF_PRODUCTION_GRPC_REQUESTS.md`, `AVF_PRODUCTION_MQTT_REQUESTS.md`, and manual guide

## Regenerate

```bash
python postman/production-enterprise/generate_enterprise_postman_project.py
```
