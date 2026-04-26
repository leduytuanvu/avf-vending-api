#!/usr/bin/env python3
"""
Validate docs/contracts/deployment-secrets-contract.yml against deploy-develop / deploy-prod workflow YAML.

- Ensures every secrets.* and vars.* in those workflows is allowlisted in the contract.
- Ensures SSH key alias groups (OR-chains) document at least one member literally in the workflow.
- Fails if a workflow references GitHub secrets using names reserved for on-host .env (vps_env_operator_keys).
- Never logs secret values: only static names and file paths.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path
from typing import Any

try:
    import yaml
except ImportError:  # pragma: no cover
    print("ERROR: PyYAML required (pip install pyyaml or apt install python3-yaml)", file=sys.stderr)
    sys.exit(1)

ROOT = Path(__file__).resolve().parents[2]
CONTRACT = ROOT / "docs" / "contracts" / "deployment-secrets-contract.yml"

# Avoid false positives (e.g. "mysecrets.FOO" as a word) — GHA context uses `secrets.FOO` after non-id char.
SECRET_REF = re.compile(r"(?<![A-Za-z0-9_])secrets\.([A-Z][A-Z0-9_]*)\b")
VAR_REF = re.compile(r"(?<![A-Za-z0-9_])vars\.([A-Z][A-Z0-9_]*)\b")


def _load_contract() -> dict[str, Any]:
    if not CONTRACT.is_file():
        print(f"ERROR: missing {CONTRACT.relative_to(ROOT)}", file=sys.stderr)
        raise SystemExit(1)
    return yaml.safe_load(CONTRACT.read_text(encoding="utf-8", errors="replace")) or {}


def _refs(text: str) -> tuple[set[str], set[str]]:
    return set(SECRET_REF.findall(text)), set(VAR_REF.findall(text))


def _check_unknown(
    used: set[str],
    allow: set[str],
    kind: str,
) -> list[str]:
    return sorted(used - allow, key=str.lower)


def main() -> None:
    doc = _load_contract()
    ga = doc.get("github_actions") or {}
    allow_secrets: set[str] = set(ga.get("allowed_secrets") or [])
    allow_vars: set[str] = set(ga.get("allowed_variables") or [])
    allow_secrets |= set((doc.get("implicit") or {}).get("allowed_secret_names") or [])

    wf_map = {k: v for k, v in (doc.get("workflows") or {}).items() if isinstance(v, str)}

    texts: dict[str, str] = {}
    for label, relp in wf_map.items():
        path = ROOT / relp
        if not path.is_file():
            print(f"ERROR: workflow path from contract not found: {relp} (key {label})", file=sys.stderr)
            raise SystemExit(1)
        texts[relp] = path.read_text(encoding="utf-8", errors="replace")

    all_used_s: set[str] = set()
    all_used_v: set[str] = set()
    for relp, text in texts.items():
        s, v = _refs(text)
        all_used_s |= s
        all_used_v |= v
        u_s = _check_unknown(s, allow_secrets, "secret")
        u_v = _check_unknown(v, allow_vars, "variable")
        if u_s:
            print(
                f"ERROR: {relp} references unknown GitHub Actions secret name(s) (add to {CONTRACT.name} or remove): "
                f"{u_s}",
                file=sys.stderr,
            )
            raise SystemExit(1)
        if u_v:
            print(
                f"ERROR: {relp} references unknown GitHub Actions variable name(s) (add to {CONTRACT.name} or remove): "
                f"{u_v}",
                file=sys.stderr,
            )
            raise SystemExit(1)

    # Forbid remote-only .env key names as GitHub *secrets* (operator confusion)
    vps: set[str] = set((doc.get("vps_env_operator_keys") or []))
    bad_vps = sorted(vps & all_used_s, key=str.lower)
    if bad_vps:
        print(
            f"ERROR: GitHub secrets must not use names reserved for on-host .env (vps_env_operator_keys): {bad_vps}. "
            "Use distinct repository secret names; keep DATABASE_URL, etc. on the server only — see contract.",
            file=sys.stderr,
        )
        raise SystemExit(1)

    # Secret literal OR-groups (SSH identity aliases)
    for group in doc.get("secret_literal_groups") or []:
        wid = group.get("id", "?")
        relp = group.get("workflow")
        if not relp or not isinstance(relp, str):
            print(f"ERROR: secret_literal_groups entry {wid!r} missing workflow path", file=sys.stderr)
            raise SystemExit(1)
        text = texts.get(relp)
        if text is None:
            print(
                f"ERROR: secret_literal_groups workflow {relp!r} not in workflows map",
                file=sys.stderr,
            )
            raise SystemExit(1)
        any_of: list[str] = list(group.get("any_of") or [])
        if not any_of:
            continue
        ok = any(f"secrets.{m}" in text for m in any_of)
        if not ok:
            print(
                f"ERROR: secret group {wid!r} in {relp} requires a literal for at least one of: {any_of} "
                f"(OR-chain must reference one name so operators can discover it).",
                file=sys.stderr,
            )
            raise SystemExit(1)

    # Stale contract entries (informational, non-fatal): many aliases exist only in OR lists — skip strict mode

    # optional_operator_integrations must not be promoted to GitHub secret names
    oi = set((doc.get("optional_operator_integrations") or []))
    bad_oi = sorted(oi & all_used_s, key=str.lower)
    if bad_oi:
        print(
            f"ERROR: these keys are listed as operator/env-only in the contract but appear as "
            f"GitHub secrets in workflows: {bad_oi}",
            file=sys.stderr,
        )
        raise SystemExit(1)

    print(
        "OK: deployment config contract (workflows match docs/contracts/deployment-secrets-contract.yml; "
        "no forbidden secret name collisions)"
    )


if __name__ == "__main__":
    main()
