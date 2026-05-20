# Deploy scripts

Release evidence, security verdict writers, production/staging smoke helpers, and deploy SLO collection. **Does not replace** `deployments/prod/scripts/release.sh` (production VPS rollout).

| Subfolder | Role |
|-----------|------|
| `release/` | Release evidence packages, manifests, deploy candidate JSON |
| `security/` | Security Release verdict (`write_security_verdict.py`), image resolution |
| `smoke/` | Production smoke JSON emitters and local field smoke |
| `monitoring/` | Deploy SLO evidence collection |

Top-level: `smoke_staging.sh`, `smoke_local.sh`, `migration_preflight.sh`, `notify_deployment_status.sh`.
