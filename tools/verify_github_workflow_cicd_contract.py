#!/usr/bin/env python3
"""
Offline check that key GitHub Actions workflows are wired for the enterprise chain:
  CI -> Build and Push Images -> Security Release (release gate; image scan + security-verdict.json)
  Repo Security (push/PR): security.yml
  Staging: workflow_run from Security Release (develop)
  Production: workflow_dispatch only (evidence: Build + Security Release run ids; no auto workflow_run)
"""
from __future__ import annotations

import sys
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError:  # pragma: no cover
    print("ERROR: PyYAML required (e.g. pip install pyyaml or apt install python3-yaml)", file=sys.stderr)
    sys.exit(1)

ROOT = Path(__file__).resolve().parents[1]
WF = ROOT / ".github" / "workflows"


def _normalize_gha_doc(raw: object) -> object:
    # PyYAML 1.1 parses the key "on" as boolean True. GitHub uses "on:" for triggers.
    if isinstance(raw, dict) and True in raw and "on" not in raw and isinstance(raw.get(True), (dict, str, type(None), list)):
        out = {k: v for k, v in raw.items() if k is not True}
        out["on"] = _normalize_gha_doc(raw[True])
        return out
    if isinstance(raw, dict):
        return {k: _normalize_gha_doc(v) for k, v in raw.items()}
    if isinstance(raw, list):
        return [_normalize_gha_doc(x) for x in raw]
    return raw


def get_on_block(doc: dict) -> Any:
    """Read workflow triggers; supports both data['on'] and data[True] (PyYAML boolean key)."""
    if not isinstance(doc, dict):
        return None
    if "on" in doc:
        return doc.get("on")
    if True in doc:
        return doc.get(True)
    return None


def load(name: str) -> dict:
    p = WF / name
    if not p.is_file():
        print(f"ERROR: missing {p}", file=sys.stderr)
        raise SystemExit(1)
    doc = yaml.safe_load(p.read_text(encoding="utf-8")) or {}
    return _normalize_gha_doc(doc)  # type: ignore[return-value]


def _as_list(x: object) -> list:
    if x is None:
        return []
    if isinstance(x, list):
        return x
    if isinstance(x, str):
        return [x]
    return [x]


def main() -> None:
    ci = load("ci.yml")
    on = get_on_block(ci) or ci.get("on")
    if isinstance(on, str):
        on = {on: None}
    else:
        on = on or {}
    if not isinstance(on, dict):
        print("ERROR: ci.yml must define on: as a mapping (triggers).", file=sys.stderr)
        raise SystemExit(1)
    if "pull_request" not in on or "push" not in on:
        print("ERROR: ci.yml must define on: pull_request and on: push (develop + main).", file=sys.stderr)
        raise SystemExit(1)
    for key in ("pull_request", "push"):
        br = (on.get(key) or {}).get("branches") or []
        for b in ("develop", "main"):
            if b not in br:
                print(f"ERROR: ci.yml on.{key}.branches must include develop and main (found {br!r}).", file=sys.stderr)
                raise SystemExit(1)

    build = load("build-push.yml")
    if (build.get("name") or "").strip() != "Build and Push Images":
        print("ERROR: build-push.yml name must be exactly 'Build and Push Images' (for workflow_run wiring).", file=sys.stderr)
        raise SystemExit(1)
    bon = get_on_block(build) or build.get("on") or {}
    if not isinstance(bon, dict):
        print("ERROR: build-push.yml must have a mapping in on: (triggers).", file=sys.stderr)
        raise SystemExit(1)
    if "workflow_run" not in bon:
        print("ERROR: build-push.yml must have on.workflow_run from CI.", file=sys.stderr)
        raise SystemExit(1)
    wr = bon.get("workflow_run") or {}
    wfw = wr.get("workflows") or []
    if "CI" not in wfw:
        print("ERROR: build-push on.workflow_run.workflows must list 'CI'.", file=sys.stderr)
        raise SystemExit(1)
    if "workflow_dispatch" not in bon:
        print("ERROR: build-push must allow workflow_dispatch.", file=sys.stderr)
        raise SystemExit(1)

    # Repo-level security (PR, push, schedule, manual) — not the release image gate.
    sec = load("security.yml")
    if (sec.get("name") or "").strip() != "Security":
        print("ERROR: security.yml name must be exactly 'Security' (repo-level scans; distinct from Security Release).", file=sys.stderr)
        raise SystemExit(1)
    son = get_on_block(sec) or sec.get("on") or {}
    if not isinstance(son, dict):
        print("ERROR: security.yml must define on: as a mapping (e.g. push, pull_request).", file=sys.stderr)
        raise SystemExit(1)
    if "workflow_run" in son:
        print(
            "ERROR: security.yml must not declare on.workflow_run (repo Security is push/PR/schedule; "
            "the release image gate is security-release.yml).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    for ev in ("pull_request", "push", "workflow_dispatch"):
        if ev not in son:
            print(f"ERROR: security.yml should define on.{ev} (repo security scans: PR + push + manual).", file=sys.stderr)
            raise SystemExit(1)
    for key in ("pull_request", "push"):
        br = (son.get(key) or {}).get("branches") or []
        for b in ("develop", "main"):
            if b not in br:
                print(
                    f"ERROR: security.yml on.{key}.branches must include develop and main (found {br!r}).",
                    file=sys.stderr,
                )
                raise SystemExit(1)

    # Release security gate: triggered after Build and Push Images (workflow_run) + optional dispatch.
    srel = load("security-release.yml")
    if (srel.get("name") or "").strip() != "Security Release":
        print("ERROR: security-release.yml name must be exactly 'Security Release' (image scan + security-verdict).", file=sys.stderr)
        raise SystemExit(1)
    sron = get_on_block(srel) or srel.get("on") or {}
    if not isinstance(sron, dict):
        print("ERROR: security-release.yml must define on: as a mapping.", file=sys.stderr)
        raise SystemExit(1)
    if "workflow_run" not in sron:
        print(
            "ERROR: security-release.yml must have on.workflow_run from Build and Push Images "
            "(enterprise chain: Build completes -> Security Release image gate).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    s_wr = sron.get("workflow_run") or {}
    s_wfw = s_wr.get("workflows") or []
    if "Build and Push Images" not in s_wfw:
        print(
            "ERROR: security-release on.workflow_run.workflows must list 'Build and Push Images' (release gate follows Build).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    wtypes = _as_list(s_wr.get("types"))
    if "completed" not in wtypes:
        print("ERROR: security-release on.workflow_run.types must include 'completed' (run when Build finishes).", file=sys.stderr)
        raise SystemExit(1)
    if "workflow_dispatch" not in sron:
        print("ERROR: security-release should allow on.workflow_dispatch (manual verification of a Build).", file=sys.stderr)
        raise SystemExit(1)

    stg = load("deploy-develop.yml")
    st_on = get_on_block(stg) or stg.get("on") or {}
    st_wr = st_on.get("workflow_run") or {}
    wfw_st = st_wr.get("workflows") or []
    st_br = st_wr.get("branches") or []
    if "Security Release" not in wfw_st or "develop" not in st_br:
        print("ERROR: deploy-develop must run on workflow_run from Security Release, branches: [develop].", file=sys.stderr)
        raise SystemExit(1)
    stg_path = WF / "deploy-develop.yml"
    stg_text = stg_path.read_text(encoding="utf-8")
    if "ENABLE_REAL_STAGING_DEPLOY" not in stg_text:
        print(
            "ERROR: deploy-develop must reference vars.ENABLE_REAL_STAGING_DEPLOY to distinguish contract-only "
            "from real staging (see docs/runbooks/staging-preprod.md).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    if "contract-only, not a real staging deployment" not in stg_text:
        print(
            "ERROR: deploy-develop must state contract-only, not a real staging deployment in the contract-only path "
            "(workflow summary and JSON) so a no-op run is never mistaken for a real deploy.",
            file=sys.stderr,
        )
        raise SystemExit(1)
    if '"deployment_mode"' not in stg_text:
        print(
            "ERROR: deploy-develop must publish deployment_mode in staging evidence (contract_only vs real_staging).",
            file=sys.stderr,
        )
        raise SystemExit(1)

    prod = load("deploy-prod.yml")
    if (prod.get("name") or "").strip() != "Deploy Production":
        print("ERROR: deploy-prod.yml name must be exactly 'Deploy Production'.", file=sys.stderr)
        raise SystemExit(1)
    p_on = get_on_block(prod) or prod.get("on") or {}
    if not isinstance(p_on, dict):
        print("ERROR: deploy-prod must define on: as a mapping.", file=sys.stderr)
        raise SystemExit(1)
    if "workflow_run" in p_on:
        print(
            "ERROR: deploy-prod must not declare on.workflow_run (production is workflow_dispatch-only; no auto deploy from main).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    if "workflow_dispatch" not in p_on:
        print("ERROR: deploy-prod must allow workflow_dispatch (manual + rollback).", file=sys.stderr)
        raise SystemExit(1)

    text = (WF / "deploy-prod.yml").read_text(encoding="utf-8")
    if "environment: production" not in text:
        print("ERROR: deploy-prod must keep environment: production on the deployment job.", file=sys.stderr)
        raise SystemExit(1)
    if "Security Release" not in text:
        print("ERROR: deploy-prod must reference Security Release (e.g. security_release_run_id) for production evidence.", file=sys.stderr)
        raise SystemExit(1)
    if "run_migration" not in text or "backup_evidence_id" not in text:
        print(
            "ERROR: deploy-prod must define run_migration and backup_evidence_id inputs (pre-migration backup evidence contract).",
            file=sys.stderr,
        )
        raise SystemExit(1)

    print("OK: GitHub workflow CI/CD contract (offline YAML)")


if __name__ == "__main__":
    main()
