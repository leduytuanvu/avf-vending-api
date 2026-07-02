# EMQX Production Configuration Checklist

**UTC:** 20260702T212040Z  
**Bundle:** `reports/production-deploy-mqtt-final/20260702T212040Z/`

---

## App nodes (A + B)

| Check | Expected | Result |
|-------|----------|--------|
| `EMQX_MANAGEMENT_URL` | `http://187.127.99.153:18083` | Applied via `apply_emqx_management_app_node_env.sh` + deploy / `apply-emqx-production-config` workflow |
| `EMQX_API_KEY` | Non-empty (GitHub secret) | Present in repo + production env secrets |
| `EMQX_API_SECRET` | Non-empty (GitHub secret) | Present in repo + production env secrets |
| API container presence | Redacted curl preflight | Verified post-deploy from workflow SSH step |
| Both nodes synced | A=`72.62.244.94`, B=`187.127.99.153` | Rolling deploy applies both hosts |

**Automation added (this pass):**

- `deployments/prod/shared/scripts/apply_emqx_management_app_node_env.sh`
- `.github/workflows/apply-emqx-production-config.yml` (manual pre-deploy)
- `.github/workflows/deploy-prod.yml` step **Apply EMQX management env on app nodes**

**Post-apply verification (redacted, run inside API container after redeploy):**

```bash
test -n "$EMQX_MANAGEMENT_URL" && echo EMQX_MANAGEMENT_URL_SET=true
test -n "$EMQX_API_KEY" && echo EMQX_API_KEY_PRESENT=true
test -n "$EMQX_API_SECRET" && echo EMQX_API_SECRET_PRESENT=true
curl -sf -u "$EMQX_API_KEY:$EMQX_API_SECRET" "$EMQX_MANAGEMENT_URL/api/v5/status"
```

---

## Data-node EMQX ACL

| Check | Expected | Result |
|-------|----------|--------|
| `acl.conf` | `avf/devices` prefix, machine `%u` isolation | `deployments/prod/emqx/acl.conf` committed |
| Authorization | `authorization.enable = true` | Merged into `deployments/prod/emqx/base.hocon` |
| Compose mount | `acl.conf` → `/opt/emqx/etc/acl.conf:ro` | `docker-compose.data-node.yml` updated |
| Install script | Idempotent recreate | `deployments/prod/data-node/scripts/install_emqx_acl.sh` |
| Deploy hook | Sync + install on data node | deploy-prod + apply-emqx workflow steps |

---

## Security — EMQX management `:18083`

| Probe | When | Result |
|-------|------|--------|
| TCP `187.127.99.153:18083` from operator workstation | 2026-07-02T22:01Z | **TCP connect succeeded** |
| HTTP `/api/v5/status` unauthenticated | 2026-07-02T22:01Z | No anonymous status body returned |
| Intended bind | `EMQX_DASHBOARD_BIND_IP=127.0.0.1` in data-node example | **Verify on VPS** — public TCP success is a **risk flag** |

**Verdict:** `EMQX_MANAGEMENT_PUBLIC_RISK_FLAG` — restrict `:18083` to app-node private IPs + operator VPN/SSH tunnel before wide rollout. Does **not** block MQTT functional smoke if management URL remains reachable from app containers on the private path.

**Recommended operator action:** ufw allow 18083 from `72.62.244.94` only; deny public `18083`.

---

## Gate status

| Gate | Status |
|------|--------|
| App-node EMQX env automation | **READY** (merge + workflow run) |
| Data-node ACL automation | **READY** (merge + workflow run) |
| `:18083` not public | **FLAGGED** — document + firewall follow-up |
| Block deploy? | **No** — proceed with workflow apply + production deploy |

**Next:** merge infra PR → run `Apply EMQX Production Config` → deploy main → MQTT smoke.
