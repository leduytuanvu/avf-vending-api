# CI / workflow contract scripts

- **`verify_workflow_contracts.sh`**: offline checks on `.github/workflows/*.yml` and related repo scripts. Invokes `tools/verify_github_workflow_cicd_contract.py` for the full release graph, permissions, and action-pin policy (no GitHub API).
- **Which workflow is canonical for deploy?** See [docs/runbooks/github-governance.md](../../docs/runbooks/github-governance.md#active-github-actions-workflows-in-this-repository) — use **`deploy-prod.yml`** for production; **`deploy-production.yml`** is a no-op pointer only.
- **Hygiene:** generated outputs and caches belong in paths listed in `.gitignore` (e.g. `__pycache__`, `security-reports/`, `trivy-*.txt`).
