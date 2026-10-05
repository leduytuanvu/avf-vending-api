# Production disk automation

## Workflows

| Workflow | Trigger | Behavior |
| -------- | ------- | -------- |
| **Production disk automation** | Cron + optional dispatch | Daily threshold check; weekly preventive cleanup |
| **Production disk hygiene** | Manual `DISK_HYGIENE_PRODUCTION` | Full cleanup on demand |

## Schedule (UTC)

- **Daily `17 3 * * *`** (~10:17 ICT): read disk on app-node A and B. Run hygiene on a host when usage ≥ warn threshold.
- **Weekly `0 4 * * 0`** (~11:00 ICT Sunday): preventive hygiene on **both** nodes (even if disk is low).

## Thresholds (repo variables, optional)

| Variable | Default | Meaning |
| -------- | ------- | ------- |
| `PRODUCTION_DISK_WARN_PERCENT` | `70` | Auto hygiene when root disk ≥ this % |
| `PRODUCTION_DISK_CRITICAL_PERCENT` | `85` | Force hygiene on **both** nodes |

Set under **Settings → Secrets and variables → Actions → Variables** (repository or `production` environment).

## Hostinger panel

Hostinger does not call these workflows. Automation runs on the **self-hosted runner** on VPS A. Refresh hPanel after a successful run to see lower disk %.

## Manual

```bash
gh workflow run production-disk-automation.yml -f force_hygiene=true
gh workflow run production-disk-hygiene.yml -f confirm=DISK_HYGIENE_PRODUCTION
```
