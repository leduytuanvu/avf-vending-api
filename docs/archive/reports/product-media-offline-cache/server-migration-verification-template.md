# Server migration verification — product media offline cache

**Template:** copy to a dated filename under `docs/reports/product-media-offline-cache/` (example: `server-migration-verification-YYYYMMDD.md`).

**Environment:** production app-node  
**Migration:** goose **`00004_product_media_offline_cache`** (revision **4**)  
**Runbook:** `docs/runbooks/product-media-offline-cache-production-migration.md`

---

## Metadata

| Field | Value |
|------|--------|
| Operator | |
| Peer reviewer | |
| Change ticket / PR | |
| Backup artifact path | |
| Backup size (bytes) | |
| `pg_restore -l` line count | |
| App image digest | |
| Goose image digest | |
| Started (UTC) | |
| Completed (UTC) | |

---

## Pre-flight

- [ ] Maintenance window communicated  
- [ ] `docker compose ps` reviewed  
- [ ] `verify_database_environment.sh` PASS  
- [ ] `pg_isready` PASS  
- [ ] `pg_dump` backup created and validated (`pg_restore -l`)  

---

## Execution

- [ ] Workloads drained / stopped per policy  
- [ ] `goose up` via **`migration`** profile OR `RUN_MIGRATION=1` release — note which: ___  
- [ ] Services healthy (`healthcheck_app_node.sh`)  

---

## Automated read-only schema verification

Command:

```bash
VERIFY_ENV_FILE=/opt/avf-vending-api/deployments/prod/app-node/.env.app-node \
  bash /opt/avf-vending-api/scripts/db/verify_product_media_migration.sh
```

Result (paste summary line only — **no secrets**):

```
(paste "db_summary ..." and PASS/FAIL lines)
```

---

## HTTP / API

| Check | Result |
|-------|--------|
| `/health/live` | |
| `/health/ready` | |
| `/version` | |
| Login + `/v1/auth/me` | |

---

## Functional smoke

| Step | Result |
|------|--------|
| Category create | |
| Brand create | |
| Tag create (if used) | |
| Media init + complete → `ready` | |
| Product create/update with `primaryMediaId` when active | |
| Product GET shows primary media | |

---

## gRPC

| RPC | Result |
|-----|--------|
| `GetMediaManifest` (representative machine) | |

---

## MQTT

| Step | Result |
|------|--------|
| `catalog.refresh` publish | |
| ACK received | |

---

## Rollback / incidents

| Item | Notes |
|------|--------|
| Issues observed | |
| Rollback performed? | |
| Follow-up tickets | |

---

## Sign-off

| Role | Name | Date (UTC) |
|------|------|------------|
| Operator | | |
| Approver | | |
