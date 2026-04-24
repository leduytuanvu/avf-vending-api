#!/usr/bin/env python3
"""Assemble docs/api/api-surface-audit.md from intro + generated table (see gen_api_surface_audit_table.py)."""
from __future__ import annotations

import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "docs" / "api" / "api-surface-audit.md"
GEN = ROOT / "scripts" / "gen_api_surface_audit_table.py"

INTRO = r"""# API surface audit (enterprise)

**OpenAPI:** `docs/swagger/swagger.json` (generated). Regenerate: `make swagger` or `python tools/build_openapi.py`.

**Server order:** Production `https://api.ldtv.dev` first, local second (enforced in `tools/build_openapi.py`, `tools/openapi_verify_release.py`, and `go test`).

**Telemetry:** High-volume device telemetry and primary command delivery are **MQTT-first**. HTTP device routes (`/v1/device/...`) are **integration / fallback**; `POST .../commands/poll` is **not** the primary command path.

**Swagger / OpenAPI hosting:** Treat `/swagger/*` as **public only when intentionally enabled** at the edge. Operators should assume documentation endpoints can be disabled in locked-down production.

**Metrics:** `GET /metrics` is an **internal / protected** ops surface (private bind, scrape token, or network ACL). Do not expose unauthenticated metrics on a public listener.

**Reports / admin lists:** `GET /v1/reports/*`, `GET /v1/orders`, `GET /v1/payments`, and most `GET /v1/admin/*` routes are **admin portal** surfaces — **not** kiosk runtime dependencies for sale execution.

**Verification:** `make verify-enterprise-release` runs `go test ./...`, Swagger drift checks, `tools/openapi_verify_release.py` (production server first, Bearer on protected `/v1` routes, write examples, success+error examples, no planned-only paths, no secret-like examples), shell/compose checks, and doc secret heuristics.

**Regenerate the matrix table:** `python scripts/compose_api_surface_audit_md.py` (preferred). For inspection only: `python scripts/gen_api_surface_audit_table.py`.

---

## Legend

| Column | Meaning |
| --- | --- |
| **intended client** | Primary consumer class: kiosk runtime app, technician setup app, admin portal, payment provider, device HTTP fallback, DevOps/monitoring. |
| **idempotency required** | `yes` = use `Idempotency-Key` / `X-Idempotency-Key` per OpenAPI; `no` = not required by header contract; `n/a` = read-only. |
| **offline retry safe** | `yes` = safe GET or clearly safe retry; `yes w/ key` = retries must reuse idempotency key; `caution` = may duplicate side effects without care; `no` = online-only. |
| **status** | `keep` = primary HTTP surface; `fallback` = secondary to MQTT or non-primary design; `internal` = ops-only expectations; `deprecated` / `roadmap` = not used in current OpenAPI (must not appear for shipped routes). |

---

## Full route matrix

The table lists **every** `paths` entry in `docs/swagger/swagger.json` (one row per HTTP method).

"""

FOOTER = r"""---

## Planned-only HTTP (not in OpenAPI)

Product ideas that are **not** shipped as public `paths` belong in [roadmap.md](roadmap.md) only. If a path appears in OpenAPI, it is **implemented** for this revision (subject to nil wiring returning 503 where applicable).

---

## Related

- [Kiosk app flow](kiosk-app-flow.md)
- [API client classification](api-client-classification.md)
- [API surface security](../runbooks/api-surface-security.md)
- [MQTT contract](mqtt-contract.md)
"""


def main() -> int:
    proc = subprocess.run(
        [sys.executable, str(GEN)],
        cwd=str(ROOT),
        capture_output=True,
        text=True,
        encoding="utf-8",
        check=True,
    )
    table = proc.stdout.lstrip("\ufeff")
    OUT.write_text(INTRO + table + "\n" + FOOTER, encoding="utf-8", newline="\n")
    print(f"wrote {OUT.relative_to(ROOT)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
