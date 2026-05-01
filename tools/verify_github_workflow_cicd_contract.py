#!/usr/bin/env python3
"""
Offline check that key GitHub Actions workflows are wired for the enterprise chain:
  CI -> Build and Push Images -> Security Release (release gate; image scan + security-verdict.json)
  Repo Security (push/PR): security.yml
  Staging: workflow_run from Security Release (develop)
  Production: workflow_dispatch only (evidence: Build + Security Release run ids; no auto workflow_run)
  Supply chain (Actions SHAs, prod images, go tools) is validated separately by
  scripts/ci/verify_supply_chain_pinning.sh (see tools/supply_chain_pinning.py), invoked from
  scripts/ci/verify_workflow_contracts.sh before this script runs.
"""
from __future__ import annotations

import re
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
MAKEFILE = ROOT / "Makefile"


def _makefile_target_names() -> set[str]:
    if not MAKEFILE.is_file():
        return set()
    out: set[str] = set()
    for line in MAKEFILE.read_text(encoding="utf-8", errors="replace").splitlines():
        s = line.split("#", 1)[0].rstrip()
        if not s or s[0:1] == "\t" or s.startswith((".PHONY", "include ")):
            continue
        m = re.match(r"^([A-Za-z_][A-Za-z0-9_.-]*)\s*:\s*([^#]*)", s)
        if not m:
            continue
        name, after = m.group(1), m.group(2)
        # Skip variable assignments: NAME :=, NAME =, etc.
        if re.match(r"^\s*[:?+]?=", after):
            continue
        if after.lstrip().startswith("="):
            continue
        if "%" in name or re.search(r"^\$", name):
            continue
        if "$" in name or "(" in name:
            continue
        out.add(name)
    return out


def _make_tokens_in_workflow_text(text: str) -> set[str]:
    """Collect make target words from run steps (GitHub workflow YAML; line-oriented)."""
    toks: set[str] = set()
    for line in text.splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        if "#" in stripped and not re.match(r"^#", stripped):
            stripped = stripped.split("#", 1)[0].rstrip()
        m1 = re.match(r"^run:\s*make(?:\s+(.*))?$", stripped)
        if m1:
            rest = m1.group(1) or ""
        elif re.search(r"^\bmake\s+", stripped):
            rest = re.sub(r"^\bmake\s+", "", stripped, count=1)
            rest = re.sub(r"(?:#.*)$", "", rest).rstrip()
        else:
            continue
        toks |= _split_run_make_args(rest)
    return toks


def _split_run_make_args(rest: str) -> set[str]:
    out: set[str] = set()
    for w in re.split(r"\s+", (rest or "").strip()):
        if w in ("&&", "||", "|", ";", "&", ">", ">>", "2>&1", "\\"):
            break
        if not w or w[0] in ("-", "$", "`", "("):
            break
        w = w.strip("\"'`")
        if not w:
            continue
        if not re.match(r"^[A-Za-z][A-Za-z0-9_.-]*$", w):
            break
        out.add(w)
    return out


def assert_workflow_make_targets() -> None:
    """Workflows in .github/workflows that invoke `make` must use targets defined in the root Makefile."""
    mnames = _makefile_target_names()
    if not mnames:
        print("ERROR: root Makefile missing or has no target entries.", file=sys.stderr)
        raise SystemExit(1)
    missing: list[str] = []
    for path in sorted(WF.glob("*.yml")):
        text = path.read_text(encoding="utf-8", errors="replace")
        for t in _make_tokens_in_workflow_text(text):
            if t in mnames:
                continue
            missing.append(f"{path.name}: make {t} (not in root Makefile)")

    if missing:
        print(
            "ERROR: .github/workflows references undefined Makefile targets (add to root Makefile or remove from workflow).",
            file=sys.stderr,
        )
        for m in missing:
            print(f"  {m}", file=sys.stderr)
        raise SystemExit(1)


def assert_reusable_deploy_attestation_verify_contract() -> None:
    """gh attestation verify: signer-workflow is workflow path only; branch enforced via --source-ref."""
    path = WF / "_reusable-deploy.yml"
    text = path.read_text(encoding="utf-8")
    if "gh attestation verify" not in text:
        print(
            "ERROR: _reusable-deploy.yml must run gh attestation verify (GitHub Artifact Attestation provenance gate).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    signer_ok = 'signer_workflow="github.com/${GITHUB_REPOSITORY}/.github/workflows/_reusable-build.yml"'
    if signer_ok not in text:
        print(
            "ERROR: _reusable-deploy.yml signer_workflow must be "
            'github.com/${GITHUB_REPOSITORY}/.github/workflows/_reusable-build.yml '
            "(no @refs/heads; gh CLI expects --signer-workflow as host/owner/repo/workflow path only).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    if "_reusable-build.yml@refs/heads" in text:
        print(
            "ERROR: _reusable-deploy.yml must not embed @refs/heads in the signer workflow string "
            "(use --source-ref refs/heads/${ARTIFACT_SOURCE_BRANCH}).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    if 'source_ref="refs/heads/${ARTIFACT_SOURCE_BRANCH}"' not in text:
        print(
            "ERROR: _reusable-deploy.yml must set source_ref=\"refs/heads/${ARTIFACT_SOURCE_BRANCH}\" "
            "for gh attestation verify --source-ref.",
            file=sys.stderr,
        )
        raise SystemExit(1)
    need = (
        '--source-ref "${source_ref}"',
        "--deny-self-hosted-runners",
        "--format json",
    )
    for needle in need:
        if needle not in text:
            print(
                f"ERROR: _reusable-deploy.yml gh attestation verify must include {needle!r}.",
                file=sys.stderr,
            )
            raise SystemExit(1)
    if "gh --version" not in text:
        print(
            "ERROR: _reusable-deploy.yml provenance verification step should run gh --version before attestation verify.",
            file=sys.stderr,
        )
        raise SystemExit(1)


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

    jobs = build.get("jobs") or {}
    pbd = jobs.get("publish-build-metadata")
    if not isinstance(pbd, dict):
        print("ERROR: build-push.yml must define jobs.publish-build-metadata.", file=sys.stderr)
        raise SystemExit(1)
    pbd_perms = pbd.get("permissions")
    need = {"contents": "read", "actions": "write", "packages": "read"}
    if pbd_perms != need:
        print(
            "ERROR: build-push jobs.publish-build-metadata.permissions must be exactly "
            f"{need!r} (actions/checkout, artifact upload/download, GHCR read); found {pbd_perms!r}.",
            file=sys.stderr,
        )
        raise SystemExit(1)

    reusable_path = WF / "_reusable-build.yml"
    reusable_text = reusable_path.read_text(encoding="utf-8")
    if re.search(r"cosign\s+verify", reusable_text):
        if "--output json" not in reusable_text:
            print(
                "ERROR: _reusable-build.yml: cosign verify must use --output json (JSON on stdout) before the image ref; "
                "keep redirects to cosign-verify-app.json / cosign-verify-goose.json for signing evidence.",
                file=sys.stderr,
            )
            raise SystemExit(1)
        for n, line in enumerate(reusable_text.splitlines(), 1):
            s = line.split("#", 1)[0]
            s = s.replace("--output json", " ")
            if re.search(r"(?<![A-Za-z0-9_])--json(?![A-Za-z0-9_-])", s):
                print(
                    "ERROR: _reusable-build.yml: cosign verify must not use the unsupported --json output flag; "
                    "use --output json before the digest-pinned ref (line "
                    f"{n}).",
                    file=sys.stderr,
                )
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

    dp_path = WF / "deploy-prod.yml"
    if dp_path.read_bytes().startswith(b"\xef\xbb\xbf"):
        print("ERROR: deploy-prod.yml must not start with a UTF-8 BOM (use plain UTF-8; BOM breaks name-line contract checks).", file=sys.stderr)
        raise SystemExit(1)
    prod = load("deploy-prod.yml")
    if (prod.get("name") or "").strip() != "Deploy Production":
        print("ERROR: deploy-prod.yml name must be exactly 'Deploy Production'.", file=sys.stderr)
        raise SystemExit(1)
    p_on = get_on_block(prod) or prod.get("on") or {}
    if not isinstance(p_on, dict):
        print("ERROR: deploy-prod must define on: as a mapping.", file=sys.stderr)
        raise SystemExit(1)
    wdk = set(p_on.keys())
    if wdk != {"workflow_dispatch"}:
        print(
            f"ERROR: deploy-prod on: must be exactly workflow_dispatch only (no on.workflow_run / on.push / etc.); "
            f"found keys: {sorted(wdk)}.",
            file=sys.stderr,
        )
        raise SystemExit(1)

    text = (WF / "deploy-prod.yml").read_text(encoding="utf-8")
    if "environment: production" not in text:
        print("ERROR: deploy-prod must keep environment: production on the deployment job.", file=sys.stderr)
        raise SystemExit(1)
    if "Security Release" not in text:
        print("ERROR: deploy-prod must reference Security Release (e.g. security_release_run_id) for production evidence.", file=sys.stderr)
        raise SystemExit(1)
    if "github.event.workflow_run" in text:
        print(
            "ERROR: deploy-prod must not reference github.event.workflow_run (production is workflow_dispatch-only; "
            "use operator inputs and gh api, not a parent workflow_run).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    if "source_commit_sha source_event run_status" in text:
        print(
            "ERROR: deploy-prod.yml must not bind the Build workflow GitHub API .event into a variable named "
            "`source_event` (semantic promotion source_event comes from security-verdict.json; "
            "use build_trigger_event for the API field).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    for needle in (
        "build_trigger_event",
        "semantic_source_event",
        "trigger_workflow_event=",
        "security-verdict-bundle/security-verdict.json",
        'p.get("source_event")',
        "TRIGGER_WORKFLOW_EVENT_BUILD_API",
    ):
        if needle not in text:
            print(
                f"ERROR: deploy-prod.yml missing {needle!r} "
                "(production immutable alignment uses semantic source_event from security-verdict, not Build workflow_run wrapper).",
                file=sys.stderr,
            )
            raise SystemExit(1)

    cand_pkg = ROOT / "scripts" / "release" / "write_production_deploy_candidate_package.py"
    cand_src = cand_pkg.read_text(encoding="utf-8")
    if "production-deploy-candidate-metadata.json" not in cand_src:
        print(
            "ERROR: write_production_deploy_candidate_package.py must write production-deploy-candidate-metadata.json "
            "(semantic source_event + diagnostic trigger_workflow_event bundle).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    if "run_migration" not in text or "backup_evidence_id" not in text:
        print(
            "ERROR: deploy-prod must define run_migration and backup_evidence_id inputs (pre-migration backup evidence contract).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    if "backup_evidence_id is required when run_migration=true" not in text:
        print(
            "ERROR: deploy-prod must fail validation when run_migration is true and backup_evidence_id is empty "
            "(expected explicit operator-facing error).",
            file=sys.stderr,
        )
        raise SystemExit(1)

    p_inputs = p_on.get("workflow_dispatch", {}) or {}
    wdi = p_inputs.get("inputs")
    if not isinstance(wdi, dict):
        print("ERROR: deploy-prod must define on.workflow_dispatch.inputs (mapping of workflow_dispatch inputs).", file=sys.stderr)
        raise SystemExit(1)
    run_migration_inp = wdi.get("run_migration")
    backup_ev_inp = wdi.get("backup_evidence_id")
    if not isinstance(run_migration_inp, dict) or str(run_migration_inp.get("type", "")).lower() != "boolean":
        print("ERROR: deploy-prod run_migration input must be type: boolean (safe default; migration is not silent).", file=sys.stderr)
        raise SystemExit(1)
    if run_migration_inp.get("default") is not False:
        print("ERROR: deploy-prod run_migration must use default: false (image-only by default; no silent migrations).", file=sys.stderr)
        raise SystemExit(1)
    if not isinstance(backup_ev_inp, dict) or str(backup_ev_inp.get("type", "")).lower() != "string":
        print("ERROR: deploy-prod backup_evidence_id input must be type: string (artifact/run/path/id).", file=sys.stderr)
        raise SystemExit(1)
    if backup_ev_inp.get("required") is True:
        print(
            "ERROR: deploy-prod backup_evidence_id must not be required: true (optional when run_migration is false).",
            file=sys.stderr,
        )
        raise SystemExit(1)

    for key, typ in (
        ("staging_evidence_id", "string"),
        ("staging_evidence_max_age_hours", "string"),
        ("allow_missing_staging_evidence", "boolean"),
        ("missing_staging_evidence_reason", "string"),
    ):
        field = wdi.get(key)
        if not isinstance(field, dict) or str(field.get("type", "")).lower() != typ:
            print(
                f"ERROR: deploy-prod on.workflow_dispatch.inputs.{key} must be type: {typ} (staging / pre-prod gate before production).",
                file=sys.stderr,
            )
            raise SystemExit(1)
    st_allow = wdi.get("allow_missing_staging_evidence")
    if st_allow is not None and st_allow.get("default") is not False:
        print(
            "ERROR: deploy-prod allow_missing_staging_evidence must default to false (strict by default; bypass is explicit).",
            file=sys.stderr,
        )
        raise SystemExit(1)

    stg_text = (WF / "deploy-develop.yml").read_text(encoding="utf-8")
    if "name: staging-release-evidence" not in stg_text:
        print("ERROR: deploy-develop.yml must upload an artifact named staging-release-evidence (legacy compat).", file=sys.stderr)
        raise SystemExit(1)
    if "name: staging-deploy-evidence" not in stg_text:
        print(
            "ERROR: deploy-develop.yml must upload an artifact named staging-deploy-evidence "
            "(staging-evidence/staging-deploy-evidence.json).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    stg_path = WF / "deploy-prod.yml"
    prod_st = stg_path.read_text(encoding="utf-8")
    if "WF_VALIDATE_STAGING_DEPLOY_EVIDENCE" not in prod_st:
        print(
            "ERROR: deploy-prod.yml must embed staging deploy evidence validation (WF_VALIDATE_STAGING_DEPLOY_EVIDENCE) "
            "for promotion_eligible, repository, digest match, and freshness (real pre-prod gate).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    for needle in (
        "staging_release_gate",
        "staging-deploy-evidence",
        "promotion_eligible",
        "INPUT_STAGING_EVIDENCE_MAX_AGE_HOURS",
    ):
        if needle not in prod_st:
            print(f"ERROR: deploy-prod.yml must include {needle!r} (staging pre-prod gate before production).", file=sys.stderr)
            raise SystemExit(1)
    for needle in (
        "validate_backup_evidence.py",
        "production-db-backup-evidence",
        "backup-evidence/backup-evidence.json",
    ):
        if needle not in prod_st:
            print(
                f"ERROR: deploy-prod.yml must include {needle!r} (backup / restore-drill evidence for run_migration).",
                file=sys.stderr,
            )
            raise SystemExit(1)
    for needle in (
        "docs/operations/two-vps-rolling-production-deploy.md",
        "rollout-timeline.json",
        "rollout_outcome_summary",
        "TRAFFIC_DRAIN_MODE",
        "traffic_drain_hook.sh",
        "docs/operations/production-smoke-tests.md",
        "SMOKE_LEVEL",
        "enable_business_synthetic_smoke",
        "emit_production_smoke_json.py",
        "build_release_evidence_package.py",
        "release-audit-package",
        "security-release-audit-package",
        "production-release-audit-package",
        "docs/operations/release-evidence-retention.md",
    ):
        if needle not in prod_st:
            print(
                f"ERROR: deploy-prod.yml must include {needle!r} (2-VPS / smoke / release evidence contract).",
                file=sys.stderr,
            )
            raise SystemExit(1)

    assert_reusable_deploy_attestation_verify_contract()
    assert_workflow_make_targets()
    print("OK: GitHub workflow CI/CD contract (offline YAML)")


if __name__ == "__main__":
    main()
