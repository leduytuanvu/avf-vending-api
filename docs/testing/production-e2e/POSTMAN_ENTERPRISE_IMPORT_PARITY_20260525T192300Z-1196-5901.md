# Postman enterprise import parity (20260525T192300Z-1196-5901)

Run Newman after filling local private environment:

```bash
newman run postman\production-enterprise\AVF_PRODUCTION_ENTERPRISE_REST.postman_collection.json \
  -e postman/production-enterprise/AVF_PRODUCTION_ENTERPRISE_LOCAL.postman_environment.json \
  --reporters cli,json
```

Status: **PENDING_OPERATOR_CREDENTIALS** until local env is configured.
