# AVF Production Enterprise Postman (happy-case)

## Import

1. Import `AVF_PRODUCTION_ENTERPRISE_REST.postman_collection.json` (**AVF Production Enterprise Happy Case API**)
2. Import `AVF_PRODUCTION_ENTERPRISE.postman_environment.json`
3. Copy `AVF_PRODUCTION_ENTERPRISE_PRIVATE.template.postman_environment.json` → `*LOCAL*.postman_environment.json` (gitignored)
4. Set `allowGatedWrites=true` and `confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION`

## Folder tree (no numeric prefixes)

- **README Safety** — how-to, write gate, actors, variables
- **Health Version**, **Auth**, **Category**, **Brand**, **Product**, **Media**, **Machine**, …
- **gRPC Reference** / **MQTT Reference** — stubs + `.md` catalogs
- **Full Business Flows** — end-to-end scenarios
- **Online Payment Happy Case Guarded** — disabled by default
- **Optional Contract Disabled**, **Cleanup**

Negative/security tests are listed in `AVF_PRODUCTION_NEGATIVE_TESTS_EXCLUDED.md` only.

## Regenerate

```bash
python postman/production-enterprise/generate_enterprise_postman_project.py
python postman/production-enterprise/check_enterprise_postman_completeness.py
```

## ZIP

`AVF_PRODUCTION_ENTERPRISE_HAPPY_CASE_POSTMAN_PROJECT.zip`
