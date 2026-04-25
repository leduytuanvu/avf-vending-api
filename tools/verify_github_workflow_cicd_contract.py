#!/usr/bin/env python3
"""
Offline check that key GitHub Actions workflows are wired for the enterprise chain:
  CI -> Build and Push Images -> Security -> (develop: Staging contract) / (main: Deploy Production)
"""
from __future__ import annotations

import sys
from pathlib import Path

try:
    import yaml
except ImportError:  # pragma: no cover
    print("ERROR: PyYAML required (e.g. pip install pyyaml or apt install python3-yaml)", file=sys.stderr)
    sys.exit(1)

ROOT = Path(__file__).resolve().parents[1]
WF = ROOT / ".github" / "workflows"


def _normalize_gha_doc(raw: object) -> object:
    # PyYAML 1.1 parses the key "on" as boolean True. GitHub uses "on:" for triggers.
    if isinstance(raw, dict) and True in raw and "on" not in raw and isinstance(raw.get(True), dict):
        out = {k: v for k, v in raw.items() if k is not True}
        out["on"] = _normalize_gha_doc(raw[True])
        return out
    if isinstance(raw, dict):
        return {k: _normalize_gha_doc(v) for k, v in raw.items()}
    if isinstance(raw, list):
        return [_normalize_gha_doc(x) for x in raw]
    return raw


def load(name: str) -> dict:
    p = WF / name
    if not p.is_file():
        print(f"ERROR: missing {p}", file=sys.stderr)
        raise SystemExit(1)
    doc = yaml.safe_load(p.read_text(encoding="utf-8")) or {}
    return _normalize_gha_doc(doc)  # type: ignore[return-value]


def main() -> None:
    ci = load("ci.yml")
    on = ci.get("on")
    if isinstance(on, str):
        on = {on: None}
    else:
        on = on or {}
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
    bon = build.get("on") or {}
    if "workflow_run" not in bon:
        print("ERROR: build-push.yml must have on.workflow_run from CI.", file=sys.stderr)
        raise SystemExit(1)
    wr = bon["workflow_run"] or {}
    wfw = wr.get("workflows") or []
    if "CI" not in wfw:
        print("ERROR: build-push on.workflow_run.workflows must list 'CI'.", file=sys.stderr)
        raise SystemExit(1)
    if "workflow_dispatch" not in bon:
        print("ERROR: build-push must allow workflow_dispatch.", file=sys.stderr)
        raise SystemExit(1)

    sec = load("security.yml")
    if (sec.get("name") or "").strip() != "Security":
        print("ERROR: security.yml name must be exactly 'Security'.", file=sys.stderr)
        raise SystemExit(1)
    son = sec.get("on") or {}
    if "workflow_run" not in son:
        print("ERROR: security.yml must have on.workflow_run from Build and Push Images.", file=sys.stderr)
        raise SystemExit(1)
    s_wr = (son.get("workflow_run") or {}).get("workflows") or []
    if "Build and Push Images" not in s_wr:
        print("ERROR: security on.workflow_run.workflows must list 'Build and Push Images'.", file=sys.stderr)
        raise SystemExit(1)
    for ev in ("pull_request", "push", "workflow_dispatch"):
        if ev not in son:
            print(f"ERROR: security.yml should define on.{ev} (non-image + manual gates).", file=sys.stderr)
            raise SystemExit(1)

    stg = load("deploy-develop.yml")
    st_on = stg.get("on") or {}
    st_wr = (st_on.get("workflow_run") or {})
    wfw_st = st_wr.get("workflows") or []
    st_br = st_wr.get("branches") or []
    if "Security" not in wfw_st or "develop" not in st_br:
        print("ERROR: deploy-develop must run on workflow_run from Security, branches: [develop].", file=sys.stderr)
        raise SystemExit(1)

    prod = load("deploy-prod.yml")
    if (prod.get("name") or "").strip() != "Deploy Production":
        print("ERROR: deploy-prod.yml name must be exactly 'Deploy Production'.", file=sys.stderr)
        raise SystemExit(1)
    p_on = prod.get("on") or {}
    p_wr = (p_on.get("workflow_run") or {})
    wfw_p = p_wr.get("workflows") or []
    p_br = p_wr.get("branches") or []
    if "Security" not in wfw_p or "main" not in p_br:
        print("ERROR: deploy-prod must run on workflow_run from Security, branches: [main].", file=sys.stderr)
        raise SystemExit(1)
    if "workflow_dispatch" not in p_on:
        print("ERROR: deploy-prod must allow workflow_dispatch (manual + rollback).", file=sys.stderr)
        raise SystemExit(1)

    text = (WF / "deploy-prod.yml").read_text(encoding="utf-8")
    if "environment: production" not in text:
        print("ERROR: deploy-prod must keep environment: production on the deployment job.", file=sys.stderr)
        raise SystemExit(1)

    print("OK: GitHub workflow CI/CD contract (offline YAML)")


if __name__ == "__main__":
    main()
