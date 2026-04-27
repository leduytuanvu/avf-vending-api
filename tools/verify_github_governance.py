#!/usr/bin/env python3
"""Verify GitHub branch protection and deployment environments (REST API).

Reads credentials from GOVERNANCE_AUDIT_TOKEN, GH_TOKEN, or GITHUB_TOKEN (never printed). Prefers `gh api`
when the GitHub CLI is available; otherwise uses urllib.

**Primary** branch policy check uses **Repository Rulesets** (`GET /repos/.../rulesets`) and matches `main` / `develop`
via `conditions.ref_name`. **Fallback** to classic `GET /repos/.../branches/{br}/protection` when no active ruleset
covers the ref or the rulesets API is unreadable.

Exit codes:
  0 — success, or skipped locally with warning + **manual UI checklist to stdout** when no token
  1 — governance check failures (missing branch protection, production reviewers, branch deploy policy, etc.)
  2 — ENFORCE_GITHUB_GOVERNANCE=true in CI without a token (checklist to stderr)

Environment:
  ENFORCE_GITHUB_GOVERNANCE — stricter required status checks (strict) when set to true.
  GITHUB_GOVERNANCE_WARN_ONLY — if true, production environment API gaps downgrade to warnings (not for CI gating).
"""
from __future__ import annotations

import fnmatch
import json
import os
import shutil
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request
from typing import Any

API_VERSION = "2022-11-28"
ACCEPT = "application/vnd.github+json"

# Status contexts as they appear in branch protection (workflow display name / job name).
MAIN_RECOMMENDED_CONTEXTS: tuple[str, ...] = (
    "CI / Workflow and Script Quality",
    "CI / GitHub repository governance",
    "CI / Go CI Gates",
    "CI / Docker Compose Config Validation",
    "Security / Secret Scan",
    "Security / Go Vulnerability Scan",
    "Security / Deployment and Config Scan",
    "Enterprise release verification / verify-enterprise-release",
    "Security Release / Security Release Signal",
)

DEVELOP_RECOMMENDED_CONTEXTS: tuple[str, ...] = (
    "CI / Workflow and Script Quality",
    "CI / GitHub repository governance",
    "CI / Go CI Gates",
    "CI / Docker Compose Config Validation",
    "Security / Secret Scan",
    "Security / Go Vulnerability Scan",
    "Security / Deployment and Config Scan",
)

# Canonical PR/UI display name -> accepted context strings. Repository Rulesets return
# `required_status_checks[].context` as the short name (e.g. "Go CI Gates"); the PR checks
# UI often shows "CI / Go CI Gates". A policy check is satisfied if *any* alias appears in
# the merged required_status_checks from the API (no fuzzy matching).
REQUIRED_STATUS_CHECK_ALIASES: dict[str, tuple[str, ...]] = {
    "CI / Workflow and Script Quality": (
        "CI / Workflow and Script Quality",
        "Workflow and Script Quality",
    ),
    "CI / GitHub repository governance": (
        "CI / GitHub repository governance",
        "GitHub repository governance",
    ),
    "CI / Go CI Gates": (
        "CI / Go CI Gates",
        "Go CI Gates",
    ),
    "CI / Docker Compose Config Validation": (
        "CI / Docker Compose Config Validation",
        "Docker Compose Config Validation",
    ),
    "Security / Secret Scan": (
        "Security / Secret Scan",
        "Secret Scan",
    ),
    "Security / Go Vulnerability Scan": (
        "Security / Go Vulnerability Scan",
        "Go Vulnerability Scan",
    ),
    "Security / Deployment and Config Scan": (
        "Security / Deployment and Config Scan",
        "Deployment and Config Scan",
    ),
    "Enterprise release verification / verify-enterprise-release": (
        "Enterprise release verification / verify-enterprise-release",
        "verify-enterprise-release",
    ),
    "Security Release / Security Release Signal": (
        "Security Release / Security Release Signal",
        "Security Release Signal",
    ),
}


def _aliases_for_recommended_status(canonical: str) -> frozenset[str]:
    t = REQUIRED_STATUS_CHECK_ALIASES.get(canonical)
    if t is not None:
        return frozenset(t)
    return frozenset((canonical,))


def _missing_recommended_status_checks(
    expected: tuple[str, ...], all_ctx: set[str]
) -> list[str]:
    """Return canonical names that have no accepted alias present in all_ctx."""
    return [c for c in expected if not _aliases_for_recommended_status(c) & all_ctx]


def _format_missing_recommended_status_message(
    label: str, missing: list[str], all_ctx: set[str], *, api_contexts_phrase: str
) -> str:
    """Failure copy: canonical names, accepted aliases, and actual API contexts (rulesets or protection)."""
    parts: list[str] = [f"{label}: required status checks missing recommended checks:"]
    for canon in missing:
        accepted = ", ".join(sorted(_aliases_for_recommended_status(canon)))
        parts.append(f"  - {canon!r} — accepted: {accepted}")
    observed = ", ".join(sorted(all_ctx)) if all_ctx else "(none)"
    parts.append(
        f"  {api_contexts_phrase} {observed} "
        "(any one accepted name per check satisfies policy). See docs/runbooks/github-governance.md."
    )
    return "\n".join(parts)

# Shown when GH_TOKEN is missing, API returns 403/404, or ENFORCE is set without token.
MANUAL_GOVERNANCE_CHECKLIST: str = """
=== Manual GitHub UI governance (repo owner) ===
Code in this repository cannot create or lock branch protection, repository rules, or
environment settings. The repo owner (or org admin) must apply these in the GitHub UI
(step-by-step: docs/operations/github-governance.md; full runbook: docs/runbooks/github-governance.md).

This repository uses **Repository rulesets** as the primary way to protect `main` and `develop`.
In **Settings → Rules → Rulesets** (or org rules), ensure **active** branch rulesets target
`main` and `develop` and include: pull requests (with approvals), required status checks, block force pushes,
and block deletions. Classic **Branch protection** is an alternative if rulesets are not used.

Settings -> Environments -> `production` (exact name: deploy job uses `environment: production`)
  - Required reviewers: at least one user or team; production deploy jobs wait in the UI before SSH steps
  - Turn on "Prevent self-review" if the UI offers it (stops the actor who created the run from sole approval)
  - Deployment branches: Selected branches / limited to `main` only (not "All branches")
  - Optional: wait timer; add deployment-specific secrets (SSH, etc.) here; never commit them to the repo

If GET /branches/{branch}/protection returns 404, you may be using only rulesets — the verifier
reads **GET /repos/.../rulesets** when possible.
================================================================================
""".strip()


def _is_ci() -> bool:
    return os.environ.get("GITHUB_ACTIONS") == "true" or os.environ.get("CI") == "true"


def _enforce() -> bool:
    return os.environ.get("ENFORCE_GITHUB_GOVERNANCE", "").lower() in ("1", "true", "yes")


def _governance_warn_only() -> bool:
    """If true, downgrades some would-be errors to warnings (e.g. migration / token limits). Not for CI gating."""
    return os.environ.get("GITHUB_GOVERNANCE_WARN_ONLY", "").lower() in ("1", "true", "yes")


def _token() -> str | None:
    for key in ("GOVERNANCE_AUDIT_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"):
        v = os.environ.get(key)
        if v and v.strip():
            return v.strip()
    return None


def _apply_repository_override() -> None:
    """REPOSITORY=owner/name overrides GITHUB_REPOSITORY (set by verify_github_governance.sh)."""
    ovr = (os.environ.get("REPOSITORY") or "").strip()
    if ovr and "/" in ovr:
        os.environ["GITHUB_REPOSITORY"] = ovr


def _repo_slug() -> tuple[str, str] | None:
    _apply_repository_override()
    ghr = os.environ.get("GITHUB_REPOSITORY", "").strip()
    if ghr and "/" in ghr:
        o, r = ghr.split("/", 1)
        if o and r:
            return o, r
    owner = os.environ.get("GITHUB_REPOSITORY_OWNER", "").strip()
    name = os.environ.get("GITHUB_REPOSITORY_NAME", "").strip()
    if owner and name:
        return owner, name
    return None


def _api_path(owner: str, repo: str, *parts: str) -> str:
    return "/".join(("repos", owner, repo, *parts))


def _environment_path(owner: str, repo: str, environment: str, *suffix: str) -> str:
    enc = urllib.parse.quote(environment, safe="")
    return "/".join(("repos", owner, repo, "environments", enc, *suffix))


def _request_with_gh(api_path: str, token: str) -> tuple[int, bytes | None, str]:
    """api_path: repos/owner/repo/... without leading slash. Uses `gh api` (REST)."""
    if not shutil.which("gh"):
        return 127, None, "gh CLI not found on PATH (verify_github_governance.sh should require it for live mode)"
    env = {**os.environ, "GH_TOKEN": token, "GITHUB_TOKEN": token}
    p = subprocess.run(
        [
            "gh",
            "api",
            "-H",
            f"Accept: {ACCEPT}",
            "-H",
            f"X-GitHub-Api-Version: {API_VERSION}",
            api_path,
        ],
        capture_output=True,
        env=env,
        text=False,
    )
    err = (p.stderr or b"").decode("utf-8", errors="replace").strip()
    if p.returncode != 0:
        return p.returncode, None, err
    return 0, p.stdout, err


def _request_with_urllib(api_path: str, token: str) -> tuple[int, bytes | None, str]:
    url = f"https://api.github.com/{api_path}"
    req = urllib.request.Request(
        url,
        headers={
            "Authorization": f"Bearer {token}",
            "Accept": ACCEPT,
            "X-GitHub-Api-Version": API_VERSION,
            "User-Agent": "avf-vending-verify-github-governance",
        },
        method="GET",
    )
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            return resp.status, resp.read(), ""
    except urllib.error.HTTPError as e:
        body = e.read().decode("utf-8", errors="replace")
        return e.code, body.encode("utf-8", errors="replace"), body[:500]
    except OSError as e:
        return -1, None, str(e)


def github_get_json(api_path: str, token: str) -> tuple[int, Any | None, str]:
    if shutil.which("gh"):
        code, data, err = _request_with_gh(api_path, token)
        if code != 0 or data is None:
            return code, None, err or "gh api failed"
        try:
            return 0, json.loads(data.decode("utf-8")), ""
        except json.JSONDecodeError as e:
            return -1, None, f"invalid JSON from gh: {e}"

    code, data, err = _request_with_urllib(api_path, token)
    if code < 0 or data is None:
        return code, None, err
    try:
        if code >= 400:
            try:
                parsed = json.loads(data.decode("utf-8"))
                msg = parsed.get("message", err) if isinstance(parsed, dict) else err
            except json.JSONDecodeError:
                msg = data.decode("utf-8", errors="replace")[:500]
            return code, None, msg
        return 0, json.loads(data.decode("utf-8")), ""
    except json.JSONDecodeError as e:
        return -1, None, f"invalid JSON: {e}"


def _rulesets_list_path(owner: str, repo: str) -> str:
    q = urllib.parse.urlencode({"includes_parents": "true", "per_page": "100"})
    return f"repos/{owner}/{repo}/rulesets?{q}"


def _fetch_full_rulesets(
    owner: str,
    repo: str,
    token: str,
) -> tuple[str, list[dict] | None, str]:
    """Returns: status ('ok'|'forbidden'|'unavailable'|http code str), list of full ruleset objects or None, err text."""
    path = _rulesets_list_path(owner, repo)
    code, data, err = github_get_json(path, token)
    if code in (401, 403):
        return "forbidden", None, err or ""
    if code == 404:
        return "unavailable", None, err or ""
    if code != 0 or not isinstance(data, list):
        return str(code), None, err or ""
    out: list[dict] = []
    for item in data:
        if not isinstance(item, dict):
            continue
        rid = item.get("id")
        if rid is None:
            continue
        c2, full, e2 = github_get_json(f"repos/{owner}/{repo}/rulesets/{rid}", token)
        if c2 == 0 and isinstance(full, dict):
            out.append(full)
    return "ok", out, ""


def _pattern_matches_ref(pattern: str, branch: str, default_branch: str) -> bool:
    pattern = (pattern or "").strip()
    if not pattern:
        return False
    if pattern == "~ALL":
        return True
    if pattern == "~DEFAULT_BRANCH":
        return branch == default_branch
    ref = f"refs/heads/{branch}"
    if pattern in (branch, ref):
        return True
    # GitHub ref patterns: fnmatch-style on full ref; also try pattern as refs/heads/NAME
    if fnmatch.fnmatchcase(ref, pattern) or fnmatch.fnmatchcase(branch, pattern):
        return True
    if not pattern.startswith("refs/") and not pattern.startswith("~"):
        return fnmatch.fnmatchcase(ref, f"refs/heads/{pattern}")
    return fnmatch.fnmatchcase(ref, pattern)


def _ruleset_covers_ref(ruleset: dict[str, Any], branch: str, default_branch: str) -> bool:
    if ruleset.get("target") != "branch":
        return False
    if (ruleset.get("enforcement") or "").lower() != "active":
        return False
    cond = ruleset.get("conditions")
    if not isinstance(cond, dict):
        return False
    ref_name = cond.get("ref_name")
    if not isinstance(ref_name, dict):
        return False
    includes = list(ref_name.get("include") or [])
    excludes = list(ref_name.get("exclude") or [])
    for ex in excludes:
        if _pattern_matches_ref(str(ex), branch, default_branch):
            return False
    if not includes:
        return False
    for inc in includes:
        if _pattern_matches_ref(str(inc), branch, default_branch):
            return True
    return False


def _merge_rules_for_branch(rulesets: list[dict[str, Any]], branch: str, default_branch: str) -> list[dict[str, Any]]:
    merged: list[dict[str, Any]] = []
    for rs in rulesets:
        if not _ruleset_covers_ref(rs, branch, default_branch):
            continue
        rules = rs.get("rules")
        if isinstance(rules, list):
            merged.extend(r for r in rules if isinstance(r, dict))
    return merged


def _verify_merged_rules(
    merged: list[dict[str, Any]],
    branch: str,
    is_main: bool,
    errors: list[str],
    warnings: list[str],
    enforce: bool,
) -> bool:
    """Return True if branch ruleset rules satisfy policy (or only warnings)."""
    n_err_before = len(errors)
    label = f"Ruleset rules for {branch!r}"

    pr_params: list[dict[str, Any]] = []
    rsc_params: list[dict[str, Any]] = []
    types: set[str] = set()
    for r in merged:
        t = (r.get("type") or "").strip().lower()
        if not t:
            continue
        types.add(t)
        p = r.get("parameters")
        if t == "pull_request" and isinstance(p, dict):
            pr_params.append(p)
        if t == "required_status_checks" and isinstance(p, dict):
            rsc_params.append(p)

    if "pull_request" not in types:
        errors.append(f"{label}: add a **pull_request** rule (require a pull request before merging).")
    else:
        max_appr = 0
        for p in pr_params:
            try:
                max_appr = max(max_appr, int(p.get("required_approving_review_count", 0) or 0))
            except (TypeError, ValueError):
                pass
        if max_appr < 1:
            errors.append(
                f"{label}: set **at least 1** required approval (required_approving_review_count >= 1). "
                "For a small (e.g. 2-person) team, keep **1** and enable **Prevent self-review** for production; "
                "do not lower required reviews to bypass governance."
            )

        saw_dismiss: bool = False
        any_dismiss_false = False
        for p in pr_params:
            if "dismiss_stale_reviews_on_push" in p:
                saw_dismiss = True
                if not p.get("dismiss_stale_reviews_on_push"):
                    any_dismiss_false = True
        if any_dismiss_false:
            errors.append(
                f"{label}: **Dismiss stale pull request approvals** when new commits are pushed must be enabled "
                "(dismiss_stale_reviews_on_push: true in API)."
            )
        if not saw_dismiss:
            warnings.append(
                f"{label}: API did not report `dismiss_stale_reviews_on_push`; enable **Dismiss stale reviews when new "
                "commits are pushed** in the ruleset and confirm in the UI."
            )

        saw_lpa = False
        any_lpa_false = False
        for p in pr_params:
            if "require_last_push_approval" in p:
                saw_lpa = True
                if not p.get("require_last_push_approval"):
                    any_lpa_false = True
        if any_lpa_false:
            errors.append(
                f"{label}: **Require approval of the most recent reviewable push** must be enabled "
                "when the API reports `require_last_push_approval`."
            )
        if not saw_lpa:
            warnings.append(
                f"{label}: `require_last_push_approval` not in API; enable **Require approval of the most recent "
                "reviewable push** if the UI offers it (verify manually)."
            )

    if "required_status_checks" not in types:
        errors.append(
            f"{label}: add a **required_status_checks** rule and list your blocking CI/Security check names (see runbook)."
        )
    else:
        strict_ok = any(bool(p.get("strict_required_status_checks_policy")) for p in rsc_params)
        if rsc_params and not strict_ok:
            msg = (
                f"{label}: enable **strict** / “up to date” required status checks (strict_required_status_checks_policy) "
                "in the ruleset."
            )
            if enforce:
                errors.append(msg)
            else:
                warnings.append(msg + " (set ENFORCE_GITHUB_GOVERNANCE=true in CI to fail on this).")
        all_ctx: set[str] = set()
        for p in rsc_params:
            for chk in p.get("required_status_checks") or []:
                if isinstance(chk, dict) and chk.get("context"):
                    all_ctx.add(str(chk["context"]))
        if not all_ctx:
            errors.append(
                f"{label}: required_status_checks is present but has no check contexts; add the checks from the runbook."
            )
        else:
            expected = MAIN_RECOMMENDED_CONTEXTS if is_main else DEVELOP_RECOMMENDED_CONTEXTS
            missing = _missing_recommended_status_checks(expected, all_ctx)
            if missing:
                errors.append(
                    _format_missing_recommended_status_message(
                        label,
                        missing,
                        all_ctx,
                        api_contexts_phrase="Rulesets API `required_status_checks[].context` values observed:",
                    )
                )

    if "non_fast_forward" not in types:
        errors.append(f"{label}: add **non_fast_forward** (Block force pushes) to the ruleset for this ref.")
    if "deletion" not in types:
        errors.append(f"{label}: add **deletion** (Block branch deletions) to the ruleset for this ref.")

    return len(errors) == n_err_before


def _ruleset_404_context(
    rulesets_status: str,
    all_rulesets: list[dict[str, Any]],
    branch: str,
) -> str:
    if rulesets_status in ("forbidden", "unavailable", "401", "403"):
        return "Could not list repository rulesets (API denied or unavailable); using classic **branch protection** API only. "
    if not all_rulesets:
        return "No rulesets are returned for this repository; using classic **branch protection** if configured. "
    return f"No **active** ruleset **includes** this branch (`{branch}`) in `ref_name` conditions; using classic **branch protection** if configured. "


def _collect_required_contexts(protection: dict[str, Any]) -> set[str]:
    out: set[str] = set()
    rsc = protection.get("required_status_checks")
    if isinstance(rsc, dict):
        for c in rsc.get("checks") or []:
            if isinstance(c, dict) and c.get("context"):
                out.add(str(c["context"]))
        for ctx in rsc.get("contexts") or []:
            out.add(str(ctx))
    return out


def _bool_flag(obj: Any, key: str = "enabled") -> bool | None:
    if not isinstance(obj, dict):
        return None
    if key not in obj:
        return None
    return bool(obj[key])


def _print_manual_checklist(stream) -> None:
    print(MANUAL_GOVERNANCE_CHECKLIST, file=stream)


def _is_github_permission_denied_error(msg: str) -> bool:
    """True when the API refused to read admin-only settings (typical for GITHUB_TOKEN on pull_request)."""
    m = (msg or "").lower()
    if "http 401" in m or "http 403" in m:
        return True
    if "no permission to read" in m:
        return True
    if "resource not accessible" in m or "not accessible by integration" in m:
        return True
    return False


def _required_approving_count(prr: dict[str, Any] | None) -> int:
    if not isinstance(prr, dict):
        return 0
    try:
        return int(prr.get("required_approving_review_count", 0) or 0)
    except (TypeError, ValueError):
        return 0


def _check_branch_protection_classic(
    owner: str,
    repo: str,
    branch: str,
    token: str,
    *,
    is_main: bool,
    errors: list[str],
    warnings: list[str],
    enforce: bool,
    not_found_prefix: str,
) -> None:
    path = _api_path(owner, repo, "branches", branch, "protection")
    code, data, err = github_get_json(path, token)
    if code in (401, 403):
        errors.append(
            f"Branch {branch!r}: no permission to read classic branch protection (HTTP {code}). "
            "Use a read-only PAT with access to read rulesets and/or branch protection. "
            + not_found_prefix
        )
        return
    if code == 404:
        errors.append(
            f"Branch {branch!r}: `GET /branches/{branch}/protection` is 404. "
            + not_found_prefix
            + "Configure an **active** **repository ruleset** for this ref or classic branch protection. "
            "See docs/operations/github-governance.md and docs/runbooks/github-governance.md."
        )
        return
    if code != 0 or not isinstance(data, dict):
        errors.append(f"Could not read classic branch protection for {branch!r}: HTTP {code} {err}".strip())
        return

    rsc = data.get("required_status_checks")
    contexts = _collect_required_contexts(data)
    if not isinstance(rsc, dict):
        errors.append(
            f"Branch {branch!r}: required status checks are not configured in classic API (required_status_checks missing)."
        )
    elif not contexts:
        errors.append(
            f"Branch {branch!r}: required status checks have no contexts (add required checks in branch protection or ruleset)."
        )
    elif is_main and not rsc.get("strict"):
        msg = (
            f"Branch {branch!r}: required status checks are not in strict mode "
            "(enable **Require branch to be up to date** in branch protection)."
        )
        if enforce:
            errors.append(msg)
        else:
            warnings.append(msg + " (set ENFORCE_GITHUB_GOVERNANCE=true to fail on this).")
    elif not is_main and not rsc.get("strict"):
        # develop: require strict when ENFORCE to align with ruleset path
        msg = (
            f"Branch {branch!r}: required status checks are not in strict mode "
            "(enable **Require branch to be up to date** in branch protection)."
        )
        if enforce:
            errors.append(msg)
        else:
            warnings.append(msg + " (set ENFORCE_GITHUB_GOVERNANCE=true to fail on this).")

    prr = data.get("required_pull_request_reviews")
    if not isinstance(prr, dict):
        errors.append(
            f"Branch {branch!r}: pull request reviews are not required in classic API (required_pull_request_reviews missing)."
        )
    else:
        if _required_approving_count(prr) < 1:
            errors.append(
                f"Branch {branch!r}: require at least **1** approving review on pull requests "
                f"(set required_approving_review_count; small teams: keep 1, enable self-review policy at environment level for prod)."
            )
        dsr = prr.get("dismiss_stale_reviews")
        if dsr is False:
            errors.append(
                f"Branch {branch!r}: enable **dismiss stale reviews** on new commits (dismiss_stale_reviews) in branch protection when available."
            )
        elif dsr is None and isinstance(prr, dict):
            warnings.append(
                f"Branch {branch!r}: could not read dismiss_stale_reviews; confirm **Dismiss stale reviews** in the UI."
            )

    force = _bool_flag(data.get("allow_force_pushes"))
    if force is True:
        errors.append(f"Branch {branch!r}: force pushes are allowed (allow_force_pushes.enabled is true); must block.")
    delete = _bool_flag(data.get("allow_deletions"))
    if delete is True:
        errors.append(
            f"Branch {branch!r}: branch deletion is allowed (allow_deletions.enabled is true); must block in classic protection or ruleset."
        )

    if is_main:
        restrictions = data.get("restrictions")
        has_push_restrictions = isinstance(restrictions, dict) and (
            (restrictions.get("users") and len(restrictions["users"]) > 0)
            or (restrictions.get("teams") and len(restrictions["teams"]) > 0)
            or (restrictions.get("apps") and len(restrictions["apps"]) > 0)
        )
        if not isinstance(data.get("required_pull_request_reviews"), dict) and not has_push_restrictions:
            warnings.append(
                "Branch main: neither required_pull_request_reviews nor restrictions (who can push) are set; "
                "confirm merges are PR-only in GitHub settings (see manual checklist)."
            )

        expected = MAIN_RECOMMENDED_CONTEXTS
        missing = _missing_recommended_status_checks(expected, contexts)
        if missing and isinstance(rsc, dict):
            errors.append(
                _format_missing_recommended_status_message(
                    "Branch main (classic branch protection API)",
                    missing,
                    contexts,
                    api_contexts_phrase="Required check `context` values observed:",
                )
            )
    else:
        expected = DEVELOP_RECOMMENDED_CONTEXTS
        missing = _missing_recommended_status_checks(expected, contexts)
        if missing and isinstance(rsc, dict):
            errors.append(
                _format_missing_recommended_status_message(
                    "Branch develop (classic branch protection API)",
                    missing,
                    contexts,
                    api_contexts_phrase="Required check `context` values observed:",
                )
            )


def _check_branch_with_rulesets_first(
    owner: str,
    repo: str,
    branch: str,
    token: str,
    *,
    is_main: bool,
    errors: list[str],
    warnings: list[str],
    enforce: bool,
    default_branch: str,
    rulesets_status: str,
    all_rulesets: list[dict[str, Any]] | None,
) -> None:
    rs_list = all_rulesets or []
    applicable = [rs for rs in rs_list if _ruleset_covers_ref(rs, branch, default_branch)]
    if applicable:
        merged = _merge_rules_for_branch(rs_list, branch, default_branch)
        if not merged:
            errors.append(
                f"Branch {branch!r}: an active ruleset names this ref in `ref_name` but **rules** is empty. "
                "Add pull request, required status checks, **non_fast_forward** (block force push), and **deletion** (block delete) rules."
            )
            return
        if _verify_merged_rules(merged, branch, is_main, errors, warnings, enforce):
            return
        return

    _check_branch_protection_classic(
        owner,
        repo,
        branch,
        token,
        is_main=is_main,
        errors=errors,
        warnings=warnings,
        enforce=enforce,
        not_found_prefix=_ruleset_404_context(rulesets_status, rs_list, branch),
    )


def _check_environment_production(
    owner: str,
    repo: str,
    token: str,
    errors: list[str],
    warnings: list[str],
) -> None:
    warn_only = _governance_warn_only()
    path = _environment_path(owner, repo, "production")
    code, env_data, err = github_get_json(path, token)
    if code in (401, 403):
        errors.append(
            f"Environment production: no permission to read (HTTP {code}). "
            "Use a token with `repo` scope and permission to read environments; verify required reviewers in UI. "
            "See docs/runbooks/github-governance.md."
        )
        return
    if code == 404:
        errors.append(
            "Environment production does not exist (404). Create it under Repository → Settings → Environments → New environment → name `production`."
        )
        return
    if code != 0 or not isinstance(env_data, dict):
        errors.append(f"Could not read environment production: HTTP {code} {err}".strip())
        return

    prules = env_data.get("protection_rules")
    if prules is None:
        msg = (
            "Environment production: API response did not include `protection_rules` (cannot verify required reviewers). "
            "Manually open Settings → Environments → production and confirm at least one **Required reviewer** and "
            "**Deployment branches** limited to `main`. If your token cannot read this field, use a `repo` PAT with "
            "appropriate access, or set GITHUB_GOVERNANCE_WARN_ONLY=true to treat this as a warning (not for merge CI)."
        )
        (warnings if warn_only else errors).append(msg)
    elif not isinstance(prules, list):
        msg = (
            "Environment production: `protection_rules` is not a list; cannot verify required reviewers. "
            "Manually confirm required reviewers in Settings → Environments → production."
        )
        (warnings if warn_only else errors).append(msg)
    else:
        reviewer_slots = 0
        required_rules = 0
        for r in prules:
            if not isinstance(r, dict):
                continue
            rtype = str(r.get("type") or "").lower().replace(" ", "_")
            if rtype != "required_reviewers":
                continue
            required_rules += 1
            rev = r.get("reviewers")
            if isinstance(rev, list):
                reviewer_slots += len(rev)
        if required_rules == 0:
            msg = (
                "Environment production: no `required_reviewers` rule in the API response. "
                "Add **Required reviewers** under the environment (Deploy Production waits on approval)."
            )
            (warnings if warn_only else errors).append(msg)
        elif reviewer_slots == 0:
            msg = (
                "Environment production: `required_reviewers` rule exists but no reviewers are listed; "
                "add at least one user or team in Settings → Environments → production."
            )
            (warnings if warn_only else errors).append(msg)

    dbp = env_data.get("deployment_branch_policy")
    if not isinstance(dbp, dict):
        msg = (
            "Environment production: no `deployment_branch_policy` in the API response (cannot verify branch restriction). "
            "Manually set **Deployment branches** to **Selected branches** → `main` only (not “All branches”)."
        )
        (warnings if warn_only else errors).append(msg)
        return

    protected = bool(dbp.get("protected_branches"))
    custom = bool(dbp.get("custom_branch_policies"))
    if not protected and not custom:
        msg = (
            "Environment production: deployment is not limited by branch policy in the API (protected_branches and "
            "custom_branch_policies are both false). In the UI, restrict deployments to `main` only."
        )
        (warnings if warn_only else errors).append(msg)
        return

    if protected and not custom:
        warnings.append(
            "Environment production: `protected_branches` policy in use; confirm the protected branch is `main` for this repository."
        )
        return

    pol_path = _environment_path(owner, repo, "production", "deployment-branch-policies")
    pcode, policies, perr = github_get_json(pol_path, token)
    if pcode != 0 or not isinstance(policies, dict):
        msg = (
            f"Environment production: could not list deployment-branch-policies (HTTP {pcode} {perr!s}). "
            "Manually verify only `main` can deploy to this environment."
        )
        (warnings if warn_only else errors).append(msg)
        return

    branch_policies = policies.get("branch_policies") or []
    patterns: list[str] = []
    for p in branch_policies:
        if isinstance(p, dict):
            raw = p.get("name") or p.get("name_pattern") or ""
            if raw:
                patterns.append(str(raw))
    if not patterns:
        msg = "Environment production: custom branch policies enabled but no branch patterns were returned; verify `main` only in the UI."
        (warnings if warn_only else errors).append(msg)
        return

    allowed_main = frozenset({"main", "refs/heads/main"})
    bad = [p for p in patterns if p not in allowed_main]
    if bad:
        errors.append(
            "Environment production: deployment branch policies must allow only main; got patterns: "
            + ", ".join(patterns)
            + " (allowed exact: main, refs/heads/main)."
        )
    elif len(patterns) > 1:
        warnings.append(
            "Environment production: multiple deployment branch patterns; confirm only `main` may deploy: "
            + ", ".join(patterns)
        )


def _check_environment_staging(owner: str, repo: str, token: str, warnings: list[str]) -> None:
    path = _environment_path(owner, repo, "staging")
    code, _, err = github_get_json(path, token)
    if code == 404:
        warnings.append(
            "Environment `staging` not found (404). Staging deploy workflows that use `environment: staging` will fail until "
            "it exists under Settings → Environments, or the workflow is updated. This is a **warning** only — production policy "
            "is enforced via the `production` environment above."
        )
    elif code != 0:
        warnings.append(f"Could not verify environment staging: HTTP {code} {err}".strip())


def main() -> int:
    _apply_repository_override()
    tok = _token()
    if not tok:
        msg = (
            "verify_github_governance: GOVERNANCE_AUDIT_TOKEN, GH_TOKEN, or GITHUB_TOKEN is not set; skipping GitHub API governance checks.\n"
            "Set a token with repo read access and re-run, or see docs/operations/github-governance.md for manual setup."
        )
        if _enforce() and _is_ci():
            print(
                "verify_github_governance: error: ENFORCE_GITHUB_GOVERNANCE is set in CI but no token is available "
                "(set GOVERNANCE_AUDIT_TOKEN, GH_TOKEN, or GITHUB_TOKEN with repo read access).",
                file=sys.stderr,
            )
            print("GOVERNANCE_CHECK: FAIL", file=sys.stderr)
            _print_manual_checklist(sys.stderr)
            return 2
        print(msg, file=sys.stderr)
        _print_manual_checklist(sys.stdout)
        print("GOVERNANCE_CHECK: SKIPPED (no token — see verify_github_governance.sh)")
        return 0

    slug = _repo_slug()
    if not slug:
        print(
            "verify_github_governance: error: set GITHUB_REPOSITORY=owner/repo or REPOSITORY=owner/repo "
            "(or GITHUB_REPOSITORY_OWNER and GITHUB_REPOSITORY_NAME).",
            file=sys.stderr,
        )
        print("GOVERNANCE_CHECK: FAIL", file=sys.stderr)
        return 1
    owner, repo = slug

    if not shutil.which("gh"):
        print(
            "verify_github_governance: error: gh CLI is required on PATH when a token is set "
            "(use scripts/ci/verify_github_governance.sh, which enforces this in live mode).",
            file=sys.stderr,
        )
        print("GOVERNANCE_CHECK: FAIL", file=sys.stderr)
        return 1

    errors: list[str] = []
    warnings: list[str] = []
    enforce = _enforce()

    rcode, repo_json, _rerr = github_get_json(f"repos/{owner}/{repo}", tok)
    default_branch = "main"
    if rcode == 0 and isinstance(repo_json, dict) and repo_json.get("default_branch"):
        default_branch = str(repo_json["default_branch"])

    rulesets_status, all_rulesets, _ = _fetch_full_rulesets(owner, repo, tok)
    if all_rulesets is None:
        all_rulesets = []

    _check_branch_with_rulesets_first(
        owner,
        repo,
        "main",
        tok,
        is_main=True,
        errors=errors,
        warnings=warnings,
        enforce=enforce,
        default_branch=default_branch,
        rulesets_status=rulesets_status,
        all_rulesets=all_rulesets,
    )
    _check_branch_with_rulesets_first(
        owner,
        repo,
        "develop",
        tok,
        is_main=False,
        errors=errors,
        warnings=warnings,
        enforce=enforce,
        default_branch=default_branch,
        rulesets_status=rulesets_status,
        all_rulesets=all_rulesets,
    )
    _check_environment_production(owner, repo, tok, errors, warnings)
    _check_environment_staging(owner, repo, tok, warnings)

    for w in warnings:
        print(f"verify_github_governance: warning: {w}", file=sys.stderr)

    pr_event = (os.environ.get("GITHUB_EVENT_NAME") or "").strip() == "pull_request"
    if pr_event and errors and all(_is_github_permission_denied_error(e) for e in errors):
        for e in errors:
            print(
                f"verify_github_governance: note (API read denied on pull_request; this is not a passing governance audit): {e}",
                file=sys.stderr,
            )
        _print_manual_checklist(sys.stdout)
        print(
            "GOVERNANCE_CHECK: MANUAL_REVIEW_REQUIRED (token cannot read rulesets, branch protection, or environments; verify in GitHub UI — docs/operations/github-governance.md, docs/runbooks/github-governance.md)",
            file=sys.stderr,
        )
        return 0

    if errors:
        print("verify_github_governance: governance check failed:", file=sys.stderr)
        for e in errors:
            print(f"  - {e}", file=sys.stderr)
        if any(
            x in (e or "")
            for e in errors
            for x in (
                "404",
                "403",
                "401",
                "no permission",
                "If you use",
            )
        ):
            print(
                "verify_github_governance: some failures may be API limits, missing **repository rulesets**, or classic **branch protection**; "
                "use the manual checklist and docs/operations/github-governance.md",
                file=sys.stderr,
            )
            _print_manual_checklist(sys.stderr)
        print("GOVERNANCE_CHECK: FAIL (rulesets, branch protection, production environment, or recommended checks — fix in GitHub UI)", file=sys.stderr)
        return 1
    if warnings:
        print(
            "verify_github_governance: (warnings only) re-check rulesets / branch and environment settings in the GitHub UI; "
            "set ENFORCE_GITHUB_GOVERNANCE=true to hard-fail on more checks for classic API.",
            file=sys.stderr,
        )
    print("verify_github_governance: all governance checks passed.")
    print("GOVERNANCE_CHECK: PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
