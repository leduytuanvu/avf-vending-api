#!/usr/bin/env python3
"""Temporary production E2E automation window — snapshot, enable, restore, status.

Uses GitHub REST API via `gh api` (never prints tokens or secrets).

Enable adds a dedicated bypass actor to active branch rulesets covering main/develop
and temporarily removes required reviewers from the `production` environment.
Restore replays the timestamped snapshot under `.e2e-runs/governance/`.

Exit codes: 0 success, 1 policy/validation failure, 2 usage/config error.
"""
from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path
from typing import Any
from urllib.parse import quote

API_VERSION = "2022-11-28"
ACCEPT = "application/vnd.github+json"
CONFIRM_VALUE = "I_ACCEPT_TEMPORARY_PRODUCTION_AUTOMATION_RISK"
PROTECTED_BRANCHES = ("main", "develop")
PRODUCTION_ENV = "production"
E2E_AUTOMATION_ENV = "production-e2e-automation"
SCHEMA_VERSION = 1
DEFAULT_TTL_MINUTES = 120


def _repo_root() -> Path:
    return Path(__file__).resolve().parent.parent


def _governance_dir() -> Path:
    d = _repo_root() / ".e2e-runs" / "governance"
    d.mkdir(parents=True, exist_ok=True)
    return d


def _active_window_path() -> Path:
    return _governance_dir() / "active-window.json"


def _fail(msg: str, code: int = 1) -> None:
    print(f"production_e2e_governance_window: error: {msg}", file=sys.stderr)
    raise SystemExit(code)


def _token() -> str:
    for key in ("GH_AUTOMATION_TOKEN", "GOVERNANCE_AUDIT_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"):
        v = (os.environ.get(key) or "").strip()
        if v:
            return v
    _fail(
        "missing API token — set GH_AUTOMATION_TOKEN (preferred) or GOVERNANCE_AUDIT_TOKEN / GH_TOKEN / GITHUB_TOKEN",
        2,
    )


def _repo_slug() -> tuple[str, str]:
    ghr = (os.environ.get("GITHUB_REPOSITORY") or os.environ.get("REPOSITORY") or "").strip()
    if ghr and "/" in ghr:
        o, r = ghr.split("/", 1)
        if o and r:
            return o, r
    _fail("set GITHUB_REPOSITORY=owner/repo (or REPOSITORY=owner/repo)", 2)


def _gh_api(
    method: str,
    api_path: str,
    token: str,
    *,
    input_json: dict[str, Any] | None = None,
) -> tuple[int, Any | None, str]:
    if not shutil.which("gh"):
        _fail("gh CLI is required on PATH", 2)
    cmd = [
        "gh",
        "api",
        "-X",
        method,
        "-H",
        f"Accept: {ACCEPT}",
        "-H",
        f"X-GitHub-Api-Version: {API_VERSION}",
    ]
    if input_json is not None:
        cmd.extend(["--input", "-"])
    cmd.append(api_path)
    env = {**os.environ, "GH_TOKEN": token, "GITHUB_TOKEN": token}
    proc = subprocess.run(
        cmd,
        input=json.dumps(input_json).encode("utf-8") if input_json is not None else None,
        capture_output=True,
        env=env,
    )
    err = (proc.stderr or b"").decode("utf-8", errors="replace").strip()
    if proc.returncode != 0:
        return proc.returncode, None, err or f"gh api {method} {api_path} failed"
    if not proc.stdout:
        return 0, None, ""
    try:
        return 0, json.loads(proc.stdout.decode("utf-8")), ""
    except json.JSONDecodeError:
        return 0, proc.stdout.decode("utf-8", errors="replace"), ""


def _gh_get(api_path: str, token: str) -> tuple[int, Any | None, str]:
    return _gh_api("GET", api_path, token)


def _now_utc() -> datetime:
    return datetime.now(timezone.utc)


def _iso(dt: datetime) -> str:
    return dt.replace(microsecond=0).isoformat().replace("+00:00", "Z")


def _parse_iso(s: str) -> datetime:
    s = s.strip()
    if s.endswith("Z"):
        s = s[:-1] + "+00:00"
    return datetime.fromisoformat(s)


def _bypass_actor() -> tuple[int, str]:
    raw_id = (os.environ.get("AVF_E2E_AUTOMATION_BYPASS_ACTOR_ID") or "").strip()
    raw_type = (os.environ.get("AVF_E2E_AUTOMATION_BYPASS_ACTOR_TYPE") or "Integration").strip()
    if not raw_id:
        _fail(
            "AVF_E2E_AUTOMATION_BYPASS_ACTOR_ID is required (GitHub App installation actor id or automation user id)",
            2,
        )
    try:
        actor_id = int(raw_id)
    except ValueError:
        _fail("AVF_E2E_AUTOMATION_BYPASS_ACTOR_ID must be an integer", 2)
    actor_type = raw_type if raw_type in ("Integration", "User", "Team", "OrganizationAdmin") else ""
    if not actor_type:
        _fail(
            "AVF_E2E_AUTOMATION_BYPASS_ACTOR_TYPE must be Integration, User, Team, or OrganizationAdmin",
            2,
        )
    return actor_id, actor_type


def _fetch_rulesets(owner: str, repo: str, token: str) -> tuple[str, list[dict[str, Any]] | None, str]:
    path = f"repos/{owner}/{repo}/rulesets?includes_parents=true&per_page=100"
    code, data, err = _gh_get(path, token)
    if code in (401, 403):
        return "forbidden", None, err
    if code == 404:
        return "unavailable", None, err
    if code != 0 or not isinstance(data, list):
        return str(code), None, err
    full: list[dict[str, Any]] = []
    for item in data:
        if not isinstance(item, dict):
            continue
        rid = item.get("id")
        if rid is None:
            continue
        c2, body, e2 = _gh_get(f"repos/{owner}/{repo}/rulesets/{rid}", token)
        if c2 == 0 and isinstance(body, dict):
            full.append(body)
    return "ok", full, ""


def _pattern_matches_ref(pattern: str, branch: str) -> bool:
    pattern = (pattern or "").strip()
    if not pattern:
        return False
    ref = f"refs/heads/{branch}"
    if pattern in (branch, ref, f"refs/heads/{branch}"):
        return True
    if pattern == "~ALL":
        return True
    if pattern == "~DEFAULT_BRANCH" and branch == "main":
        return True
    if not pattern.startswith("refs/") and not pattern.startswith("~"):
        return pattern == branch or ref == f"refs/heads/{pattern}"
    return pattern == ref


def _ruleset_covers_branch(ruleset: dict[str, Any], branch: str) -> bool:
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
        if _pattern_matches_ref(str(ex), branch):
            return False
    if not includes:
        return False
    return any(_pattern_matches_ref(str(inc), branch) for inc in includes)


def _rulesets_for_branches(rulesets: list[dict[str, Any]]) -> dict[str, list[dict[str, Any]]]:
    out: dict[str, list[dict[str, Any]]] = {b: [] for b in PROTECTED_BRANCHES}
    for rs in rulesets:
        for b in PROTECTED_BRANCHES:
            if _ruleset_covers_branch(rs, b):
                out[b].append(rs)
    return out


def _fetch_classic_protection(owner: str, repo: str, branch: str, token: str) -> dict[str, Any] | None:
    code, data, _ = _gh_get(f"repos/{owner}/{repo}/branches/{quote(branch, safe='')}/protection", token)
    if code == 404:
        return None
    if code != 0 or not isinstance(data, dict):
        return {"_fetch_error": code}
    return data


def _fetch_environment(owner: str, repo: str, env_name: str, token: str) -> dict[str, Any] | None:
    enc = quote(env_name, safe="")
    code, data, err = _gh_get(f"repos/{owner}/{repo}/environments/{enc}", token)
    if code == 404:
        return None
    if code != 0:
        return {"_fetch_error": code, "_fetch_message": err}
    return data if isinstance(data, dict) else None


def _fetch_env_deployment_rules(owner: str, repo: str, env_name: str, token: str) -> list[dict[str, Any]]:
    enc = quote(env_name, safe="")
    code, data, _ = _gh_get(
        f"repos/{owner}/{repo}/environments/{enc}/deployment-protection-rules",
        token,
    )
    if code != 0 or not isinstance(data, list):
        return []
    return [r for r in data if isinstance(r, dict)]


def _build_snapshot(owner: str, repo: str, token: str, *, ttl_minutes: int) -> dict[str, Any]:
    rs_status, rulesets, rs_err = _fetch_rulesets(owner, repo, token)
    classic: dict[str, Any] = {}
    for b in PROTECTED_BRANCHES:
        classic[b] = _fetch_classic_protection(owner, repo, b, token)
    prod_env = _fetch_environment(owner, repo, PRODUCTION_ENV, token)
    prod_rules = _fetch_env_deployment_rules(owner, repo, PRODUCTION_ENV, token)
    e2e_env = _fetch_environment(owner, repo, E2E_AUTOMATION_ENV, token)
    now = _now_utc()
    expires = now + timedelta(minutes=ttl_minutes)
    return {
        "schema_version": SCHEMA_VERSION,
        "created_at": _iso(now),
        "expires_at": _iso(expires),
        "ttl_minutes": ttl_minutes,
        "repository": f"{owner}/{repo}",
        "rulesets_status": rs_status,
        "rulesets_error": rs_err,
        "rulesets": rulesets or [],
        "classic_branch_protection": classic,
        "environments": {
            PRODUCTION_ENV: prod_env,
            E2E_AUTOMATION_ENV: e2e_env,
        },
        "production_deployment_protection_rules": prod_rules,
        "window_state": {"enabled": False},
    }


def _write_snapshot(snapshot: dict[str, Any]) -> Path:
    ts = _now_utc().strftime("%Y%m%dT%H%M%SZ")
    path = _governance_dir() / f"governance-snapshot-{ts}.json"
    path.write_text(json.dumps(snapshot, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    return path


def _load_snapshot(path: Path | None = None) -> tuple[dict[str, Any], Path]:
    if path is None:
        active = _active_window_path()
        if active.is_file():
            data = json.loads(active.read_text(encoding="utf-8"))
            snap_path = Path(data.get("snapshot_path", ""))
            if snap_path.is_file():
                return json.loads(snap_path.read_text(encoding="utf-8")), snap_path
        snaps = sorted(_governance_dir().glob("governance-snapshot-*.json"), reverse=True)
        if not snaps:
            _fail("no governance snapshot found under .e2e-runs/governance/")
        path = snaps[0]
    if not path.is_file():
        _fail(f"snapshot not found: {path}")
    return json.loads(path.read_text(encoding="utf-8")), path


def _ruleset_supports_bypass(rulesets: list[dict[str, Any]]) -> bool:
    """True when at least one active branch ruleset covers main or develop."""
    for b in PROTECTED_BRANCHES:
        if any(_ruleset_covers_branch(rs, b) for rs in rulesets):
            return True
    return False


def _bypass_actor_present(ruleset: dict[str, Any], actor_id: int, actor_type: str) -> bool:
    for ba in ruleset.get("bypass_actors") or []:
        if not isinstance(ba, dict):
            continue
        if int(ba.get("actor_id", -1)) == actor_id and ba.get("actor_type") == actor_type:
            return True
    return False


def _enable_ruleset_bypass(
    owner: str,
    repo: str,
    token: str,
    ruleset: dict[str, Any],
    actor_id: int,
    actor_type: str,
) -> None:
    rid = ruleset.get("id")
    if rid is None:
        return
    if _bypass_actor_present(ruleset, actor_id, actor_type):
        print(f"  ruleset {rid}: bypass actor already present")
        return
    payload = dict(ruleset)
    bypass = list(payload.get("bypass_actors") or [])
    bypass.append(
        {
            "actor_id": actor_id,
            "actor_type": actor_type,
            "bypass_mode": "always",
        }
    )
    payload["bypass_actors"] = bypass
    for key in ("created_at", "updated_at", "source", "source_type", "current_user_can_bypass"):
        payload.pop(key, None)
    code, _, err = _gh_api("PUT", f"repos/{owner}/{repo}/rulesets/{rid}", token, input_json=payload)
    if code != 0:
        _fail(f"failed to update ruleset {rid} with bypass actor: {err}")


def _remove_ruleset_bypass(
    owner: str,
    repo: str,
    token: str,
    ruleset: dict[str, Any],
    actor_id: int,
    actor_type: str,
) -> None:
    rid = ruleset.get("id")
    if rid is None:
        return
    if not _bypass_actor_present(ruleset, actor_id, actor_type):
        print(f"  ruleset {rid}: bypass actor not present (skip)")
        return
    payload = dict(ruleset)
    payload["bypass_actors"] = [
        ba
        for ba in (payload.get("bypass_actors") or [])
        if not (
            isinstance(ba, dict)
            and int(ba.get("actor_id", -1)) == actor_id
            and ba.get("actor_type") == actor_type
        )
    ]
    for key in ("created_at", "updated_at", "source", "source_type", "current_user_can_bypass"):
        payload.pop(key, None)
    code, _, err = _gh_api("PUT", f"repos/{owner}/{repo}/rulesets/{rid}", token, input_json=payload)
    if code != 0:
        _fail(f"failed to restore ruleset {rid} bypass actors: {err}")


def _delete_production_required_reviewers(
    owner: str,
    repo: str,
    token: str,
    rules: list[dict[str, Any]],
) -> list[int]:
    """Remove required_reviewers deployment rules; return deleted rule ids."""
    deleted: list[int] = []
    enc = quote(PRODUCTION_ENV, safe="")
    for rule in rules:
        if not isinstance(rule, dict):
            continue
        rtype = str(rule.get("type") or "").lower().replace(" ", "_")
        if rtype != "required_reviewers":
            continue
        rid = rule.get("id")
        if rid is None:
            continue
        code, _, err = _gh_api(
            "DELETE",
            f"repos/{owner}/{repo}/environments/{enc}/deployment-protection-rules/{rid}",
            token,
        )
        if code != 0:
            _fail(f"failed to remove production required_reviewers rule {rid}: {err}")
        deleted.append(int(rid))
        print(f"  removed production deployment rule {rid} (required_reviewers)")
    return deleted


def _restore_production_required_reviewers(
    owner: str,
    repo: str,
    token: str,
    snapshot_rules: list[dict[str, Any]],
) -> None:
    enc = quote(PRODUCTION_ENV, safe="")
    current = _fetch_env_deployment_rules(owner, repo, PRODUCTION_ENV, token)
    current_reviewer_rules = [
        r
        for r in current
        if str(r.get("type") or "").lower().replace(" ", "_") == "required_reviewers"
    ]
    if current_reviewer_rules:
        print("  production environment already has required_reviewers rules")
        return
    for rule in snapshot_rules:
        if not isinstance(rule, dict):
            continue
        rtype = str(rule.get("type") or "").lower().replace(" ", "_")
        if rtype != "required_reviewers":
            continue
        reviewers = rule.get("reviewers")
        if not isinstance(reviewers, list) or not reviewers:
            _fail("snapshot required_reviewers rule has no reviewers to restore")
        payload = {"type": "required_reviewers", "reviewers": reviewers}
        code, _, err = _gh_api(
            "POST",
            f"repos/{owner}/{repo}/environments/{enc}/deployment-protection-rules",
            token,
            input_json=payload,
        )
        if code != 0:
            _fail(f"failed to recreate production required_reviewers rule: {err}")
        print("  restored production required_reviewers deployment rule")


def _ensure_e2e_automation_environment(owner: str, repo: str, token: str) -> None:
    """Create production-e2e-automation environment (no reviewers) for optional deploy path."""
    enc = quote(E2E_AUTOMATION_ENV, safe="")
    code, _, _ = _gh_get(f"repos/{owner}/{repo}/environments/{enc}", token)
    if code == 0:
        print(f"  environment {E2E_AUTOMATION_ENV}: already exists")
        return
    payload = {
        "wait_timer": 0,
        "deployment_branch_policy": {
            "protected_branches": False,
            "custom_branch_policies": True,
        },
    }
    code, _, err = _gh_api("PUT", f"repos/{owner}/{repo}/environments/{enc}", token, input_json=payload)
    if code != 0:
        print(f"  warning: could not create {E2E_AUTOMATION_ENV} environment: {err}", file=sys.stderr)
        return
    # Restrict to main only
    branch_payload = {"name": "main"}
    _gh_api(
        "POST",
        f"repos/{owner}/{repo}/environments/{enc}/deployment-branch-policies",
        token,
        input_json=branch_payload,
    )
    print(f"  created environment {E2E_AUTOMATION_ENV} (no required reviewers; main-only deploy branch)")


def _require_confirmation() -> None:
    if (os.environ.get("AVF_E2E_AUTOMATION_WINDOW_CONFIRM") or "").strip() != CONFIRM_VALUE:
        _fail(
            f"set AVF_E2E_AUTOMATION_WINDOW_CONFIRM={CONFIRM_VALUE} to acknowledge temporary production automation risk",
            2,
        )


def cmd_enable(args: argparse.Namespace) -> int:
    _require_confirmation()
    token = _token()
    owner, repo = _repo_slug()
    actor_id, actor_type = _bypass_actor()
    ttl = int(args.ttl_minutes or DEFAULT_TTL_MINUTES)
    if ttl < 15 or ttl > 480:
        _fail("ttl_minutes must be between 15 and 480")

    active = _active_window_path()
    if active.is_file() and not args.force:
        _fail(
            "an active automation window marker exists — run restore first or pass --force after manual verification",
        )

    print("production_e2e_governance_window: capturing governance snapshot…")
    snapshot = _build_snapshot(owner, repo, token, ttl_minutes=ttl)
    rulesets = snapshot.get("rulesets") or []
    rs_status = snapshot.get("rulesets_status")

    if rs_status == "forbidden":
        _fail(
            "token cannot read repository rulesets — cannot safely add bypass actors; "
            "use a token with Administration: Read and Write. Will NOT disable branch protection.",
        )
    if rs_status != "ok" or not isinstance(rulesets, list):
        _fail(
            f"rulesets API unavailable (status={rs_status!r}) — cannot add bypass actors; "
            "will NOT remove branch protection. Configure repository rulesets or fix token permissions.",
        )
    if not _ruleset_supports_bypass(rulesets):
        _fail(
            "no active branch ruleset covers main or develop — bypass actors are not supported on this repository. "
            "Configure Settings → Rules → Rulesets for main/develop. Will NOT remove all protection.",
        )

    snap_path = _write_snapshot(snapshot)
    print(f"  snapshot: {snap_path}")

    touched_ruleset_ids: list[int] = []
    seen_ids: set[int] = set()
    for rs in rulesets:
        rid = rs.get("id")
        if rid is None or rid in seen_ids:
            continue
        covers = any(_ruleset_covers_branch(rs, b) for b in PROTECTED_BRANCHES)
        if not covers:
            continue
        seen_ids.add(int(rid))
        print(f"  adding bypass actor to ruleset {rid}…")
        _enable_ruleset_bypass(owner, repo, token, rs, actor_id, actor_type)
        touched_ruleset_ids.append(int(rid))

    prod_rules = snapshot.get("production_deployment_protection_rules") or []
    deleted_rule_ids = _delete_production_required_reviewers(owner, repo, token, prod_rules)
    _ensure_e2e_automation_environment(owner, repo, token)

    expires_at = snapshot["expires_at"]
    snapshot["window_state"] = {
        "enabled": True,
        "bypass_actor_id": actor_id,
        "bypass_actor_type": actor_type,
        "touched_ruleset_ids": touched_ruleset_ids,
        "deleted_production_protection_rule_ids": deleted_rule_ids,
    }
    snap_path.write_text(json.dumps(snapshot, indent=2, sort_keys=True) + "\n", encoding="utf-8")

    active.write_text(
        json.dumps(
            {
                "snapshot_path": str(snap_path),
                "enabled_at": snapshot["created_at"],
                "expires_at": expires_at,
                "repository": snapshot["repository"],
            },
            indent=2,
        )
        + "\n",
        encoding="utf-8",
    )

    print("GOVERNANCE_WINDOW: ENABLED")
    print(f"  expires_at: {expires_at}")
    print(f"  restore: bash scripts/governance/restore-production-protections.sh")
    print(f"  status:  bash scripts/governance/restore-production-protections.sh --status")
    return 0


def _verify_branch_protection(owner: str, repo: str, token: str, errors: list[str]) -> None:
    rs_status, rulesets, _ = _fetch_rulesets(owner, repo, token)
    if rs_status == "ok" and rulesets:
        for b in PROTECTED_BRANCHES:
            if not any(_ruleset_covers_branch(rs, b) for rs in rulesets):
                errors.append(f"no active ruleset covers branch {b!r}")
        return
    for b in PROTECTED_BRANCHES:
        code, data, _ = _gh_get(
            f"repos/{owner}/{repo}/branches/{quote(b, safe='')}/protection",
            token,
        )
        if code == 404:
            errors.append(f"branch {b!r} has no ruleset or classic protection")


def _verify_production_reviewers(owner: str, repo: str, token: str, errors: list[str]) -> None:
    rules = _fetch_env_deployment_rules(owner, repo, PRODUCTION_ENV, token)
    reviewer_rules = [
        r
        for r in rules
        if str(r.get("type") or "").lower().replace(" ", "_") == "required_reviewers"
    ]
    if not reviewer_rules:
        errors.append("production environment has no required_reviewers deployment rule")
        return
    slots = 0
    for r in reviewer_rules:
        rev = r.get("reviewers")
        if isinstance(rev, list):
            slots += len(rev)
    if slots < 1:
        errors.append("production required_reviewers rule exists but no reviewers are configured")


def cmd_restore(args: argparse.Namespace) -> int:
    token = _token()
    owner, repo = _repo_slug()
    snap_path = Path(args.snapshot) if args.snapshot else None
    snapshot, path = _load_snapshot(snap_path)
    ws = snapshot.get("window_state") or {}
    actor_id = int(ws.get("bypass_actor_id", 0) or 0)
    actor_type = str(ws.get("bypass_actor_type") or "")
    rulesets = snapshot.get("rulesets") or []

    print(f"production_e2e_governance_window: restoring from {path}…")

    if actor_id and actor_type:
        seen: set[int] = set()
        for rs in rulesets:
            rid = rs.get("id")
            if rid is None or int(rid) in seen:
                continue
            if not any(_ruleset_covers_branch(rs, b) for b in PROTECTED_BRANCHES):
                continue
            seen.add(int(rid))
            # Re-fetch live ruleset before restore (snapshot may be stale)
            c, live, _ = _gh_get(f"repos/{owner}/{repo}/rulesets/{rid}", token)
            target = live if c == 0 and isinstance(live, dict) else rs
            print(f"  removing temporary bypass actor from ruleset {rid}…")
            _remove_ruleset_bypass(owner, repo, token, target, actor_id, actor_type)

    snap_rules = snapshot.get("production_deployment_protection_rules") or []
    _restore_production_required_reviewers(owner, repo, token, snap_rules)

    errors: list[str] = []
    _verify_branch_protection(owner, repo, token, errors)
    _verify_production_reviewers(owner, repo, token, errors)

    active = _active_window_path()
    if active.is_file():
        active.unlink()

    if errors:
        for e in errors:
            print(f"  RESTORE_VERIFY: FAIL — {e}", file=sys.stderr)
        _fail("restore incomplete — manual intervention required in GitHub Settings (see runbook)")

    print("GOVERNANCE_WINDOW: RESTORED")
    print("  develop/main protection: verified")
    print("  production environment approval: verified")
    return 0


def cmd_status(args: argparse.Namespace) -> int:
    token = _token()
    owner, repo = _repo_slug()
    active = _active_window_path()
    now = _now_utc()

    print("GOVERNANCE_WINDOW: STATUS")
    print(f"  repository: {owner}/{repo}")
    print(f"  now_utc: {_iso(now)}")

    if not active.is_file():
        print("  window: inactive (no active-window.json)")
    else:
        meta = json.loads(active.read_text(encoding="utf-8"))
        expires = meta.get("expires_at", "")
        print(f"  window: active")
        print(f"  snapshot: {meta.get('snapshot_path', '(unknown)')}")
        print(f"  enabled_at: {meta.get('enabled_at', '')}")
        print(f"  expires_at: {expires}")
        if expires:
            exp_dt = _parse_iso(expires)
            if now >= exp_dt:
                print("  ttl: EXPIRED — restore is mandatory", file=sys.stderr)
            else:
                remaining = exp_dt - now
                mins = int(remaining.total_seconds() // 60)
                print(f"  ttl_remaining_minutes: {mins}")

    errors: list[str] = []
    _verify_branch_protection(owner, repo, token, errors)
    _verify_production_reviewers(owner, repo, token, errors)

    if errors:
        print("  protection_gaps:")
        for e in errors:
            print(f"    - {e}")
    else:
        print("  protection_gaps: none detected (main/develop + production reviewers)")

    if args.dry_run:
        print("  dry_run: restore would replay latest snapshot (no changes made)")
    return 0


def main() -> int:
    parser = argparse.ArgumentParser(description="Production E2E governance automation window")
    sub = parser.add_subparsers(dest="command", required=True)

    p_enable = sub.add_parser("enable", help="Open automation window (snapshot + temporary bypass)")
    p_enable.add_argument("--ttl-minutes", type=int, default=DEFAULT_TTL_MINUTES)
    p_enable.add_argument("--force", action="store_true", help="Allow enable when active marker exists")

    p_restore = sub.add_parser("restore", help="Restore protections from snapshot")
    p_restore.add_argument("--snapshot", help="Path to governance snapshot JSON")

    p_status = sub.add_parser("status", help="Report window and protection state")
    p_status.add_argument("--dry-run", action="store_true", help="Describe restore without applying")

    args = parser.parse_args()
    if args.command == "enable":
        return cmd_enable(args)
    if args.command == "restore":
        return cmd_restore(args)
    if args.command == "status":
        return cmd_status(args)
    _fail(f"unknown command: {args.command}", 2)


if __name__ == "__main__":
    raise SystemExit(main())
