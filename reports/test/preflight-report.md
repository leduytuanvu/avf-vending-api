# Local test preflight — report

**Host:** Windows 10 (build 26200), **Shell:** Windows PowerShell 5.1 (`Desktop`)  
**Repo:** `avf-vending-api`  
**When:** generated during automated preflight (see commands below)

## Overall status

| Gate | Result |
|------|--------|
| **Overall preflight** | **FAIL** — Docker Engine unreachable; Redis / NATS listeners down; TCP **8080** is not serving this API’s routes; Mosquitto CLI missing. |
| Tool discovery (Go / sqlc / JSON / HTTP CLIs) | **PASS** (with notes below) |
| Env example sanity (Phase-spec retired tokens) | **PASS** — no matches in sampled `.env*` / loadtest example |
| Generated artifacts on disk | **PASS** |
| Strict `git grep` (Phase-spec list) | **PASS** — zero tracked hits (`git grep` exit code **1**) |

---

## 1) Environment detection & tool versions

### Commands run

```powershell
[System.Environment]::OSVersion.VersionString
$PSVersionTable.PSEdition; $PSVersionTable.PSVersion
git branch --show-current
git status --short
go version
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0 version
docker version
docker compose version
jq --version
curl.exe --version
newman --version
grpcurl --version
Get-Command mosquitto_pub -ErrorAction SilentlyContinue
```

### Results

| Check | Output / verdict |
|-------|------------------|
| **Git branch** | `test/openapi-json-body-shape-proof` |
| **Git status** | **Dirty** — **668** lines from `git status --short` (large working tree; not triaged here). |
| **Go** | `go version go1.26.2 windows/amd64` |
| **sqlc** | `v1.29.0` (via pinned `go run …@v1.29.0 version`) |
| **Docker CLI** | Client **29.2.1**, context `desktop-linux` |
| **Docker Engine** | **FAIL** — `docker ps` → cannot connect to `dockerDesktopLinuxEngine` named pipe (daemon not running or Desktop backend stopped). |
| **Docker Compose plugin** | `v5.0.2` |
| **jq** | `jq-1.8.1` |
| **curl** | `curl 8.19.0 (Windows)` — use **`curl.exe`** in PowerShell (`curl` is an alias to `Invoke-WebRequest`). |
| **Newman** | `6.2.2` |
| **grpcurl** | Binary present; **`grpcurl --version` writes `grpcurl.exe dev build <no version set>` to stderr** (non-zero semantics in PowerShell; treat as “installed, semver unknown”). |
| **mosquitto_pub / mosquitto_sub** | **NOT FOUND** on `PATH`. |

---

## 2) Local infrastructure reachability

### Commands run

```powershell
docker ps --format "table {{.Names}}\t{{.Ports}}\t{{.Status}}"
# TCP probes (PowerShell)
foreach ($p in 8080,9090,5432,15432,6379,4222,8222,1883) {
  Test-NetConnection -ComputerName localhost -Port $p -WarningAction SilentlyContinue | Select-Object TcpTestSucceeded
}
curl.exe -s -o NUL -w "HTTP %{http_code}\n" http://127.0.0.1:8080/health/live
curl.exe -s -o NUL -w "HTTP %{http_code}\n" http://127.0.0.1:8080/version
where.exe psql
where.exe pg_isready
```

### Results

| Dependency | Expected (from `.env.example` + `deployments/docker/docker-compose.yml`) | Observed | Status |
|------------|----------------------------------------------------------------------|----------|--------|
| **Docker** | Running Desktop Linux engine | Named pipe connection **failed** | **FAIL** |
| **PostgreSQL** | `.env.example` DSN uses `localhost:5432`; compose publishes **`localhost:15432` → container 5432** | TCP **5432** accepts connections | **PARTIAL** — listener present; **`psql` / `pg_isready` not installed** here for auth/`SELECT 1` proof |
| **Redis** | `localhost:6379` when enabled | TCP **6379** **closed** | **FAIL** |
| **NATS** | Client `4222`, monitoring `8222` per compose | Both **closed** | **FAIL** (needed for default compose stack / async paths) |
| **MQTT broker** | `1883` when `--profile broker` (EMQX) | TCP **1883** **open** | **PASS** (listener only; no CLI on PATH to publish) |

---

## 3) Ports (expected vs probe)

| Port | Intended service | Probe (`Test-NetConnection localhost`) | Notes |
|------|------------------|----------------------------------------|-------|
| **8080** | HTTP API (`HTTP_ADDR=:8080`) | **Open** | **But** `GET /health/live` and `GET /version` return **Apache HTML 404** — port is **not** this repo’s chi router (wrong process or reverse proxy). |
| **9090** | gRPC (`GRPC_ADDR=:9090`) when enabled | **Closed** | Matches `.env.example` defaults (`MACHINE_GRPC_ENABLED=false`). |
| **5432** | Postgres (optional native install) | **Open** | Compose alternative maps **`15432:5432`** — align `DATABASE_URL` with whichever instance you use. |
| **15432** | Postgres via compose | **Closed** | Compose stack not reachable while Docker Engine is down. |
| **6379** | Redis | **Closed** | Start Redis (compose) or enable remote URL. |
| **4222 / 8222** | NATS | **Closed** | Start core compose services. |
| **1883** | MQTT (EMQX profile or external broker) | **Open** | Good for MQTT ingest smoke **if** broker credentials match env. |

**Config drift warning:** `.env.example` claims compose Postgres on `:5432`, but `deployments/docker/docker-compose.yml` binds host **`15432`**. Pick one convention locally to avoid silent connection failures.

---

## 4) Env examples — retired multi-entity variables

### Commands run

PowerShell `Select-String` against `.env.example`, `.env.local.example`, and `deployments/loadtest/env.example`, using the **same alternation list as preflight task §4** (case-insensitive; copy/paste from the ticket so this Markdown file stays free of those literals).

### Result

**PASS** — no matches in `.env.example`, `.env.local.example`, or `deployments/loadtest/env.example`.

---

## 5) Generated artifacts

### Commands run

```powershell
Test-Path docs/swagger/swagger.json
Test-Path docs/swagger/docs.go
Test-Path internal/gen/db/models.go
Get-ChildItem docs/postman -Filter *.json
```

### Result

**PASS**

- `docs/swagger/swagger.json` — present  
- `docs/swagger/docs.go` — present  
- `internal/gen/db/models.go` — present  
- `docs/postman/*.postman_collection.json` + `*.postman_environment.json` — present (nine JSON files enumerated by tooling earlier in this session)

---

## 6) Strict repository grep (tracked files)

### Command

Run **`git grep`** exactly as specified in **preflight task §6** (extended `-E` alternation and exclusions `:!vendor`, `:!node_modules`, `:!.git`). **Do not paste the alternation into documentation** if your repo policy forbids those literals in Markdown.

### Result

**PASS** — no output; exit code **1** (`git grep` “no matches”). Suitable as a gate before API/MQTT/gRPC/E2E suites **for tracked sources**.

---

## Blockers (actionable)

1. **Docker Engine / Desktop not running** — cannot start Postgres/Redis/NATS/MinIO/EMQX via compose.  
2. **Redis not listening on 6379** — any path requiring caches / rate limits / sessions will fail when enabled.  
3. **NATS not listening on 4222/8222** — worker / telemetry async assumptions break when wired to NATS.  
4. **TCP 8080 occupied by a non-AVF HTTP stack** — health endpoints expected by this repo return **404** HTML from another server. Free the port or change `HTTP_ADDR` and client URLs.  
5. **`psql` / `pg_isready` absent** — cannot complete application-level DB readiness without installing PostgreSQL client tools or using `docker exec`.  
6. **`mosquitto_pub` / `mosquitto_sub` absent** — optional for MQTT tests; use broker vendor CLI, `docker exec`, or install Eclipse Mosquitto clients.

---

## Recommended next commands (ordered)

1. **Start Docker Desktop** (Windows) until `docker ps` succeeds.  
2. Bring up the minimal stack (from repo root):

   ```powershell
   docker compose -f deployments/docker/docker-compose.yml up -d postgres redis nats
   ```

   Add `--profile broker` if MQTT via EMQX is required:

   ```powershell
   docker compose -f deployments/docker/docker-compose.yml --profile broker up -d emqx
   ```

3. **Align `DATABASE_URL`** with the running Postgres port (**`15432`** for compose mapping above, or **`5432`** if using a local server on the default port).  
4. **Resolve HTTP port conflict** — stop the foreign listener on **8080** *or* export `HTTP_ADDR` to a free port (e.g. `:18080`) before `go run ./cmd/api`. Re-check:

   ```powershell
   curl.exe -s -w "HTTP %{http_code}\n" http://127.0.0.1:<port>/health/live
   ```

5. Export **`TEST_DATABASE_URL`** for Postgres-backed `go test` / E2E (dedicated DB recommended per `.env.example` comments).  
6. (Optional) Install **PostgreSQL client tools** and **Mosquitto clients**, or run CLI commands via `docker exec` into `avf-postgres` / `avf-emqx`.

---

## Pass/fail summary

| Area | Verdict |
|------|---------|
| Core dev toolchain (Go, sqlc, jq, curl.exe, newman) | **PASS** |
| Container runtime | **FAIL** (daemon down) |
| Redis / NATS | **FAIL** |
| Postgres | **PARTIAL** (port open; tooling limits proof) |
| MQTT listener | **PASS** (1883 open; no mosquitto CLI) |
| HTTP API readiness | **FAIL** (wrong service on 8080) |
| Env templates vs retired identifiers | **PASS** |
| Generated artifacts | **PASS** |
| Strict `git grep` gate | **PASS** |

**Recommended immediate action:** start Docker Desktop, launch compose core services, fix **8080** conflict, then rerun this checklist starting at §2.
