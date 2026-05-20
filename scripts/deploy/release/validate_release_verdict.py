#!/usr/bin/env python3
"""Validate security-verdict.json for deploy / staging gates (extracted from workflows)."""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
from datetime import datetime, timezone
from pathlib import Path

PRIVATE_REPO_PROVENANCE_FALLBACK = "accepted-private-repo-no-github-attestations"


def _fail(msg: str, code: int = 1) -> None:
    print(msg, file=sys.stderr)
    raise SystemExit(code)


def run_staging_candidate() -> None:
    """Resolve staging candidate outputs from downloaded security-verdict (env: SECURITY_RELEASE_RUN_ID, etc.)."""
    path = Path("security-verdict-bundle/security-verdict.json")
    payload = json.loads(path.read_text(encoding="utf-8"))
    sec_run = os.environ["SECURITY_RELEASE_RUN_ID"]
    conc = os.environ["SECURITY_RELEASE_CONCLUSION"]

    def out(k: str, v: str) -> None:
        gh_out = os.environ["GITHUB_OUTPUT"]
        with open(gh_out, "a", encoding="utf-8") as fh:
            fh.write("%s=%s\n" % (k, v))

    def need_digest_pinned(name: str, ref: str) -> None:
        sref = (ref or "").strip()
        if not sref or "@sha256:" not in sref or not re.search(r"sha256:[0-9a-f]{64}", sref):
            _fail("error: %s must be digest-pinned (registry/.../img@sha256:...), got %r" % (name, ref))

    if conc != "success":
        _fail("Security Release run did not complete with conclusion success; refusing staging")

    if str(payload.get("workflow_run_id", "")) != str(sec_run):
        _fail(
            "error: security-verdict workflow_run_id %r does not match triggering Security Release run %r"
            % (payload.get("workflow_run_id"), sec_run)
        )

    verdict = (payload.get("verdict") or "").lower()
    if verdict in ("", "unavailable"):
        _fail("error: security-verdict field verdict is missing or unavailable (got %r)" % payload.get("verdict"))

    if verdict in ("skipped", "no-candidate", "fail"):
        for r in payload.get("failure_reasons") or []:
            print("failure_reason: %s" % r, file=sys.stderr)
        summary_path = os.environ.get("GITHUB_STEP_SUMMARY")
        if summary_path:
            with open(summary_path, "a", encoding="utf-8") as fh:
                fh.write("\n## Staging deploy — blocked (non-pass Security Release verdict)\n\n")
                fh.write(
                    "**Security Release** run `%s` produced `verdict=%s` — **staging does not deploy**. "
                    "Only `verdict=pass` from `.github/workflows/security-release.yml` authorizes staging.\n\n"
                    % (sec_run, verdict)
                )
                fh.write("| Field | Value |\n|---|---|\n")
                fh.write("| Security Release run id | `%s` |\n" % sec_run)
                fh.write("| verdict | `%s` |\n" % verdict)
        _fail(
            "error: security-verdict verdict is %r (staging deploy requires verdict=pass from Security Release only)"
            % verdict
        )

    if verdict != "pass":
        for r in payload.get("failure_reasons") or []:
            print("failure_reason: %s" % r, file=sys.stderr)
        _fail("error: security-verdict verdict is %r (staging allows only pass)" % verdict)

    secv = payload.get("security_verdict")
    if secv is not None and str(secv).lower() != "pass":
        _fail("security-verdict security_verdict is not pass; refusing staging")

    rgv = (payload.get("release_gate_verdict") or "").strip()
    if rgv != "pass":
        _fail("error: release_gate_verdict must be pass, got %r" % rgv)

    branch = (payload.get("source_branch") or "").strip()
    if branch != "develop":
        _fail("error: staging candidate source_branch must be develop, got %r" % branch)

    sha = (payload.get("source_sha") or "").strip()
    if not sha or not re.fullmatch(r"[0-9a-f]{40}", sha):
        _fail("error: source_sha must be a non-empty 40-character hex SHA, got %r" % sha)

    build_id = str(payload.get("source_build_run_id") or "").strip()
    if not build_id.isdigit():
        _fail("error: source_build_run_id must be a numeric Actions run id, got %r" % build_id)

    src_ev = (payload.get("source_event") or "").strip()
    if src_ev != "push":
        _fail(
            "error: automatic staging requires source_event push (semantic build trigger), got %r" % src_ev
        )

    pub = payload.get("published_images") or {}
    need_digest_pinned("published_images.app_image_ref", pub.get("app_image_ref", ""))
    need_digest_pinned("published_images.goose_image_ref", pub.get("goose_image_ref", ""))

    out("workflow_run_id", build_id)
    out("source_commit_sha", sha)
    out("source_branch", "develop")
    out("source_event", src_ev)
    out("security_workflow_run_id", sec_run)
    out("staging_verdict", "pass")
    summary_path = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary_path:
        with open(summary_path, "a", encoding="utf-8") as fh:
            fh.write("\n## Staging deploy candidate resolved\n\n")
            fh.write("| Field | Value |\n|---|---|\n")
            fh.write("| **deploy decision** | **proceed_to_image_resolution** (verdict=pass, release_gate_verdict=pass) |\n")
            fh.write("| Security Release run id | `%s` |\n" % sec_run)
            fh.write("| source_branch | `develop` |\n")
            fh.write("| source_sha | `%s` |\n" % sha)
            fh.write("| source_build_run_id (Build and Push Images) | `%s` |\n" % build_id)
            fh.write("| app_image_ref (digest-pinned) | `%s` |\n" % pub.get("app_image_ref", ""))
            fh.write("| goose_image_ref (digest-pinned) | `%s` |\n" % pub.get("goose_image_ref", ""))


def run_staging_gate() -> None:
    verdict_path = Path(os.environ["VERDICT_FILE"])
    payload = json.loads(verdict_path.read_text(encoding="utf-8"))
    run_id = os.environ["SECURITY_RELEASE_RUN_ID"]
    expected_build = os.environ["EXPECTED_BUILD_RUN_ID"]
    expected_sha = os.environ["EXPECTED_SOURCE_SHA"]
    expected_app = os.environ["EXPECTED_APP_IMAGE_REF"]
    expected_goose = os.environ["EXPECTED_GOOSE_IMAGE_REF"]
    max_age_hours = int(os.environ["SECURITY_VERDICT_MAX_AGE_HOURS"])

    def fail_msg(m: str) -> None:
        print(m, file=sys.stderr)
        raise SystemExit(1)

    required_keys = (
        "schema_version",
        "event_name",
        "verdict",
        "source_build_run_id",
        "source_sha",
        "workflow_run_id",
        "generated_at_utc",
        "published_images",
        "release_gate_verdict",
        "release_gate_mode",
        "release_gate",
        "repo_level_checks",
        "image_level_checks",
        "provenance_release_checks",
    )
    missing = [k for k in required_keys if k not in payload]
    if missing:
        fail_msg("security-verdict.json missing keys: %s" % missing)

    if payload.get("schema_version") != "v1":
        fail_msg("security-verdict schema_version must be v1")

    if str(payload.get("workflow_run_id", "")) != str(run_id):
        fail_msg("security-verdict workflow_run_id does not match triggering Security Release run")

    if str(payload.get("source_build_run_id", "")) != str(expected_build):
        fail_msg("security-verdict source_build_run_id does not match candidate build run id")

    if payload.get("source_sha", "") != expected_sha:
        fail_msg("security-verdict source_sha does not match candidate source_commit_sha")

    if payload.get("event_name") not in ("workflow_run", "workflow_dispatch"):
        fail_msg("unexpected security-verdict event_name: %r" % payload.get("event_name"))

    if (payload.get("verdict") or "").lower() != "pass":
        fail_msg("Security Release verdict is not pass; refusing staging deploy")

    secv = payload.get("security_verdict")
    if secv is not None and str(secv).lower() != "pass":
        fail_msg("Security Release verdict is not pass; refusing staging deploy")

    if payload.get("release_gate_verdict", "") != "pass":
        fail_msg("release_gate_verdict must be pass, got %r" % payload.get("release_gate_verdict"))

    repo_release_verdict = (
        ((payload.get("repo_level_checks") or {}).get("release_evidence") or {}).get(
            "required_for_release_verdict", ""
        )
    )
    if repo_release_verdict != "pass":
        fail_msg("repo release evidence not pass: %r" % repo_release_verdict)

    image_release_verdict = ((payload.get("image_level_checks") or {}).get("required_for_release_verdict", ""))
    if image_release_verdict != "pass":
        fail_msg("image-level release evidence not pass: %r" % image_release_verdict)

    provenance_release_verdict = (
        (payload.get("provenance_release_checks") or {}).get("required_for_release_verdict", "")
    )
    if provenance_release_verdict not in ("pass", PRIVATE_REPO_PROVENANCE_FALLBACK):
        fail_msg("provenance release evidence not acceptable: %r" % provenance_release_verdict)

    published_images = payload.get("published_images") or {}
    if published_images.get("app_image_ref", "") != expected_app:
        fail_msg("security-verdict app_image_ref does not match resolved immutable app ref")
    if published_images.get("goose_image_ref", "") != expected_goose:
        fail_msg("security-verdict goose_image_ref does not match resolved immutable goose ref")

    signing_enforcement = (os.environ.get("SIGNING_ENFORCEMENT") or "warn").strip().lower()
    if signing_enforcement not in ("warn", "enforce"):
        signing_enforcement = "warn"
    if signing_enforcement == "enforce":
        img_sig = payload.get("image_signing") or {}
        if not isinstance(img_sig, dict) or img_sig.get("overall") != "pass":
            fail_msg(
                "SIGNING_ENFORCEMENT=enforce requires security-verdict image_signing.overall=pass (got %r)" % img_sig
            )

    provenance_enforcement = (os.environ.get("PROVENANCE_ENFORCEMENT") or "warn").strip().lower()
    if provenance_enforcement not in ("warn", "enforce"):
        provenance_enforcement = "warn"
    if provenance_enforcement == "enforce":
        if provenance_release_verdict != "pass":
            fail_msg(
                "PROVENANCE_ENFORCEMENT=enforce requires provenance_release_checks.required_for_release_verdict=pass "
                "(%r; private-repo attestation fallback is not allowed for deploy)" % provenance_release_verdict
            )
        pub_pv = (published_images.get("provenance_verdict") or "").strip()
        if pub_pv != "verified":
            fail_msg(
                "PROVENANCE_ENFORCEMENT=enforce requires published_images.provenance_verdict=verified (got %r)"
                % pub_pv
            )

    generated_at_raw = payload.get("generated_at_utc", "")
    try:
        generated_at = datetime.strptime(generated_at_raw, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
    except ValueError:
        fail_msg("generated_at_utc malformed: %r" % generated_at_raw)

    age_seconds = int((datetime.now(timezone.utc) - generated_at).total_seconds())
    if age_seconds < 0 or age_seconds > max_age_hours * 3600:
        fail_msg(
            "security-verdict is stale (%ss old, max %sh); rerun Security Release or adjust STAGING_SECURITY_VERDICT_MAX_AGE_HOURS"
            % (age_seconds, max_age_hours)
        )

    out_path = os.environ["GITHUB_OUTPUT"]
    with open(out_path, "a", encoding="utf-8") as fh:
        fh.write("security_verdict_age_seconds=%s\n" % age_seconds)
        fh.write("security_verdict_max_age_hours=%s\n" % max_age_hours)
        fh.write("release_gate_mode=%s\n" % payload.get("release_gate_mode", ""))
        fh.write("release_gate_verdict=%s\n" % payload.get("release_gate_verdict", ""))
        fh.write("repo_release_verdict=%s\n" % repo_release_verdict)
        fh.write("provenance_release_verdict=%s\n" % provenance_release_verdict)
        fh.write("security_workflow_run_id=%s\n" % run_id)
        fh.write("security_verdict_generated_at_utc=%s\n" % generated_at_raw)


def run_production_match(
    verdict_path: Path,
    expected_build_run_id: str,
    expected_source_sha: str,
    expected_source_workflow_name: str,
    run_id: str,
    run_conclusion: str,
    expected_app_image_ref: str,
    expected_goose_image_ref: str,
    max_age_hours: int,
) -> str:
    """Return stdout text (key=value lines). Exit codes: 0 match, 2 sha mismatch, 3 malformed, 4 ref mismatch, 5 policy, 6 stale."""
    payload = json.loads(verdict_path.read_text(encoding="utf-8"))

    def reason_exit(msg: str, code: int) -> None:
        print("reason=%s" % msg)
        raise SystemExit(code)

    required_keys = (
        "schema_version",
        "event_name",
        "verdict",
        "source_build_run_id",
        "source_sha",
        "workflow_run_id",
        "generated_at_utc",
        "scanner_summary",
        "published_images",
        "release_gate_verdict",
        "release_gate_mode",
        "release_gate",
        "repo_level_checks",
        "image_level_checks",
        "provenance_release_checks",
    )
    missing = [key for key in required_keys if key not in payload]
    if missing:
        reason_exit("security-verdict.json is missing required keys: %s" % missing, 3)

    if payload.get("schema_version", "") != "v1":
        reason_exit(
            "security-verdict schema_version is unsupported: %s" % payload.get("schema_version", ""),
            3,
        )

    if (payload.get("verdict") or "").lower() != "pass":
        reason_exit(
            "security-verdict verdict must be pass for production deploy (got %s)" % payload.get("verdict"),
            5,
        )

    if payload.get("event_name") not in ("workflow_run", "workflow_dispatch"):
        reason_exit(
            "production release gate requires workflow_run or workflow_dispatch security evidence, found event_name=%s"
            % payload.get("event_name", ""),
            4,
        )

    if str(payload.get("workflow_run_id", "")) != run_id:
        reason_exit("security-verdict workflow_run_id does not match the artifact owner run", 3)

    if str(payload.get("source_build_run_id", "")) != expected_build_run_id:
        raise SystemExit(2)

    br_main = (payload.get("source_branch") or "").strip()
    if br_main != "main":
        reason_exit("security-verdict source_branch must be main for production, got %r" % br_main, 4)

    raw_sha = (payload.get("source_sha") or "").strip()
    if not raw_sha or not re.fullmatch(r"[0-9a-f]{40}", raw_sha):
        reason_exit("security-verdict source_sha must be a non-empty 40-character hex commit SHA", 3)

    if payload.get("source_sha", "") != expected_source_sha:
        reason_exit("matching security verdict has a different source SHA than the selected build", 4)

    source_workflow_name = payload.get("source_workflow_name", "")
    if expected_source_workflow_name and source_workflow_name and source_workflow_name != expected_source_workflow_name:
        reason_exit(
            "matching security verdict has a different source workflow linkage than the selected build",
            4,
        )

    published_images = payload.get("published_images") or {}
    if not isinstance(published_images, dict):
        reason_exit("security-verdict published_images payload is malformed", 3)

    if payload.get("release_gate_mode", "") != "full-security-release-gate":
        reason_exit(
            "production release gate requires full-security-release-gate mode, found %s"
            % payload.get("release_gate_mode", ""),
            4,
        )

    if (payload.get("release_gate_verdict", "") or "").strip() != "pass":
        reason_exit(
            "production release gate is %s with required_evidence=%s"
            % (payload.get("release_gate_verdict"), (payload.get("release_gate") or {}).get("required_evidence", {})),
            5,
        )

    repo_release_verdict = (
        ((payload.get("repo_level_checks") or {}).get("release_evidence") or {}).get(
            "required_for_release_verdict", ""
        )
    )
    if repo_release_verdict != "pass":
        reason_exit(
            "production release gate is missing passing repo-level release evidence: %s" % repo_release_verdict,
            5,
        )

    image_release_verdict = ((payload.get("image_level_checks") or {}).get("required_for_release_verdict", ""))
    if image_release_verdict != "pass":
        reason_exit(
            "production release gate image-level evidence is not pass: %s" % image_release_verdict,
            5,
        )

    provenance_release_verdict = (
        (payload.get("provenance_release_checks") or {}).get("required_for_release_verdict", "")
    )
    if provenance_release_verdict not in ("pass", PRIVATE_REPO_PROVENANCE_FALLBACK):
        reason_exit(
            "production release gate provenance evidence is not acceptable: %s" % provenance_release_verdict,
            5,
        )

    if published_images.get("app_image_ref", "") != expected_app_image_ref:
        reason_exit("matching security verdict app image ref does not match the selected immutable artifact", 4)

    if published_images.get("goose_image_ref", "") != expected_goose_image_ref:
        reason_exit("matching security verdict goose image ref does not match the selected immutable artifact", 4)

    expected_app_digest = expected_app_image_ref.split("@", 1)[1]
    expected_goose_digest = expected_goose_image_ref.split("@", 1)[1]
    app_digest = published_images.get("app_digest", "")
    goose_digest = published_images.get("goose_digest", "")
    if app_digest and app_digest != expected_app_digest:
        reason_exit("matching security verdict app digest does not match the selected immutable artifact", 4)

    if goose_digest and goose_digest != expected_goose_digest:
        reason_exit("matching security verdict goose digest does not match the selected immutable artifact", 4)

    generated_at_raw = payload.get("generated_at_utc", "")
    try:
        generated_at = datetime.strptime(generated_at_raw, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)
    except ValueError:
        reason_exit("security-verdict generated_at_utc is malformed: %s" % generated_at_raw, 3)

    age_seconds = int((datetime.now(timezone.utc) - generated_at).total_seconds())
    if age_seconds < 0:
        reason_exit("matching security verdict timestamp is in the future", 6)

    if age_seconds > max_age_hours * 3600:
        print("reason=matching security verdict is stale for production release hygiene (%ss old, max %sh); rerun Security Release for this build or set PRODUCTION_SECURITY_VERDICT_MAX_AGE_HOURS / SECURITY_VERDICT_MAX_AGE_HOURS to an explicitly approved override" % (age_seconds, max_age_hours))
        print("generated_at_utc=%s" % generated_at_raw)
        raise SystemExit(6)

    if run_conclusion != "success":
        print("reason=matching security workflow run conclusion is %s" % run_conclusion)
        print("generated_at_utc=%s" % generated_at_raw)
        raise SystemExit(5)

    signing_enforcement = (os.environ.get("SIGNING_ENFORCEMENT") or "warn").strip().lower()
    if signing_enforcement not in ("warn", "enforce"):
        signing_enforcement = "warn"
    if signing_enforcement == "enforce":
        img_sig = payload.get("image_signing") or {}
        if not isinstance(img_sig, dict) or img_sig.get("overall") != "pass":
            reason_exit(
                "production requires security-verdict image_signing.overall=pass when SIGNING_ENFORCEMENT=enforce",
                5,
            )

    provenance_enforcement = (os.environ.get("PROVENANCE_ENFORCEMENT") or "warn").strip().lower()
    if provenance_enforcement not in ("warn", "enforce"):
        provenance_enforcement = "warn"
    if provenance_enforcement == "enforce":
        if provenance_release_verdict != "pass":
            reason_exit(
                "PROVENANCE_ENFORCEMENT=enforce requires provenance_release_verdict=pass (got %s)"
                % provenance_release_verdict,
                5,
            )
        pub_pv = (published_images.get("provenance_verdict") or "").strip()
        if pub_pv != "verified":
            reason_exit(
                "PROVENANCE_ENFORCEMENT=enforce requires published_images.provenance_verdict=verified (got %s)"
                % pub_pv,
                5,
            )

    lines = [
        "release_gate_mode=%s" % payload.get("release_gate_mode", ""),
        "release_gate_verdict=%s" % payload.get("release_gate_verdict", ""),
        "repo_release_verdict=%s" % repo_release_verdict,
        "provenance_release_verdict=%s" % provenance_release_verdict,
        "security_verdict_age_seconds=%s" % age_seconds,
        "security_workflow_run_id=%s" % run_id,
        "security_verdict_generated_at_utc=%s" % generated_at_raw,
        "security_source_build_run_id=%s" % payload.get("source_build_run_id", ""),
        "security_source_sha=%s" % payload.get("source_sha", ""),
    ]
    return "\n".join(lines) + "\n"


def run_self_check() -> None:
    repo = Path(__file__).resolve().parents[2]
    import tempfile

    good = {
        "schema_version": "v1",
        "verdict": "pass",
        "security_verdict": "pass",
        "release_gate_verdict": "pass",
        "event_name": "workflow_run",
        "workflow_run_id": "999",
        "source_build_run_id": "888",
        "source_sha": "a" * 40,
        "source_branch": "main",
        "source_event": "push",
        "generated_at_utc": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "published_images": {
            "app_image_ref": "ghcr.io/x/app@sha256:" + "b" * 64,
            "goose_image_ref": "ghcr.io/x/goose@sha256:" + "c" * 64,
            "app_digest": "sha256:" + "b" * 64,
            "goose_digest": "sha256:" + "c" * 64,
            "provenance_verdict": "verified",
        },
        "release_gate_mode": "full-security-release-gate",
        "release_gate": {"required_evidence": {}},
        "repo_level_checks": {"release_evidence": {"required_for_release_verdict": "pass"}},
        "image_level_checks": {"required_for_release_verdict": "pass"},
        "provenance_release_checks": {"required_for_release_verdict": "pass"},
        "scanner_summary": {},
        "image_signing": {"overall": "pass"},
    }
    with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False, encoding="utf-8") as f:
        json.dump(good, f)
        good_path = Path(f.name)
    try:
        os.environ.setdefault("SIGNING_ENFORCEMENT", "enforce")
        os.environ.setdefault("PROVENANCE_ENFORCEMENT", "enforce")
        out = run_production_match(
            good_path,
            "888",
            "a" * 40,
            "",
            "999",
            "success",
            good["published_images"]["app_image_ref"],
            good["published_images"]["goose_image_ref"],
            72,
        )
        assert "security_workflow_run_id=999" in out
    finally:
        good_path.unlink(missing_ok=True)

    wrong_branch = dict(good)
    wrong_branch["source_branch"] = "develop"
    with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False, encoding="utf-8") as f:
        json.dump(wrong_branch, f)
        p = Path(f.name)
    try:
        try:
            run_production_match(
                p,
                "888",
                "a" * 40,
                "",
                "999",
                "success",
                good["published_images"]["app_image_ref"],
                good["published_images"]["goose_image_ref"],
                72,
            )
        except SystemExit as e:
            if e.code != 4:
                raise AssertionError("expected exit 4 for non-main source_branch, got %s" % e.code)
        else:
            raise AssertionError("expected failure for develop branch in production match")
    finally:
        p.unlink(missing_ok=True)

    for bad_verdict in ("skipped", "fail", "no-candidate"):
        bad = dict(good)
        bad["verdict"] = bad_verdict
        with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False, encoding="utf-8") as f:
            json.dump(bad, f)
            p = Path(f.name)
        try:
            try:
                run_production_match(
                    p,
                    "888",
                    "a" * 40,
                    "",
                    "999",
                    "success",
                    good["published_images"]["app_image_ref"],
                    good["published_images"]["goose_image_ref"],
                    72,
                )
            except SystemExit as e:
                if e.code != 5:
                    raise AssertionError("expected exit 5 for verdict %s, got %s" % (bad_verdict, e.code))
            else:
                raise AssertionError("expected failure for verdict %s" % bad_verdict)
        finally:
            p.unlink(missing_ok=True)

    wrong_build = {
        "schema_version": "v1",
        "verdict": "pass",
        "event_name": "workflow_run",
        "workflow_run_id": "999",
        "source_build_run_id": "777",
        "source_sha": "a" * 40,
        "source_branch": "develop",
        "source_event": "push",
        "generated_at_utc": good["generated_at_utc"],
        "published_images": good["published_images"],
        "release_gate_verdict": "pass",
        "release_gate_mode": "full-security-release-gate",
        "release_gate": {"required_evidence": {}},
        "repo_level_checks": {"release_evidence": {"required_for_release_verdict": "pass"}},
        "image_level_checks": {"required_for_release_verdict": "pass"},
        "provenance_release_checks": {"required_for_release_verdict": "pass"},
        "scanner_summary": {},
        "image_signing": {"overall": "pass"},
    }
    with tempfile.NamedTemporaryFile(mode="w", suffix=".json", delete=False, encoding="utf-8") as f:
        json.dump(wrong_build, f)
        p = Path(f.name)
    try:
        os.environ["VERDICT_FILE"] = str(p)
        os.environ["SECURITY_RELEASE_RUN_ID"] = "999"
        os.environ["EXPECTED_BUILD_RUN_ID"] = "888"
        os.environ["EXPECTED_SOURCE_SHA"] = "a" * 40
        os.environ["EXPECTED_APP_IMAGE_REF"] = good["published_images"]["app_image_ref"]
        os.environ["EXPECTED_GOOSE_IMAGE_REF"] = good["published_images"]["goose_image_ref"]
        os.environ["SECURITY_VERDICT_MAX_AGE_HOURS"] = "72"
        gh_out = tempfile.NamedTemporaryFile(mode="w+", delete=False, encoding="utf-8")
        gh_out.close()
        os.environ["GITHUB_OUTPUT"] = gh_out.name
        try:
            try:
                run_staging_gate()
            except SystemExit as e:
                if e.code != 1:
                    raise AssertionError("expected exit 1 for mismatched build id, got %s" % e.code)
            else:
                raise AssertionError("expected staging gate failure")
        finally:
            Path(gh_out.name).unlink(missing_ok=True)
    finally:
        p.unlink(missing_ok=True)

    print("self-check: ok", file=sys.stderr)


def main() -> None:
    p = argparse.ArgumentParser(description=__doc__)
    sub = p.add_subparsers(dest="cmd", required=True)

    sub.add_parser("staging-candidate", help="Staging: validate verdict bundle and emit GITHUB_OUTPUT")
    sub.add_parser("staging-gate", help="Staging: re-validate verdict file vs resolved images")
    pm = sub.add_parser("production-match", help="Production: match verdict to selected build")
    pm.add_argument("--verdict-file", type=Path, required=True)
    pm.add_argument("expected_build_run_id")
    pm.add_argument("expected_source_sha")
    pm.add_argument("expected_source_workflow_name")
    pm.add_argument("run_id")
    pm.add_argument("run_conclusion")
    pm.add_argument("expected_app_image_ref")
    pm.add_argument("expected_goose_image_ref")
    pm.add_argument("max_age_hours", type=int)

    sub.add_parser("self-check", help="Run lightweight local assertions")

    args = p.parse_args()
    if args.cmd == "staging-candidate":
        run_staging_candidate()
    elif args.cmd == "staging-gate":
        run_staging_gate()
    elif args.cmd == "production-match":
        text = run_production_match(
            args.verdict_file,
            args.expected_build_run_id,
            args.expected_source_sha,
            args.expected_source_workflow_name,
            args.run_id,
            args.run_conclusion,
            args.expected_app_image_ref,
            args.expected_goose_image_ref,
            args.max_age_hours,
        )
        sys.stdout.write(text)
    elif args.cmd == "self-check":
        run_self_check()


if __name__ == "__main__":
    main()
