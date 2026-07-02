# Field repair scripts (PowerShell)

Operational repair helpers for machine bootstrap metadata and sell readiness. **Not** invoked by CI — used in field testing and metadata contract checks.

| Script | Purpose |
|--------|---------|
| [`repair-machine-bootstrap-metadata.ps1`](repair-machine-bootstrap-metadata.ps1) | Repair machine bootstrap/layout metadata via admin API (used by [`../e2e/tests/test-metadata-contract.ps1`](../e2e/tests/test-metadata-contract.ps1)) |
| [`ensure-machine-sell-readiness.ps1`](ensure-machine-sell-readiness.ps1) | Follow-on sell-readiness checks after bootstrap repair |

Run with `-DryRun -NonInteractive` first when exploring. Production writes require explicit confirmation flags documented in each script header.
