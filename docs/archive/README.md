# Documentation archive

Historical audits, verification reports, and one-off gate evidence. **Not canonical** for current operations — use active docs under [`../README.md`](../README.md).

## Layout

| Path | Contents |
|------|----------|
| [`audits/`](audits/) | Superseded readiness audits, cleanup inventories, UUID/migration pre-flight reports |
| [`audits/audit/`](audits/audit/) | Backend ↔ Android contract audits (point-in-time) |
| [`reports/`](reports/) | Git/deploy phase reports, product-media migration verification |
| [`verification/`](verification/) | `FINAL_*` and full-system signoff reports |
| [`testing/`](testing/) | Historical Postman/E2E audit JSON and enterprise audit traces |
| [`cleanup/`](cleanup/) | Deep repo cleanup audits (2026-05-20 / 2026-05-26) |

## Conventions

- **Active docs** live outside `docs/archive/` — runbooks, production checklists, testing guides, and current enterprise audits (`docs/audits/final-enterprise-audit.md`, etc.).
- **Do not delete** archive files without a replacement evidence trail; link from active docs when historical context is needed.
- **Link checker** (`tools/check_markdown_links.py`) skips `docs/archive/**`; fix broken links inside archive only when editing those files.
- **New one-off reports** from completed gates should land here (under the matching subfolder), not in `docs/audits/` or `docs/reports/verification/`.
- **2026-06 cleanup manifest:** [`../reports/cleanup/2026-06-repo-cleanup-manifest.md`](../reports/cleanup/2026-06-repo-cleanup-manifest.md)

## Active pointers

- Enterprise readiness: [`../audits/final-enterprise-audit.md`](../audits/final-enterprise-audit.md)
- Operational runbooks: [`../runbooks/README.md`](../runbooks/README.md)
- Testing guides: [`../testing/README.md`](../testing/README.md)
- Ongoing verification outputs (non-FINAL): [`../reports/verification/`](../reports/verification/)
