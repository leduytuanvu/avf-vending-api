# Self-hosted Postgres + PgBouncer (VPS A)

Replaces Supabase. Runs on **app-node-a** (`72.62.244.94`) next to Redis/API.

Do not bind Postgres to the public internet. Default bind is `127.0.0.1`. For app-node-b, bind to the private/public IP and **UFW-allow only `187.127.99.153`** on 5432/6432.

## First boot

```bash
cd deployments/prod/db-node
cp .env.db-node.example .env.db-node
cp userlist.txt.example userlist.txt
# set the same password in both files
docker compose --env-file .env.db-node -f docker-compose.db-node.yml up -d
```

Init scripts in `init/` run only on an empty data volume (`pgcrypto` in `extensions`, `btree_gist`).

## Cutover from Supabase

See `exports/db-colocation-runbook.md` and `scripts/cutover_selfhost_postgres.sh`.

Runtime `DATABASE_URL` must point at **PgBouncer :6432** (transaction pool). `BACKUP_DATABASE_URL` / goose / `pg_dump` must use **Postgres :5432** directly.

PgBouncer `POOL_MODE=transaction` does not keep prepared statements across clients. Runtime app DSN must include `default_query_exec_mode=simple_protocol` (pgx). The API pool bootstrap also forces SimpleProtocol when the host is `pgbouncer` or the port is `6432`, unless the URL already sets `default_query_exec_mode`. Ops scripts strip that query param before `psql` / `pg_dump` / goose.

Postgres compose sets SSD/OLTP GUCs (`shared_buffers=384MB`, `random_page_cost=1.1`, `jit=off`, `effective_cache_size=3GB`, `shared_preload_libraries=pg_stat_statements`). Do not raise container memory/CPU while the fleet is idle.
