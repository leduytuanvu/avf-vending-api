#!/usr/bin/env python3
"""
Offline check that GitHub Actions workflows match the enterprise release architecture (enforced in code below).

- ci.yml: CI only (no deploy, no image publish).
- security.yml: repo Security scans only; not a deploy/verdict approval source; no security-verdict upload.
- build-push.yml: images + promotion artifacts after successful CI (push) or manual dispatch to develop/main.
- security-release.yml: final gate; produces security-reports/security-verdict.json and uploads security-verdict.
- deploy-develop (staging) / deploy-prod: only after Security Release; verdict=pass; digest-pinned images;
  develop vs main source_branch; deploy-prod uses environment: production.

- Action reference pinning: by default, emit stderr warnings (and a high-impact workflow inventory) for third-party
  actions not pinned to a commit SHA. Set env ENFORCE_ACTION_SHA_PINNING=true to fail on those (actions/* exempt).

Errors use CONTRACT VIOLATION: <file> + Violated contract + Expected behavior.
"""
from __future__ import annotations

import os
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


# High-impact paths: print an inventory of *third-party* (non-official) action references in the audit.
_ACTION_PINNING_CRITICAL_WORKFLOWS: frozenset[str] = frozenset(
    {
        "build-push.yml",
        "deploy-develop.yml",
        "deploy-prod.yml",
        "security-release.yml",
    }
)

# Treated as "official" for *enforce* mode: we never fail on these (tag pins allowed until a repo-wide SHA policy).
# Third-party (everything else) must use a commit SHA when ENFORCE_ACTION_SHA_PINNING is enabled.
_OFFICIAL_ACTION_PREFIXES: tuple[str, ...] = (
    "actions/",  # github.com/actions/…
    "github/codeql-action/",  # github.com/github/codeql-action
)


def _action_ref_is_commit_sha(ref: str) -> bool:
    """Heuristic: tag/branch (not SHA) vs git commit id after @ in uses: org/repo@ref."""
    s = (ref or "").strip()
    if len(s) < 7:
        return False
    if s in ("main", "master", "HEAD", "develop", "release"):
        return False
    # Typical semver or CalVer tags: v1, v1.2.0, 2024.01.01, …
    if re.match(r"^v[0-9]+(\.[0-9]+)*(-[0-9A-Za-z.-]+)?$", s):
        return False
    if s.startswith("v") and s[1:].isdigit():
        return False
    if re.fullmatch(r"[0-9a-fA-F]{7,40}", s) or re.fullmatch(r"[0-9a-fA-F]{64}", s):
        return True
    return False


def _action_spec_parts(spec: str) -> tuple[str, str] | None:
    """Return (path_before_at, ref) for owner/...@ref or None if not a pin-able remote uses: string."""
    s = (spec or "").strip().strip("'\"")
    if not s or s.startswith("${{") or s.startswith("docker://"):
        return None
    if s.startswith("./"):
        return None
    if "@" not in s:
        return None
    path_part, ref = s.rsplit("@", 1)
    if not path_part or not ref.strip():
        return None
    return (path_part.strip(), ref.strip())


def _is_official_github_action_path(path_part: str) -> bool:
    p = (path_part or "").strip()
    for prefix in _OFFICIAL_ACTION_PREFIXES:
        if p.startswith(prefix):
            return True
    return False


def _is_third_party_versioned_action_path(path_part: str) -> bool:
    """True if this is a non-local, non-official, non-docker remote action / reusable ref."""
    p = (path_part or "").strip()
    if not p or p.startswith(("./", "${{", "docker://")):
        return False
    if "/" not in p:
        return False
    return not _is_official_github_action_path(p)


def verify_action_reference_pinning_policy() -> None:
    """Check third-party `uses:` refs; warn on non-SHA by default; enforce with ENFORCE_ACTION_SHA_PINNING.

    Official `actions/*` and `github/codeql-action/*` are never *failed* in enforce mode (tag pins OK for now).
    Set VERIFY_ACTION_SHA_PINNING=1 to also list official tag/branch-based refs to stderr (extra audit noise).
    """
    enforce = os.environ.get("ENFORCE_ACTION_SHA_PINNING", "").strip().lower() in (
        "1",
        "true",
        "yes",
    )
    include_official_hints = os.environ.get("VERIFY_ACTION_SHA_PINNING", "").strip().lower() in (
        "1",
        "true",
        "yes",
    )
    critical_inventory: list[tuple[str, int, str, str, str, str, bool]] = []  # wname, line, spec, path, ref, kind, is_sha
    third_party_unpinned: list[str] = []
    official_unpinned: list[str] = []

    for path in sorted(WF.glob("*.yml")):
        raw = path.read_text(encoding="utf-8", errors="replace")
        for i, line in enumerate(raw.splitlines(), 1):
            stripped = line.split("#", 1)[0].strip()
            if not stripped.startswith("uses:"):
                continue
            spec = stripped.split("uses:", 1)[-1].strip().strip("'\"").strip()
            if not spec or spec.startswith("${{") or spec.startswith("docker://") or spec.startswith("./"):
                continue
            parts = _action_spec_parts(spec)
            if parts is None:
                continue
            path_part, ref = parts
            is_off = _is_official_github_action_path(path_part)
            is_t = _is_third_party_versioned_action_path(path_part) if not is_off else False
            is_sha = _action_ref_is_commit_sha(ref)
            label = f"{path.name}:{i}: {spec}"
            if path.name in _ACTION_PINNING_CRITICAL_WORKFLOWS and (is_t or is_off):
                critical_inventory.append(
                    (path.name, i, spec, path_part, ref, "third-party" if is_t else "official", is_sha)
                )
            if is_off and not is_sha and include_official_hints:
                official_unpinned.append(label)
            if is_t and not is_sha:
                third_party_unpinned.append(label)

    if critical_inventory:
        print(
            "ACTION_PINNING (audit) - third-party and official `uses` in build / Security Release / deploy workflows (inventory):",
            file=sys.stderr,
        )
        for wname, i, spec, _pp, ref, _kind, is_sha in critical_inventory:
            state = "pin=sha" if is_sha or _action_ref_is_commit_sha(ref) else "pin=tag/branch (review)"
            print(f"  {wname}:{i}: {spec}  ({_kind}; {state})", file=sys.stderr)
    if third_party_unpinned:
        print(
            "ACTION_PINNING (warn) - third-party actions not pinned to a commit SHA (set ENFORCE_ACTION_SHA_PINNING=true to fail CI):",
            file=sys.stderr,
        )
        for line in third_party_unpinned:
            print(f"  {line}", file=sys.stderr)
    if include_official_hints and official_unpinned:
        print(
            "ACTION_PINNING (info) - VERIFY_ACTION_SHA_PINNING=1: official actions/* (or codeql) on tag/branch (allowed; SHA recommended for high assurance):",
            file=sys.stderr,
        )
        for line in official_unpinned[:100]:
            print(f"  {line}", file=sys.stderr)
        if len(official_unpinned) > 100:
            print(f"  ... and {len(official_unpinned) - 100} more", file=sys.stderr)
    if enforce and third_party_unpinned:
        print("CONTRACT VIOLATION: .github/workflows (action reference pinning)", file=sys.stderr)
        print(
            "  Violated contract: third-party `uses` must be pinned to a full git commit SHA when "
            "ENFORCE_ACTION_SHA_PINNING is enabled; official `actions/*` and `github/codeql-action/*` are exempt.",
            file=sys.stderr,
        )
        print(
            "  Expected behavior: pin to a verified commit SHA, then record the change in CI_CD_FINAL_AUDIT.md; "
            "see docs/runbooks/supply-chain-security.md#github-actions-version-pins.",
            file=sys.stderr,
        )
        for line in third_party_unpinned:
            print(f"  Offending: {line}", file=sys.stderr)
        raise SystemExit(1)


def contract_violation(file_label: str, violated: str, expected: str) -> None:
    """Print a structured contract error (file, rule, fix) and exit non-zero."""
    print(f"CONTRACT VIOLATION: {file_label}", file=sys.stderr)
    print(f"  Violated contract: {violated}", file=sys.stderr)
    print(f"  Expected behavior: {expected}", file=sys.stderr)
    raise SystemExit(1)


def contract_fail(workflow_file: str, violated: str, expected: str) -> None:
    """Same as contract_violation for a workflow under .github/workflows/."""
    contract_violation(f".github/workflows/{workflow_file}", violated, expected)


# PyYAML 1.1 parses the key "on" as boolean True. Normalize to string key "on".
_TRUE = True


def _normalize_gha_doc(raw: object) -> object:
    if isinstance(raw, dict) and _TRUE in raw and "on" not in raw and isinstance(raw.get(_TRUE), (dict, str, type(None), list)):
        out = {k: v for k, v in raw.items() if k is not _TRUE}
        out["on"] = _normalize_gha_doc(raw[_TRUE])
        return out
    if isinstance(raw, dict):
        return {k: _normalize_gha_doc(v) for k, v in raw.items()}
    if isinstance(raw, list):
        return [_normalize_gha_doc(x) for x in raw]
    return raw


def get_on(data: dict) -> Any:
    """Read workflow `on:` triggers; supports data['on'] and data[True] (PyYAML boolean key)."""
    if not isinstance(data, dict):
        return None
    if "on" in data:
        return data.get("on")
    if _TRUE in data:
        return data.get(_TRUE)
    return None


# Backward-compatible name used elsewhere in docs/scripts.
get_on_block = get_on


def load(name: str) -> dict:
    p = WF / name
    if not p.is_file():
        print(f"ERROR: missing {p}", file=sys.stderr)
        raise SystemExit(1)
    doc = yaml.safe_load(p.read_text(encoding="utf-8")) or {}
    return _normalize_gha_doc(doc)  # type: ignore[return-value]


ALLOW_PACKAGES_WRITE = frozenset({"build-push.yml", "_reusable-build.yml"})
ALLOW_ID_TOKEN_WRITE = frozenset({"build-push.yml", "_reusable-build.yml", "deploy-prod.yml"})
ALLOW_DEPLOYMENTS_WRITE = frozenset(
    {
        "deploy-develop.yml",
        "deploy-prod.yml",
        "environment-separation-gates.yml",
        "telemetry-storm-staging.yml",
    }
)
ALLOW_ATTESTATIONS_WRITE = frozenset({"build-push.yml", "_reusable-build.yml"})
_DISALLOW_GLOB = frozenset({"write-all", "read-all"})


def _iter_permission_maps(data: dict) -> list[tuple[str, dict[str, object]]]:
    found: list[tuple[str, dict[str, object]]] = []
    top = data.get("permissions")
    if isinstance(top, dict):
        found.append(("workflow", top))
    jobs = data.get("jobs")
    if isinstance(jobs, dict):
        for jname, job in jobs.items():
            if not isinstance(job, dict):
                continue
            jp = job.get("permissions")
            if isinstance(jp, dict):
                found.append((f"jobs.{jname}", jp))
    return found


def _any_permission(blocks: list[dict[str, object]], scope: str, level: str) -> bool:
    for pm in blocks:
        if pm.get(scope) == level:
            return True
    return False


def verify_explicit_permissions() -> None:
    """Enforce least-privilege GITHUB_TOKEN: explicit permissions, no write-all, scoped elevated scopes."""
    for path in sorted(WF.glob("*.yml")):
        raw = path.read_text(encoding="utf-8", errors="replace")
        if re.search(r"(?m)^[ \t]*permissions:\s*(write-all|read-all)\s*$", raw, re.IGNORECASE):
            contract_fail(
                path.name,
                "YAML declares permissions: write-all or read-all",
                "List explicit scopes under permissions: (contents, actions, etc.); never use repository-wide write-all/read-all.",
            )
        doc = yaml.safe_load(raw) or {}
        doc = _normalize_gha_doc(doc)
        if not isinstance(doc, dict):
            contract_violation(
                f".github/workflows/{path.name}",
                "workflow root YAML is not a mapping",
                "Each workflow file must parse to a top-level mapping (jobs, on, permissions, …).",
            )

        top = doc.get("permissions")
        if top is None:
            contract_fail(
                path.name,
                "missing top-level permissions block",
                "Every workflow must declare explicit permissions: (do not rely on implicit GITHUB_TOKEN defaults).",
            )
        if isinstance(top, str):
            norm = top.strip().lower().replace("_", "-")
            if norm in _DISALLOW_GLOB:
                contract_fail(
                    path.name,
                    f"permissions uses broad token scope {top!r}",
                    "Replace with an explicit permissions: mapping listing only required scopes (never write-all / read-all).",
                )
            contract_fail(
                path.name,
                f"top-level permissions is a string {top!r}",
                "Use a permissions: mapping (key/value scopes), not a shorthand string.",
            )
        if not isinstance(top, dict) or not top:
            contract_fail(
                path.name,
                "top-level permissions is empty or not a mapping",
                "Declare a non-empty permissions: block with least-privilege scopes.",
            )

        blocks = [pm for _, pm in _iter_permission_maps(doc)]
        for loc, pmap in _iter_permission_maps(doc):
            for scope, val in pmap.items():
                if isinstance(val, str) and val.strip().lower().replace("_", "-") in _DISALLOW_GLOB:
                    contract_fail(
                        path.name,
                        f"{loc}: permission {scope!r} uses disallowed value {val!r}",
                        "Never use write-all or read-all; set explicit scopes per job or workflow.",
                    )
                if scope == "contents" and val == "write":
                    contract_fail(
                        path.name,
                        f"{loc}: contents: write is not allowed",
                        "Use contents: read unless a documented exception applies repo-wide.",
                    )

        name = path.name
        if _any_permission(blocks, "packages", "write") and name not in ALLOW_PACKAGES_WRITE:
            contract_violation(
                f".github/workflows/{name}",
                f"packages: write is present but file is not in allowlist {sorted(ALLOW_PACKAGES_WRITE)}",
                "GHCR image publish is restricted to Build workflows; drop packages: write or add a reviewed allowlist exception.",
            )
        if _any_permission(blocks, "id-token", "write") and name not in ALLOW_ID_TOKEN_WRITE:
            contract_violation(
                f".github/workflows/{name}",
                f"id-token: write is present but file is not in allowlist {sorted(ALLOW_ID_TOKEN_WRITE)}",
                "OIDC id-token write is only for image signing / production deploy jobs that need it.",
            )
        if _any_permission(blocks, "deployments", "write") and name not in ALLOW_DEPLOYMENTS_WRITE:
            contract_violation(
                f".github/workflows/{name}",
                f"deployments: write is present but file is not in allowlist {sorted(ALLOW_DEPLOYMENTS_WRITE)}",
                "GitHub Deployments API write is only for deploy / environment-gate workflows.",
            )
        if _any_permission(blocks, "attestations", "write") and name not in ALLOW_ATTESTATIONS_WRITE:
            contract_violation(
                f".github/workflows/{name}",
                f"attestations: write is present but file is not in allowlist {sorted(ALLOW_ATTESTATIONS_WRITE)}",
                "Artifact attestation writes belong in the reusable build path only.",
            )

        for risky_scope in ("pull-requests", "issues", "discussions"):
            if _any_permission(blocks, risky_scope, "write"):
                contract_violation(
                    f".github/workflows/{name}",
                    f"{risky_scope}: write is granted at workflow or job level",
                    "Avoid broad PR/issue/discussion write scopes unless a documented exception applies.",
                )

        if "actions/upload-artifact" in raw and not _any_permission(blocks, "actions", "write"):
            contract_violation(
                f".github/workflows/{name}",
                "uses actions/upload-artifact without actions: write",
                "Grant actions: write where artifact uploads occur (GHA token needs write on actions scope for uploads).",
            )

        uses_download = (
            "actions/download-artifact" in raw
            or "action-download-artifact" in raw
            or "dawidd6/action-download-artifact" in raw
        )
        if uses_download and not (
            _any_permission(blocks, "actions", "read") or _any_permission(blocks, "actions", "write")
        ):
            contract_violation(
                f".github/workflows/{name}",
                "downloads artifacts but does not grant actions: read or actions: write",
                "Artifact download with GITHUB_TOKEN requires actions: read or write on the token.",
            )

    # workflow_run artifact reads use the Actions API — require actions: read or write on the consumer workflow.
    for consumer in ("security-release.yml", "deploy-develop.yml", "deploy-prod.yml", "build-push.yml"):
        p = WF / consumer
        if not p.is_file():
            continue
        raw = p.read_text(encoding="utf-8", errors="replace")
        doc = _normalize_gha_doc(yaml.safe_load(raw) or {})
        if not isinstance(doc, dict):
            continue
        blocks = [pm for _, pm in _iter_permission_maps(doc)]
        if not (
            _any_permission(blocks, "actions", "read") or _any_permission(blocks, "actions", "write")
        ):
            contract_violation(
                f".github/workflows/{consumer}",
                "must grant actions: read or actions: write (workflow uses workflow_run and/or artifacts)",
                "These workflows list other runs' artifacts; the token must include actions: read or write.",
            )


def _as_list(x: object) -> list:
    if x is None:
        return []
    if isinstance(x, list):
        return x
    if isinstance(x, str):
        return [x]
    return [x]


def _workflow_run_workflows(on_map: dict) -> list[str]:
    wr = on_map.get("workflow_run") or {}
    raw = wr.get("workflows") or []
    out: list[str] = []
    for item in _as_list(raw):
        if isinstance(item, str):
            out.append(item.strip())
    return out


def _assert_deploy_not_build_or_repo_security(label: str, workflows: list[str]) -> None:
    for w in workflows:
        if w == "Security Release":
            continue
        if w == "Security":
            print(
                f"ERROR: {label} must not use on.workflow_run from repo 'Security' (security.yml). "
                "Security Release is the only release gate for deployments.",
                file=sys.stderr,
            )
            raise SystemExit(1)
        if w in ("CI", "Build and Push Images"):
            print(
                f"ERROR: {label} must not be triggered by '{w}' success alone. "
                "Use workflow_run from Security Release only (Build is not a deployment trigger).",
                file=sys.stderr,
            )
            raise SystemExit(1)
        print(f"ERROR: {label} on.workflow_run has disallowed workflow {w!r} (expected only 'Security Release').", file=sys.stderr)
        raise SystemExit(1)


def _read_text(name: str) -> str:
    return (WF / name).read_text(encoding="utf-8")


def verify_pull_request_workflows_never_deploy() -> None:
    """on.pull_request must not deploy, target production/staging environments, or call deploy workflows."""
    for path in sorted(WF.glob("*.yml")):
        doc = yaml.safe_load(path.read_text(encoding="utf-8", errors="replace")) or {}
        doc = _normalize_gha_doc(doc)  # type: ignore[assignment]
        on_block = get_on(doc) or {}
        if not isinstance(on_block, dict) or "pull_request" not in on_block:
            continue
        raw = path.read_text(encoding="utf-8", errors="replace")
        if re.search(r"(?m)^[ \t]*uses:\s*\./\.github/workflows/deploy-", raw):
            contract_violation(
                f".github/workflows/{path.name}",
                "workflow declares on.pull_request and calls ./.github/workflows/deploy-* (reusable deploy workflow path)",
                "PRs run CI/Security/contract checks only; deploy workflows are triggered from Security Release after merge/push, not from pull_request.",
            )
        if re.search(r"(?m)^[ \t]*environment:\s*(production|staging)\s*(#.*)?$", raw, re.IGNORECASE):
            contract_violation(
                f".github/workflows/{path.name}",
                "workflow declares on.pull_request and sets environment: production or environment: staging on a job",
                "Do not use deployment environments on PR workflows; use deploy-develop.yml / deploy-prod.yml after Security Release verdict=pass.",
            )


def verify_codeql_workflow() -> None:
    """CodeQL must target develop/main on PR and push, run on a schedule, and gate on ENABLE_CODE_SCANNING."""
    path = WF / "codeql.yml"
    if not path.is_file():
        print("ERROR: .github/workflows/codeql.yml is required.", file=sys.stderr)
        raise SystemExit(1)
    doc = load("codeql.yml")
    if (doc.get("name") or "").strip() != "CodeQL":
        contract_fail("codeql.yml", "workflow name must be CodeQL", "Keep display name CodeQL for governance lists.")
    con = get_on(doc) or {}
    if not isinstance(con, dict):
        contract_fail("codeql.yml", "on: is not a mapping", "Declare CodeQL triggers as a mapping.")
    for trig in ("pull_request", "push", "schedule"):
        if trig not in con:
            contract_fail(
                "codeql.yml",
                f"missing on.{trig}",
                "CodeQL must run on pull_request, push (develop/main), and schedule.",
            )
    for trig in ("pull_request", "push"):
        br = (con.get(trig) or {}).get("branches") or []
        for b in ("develop", "main"):
            if b not in br:
                contract_fail(
                    "codeql.yml",
                    f"on.{trig}.branches missing {b!r}",
                    "Scope CodeQL to develop and main.",
                )
    jobs = doc.get("jobs") or {}
    analyze = jobs.get("analyze")
    if not isinstance(analyze, dict):
        contract_fail("codeql.yml", "missing jobs.analyze", "Define the CodeQL analyze job.")
    a_if = analyze.get("if")
    if a_if is None or "ENABLE_CODE_SCANNING" not in str(a_if):
        contract_fail(
            "codeql.yml",
            "analyze job must gate on vars.ENABLE_CODE_SCANNING",
            "Document intentional CodeQL skip when the repository variable is not 'true' (see github-governance runbook).",
        )
    cq_txt = _read_text("codeql.yml")
    if "vars.ENABLE_CODE_SCANNING == 'true'" not in cq_txt:
        contract_fail(
            "codeql.yml",
            "analyze job must use vars.ENABLE_CODE_SCANNING == 'true' (string)",
            "CodeQL is intentionally off until the org sets repository variable ENABLE_CODE_SCANNING to the string true (Advanced Security / licensing). A skipped job is not a passing scan.",
        )
    if "github/codeql-action/init@v3" not in cq_txt:
        contract_fail("codeql.yml", "missing codeql-action init", "Initialize CodeQL via github/codeql-action.")


def verify_enterprise_architecture() -> None:
    """Enforce the target chain: CI+Security on PR/push; Build after CI; Security Release after Build; Deploy after Security Release."""
    ci = load("ci.yml")
    ci_jobs = ci.get("jobs") or {}
    if not isinstance(ci_jobs, dict):
        contract_fail("ci.yml", "jobs: is not a mapping", "Define jobs as a YAML mapping.")
    for job_key in ("workflow-script-quality", "go-ci", "compose-config"):
        if job_key not in ci_jobs:
            contract_fail(
                "ci.yml",
                f"missing required job key {job_key!r}",
                "ci.yml must define workflow-script-quality, go-ci, and compose-config (Workflow and Script Quality, Go CI Gates, Docker Compose Config Validation).",
            )
    ci_txt = _read_text("ci.yml")
    for needle, desc in (
        ("name: Go CI Gates", "Go CI Gates job title"),
        ("name: Workflow and Script Quality", "Workflow and Script Quality job title"),
        ("name: Docker Compose Config Validation", "Docker Compose Config Validation job title"),
        ("scripts/ci/verify_workflow_contracts.sh", "scripts/ci/verify_workflow_contracts.sh invocation"),
    ):
        if needle not in ci_txt:
            contract_fail(
                "ci.yml",
                f"missing {desc}",
                "Keep required gates and the shell contract verifier in ci.yml.",
            )
    if re.search(r"(?m)^[ \t]*environment:[ \t]*(production|staging)[ \t]*$", ci_txt):
        contract_fail(
            "ci.yml",
            "uses GitHub Environment production or staging",
            "CI must not deploy; staging/production environments belong in deploy workflows only.",
        )
    if "uses: ./.github/workflows/deploy-" in ci_txt:
        contract_fail(
            "ci.yml",
            "invokes a deploy workflow via workflow call",
            "CI must not trigger deploy-develop or deploy-prod.",
        )

    sec = load("security.yml")
    sec_jobs = sec.get("jobs") or {}
    if not isinstance(sec_jobs, dict):
        contract_fail("security.yml", "jobs: is not a mapping", "Define Security jobs as a mapping.")
    for job_key in ("govulncheck-pr", "secret-scan", "config-scan"):
        if job_key not in sec_jobs:
            contract_fail(
                "security.yml",
                f"missing required job key {job_key!r}",
                "security.yml must run repo-level govulncheck, secret scan, and config scan jobs.",
            )
    sec_txt = _read_text("security.yml")
    if "dependency-review" not in sec_txt:
        contract_fail(
            "security.yml",
            "missing dependency-review job",
            "Declare dependency-review (even if gated by vars.ENABLE_DEPENDENCY_REVIEW) for PR supply-chain review.",
        )
    if "name: security-verdict" in sec_txt:
        contract_fail(
            "security.yml",
            "references security-verdict artifact upload",
            "Repo Security must not publish security-verdict; only Security Release emits that artifact for deploy gates.",
        )
    if re.search(r"(?m)^[ \t]*environment:[ \t]*(production|staging)[ \t]*$", sec_txt):
        contract_fail(
            "security.yml",
            "uses GitHub Environment production or staging",
            "Repo Security is scan-only; it must not target deployment environments.",
        )

    for job_key in ("secret-scan", "config-scan"):
        job = sec_jobs[job_key]
        if_j = job.get("if")
        if if_j is not None:
            if_s = str(if_j).strip().lower().replace(" ", "")
            if if_s in ("false", "${{false}}"):
                contract_fail(
                    "security.yml",
                    f"job {job_key} uses if: false",
                    "Blocking scan jobs must not be disabled with a constant-false condition.",
                )
    def _blocking_job_if_ok(if_str: str) -> bool:
        s = if_str
        if "vars." in s:
            return False
        return (
            "pull_request" in s
            and "push" in s
            and "workflow_dispatch" in s
        )

    for job_key, label in (
        ("govulncheck-pr", "Go Vulnerability Scan"),
        ("secret-scan", "Secret Scan"),
        ("config-scan", "Deployment and Config Scan"),
    ):
        jif = str((sec_jobs.get(job_key) or {}).get("if") or "")
        if not _blocking_job_if_ok(jif):
            contract_fail(
                "security.yml",
                f"job {job_key} if: must run on pull_request, push, and workflow_dispatch (no var gates)",
                f"{label} is blocking; it must not be skipped via repository variables. Match govulncheck-pr, secret-scan, and config-scan.",
            )
    dr_job = sec_jobs.get("dependency-review") or {}
    dr_if = str(dr_job.get("if") or "")
    dr_norm = dr_if.replace(" ", "")
    if "github.event_name=='pull_request'" not in dr_norm and "github.event_name == 'pull_request'" not in dr_if:
        contract_fail(
            "security.yml",
            "dependency-review must gate on github.event_name == 'pull_request'",
            "dependency-review-action is PR-only; never run this job on push.",
        )
    if "ENABLE_DEPENDENCY_REVIEW" not in dr_if:
        contract_fail(
            "security.yml",
            "dependency-review must reference vars.ENABLE_DEPENDENCY_REVIEW",
            "Document the org toggle for Dependency Review (skipped when var is not 'true').",
        )
    if re.search(r"github\.event_name\s*==\s*'push'", dr_if):
        contract_fail(
            "security.yml",
            "dependency-review if: must not require push",
            "dependency-review-action does not support push events.",
        )

    build = load("build-push.yml")
    bon = get_on(build) or {}
    if not isinstance(bon, dict):
        contract_fail("build-push.yml", "on: is not a mapping", "Declare triggers as a mapping.")
    if "push" in bon:
        contract_fail(
            "build-push.yml",
            "declares on.push",
            "Images must build only after successful CI on push (on.workflow_run) or via on.workflow_dispatch to develop/main — not a direct push trigger to this workflow.",
        )
    wr_b = bon.get("workflow_run") or {}
    br_b = wr_b.get("branches") or []
    for b in ("develop", "main"):
        if b not in br_b:
            contract_fail(
                "build-push.yml",
                f"on.workflow_run.branches missing {b!r}",
                "Release images are only for develop and main; both must be listed under workflow_run.branches.",
            )
    wfw_b = _as_list(wr_b.get("workflows") or [])
    if not wfw_b:
        contract_fail(
            "build-push.yml",
            "on.workflow_run.workflows is empty",
            "List exactly the CI workflow (display name 'CI') under workflow_run.workflows.",
        )
    for w in wfw_b:
        if w != "CI":
            contract_fail(
                "build-push.yml",
                f"workflow_run listens to {w!r}",
                "Build and Push Images must chain only from workflow CI success, never from Security or Security Release.",
            )

    srel = load("security-release.yml")
    sron = get_on(srel) or {}
    s_wfw = _workflow_run_workflows(sron)
    if not s_wfw:
        contract_fail(
            "security-release.yml",
            "workflow_run.workflows is empty",
            "Security Release must declare on.workflow_run.workflows: [Build and Push Images].",
        )
    for w in s_wfw:
        if w != "Build and Push Images":
            contract_fail(
                "security-release.yml",
                f"workflow_run listens to {w!r}",
                "Security Release must run only after 'Build and Push Images' completes (not directly after repo Security).",
            )
    sr_txt = _read_text("security-release.yml")
    for needle, desc in (
        ("cosign verify", "published image cosign verification"),
        ("trivy", "Trivy image vulnerability scan"),
        ("write_security_verdict.py", "structured verdict writer"),
        ("PROVENANCE_ENFORCEMENT", "provenance enforcement repo variable wiring"),
        ("ALLOW_PRIVATE_REPO_PROVENANCE_FALLBACK", "private-repo provenance fallback toggle"),
    ):
        if needle.lower() not in sr_txt.lower():
            contract_fail(
                "security-release.yml",
                f"missing {desc}",
                "Security Release must scan images, verify signatures as implemented, and write verdict JSON via write_security_verdict.py.",
            )

    wsv = (ROOT / "scripts" / "security" / "write_security_verdict.py").read_text(
        encoding="utf-8", errors="replace"
    )
    for needle, desc in (
        ("CONTRACT_ROOT_STRING_KEYS", "machine-readable top-level string keys in security-verdict.json"),
        ("_ensure_contract_baseline", "deterministic baseline for security-verdict.json"),
        ("app_image_ref", "top-level app image ref in verdict JSON (mirrors published_images)"),
    ):
        if needle not in wsv:
            contract_fail(
                "scripts/security/write_security_verdict.py",
                f"missing {desc}",
                "Security Release verdict must always include the full contract (pass/fail/skipped/no-candidate + refs + job_results).",
            )
    emit_out = (ROOT / "scripts" / "security" / "emit_security_verdict_outputs.py").read_text(
        encoding="utf-8", errors="replace"
    )
    for needle, desc in (
        ("_image_refs", "emit GITHUB_OUTPUT image refs from verdict JSON (not guessed)"),
        ("generated_at_utc", "emit generated_at_utc from JSON"),
    ):
        if needle not in emit_out:
            contract_fail(
                "scripts/security/emit_security_verdict_outputs.py",
                f"missing {desc}",
                "Signal step outputs must be derived from security-verdict.json for observability and downstream gates.",
            )

    dd_txt = _read_text("deploy-develop.yml")
    vr_early = (ROOT / "scripts" / "release" / "validate_release_verdict.py").read_text(encoding="utf-8", errors="replace")
    if 'out("staging_verdict", "skipped")' in dd_txt or "out('staging_verdict', 'skipped')" in dd_txt:
        contract_fail(
            "deploy-develop.yml",
            "sets staging_verdict to skipped as a success outcome",
            "skipped/no-candidate/fail must fail the deploy workflow; only verdict pass may proceed to image resolution.",
        )
    _non_pass = 'verdict in ("skipped", "no-candidate", "fail")'
    if _non_pass not in dd_txt and _non_pass not in vr_early:
        contract_fail(
            "deploy-develop.yml / validate_release_verdict.py",
            "does not group non-pass verdicts for fail-closed handling",
            "Deploy develop must reject skipped, no-candidate, and fail with exit 1 (non-pass Security Release verdict).",
        )

    dp_txt = _read_text("deploy-prod.yml")
    if "skipped_no_production_candidate" in dp_txt:
        contract_fail(
            "deploy-prod.yml",
            "still references skipped_no_production_candidate",
            "Production must fail closed when Security Release verdict is not pass (no neutral skip action_mode).",
        )
    if 'verdict in ("skipped", "no-candidate", "fail")' not in dp_txt:
        contract_fail(
            "deploy-prod.yml",
            "does not group non-pass verdicts for fail-closed handling",
            "Deploy production must reject skipped, no-candidate, and fail (non-pass Security Release verdict).",
        )
    for wf_file, txt in (("deploy-develop.yml", dd_txt), ("deploy-prod.yml", dp_txt)):
        if "actions/workflows/security.yml/runs" in txt:
            contract_fail(
                wf_file,
                "calls GitHub API for security.yml workflow runs",
                "Release verdict must come only from Security Release security-verdict artifact, not repo Security workflow runs.",
            )
        if "github.event.workflow_run.name == 'Security Release'" not in txt:
            contract_fail(
                wf_file,
                "no job 'if' condition gates on github.event.workflow_run.name == 'Security Release'",
                "Automatic deploy steps must be reachable only when the upstream workflow is Security Release (not repo Security or Build and Push Images).",
            )
        if "need_digest_pinned" not in txt and "digest-pinned" not in txt and "@sha256" not in txt:
            contract_fail(
                wf_file,
                "missing digest-pinned / @sha256 image validation",
                "Deploy workflows must require digest-pinned image refs (need_digest_pinned, digest-pinned, or @sha256 in checks) — no mutable tag deploys.",
            )
    if "*\":latest\"*" not in dp_txt and "latest tag" not in dp_txt:
        contract_fail(
            "deploy-prod.yml",
            "missing inline checks that reject :latest (mutable) tags on app/goose image refs",
            "Production deploy (including rollback) must fail when an image ref uses the latest tag; use digests only.",
        )
    for wf_name, ttxt, need in (
        ("deploy-develop.yml", dd_txt, "source_branch must be develop"),
        ("deploy-prod.yml", dp_txt, "source_branch in security-verdict must be main"),
    ):
        if need not in ttxt and need not in vr_early:
            contract_fail(
                wf_name,
                f"missing branch gate string {need!r} (or equivalent in validate_release_verdict.py)",
                "Deploy must align security-verdict source_branch with the target line (develop vs main).",
            )

    verify_codeql_workflow()


def main() -> None:
    verify_explicit_permissions()

    ci = load("ci.yml")
    on_ci = get_on(ci)
    if isinstance(on_ci, str):
        on_ci = {on_ci: None}
    else:
        on_ci = on_ci or {}
    if not isinstance(on_ci, dict):
        print("ERROR: ci.yml must define on: as a mapping (triggers).", file=sys.stderr)
        raise SystemExit(1)
    if "pull_request" not in on_ci or "push" not in on_ci:
        print("ERROR: ci.yml must define on: pull_request and on: push (develop + main).", file=sys.stderr)
        raise SystemExit(1)
    for key in ("pull_request", "push"):
        br = (on_ci.get(key) or {}).get("branches") or []
        for b in ("develop", "main"):
            if b not in br:
                print(f"ERROR: ci.yml on.{key}.branches must include develop and main (found {br!r}).", file=sys.stderr)
                raise SystemExit(1)

    ci_txt = _read_text("ci.yml")
    if "Migration Safety Check" not in ci_txt:
        print("ERROR: ci.yml must define a 'Migration Safety Check' job.", file=sys.stderr)
        raise SystemExit(1)
    if "scripts/ci/verify_migrations.sh" not in ci_txt:
        print("ERROR: ci.yml must invoke scripts/ci/verify_migrations.sh (migration safety gate).", file=sys.stderr)
        raise SystemExit(1)

    build = load("build-push.yml")
    if (build.get("name") or "").strip() != "Build and Push Images":
        print("ERROR: build-push.yml name must be exactly 'Build and Push Images' (for workflow_run wiring).", file=sys.stderr)
        raise SystemExit(1)
    bon = get_on(build) or {}
    if not isinstance(bon, dict):
        print("ERROR: build-push.yml must have a mapping in on: (triggers).", file=sys.stderr)
        raise SystemExit(1)
    if "workflow_run" not in bon:
        print("ERROR: build-push.yml must have on.workflow_run from CI.", file=sys.stderr)
        raise SystemExit(1)
    wr_b = bon.get("workflow_run") or {}
    wfw_b = wr_b.get("workflows") or []
    if "CI" not in wfw_b:
        print("ERROR: build-push on.workflow_run.workflows must list 'CI'.", file=sys.stderr)
        raise SystemExit(1)
    if "workflow_dispatch" not in bon:
        print("ERROR: build-push must allow workflow_dispatch.", file=sys.stderr)
        raise SystemExit(1)

    build_txt = _read_text("build-push.yml")
    for needle, label in (
        ("name: promotion-manifest", "promotion-manifest artifact"),
        ("name: immutable-image-contract", "immutable-image-contract artifact"),
        ("name: sbom-reports", "sbom-reports artifact"),
        ("name: cosign-signing-evidence", "cosign-signing-evidence artifact"),
        ("name: release-candidate", "release-candidate artifact"),
    ):
        if needle not in build_txt:
            print(f"ERROR: build-push.yml must upload {label} for downstream Security Release / digest contract.", file=sys.stderr)
            raise SystemExit(1)
    if "uses: ./.github/workflows/_reusable-build.yml" not in build_txt:
        print("ERROR: build-push.yml must call ./.github/workflows/_reusable-build.yml (publishes image-metadata for Security Release).", file=sys.stderr)
        raise SystemExit(1)
    rb_txt = _read_text("_reusable-build.yml")
    if "name: image-metadata" not in rb_txt:
        print("ERROR: _reusable-build.yml must upload the image-metadata artifact (digest-pinned contract for release chain).", file=sys.stderr)
        raise SystemExit(1)
    if "cosign sign" not in rb_txt and "Cosign sign" not in rb_txt:
        print(
            "ERROR: _reusable-build.yml must cosign-sign digest-pinned images (keyless signing for supply chain).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    if "@sha256:" not in build_txt and "sha256:" not in build_txt:
        print(
            "ERROR: build-push.yml should reference digest-pinned images (@sha256:) for immutable release artifacts.",
            file=sys.stderr,
        )
        raise SystemExit(1)

    sec = load("security.yml")
    if (sec.get("name") or "").strip() != "Security":
        print("ERROR: security.yml name must be exactly 'Security' (repo-level scans; distinct from Security Release).", file=sys.stderr)
        raise SystemExit(1)
    son = get_on(sec) or {}
    if not isinstance(son, dict):
        print("ERROR: security.yml must define on: as a mapping (e.g. push, pull_request).", file=sys.stderr)
        raise SystemExit(1)
    if "workflow_run" in son:
        print(
            "ERROR: security.yml must not declare on.workflow_run (repo Security is not a deployment trigger; "
            "security-release.yml must have on.workflow_run from Build and Push Images).",
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

    srel = load("security-release.yml")
    if (srel.get("name") or "").strip() != "Security Release":
        print("ERROR: security-release.yml name must be exactly 'Security Release' (image scan + security-verdict).", file=sys.stderr)
        raise SystemExit(1)
    sron = get_on(srel) or {}
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

    sr_txt = _read_text("security-release.yml")
    for needle in (
        "security-reports/security-verdict.json",
        "name: security-verdict",
        "name: release-manifest",
        "tools/generate_release_manifest.py",
        "blocking security verdict is empty",
        "emergency",
        "PROVENANCE_ENFORCEMENT",
        "ALLOW_PRIVATE_REPO_PROVENANCE_FALLBACK",
        "provenance_enforcement:",
    ):
        if needle not in sr_txt:
            print(
                f"ERROR: security-release.yml must contain {needle!r} "
                "(verdict JSON, artifact upload, enforce step, or emergency fallback).",
                file=sys.stderr,
            )
            raise SystemExit(1)
    if "RESOLVED_SOURCE_SHA" not in sr_txt or "RESOLVED_SOURCE_BRANCH" not in sr_txt:
        print(
            "ERROR: security-release.yml must prioritize RESOLVED_SOURCE_SHA / RESOLVED_SOURCE_BRANCH from artifacts over trigger metadata.",
            file=sys.stderr,
        )
        raise SystemExit(1)
    if "scripts/release/security_release_signal.sh" not in sr_txt:
        print(
            "ERROR: security-release.yml must invoke scripts/release/security_release_signal.sh (orchestration-only signal step).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    if "release-candidate" not in sr_txt or "SBOM_POLICY" not in sr_txt:
        print(
            "ERROR: security-release.yml must preflight the Build `release-candidate` artifact and pass SBOM_POLICY into the security signal.",
            file=sys.stderr,
        )
        raise SystemExit(1)
    _sig = ROOT / "scripts" / "release" / "security_release_signal.sh"
    if not _sig.is_file():
        print("ERROR: missing scripts/release/security_release_signal.sh", file=sys.stderr)
        raise SystemExit(1)
    sig_txt = _sig.read_text(encoding="utf-8", errors="replace")
    if "WORKFLOW_SHA fallback is allowed only for workflow_dispatch" not in sig_txt:
        print(
            "ERROR: security_release_signal.sh must document WORKFLOW_SHA only for manual workflow_dispatch when artifacts lack SHA "
            "(automatic workflow_run must use Build promotion manifest / RESOLVED_SOURCE_SHA).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    if "RELEASE_CANDIDATE_JSON" not in sig_txt:
        print(
            "ERROR: security_release_signal.sh must export RELEASE_CANDIDATE_JSON from the Build `release-candidate` artifact (enterprise audit path).",
            file=sys.stderr,
        )
        raise SystemExit(1)

    stg = load("deploy-develop.yml")
    st_on = get_on(stg) or {}
    if not isinstance(st_on, dict) or "workflow_run" not in st_on:
        print("ERROR: deploy-develop must declare on.workflow_run from Security Release.", file=sys.stderr)
        raise SystemExit(1)
    st_wr = st_on.get("workflow_run") or {}
    st_wfw = _workflow_run_workflows(st_on)
    st_br = st_wr.get("branches") or []
    st_types = _as_list(st_wr.get("types"))
    if "completed" not in st_types:
        print("ERROR: deploy-develop on.workflow_run.types must include 'completed'.", file=sys.stderr)
        raise SystemExit(1)
    if "Security Release" not in st_wfw or "develop" not in st_br:
        print("ERROR: deploy-develop must run on workflow_run from Security Release, branches: [develop].", file=sys.stderr)
        raise SystemExit(1)
    _assert_deploy_not_build_or_repo_security("deploy-develop.yml", st_wfw)

    dd_txt = _read_text("deploy-develop.yml")
    vr_txt = (ROOT / "scripts" / "release" / "validate_release_verdict.py").read_text(encoding="utf-8", errors="replace")
    if "actions/workflows/security.yml/runs" in dd_txt:
        print(
            "ERROR: deploy-develop.yml must not call actions/workflows/security.yml/runs to resolve release verdicts "
            "(use security-verdict from the triggering Security Release run).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    if "github.event.workflow_run.name == 'Security Release'" not in dd_txt:
        print("ERROR: deploy-develop.yml must gate on triggering workflow name Security Release.", file=sys.stderr)
        raise SystemExit(1)
    if "github.event.workflow_run.id" not in dd_txt:
        print("ERROR: deploy-develop.yml must use github.event.workflow_run.id to download security-verdict.", file=sys.stderr)
        raise SystemExit(1)
    if ".github/workflows/security-release.yml" not in dd_txt:
        print(
            "ERROR: deploy-develop.yml must verify Security Release runs use workflow path "
            ".github/workflows/security-release.yml (not repo Security / security.yml).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    for needle in (
        "Security Release verdict is not pass; refusing staging deploy",
        "staging candidate source_branch must be develop",
        "need_digest_pinned",
        "provenance_enforcement:",
        "PROVENANCE_ENFORCEMENT",
    ):
        if needle not in dd_txt and needle not in vr_txt:
            print(
                f"ERROR: deploy-develop.yml or scripts/release/validate_release_verdict.py must enforce release gate "
                f"(verdict pass, develop branch, digest-pinned refs); missing {needle!r}.",
                file=sys.stderr,
            )
            raise SystemExit(1)
    if "migration_preflight.sh staging" not in dd_txt:
        print(
            "ERROR: deploy-develop.yml must run scripts/deploy/migration_preflight.sh staging before SSH/sync deploy.",
            file=sys.stderr,
        )
        raise SystemExit(1)
    for needle in (
        "scripts/release/validate_release_verdict.py staging-candidate",
        "scripts/release/validate_release_verdict.py staging-gate",
    ):
        if needle not in dd_txt:
            print(
                f"ERROR: deploy-develop.yml must invoke {needle!r} (verdict validation extracted from inline workflow logic).",
                file=sys.stderr,
            )
            raise SystemExit(1)
    if "scripts/deploy/staging_smoke_evidence.sh" not in dd_txt:
        print(
            "ERROR: deploy-develop.yml must invoke scripts/deploy/staging_smoke_evidence.sh for post-deploy smoke evidence.",
            file=sys.stderr,
        )
        raise SystemExit(1)
    if "staging-smoke-evidence" not in dd_txt:
        print("ERROR: deploy-develop.yml must upload a staging-smoke-evidence artifact.", file=sys.stderr)
        raise SystemExit(1)

    prod = load("deploy-prod.yml")
    if (prod.get("name") or "").strip() != "Deploy Production":
        print("ERROR: deploy-prod.yml name must be exactly 'Deploy Production'.", file=sys.stderr)
        raise SystemExit(1)
    p_on = get_on(prod) or {}
    if not isinstance(p_on, dict):
        print("ERROR: deploy-prod must define on: as a mapping.", file=sys.stderr)
        raise SystemExit(1)
    if "workflow_run" not in p_on:
        print(
            "ERROR: deploy-prod must declare on.workflow_run from Security Release (automatic production gate).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    p_wr = p_on.get("workflow_run") or {}
    p_wfw = _workflow_run_workflows(p_on)
    p_br = p_wr.get("branches") or []
    p_types = _as_list(p_wr.get("types"))
    if "Security Release" not in p_wfw:
        print("ERROR: deploy-prod on.workflow_run.workflows must list 'Security Release'.", file=sys.stderr)
        raise SystemExit(1)
    if "main" not in p_br:
        print("ERROR: deploy-prod on.workflow_run.branches must list 'main'.", file=sys.stderr)
        raise SystemExit(1)
    if "develop" in p_br:
        print("ERROR: deploy-prod on.workflow_run.branches must not list 'develop'.", file=sys.stderr)
        raise SystemExit(1)
    if "completed" not in p_types:
        print("ERROR: deploy-prod on.workflow_run.types must include 'completed'.", file=sys.stderr)
        raise SystemExit(1)
    if "workflow_dispatch" not in p_on:
        print("ERROR: deploy-prod must allow workflow_dispatch (manual + rollback).", file=sys.stderr)
        raise SystemExit(1)
    _assert_deploy_not_build_or_repo_security("deploy-prod.yml", p_wfw)

    dp_txt = _read_text("deploy-prod.yml")
    if "actions/workflows/security.yml/runs" in dp_txt:
        print(
            "ERROR: deploy-prod.yml must not call actions/workflows/security.yml/runs to resolve release verdicts.",
            file=sys.stderr,
        )
        raise SystemExit(1)
    if "github.event.workflow_run.id" not in dp_txt:
        print("ERROR: deploy-prod.yml must use github.event.workflow_run.id for Security Release security-verdict download.", file=sys.stderr)
        raise SystemExit(1)
    if ".github/workflows/security-release.yml" not in dp_txt:
        print(
            "ERROR: deploy-prod.yml must verify Security Release runs use workflow path "
            ".github/workflows/security-release.yml (not repo Security / security.yml).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    for needle in (
        "source_branch in security-verdict must be main",
        "automatic production candidate source_branch must be main",
        "need_digest_pinned",
        "error: security-verdict verdict must be pass for production",
        "release-manifest-post-deploy",
        "generate_release_manifest.py",
        "rollout_mode_effective",
        "deploy_canary_wave",
        "provenance_enforcement:",
        "PROVENANCE_ENFORCEMENT=enforce",
    ):
        if needle not in dp_txt:
            print(f"ERROR: deploy-prod.yml must enforce main + digest + pass verdict paths; missing {needle!r}.", file=sys.stderr)
            raise SystemExit(1)
    vrv_path = ROOT / "scripts" / "release" / "validate_release_verdict.py"
    vrv_txt = vrv_path.read_text(encoding="utf-8", errors="replace")
    if "source_branch must be main for production" not in vrv_txt:
        print(
            "ERROR: validate_release_verdict.py production-match must reject non-main source_branch in security-verdict.json.",
            file=sys.stderr,
        )
        raise SystemExit(1)

    if "environment: production" not in dp_txt:
        print("ERROR: deploy-prod must keep environment: production on the deployment job.", file=sys.stderr)
        raise SystemExit(1)
    if "Security Release" not in dp_txt:
        print("ERROR: deploy-prod must reference Security Release for production evidence.", file=sys.stderr)
        raise SystemExit(1)
    if "migration_preflight.sh production" not in dp_txt:
        print(
            "ERROR: deploy-prod.yml must run scripts/deploy/migration_preflight.sh production before SSH rollout.",
            file=sys.stderr,
        )
        raise SystemExit(1)
    if "ALLOW_PROD_DESTRUCTIVE_MIGRATIONS" not in dp_txt:
        print(
            "ERROR: deploy-prod.yml must pass vars.ALLOW_PROD_DESTRUCTIVE_MIGRATIONS into the migration safety gate.",
            file=sys.stderr,
        )
        raise SystemExit(1)
    for needle in (
        "scripts/release/validate_release_verdict.py production-match",
        "scripts/release/derive_rollout_outcomes.py",
        "scripts/release/write_deployment_manifest.py",
    ):
        if needle not in dp_txt:
            print(
                f"ERROR: deploy-prod.yml must invoke {needle!r} (release gate / manifest logic must not be huge inline only).",
                file=sys.stderr,
            )
            raise SystemExit(1)

    nightly = load("nightly-security.yml")
    if (nightly.get("name") or "").strip() != "Nightly Security Rescan":
        print(
            "ERROR: nightly-security.yml name must be exactly 'Nightly Security Rescan' (scheduled rescans; not Security).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    n_on = get_on(nightly) or {}
    if not isinstance(n_on, dict) or "schedule" not in n_on or "workflow_dispatch" not in n_on:
        print(
            "ERROR: nightly-security.yml must declare on.schedule and on.workflow_dispatch.",
            file=sys.stderr,
        )
        raise SystemExit(1)
    if "workflow_run" in n_on:
        print("ERROR: nightly-security.yml must not declare on.workflow_run.", file=sys.stderr)
        raise SystemExit(1)
    n_txt = _read_text("nightly-security.yml")
    if re.search(r"(?m)^[ \t]*environment:[ \t]*production[ \t]*$", n_txt):
        print(
            "ERROR: nightly-security.yml must not use environment: production (no deploy gates).",
            file=sys.stderr,
        )
        raise SystemExit(1)
    for needle in (
        "resolve_latest_main_release_images.sh",
        "nightly-repo-scans",
        "nightly-image-rescan",
        "aquasec/trivy:0.57.1",
        "go-module-updates-nightly.json",
    ):
        if needle not in n_txt:
            print(
                f"ERROR: nightly-security.yml must contain {needle!r} (rescan jobs / Trivy pin).",
                file=sys.stderr,
            )
            raise SystemExit(1)

    verify_pull_request_workflows_never_deploy()

    verify_enterprise_architecture()

    verify_action_reference_pinning_policy()

    print("OK: GitHub workflow CI/CD contract (offline YAML)")


if __name__ == "__main__":
    main()
