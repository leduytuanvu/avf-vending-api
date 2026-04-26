#!/usr/bin/env python3
"""Verify GitHub branch protection and deployment environments (REST API).

Reads credentials from GH_TOKEN or GITHUB_TOKEN (never printed). Prefers `gh api`
when the GitHub CLI is available; otherwise uses urllib.

Exit codes:
  0 — success, or skipped locally with warning + **manual UI checklist to stdout** when no token
  1 — governance check failures (missing branch protection, production reviewers, branch deploy policy, etc.)
  2 — ENFORCE_GITHUB_GOVERNANCE=true in CI without a token (checklist to stderr)

Environment:
  ENFORCE_GITHUB_GOVERNANCE — stricter branch protection (e.g. strict required checks on main) when set to true.
  GITHUB_GOVERNANCE_WARN_ONLY — if true, production environment API gaps downgrade to warnings (not for CI gating).
"""
from __future__ import annotations

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
    "CI / Go CI Gates",
    "CI / Docker Compose Config Validation",
    "Security / Secret Scan",
    "Security / Go Vulnerability Scan",
    "Security / Deployment and Config Scan",
)

# Shown when GH_TOKEN is missing, API returns 403/404, or ENFORCE is set without token.
MANUAL_GOVERNANCE_CHECKLIST: str = """
=== Manual GitHub UI governance (repo owner) ===
Code in this repository cannot create or lock branch protection, repository rules, or
environment settings. The repo owner (or org admin) must apply these in the GitHub UI.

Settings -> Branches -> Branch protection rules -> Add (or edit) rule for `main`
  - Protect `main` (pattern: main)
  - Require a pull request before merging
  - Require approvals (at least 1) on PRs; optionally Code Owners
  - Require status checks to pass; add the checks named in docs/runbooks/github-governance.md
  - Require branch to be up to date before merging (strict / "up to date" toggle)
  - Do not allow bypassing the required pull requests and status checks (no admin bypass, per policy)
  - Do not allow force pushes; do not allow deletions
  - Restrict who can push to matching branches (only automation/release roles) where policy allows
    (PR-only flow + required reviews is the default safe pattern; avoid casual direct push to `main`)

Settings -> Branches -> branch protection for `develop` (pattern: develop)
  - Require PRs and/or required status checks (see recommended checks in docs/runbooks/github-governance.md)
  - Block force pushes and deletions (recommended for shared integration branches)

Settings -> Environments -> `production` (exact name: deploy job uses `environment: production`)
  - Required reviewers: at least one user or team; production deploy jobs wait in the UI before SSH steps
  - Turn on "Prevent self-review" if the UI offers it (stops the actor who created the run from sole approval)
  - Deployment branches: Selected branches / limited to `main` only (not "All branches")
  - Optional: wait timer; add deployment-specific secrets (SSH, etc.) here; never commit them to the repo

If GET /branches/{branch}/protection returns 404 but the branch is protected, you may be using
Repository rulesets: extend automation to the rulesets API or verify rules in the UI manually.
See docs/runbooks/github-governance.md
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
    for key in ("GH_TOKEN", "GITHUB_TOKEN"):
        v = os.environ.get(key)
        if v and v.strip():
            return v.strip()
    return None


def _repo_slug() -> tuple[str, str] | None:
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
    """api_path: repos/owner/repo/... without leading slash."""
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


def _required_approving_count(prr: dict[str, Any] | None) -> int:
    if not isinstance(prr, dict):
        return 0
    try:
        return int(prr.get("required_approving_review_count", 0) or 0)
    except (TypeError, ValueError):
        return 0


def _check_branch_protection(
    owner: str,
    repo: str,
    branch: str,
    token: str,
    *,
    is_main: bool,
    errors: list[str],
    warnings: list[str],
    enforce: bool,
) -> None:
    path = _api_path(owner, repo, "branches", branch, "protection")
    code, data, err = github_get_json(path, token)
    if code in (401, 403):
        errors.append(
            f"Branch {branch!r}: no permission to read branch protection (HTTP {code}). "
            "Use a token with `repo` or `admin:org` (read) scope; see below for manual UI checklist."
        )
        return
    if code == 404:
        errors.append(
            f"Branch {branch!r} has no classic branch protection (404). If you use **repository rulesets**, "
            "this endpoint will still be 404 — verify in UI or use the rulesets API; see docs/runbooks/github-governance.md."
        )
        return
    if code != 0 or not isinstance(data, dict):
        errors.append(f"Could not read branch protection for {branch!r}: HTTP {code} {err}".strip())
        return

    rsc = data.get("required_status_checks")
    contexts = _collect_required_contexts(data)
    if not isinstance(rsc, dict):
        errors.append(f"Branch {branch!r}: required status checks are not configured (required_status_checks missing).")
    elif not contexts:
        errors.append(f"Branch {branch!r}: required status checks have no contexts (add required checks in branch protection).")
    elif is_main and not rsc.get("strict"):
        msg = (
            f"Branch {branch!r}: required status checks are not in strict mode "
            "(enable **Require branch to be up to date** / strict in branch protection)."
        )
        if enforce:
            errors.append(msg)
        else:
            warnings.append(msg + " (set ENFORCE_GITHUB_GOVERNANCE=true to fail on this).")

    prr = data.get("required_pull_request_reviews")
    if is_main:
        if not isinstance(prr, dict):
            errors.append("Branch main: pull request reviews are not required (required_pull_request_reviews missing).")
        else:
            if _required_approving_count(prr) < 1:
                errors.append(
                    "Branch main: require at least **1 approving review** (required_approving_review_count >= 1) on pull requests."
                )
        force = _bool_flag(data.get("allow_force_pushes"))
        if force is True:
            errors.append("Branch main: force pushes are allowed (allow_force_pushes.enabled is true); must block.")
        delete = _bool_flag(data.get("allow_deletions"))
        if delete is True:
            errors.append("Branch main: branch deletion is allowed (allow_deletions.enabled is true); must block.")

        # Direct-push restriction: classic API exposes explicit push restrictions or PR-only flow.
        restrictions = data.get("restrictions")
        has_push_restrictions = isinstance(restrictions, dict) and (
            (restrictions.get("users") and len(restrictions["users"]) > 0)
            or (restrictions.get("teams") and len(restrictions["teams"]) > 0)
            or (restrictions.get("apps") and len(restrictions["apps"]) > 0)
        )
        if isinstance(data.get("required_pull_request_reviews"), dict) or has_push_restrictions:
            pass  # satisfied
        else:
            warnings.append(
                "Branch main: neither required_pull_request_reviews nor restrictions (who can push) are set; "
                "confirm merges are PR-only in GitHub settings (see manual checklist: block direct push to `main`)."
            )

        expected = MAIN_RECOMMENDED_CONTEXTS
        missing = [c for c in expected if c not in contexts]
        if missing:
            errors.append(
                "Branch main: required checks missing recommended contexts: "
                + ", ".join(missing)
                + " (see docs/runbooks/github-governance.md)."
            )
    else:
        # develop: require PR reviews (>=1) and/or at least one required context; block force delete when API exposes
        d_force = _bool_flag(data.get("allow_force_pushes"))
        if d_force is True:
            errors.append("Branch develop: force pushes are allowed (allow_force_pushes.enabled is true); must block.")
        d_delete = _bool_flag(data.get("allow_deletions"))
        if d_delete is True:
            errors.append("Branch develop: branch deletion is allowed (allow_deletions.enabled is true); must block.")

        has_pr_approval = isinstance(prr, dict) and _required_approving_count(prr) >= 1
        if not has_pr_approval and len(contexts) == 0:
            errors.append(
                "Branch develop: require pull request reviews (at least 1) and/or required status check contexts; "
                "neither is configured (if review count is 0, increase it or add required checks)."
            )
        expected = DEVELOP_RECOMMENDED_CONTEXTS
        missing = [c for c in expected if c not in contexts]
        if missing:
            errors.append(
                "Branch develop: required checks missing recommended contexts: "
                + ", ".join(missing)
                + " (see docs/runbooks/github-governance.md)."
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
            "Environment staging not found (404). This repo uses environment: staging in workflows; create it or document an exception "
            "(see docs/runbooks/github-governance.md)."
        )
    elif code != 0:
        warnings.append(f"Could not verify environment staging: HTTP {code} {err}".strip())


def main() -> int:
    tok = _token()
    if not tok:
        msg = (
            "verify_github_governance: GH_TOKEN or GITHUB_TOKEN is not set; skipping GitHub API governance checks.\n"
            "Set a token with repo read access and re-run, or see docs/runbooks/github-governance.md for manual setup."
        )
        if _enforce() and _is_ci():
            print(
                "verify_github_governance: error: ENFORCE_GITHUB_GOVERNANCE is set in CI but no token is available "
                "(set GH_TOKEN or GITHUB_TOKEN with repo read access).",
                file=sys.stderr,
            )
            _print_manual_checklist(sys.stderr)
            return 2
        print(msg, file=sys.stderr)
        _print_manual_checklist(sys.stdout)
        return 0

    slug = _repo_slug()
    if not slug:
        print(
            "verify_github_governance: error: set GITHUB_REPOSITORY=owner/repo (or GITHUB_REPOSITORY_OWNER and GITHUB_REPOSITORY_NAME).",
            file=sys.stderr,
        )
        return 1
    owner, repo = slug

    errors: list[str] = []
    warnings: list[str] = []

    enforce = _enforce()

    _check_branch_protection(
        owner, repo, "main", tok, is_main=True, errors=errors, warnings=warnings, enforce=enforce
    )
    _check_branch_protection(
        owner, repo, "develop", tok, is_main=False, errors=errors, warnings=warnings, enforce=enforce
    )
    _check_environment_production(owner, repo, tok, errors, warnings)
    _check_environment_staging(owner, repo, tok, warnings)

    for w in warnings:
        print(f"verify_github_governance: warning: {w}", file=sys.stderr)
    if errors:
        print("verify_github_governance: governance check failed:", file=sys.stderr)
        for e in errors:
            print(f"  - {e}", file=sys.stderr)
        if any(
            x in e
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
                "verify_github_governance: some failures may be API limits or **repository rulesets**; "
                "use the manual checklist below and docs/runbooks/github-governance.md",
                file=sys.stderr,
            )
            _print_manual_checklist(sys.stderr)
        return 1
    if warnings:
        print(
            "verify_github_governance: (warnings only) re-check branch/environment settings in the GitHub UI; "
            "set ENFORCE_GITHUB_GOVERNANCE=true to hard-fail on more checks.",
            file=sys.stderr,
        )
    print("verify_github_governance: all governance checks passed.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
