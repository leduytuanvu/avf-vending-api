# AVF Production Enterprise Postman (market-ready)

## Import

1. Import `AVF_PRODUCTION_ENTERPRISE_REST.postman_collection.json`
2. Import `AVF_PRODUCTION_ENTERPRISE.postman_environment.json`
3. Copy `AVF_PRODUCTION_ENTERPRISE_PRIVATE.template.postman_environment.json` → `*LOCAL*.postman_environment.json` (gitignored)
4. Set `allowGatedWrites=true` and `confirmProductionWrites=I_UNDERSTAND_THIS_WRITES_TO_PRODUCTION`

## Folder tree

- **00** README Safety (how-to, write gate, actors, variables)
- **01–19** REST by module (Auth, Category, Brand, Tag, Product, Media, Site, Machine, …)
- **20–21** gRPC/MQTT reference stubs + `.md` catalogs
- **90** Full business flows (20 scenarios)
- **97** Online payment guarded (disabled by default)
- **98** Contract-disabled optional APIs
- **99** Cleanup

## Regenerate

```bash
python postman/production-enterprise/generate_enterprise_postman_project.py
python postman/production-enterprise/check_enterprise_postman_completeness.py
```

## ZIP

`AVF_PRODUCTION_ENTERPRISE_MARKET_READY_POSTMAN_PROJECT.zip` (generated locally; may be gitignored)
