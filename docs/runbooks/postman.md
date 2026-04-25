# Postman and OpenAPI (AVF Vending API)

## 1. Import OpenAPI directly (recommended for always-current paths)

In Postman: **Import → Link** and use the production OpenAPI URL:

- `https://api.ldtv.dev/swagger/doc.json`

This returns **OpenAPI 3.0 JSON**, not a Postman collection. Postman will generate requests from the spec. Keep using this when you only need an up-to-date contract.

## 2. Import native Postman files (guards, variables, headers)

The repository ships separate artifacts under `docs/postman/` (these are **not** substitutes for `/swagger/doc.json`):

- `docs/postman/avf-vending-api.postman_collection.json` — collection with `{{base_url}}`, `{{api_prefix}}`, and collection scripts (request IDs, staging/production safety checks, production mutation block).
- `docs/postman/avf-local.postman_environment.json` — local development.
- `docs/postman/avf-staging.postman_environment.json` — staging.
- `docs/postman/avf-production.postman_environment.json` — production (read-only by default in Postman).

In Postman: **Import → File** and select the collection, then the environment(s). Set the **active environment** before sending requests.

Regenerate the collection from the same OpenAPI source the API embeds: `make postman-generate` (after `make swagger`).

## 3. How this fits together

- `/swagger/doc.json` is **OpenAPI JSON** (`openapi`, `info`, `paths`, `components`, …). It must **not** be replaced by Postman’s JSON format.
- **Production** defaults: **Swagger UI is off** (`HTTP_SWAGGER_UI_ENABLED=false`) so `/swagger/` and `/swagger/index.html` are not served, but **OpenAPI JSON stays on** (`HTTP_OPENAPI_JSON_ENABLED=true`) so importers and Postman can use the public URL.
- The native Postman collection is a **separate** artifact; it adds guardrails and variables for humans. It does not replace the OpenAPI document.
- **Production writes in Postman** are blocked by default: the collection script requires `allow_mutation=true`, `allow_production_mutation=true`, and `confirm_production_run=I_UNDERSTAND_PRODUCTION_MUTATION` on the **production** environment to send POST/PUT/PATCH/DELETE in production. Do not enable this unless you fully understand the blast radius.
- Staging and production also validate **payment** and **MQTT** settings (`payment_env`, `mqtt_topic_prefix`) so the wrong environment is harder to use by mistake.

## 4. Relatively safe production requests

These are read-only and commonly used for health and operations:

- `GET /health/live`
- `GET /health/ready`
- `GET /version`
- `GET /swagger/doc.json`
- **Read-only GET** APIs under `/v1/…` (still require a valid **Bearer** token where the API enforces one).

## 5. High-risk production requests (examples)

Treat with extreme care; many mutate state or move money:

- **Orders / payments / refunds:** e.g. `POST` paths for orders, refunds, payment intents.
- **Vend / telemetry:** e.g. posting vend results or telemetry replay.
- **Inventory and machine configuration:** any write that changes stock, planograms, or device config.
- **Admin mutating methods:** `DELETE`, `PATCH`, and `POST`/`PUT` on admin or operator APIs unless you have an explicit runbook and approvals.

## 6. If you must unlock production mutation in Postman

Only after explicit approval and a controlled change window:

1. Select the **production** environment in Postman.
2. Set `allow_mutation=true`, `allow_production_mutation=true`, and `confirm_production_run=I_UNDERSTAND_PRODUCTION_MUTATION`.
3. Use **Bearer** tokens with least privilege; rotate after testing.

The API still enforces authorization and idempotency; Postman only reduces accidental clicks—it is not a safety net by itself.
