#!/usr/bin/env python3
"""Write security-reports/security-verdict.json for Security Release (mode via argv or embedded calls)."""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
from datetime import datetime, timezone
from pathlib import Path


REPORTS = Path("security-reports")
VERDICT_PATH = REPORTS / "security-verdict.json"
PRIVATE_REPO_PROVENANCE_FALLBACK = "accepted-private-repo-no-github-attestations"

# Every mode here must be invocable from security-release (signal step) and listed by verify_workflow_contracts.sh;
# keep argparse `choices` in main() in sync. "full" covers pass and fail (JSON verdict from write_full()).
CONTRACT_VERDICT_MODES: tuple[str, ...] = (
    "skipped",
    "no-candidate",
    "unsupported-trigger",
    "ineligible-branch",
    "unsupported-artifact-source-event",
    "metadata-mismatch",
    "full",
    "emergency",
)

# security-verdict.json must always include these top-level fields (empty string allowed where N/A).
CONTRACT_ROOT_STRING_KEYS: tuple[str, ...] = (
    "verdict",
    "release_gate_verdict",
    "release_gate_mode",
    "repo_security_verdict",
    "repo_release_verdict",
    "published_image_verdict",
    "provenance_release_verdict",
    "source_sha",
    "source_branch",
    "source_build_run_id",
    "source_workflow_name",
    "app_image_ref",
    "goose_image_ref",
    "security_workflow_run_id",
    "generated_at_utc",
)


def _utc_now() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def _s(key: str, default: str = "") -> str:
    return os.environ.get(key, default)


def _ensure_contract_baseline(p: dict) -> None:
    """Ensure security-verdict.json always exposes the enterprise contract fields (machine-readable)."""
    wrid = str(p.get("workflow_run_id") or p.get("security_workflow_run_id") or _s("WORKFLOW_RUN_ID", "") or "")
    p.setdefault("schema_version", "v1")
    p.setdefault("verdict", "")
    if "security_verdict" not in p or p.get("security_verdict") in (None, ""):
        p["security_verdict"] = p.get("verdict") or ""
    p.setdefault("release_gate_verdict", "")
    p.setdefault("release_gate_mode", "")
    p.setdefault("repo_security_verdict", "")
    p.setdefault("repo_release_verdict", "")
    p.setdefault("published_image_verdict", "")
    p.setdefault("provenance_release_verdict", "")
    p.setdefault("source_sha", "")
    p.setdefault("source_branch", "")
    p.setdefault("source_build_run_id", "")
    p.setdefault("source_workflow_name", "")
    p.setdefault("workflow_run_id", p.get("workflow_run_id") or wrid)
    p.setdefault("security_workflow_run_id", p.get("security_workflow_run_id") or wrid)
    p.setdefault("generated_at_utc", p.get("generated_at_utc") or _utc_now())
    fr = p.get("failure_reasons")
    p["failure_reasons"] = list(fr) if isinstance(fr, list) else []
    jr = p.get("job_results")
    p["job_results"] = dict(jr) if isinstance(jr, dict) else {}
    w = p.get("warnings")
    p["warnings"] = list(w) if isinstance(w, list) else []
    if "scanner_summary" not in p or not isinstance(p.get("scanner_summary"), dict):
        p["scanner_summary"] = {}
    if "image_level_checks" not in p or not isinstance(p.get("image_level_checks"), dict):
        p["image_level_checks"] = {
            "required_for_release_verdict": p.get("published_image_verdict") or "not_applicable",
            "checks": {},
        }
    if "provenance_release_checks" not in p or not isinstance(p.get("provenance_release_checks"), dict):
        p["provenance_release_checks"] = {
            "required_for_release_verdict": p.get("provenance_release_verdict") or "not_applicable",
            "published_image_provenance_verdict": "",
            "evidence_source": "not_applicable",
        }
    if "repo_level_checks" not in p or not isinstance(p.get("repo_level_checks"), dict):
        p["repo_level_checks"] = {
            "current_event_verdict": p.get("repo_security_verdict") or "not_applicable",
            "release_evidence": {
                "evidence_source": "",
                "source_event_name": "",
                "matched_workflow_run_id": "",
                "matched_workflow_conclusion": "",
                "required_for_release_verdict": p.get("repo_release_verdict") or "not_applicable",
                "summary": "",
            },
            "checks": {},
        }
    if "release_gate" not in p or not isinstance(p.get("release_gate"), dict):
        p["release_gate"] = {
            "mode": p.get("release_gate_mode") or "not-applicable",
            "verdict": p.get("release_gate_verdict") or "",
            "generated_at_utc": p.get("generated_at_utc") or _utc_now(),
            "trust_model": "not-applicable",
            "summary": "",
        }
    if "published_images" not in p or not isinstance(p.get("published_images"), dict):
        p["published_images"] = {
            "app_image_ref": "",
            "app_digest": "",
            "goose_image_ref": "",
            "goose_digest": "",
            "provenance_verdict": "",
            "provenance_verdict_source": "not_applicable",
        }
    for k in CONTRACT_ROOT_STRING_KEYS:
        p.setdefault(k, "")
    pubm = p.get("published_images")
    if not isinstance(pubm, dict):
        pubm = {}
    a_top = str(p.get("app_image_ref", "") or "").strip()
    g_top = str(p.get("goose_image_ref", "") or "").strip()
    a_pub = str(pubm.get("app_image_ref", "") or "").strip() if isinstance(pubm, dict) else ""
    g_pub = str(pubm.get("goose_image_ref", "") or "").strip() if isinstance(pubm, dict) else ""
    p["app_image_ref"] = a_top or a_pub
    p["goose_image_ref"] = g_top or g_pub


def _write(payload: dict) -> None:
    REPORTS.mkdir(parents=True, exist_ok=True)
    p = dict(payload)
    if "verdict" in p and ("security_verdict" not in p or not p.get("security_verdict")):
        p["security_verdict"] = p["verdict"]
    _ensure_contract_baseline(p)
    VERDICT_PATH.write_text(json.dumps(p, indent=2) + "\n", encoding="utf-8")


def write_emergency(reason: str, *, force: bool = False) -> None:
    """Write emergency fail verdict. When force is False, do not replace an existing valid contract verdict."""
    if (
        not force
        and VERDICT_PATH.is_file()
        and VERDICT_PATH.stat().st_size > 0
    ):
        try:
            existing = json.loads(VERDICT_PATH.read_text(encoding="utf-8"))
            v = (existing.get("verdict") or "").strip().lower()
            if v in ("pass", "fail", "skipped", "no-candidate"):
                return
        except (json.JSONDecodeError, OSError, ValueError):
            pass
    gen = _s("GENERATED_AT_UTC") or _utc_now()
    wrid = _s("GITHUB_RUN_ID", "")
    reason_final = reason or (
        "Internal: the primary security-verdict JSON writer failed; this emergency fail verdict was written so the workflow never returns an empty verdict."
    )
    p = {
        "schema_version": "v1",
        "verdict": "fail",
        "security_verdict": "fail",
        "release_gate_verdict": "fail",
        "release_gate_mode": "emergency-fail",
        "repo_security_verdict": "fail",
        "repo_release_verdict": "unavailable",
        "published_image_verdict": "fail",
        "provenance_release_verdict": "unavailable",
        "app_image_ref": "",
        "goose_image_ref": "",
        "source_build_run_id": _s("BUILD_RUN_ID", ""),
        "source_sha": (_s("CANONICAL_SOURCE_SHA") or _s("RESOLVED_SOURCE_SHA", "")),
        "source_branch": (_s("CANONICAL_SOURCE_BRANCH") or _s("RESOLVED_SOURCE_BRANCH", "")),
        "source_event": "",
        "trigger_workflow_event": _s("GITHUB_EVENT_NAME", ""),
        "source_workflow_name": "Build and Push Images",
        "event_name": _s("GITHUB_EVENT_NAME", "unknown"),
        "workflow_name": _s("GITHUB_WORKFLOW", "Security Release"),
        "workflow_run_id": wrid,
        "security_workflow_run_id": wrid,
        "generated_at_utc": gen,
        "failure_reasons": [reason_final],
        "warnings": [],
        "decision_reasons": [reason_final],
        "job_results": {},
        "nightly_security_verdict": "not_applicable",
        "release_gate": {
            "mode": "emergency-fail",
            "verdict": "fail",
            "generated_at_utc": gen,
            "trust_model": "not-applicable",
            "summary": "emergency writer",
        },
        "repo_level_checks": {
            "current_event_verdict": "fail",
            "release_evidence": {
                "evidence_source": "emergency",
                "source_event_name": "",
                "matched_workflow_run_id": "",
                "matched_workflow_conclusion": "",
                "required_for_release_verdict": "unavailable",
                "summary": "emergency",
            },
        },
        "published_images": {
            "app_image_ref": "",
            "app_digest": "",
            "goose_image_ref": "",
            "goose_digest": "",
            "provenance_verdict": "",
            "provenance_verdict_source": "not_applicable",
        },
    }
    _write(p)


def write_skipped() -> None:
    gen = _s("GENERATED_AT_UTC") or _utc_now()
    ev = _s("EVENT_NAME", "")
    wrid = _s("WORKFLOW_RUN_ID", "")
    wn = _s("WORKFLOW_NAME", "Security Release")
    conc = _s("SKIP_BUILD_CONC", "")
    hb = _s("SKIP_BUILD_HB", "")
    bid = _s("SKIP_BUILD_ID", "")
    b_ev = _s("SKIP_BUILD_EVENT", "")
    b_sha = _s("SKIP_BUILD_HEAD_SHA", "")
    b_wn = _s("SKIP_WF_NAME", "Build and Push Images")
    reasons: list[str] = []
    if hb and hb not in ("develop", "main"):
        reasons.append(
            "Non-release candidate: triggering Build head_branch is not develop or main; full Security Release gate was not evaluated."
        )
    elif conc and conc != "success":
        reasons.append(
            "Non-release candidate: triggering Build and Push Images workflow_run conclusion was %r (required success for gating); no image scan or artifact-driven gate was run."
            % conc
        )
    elif b_wn and b_wn != "Build and Push Images":
        reasons.append("Non-release candidate: upstream workflow name was %r (expected Build and Push Images)." % b_wn)
    else:
        reasons.append("Security Release skipped: upstream Build run was ineligible for develop/main release gating (see workflow filters).")
    jr = {
        "skip_job": "success",
        "resolve_build_run": "skipped",
        "resolve_image_refs": "skipped",
        "image_vulnerability_scan": "skipped",
    }
    payload = {
        "schema_version": "v1",
        "verdict": "skipped",
        "security_verdict": "skipped",
        "canonical_security_artifact": "security-verdict",
        "nightly_security_verdict": "not_applicable",
        "event_name": ev,
        "workflow_name": wn,
        "workflow_run_id": wrid,
        "security_workflow_run_id": wrid,
        "generated_at_utc": gen,
        "source_build_run_id": bid or "",
        "source_sha": b_sha or "",
        "source_branch": hb or "",
        "source_event": "",
        "trigger_workflow_event": b_ev or "",
        "source_workflow_name": b_wn,
        "release_gate_verdict": "skipped",
        "release_gate_mode": "skipped-non-candidate",
        "repo_security_verdict": "skipped",
        "repo_release_verdict": "skipped",
        "published_image_verdict": "skipped",
        "provenance_release_verdict": "skipped",
        "published_images": {
            "app_image_ref": "",
            "app_digest": "",
            "goose_image_ref": "",
            "goose_digest": "",
            "provenance_verdict": "",
            "provenance_verdict_source": "not_applicable",
        },
        "failure_reasons": list(reasons),
        "warnings": [],
        "decision_reasons": list(reasons),
        "job_results": jr,
        "repo_level_checks": {
            "current_event_verdict": "skipped",
            "release_evidence": {
                "evidence_source": "skipped",
                "source_event_name": "",
                "matched_workflow_run_id": "",
                "matched_workflow_conclusion": "",
                "required_for_release_verdict": "skipped",
                "summary": "Security Release was skipped before matching repo **Security** (Build was not a develop/main success candidate).",
            },
            "checks": {},
        },
        "release_gate": {
            "mode": "skipped-non-candidate",
            "verdict": "skipped",
            "generated_at_utc": gen,
            "trust_model": "not-applicable",
            "required_evidence": {
                "repo_level_checks": "skipped",
                "image_level_checks": "skipped",
                "provenance_release_checks": "skipped",
            },
            "summary": "Upstream **Build and Push Images** was not a release candidate for this run; the full image scan and repo **Security** gate were not executed.",
        },
    }
    _write(payload)


def write_ineligible_branch_skipped() -> None:
    """Canonical branch (from promotion-manifest) is not develop or main; neutral skipped verdict."""
    gen = _s("GENERATED_AT_UTC") or _utc_now()
    ev = _s("EVENT_NAME", "")
    wrid = _s("WORKFLOW_RUN_ID", "")
    wn = _s("WORKFLOW_NAME", "Security Release")
    br = (_s("CANONICAL_SOURCE_BRANCH") or _s("RESOLVED_SOURCE_BRANCH") or "").strip()
    tbr = (_s("TRIGGERING_BUILD_HEAD_BRANCH") or "").strip()
    ssha = (_s("RESOLVED_SOURCE_SHA") or _s("CANONICAL_SOURCE_SHA") or "").strip()
    bid = _s("BUILD_RUN_ID", _s("TRIGGERING_BUILD_ID", ""))
    reasons = [
        "Release candidate branch is not develop or main (from promotion-manifest / image-metadata): %r. "
        "Build workflow_run head branch (diagnostic) was %r; identity uses resolved artifact metadata, not the triggering run head."
        % (br, tbr)
    ]
    jr = {
        "resolve_build_run": "success",
        "resolve_image_refs": "success",
        "image_vulnerability_scan": "skipped",
    }
    payload = {
        "schema_version": "v1",
        "verdict": "skipped",
        "security_verdict": "skipped",
        "canonical_security_artifact": "security-verdict",
        "nightly_security_verdict": "not_applicable",
        "event_name": ev,
        "workflow_name": wn,
        "workflow_run_id": wrid,
        "security_workflow_run_id": wrid,
        "generated_at_utc": gen,
        "source_build_run_id": bid or "",
        "source_sha": ssha,
        "source_branch": br,
        "source_event": (_s("ARTIFACT_SOURCE_EVENT") or "").strip(),
        "trigger_workflow_event": _s("TRIGGERING_BUILD_EVENT", ""),
        "source_workflow_name": "Build and Push Images",
        "release_gate_verdict": "skipped",
        "release_gate_mode": "skipped-ineligible-candidate-branch",
        "repo_security_verdict": "skipped",
        "repo_release_verdict": "skipped",
        "published_image_verdict": "skipped",
        "provenance_release_verdict": "skipped",
        "published_images": {
            "app_image_ref": "",
            "app_digest": "",
            "goose_image_ref": "",
            "goose_digest": "",
            "provenance_verdict": "",
            "provenance_verdict_source": "not_applicable",
        },
        "failure_reasons": list(reasons),
        "warnings": [],
        "decision_reasons": list(reasons),
        "job_results": jr,
        "release_gate": {
            "mode": "skipped-ineligible-candidate-branch",
            "verdict": "skipped",
            "generated_at_utc": gen,
            "trust_model": "not-applicable",
            "summary": "Canonical source branch is not an eligible develop/main release line.",
        },
    }
    _write(payload)


def write_unsupported_artifact_source_event_skipped() -> None:
    """ARTIFACT source_event (semantic) is not an allowed value for this gate."""
    gen = _s("GENERATED_AT_UTC") or _utc_now()
    ev = _s("EVENT_NAME", "")
    wrid = _s("WORKFLOW_RUN_ID", "")
    wn = _s("WORKFLOW_NAME", "Security Release")
    a_ev = (_s("ARTIFACT_SOURCE_EVENT") or "").strip()
    reasons = [
        "promotion-manifest source_event is %r (release promotion allows only: push, workflow_dispatch, or manual mapped to dispatch; "
        "semantic source_event must not be workflow_run / chain-only)."
        % (a_ev,)
    ]
    _write(
        {
            "schema_version": "v1",
            "verdict": "skipped",
            "security_verdict": "skipped",
            "canonical_security_artifact": "security-verdict",
            "nightly_security_verdict": "not_applicable",
            "event_name": ev,
            "workflow_name": wn,
            "workflow_run_id": wrid,
            "security_workflow_run_id": wrid,
            "generated_at_utc": gen,
            "source_build_run_id": _s("BUILD_RUN_ID", ""),
            "source_sha": (_s("RESOLVED_SOURCE_SHA") or "").strip(),
            "source_branch": (_s("RESOLVED_SOURCE_BRANCH") or "").strip(),
            "source_event": a_ev,
            "release_gate_verdict": "skipped",
            "release_gate_mode": "skipped-unsupported-artifact-source-event",
            "repo_security_verdict": "skipped",
            "repo_release_verdict": "skipped",
            "published_image_verdict": "skipped",
            "provenance_release_verdict": "skipped",
            "failure_reasons": list(reasons),
            "warnings": [],
            "decision_reasons": list(reasons),
            "job_results": {},
            "release_gate": {
                "mode": "skipped-unsupported-artifact-source-event",
                "verdict": "skipped",
                "generated_at_utc": gen,
                "trust_model": "not-applicable",
                "summary": "Unsupported artifact source_event for security release gating.",
            },
        }
    )


def write_metadata_mismatch_skipped() -> None:
    """Triggering Build API metadata disagrees with promotion-manifest / same-run contract (no full gate, not emergency)."""
    gen = _s("GENERATED_AT_UTC") or _utc_now()
    ev = _s("EVENT_NAME", "")
    wrid = _s("WORKFLOW_RUN_ID", "")
    wn = _s("WORKFLOW_NAME", "Security Release")
    primary = (_s("METADATA_CONFLICT_REASON") or "").strip() or (
        "Triggering **Build and Push Images** run metadata does not match promotion-manifest / expected same-run contract."
    )
    reasons: list[str] = [primary]
    extra = (_s("METADATA_CONFLICT_EXTRA") or "").strip()
    if extra:
        reasons.append(extra)
    mv: dict = {
        "triggering_build_run_id": _s("TRIGGERING_BUILD_ID", ""),
        "triggering_build_event": _s("TRIGGERING_BUILD_EVENT", ""),
        "triggering_build_head_branch": _s("TRIGGERING_BUILD_HEAD_BRANCH", ""),
        "triggering_build_head_sha": _s("TRIGGERING_BUILD_HEAD_SHA", ""),
        "triggering_workflow_name": _s("TRIGGERING_BUILD_WF_NAME", ""),
        "triggering_workflow_conclusion": _s("TRIGGERING_BUILD_CONCLUSION", ""),
        "artifact_source_event": _s("ARTIFACT_SOURCE_EVENT", ""),
        "resolved_source_branch": _s("RESOLVED_SOURCE_BRANCH", ""),
        "resolved_source_sha": _s("RESOLVED_SOURCE_SHA", ""),
        "build_run_id_from_artifacts": _s("BUILD_RUN_ID", ""),
        "decision": "skipped",
    }
    jr = {
        "resolve_build_run": "success",
        "resolve_image_refs": "success",
        "image_vulnerability_scan": "success",
    }
    te = _s("TRIGGERING_BUILD_EVENT", "")
    _write(
        {
            "schema_version": "v1",
            "verdict": "skipped",
            "security_verdict": "skipped",
            "canonical_security_artifact": "security-verdict",
            "nightly_security_verdict": "not_applicable",
            "event_name": ev,
            "workflow_name": wn,
            "workflow_run_id": wrid,
            "security_workflow_run_id": wrid,
            "generated_at_utc": gen,
            "source_build_run_id": _s("BUILD_RUN_ID", ""),
            "source_sha": (_s("RESOLVED_SOURCE_SHA") or "").strip(),
            "source_branch": (_s("RESOLVED_SOURCE_BRANCH") or "").strip(),
            "source_event": (_s("ARTIFACT_SOURCE_EVENT") or "").strip(),
            "trigger_workflow_event": te,
            "source_workflow_name": "Build and Push Images",
            "metadata_validation": mv,
            "release_gate_verdict": "skipped",
            "release_gate_mode": "skipped-artifact-trigger-mismatch",
            "repo_security_verdict": "skipped",
            "repo_release_verdict": "skipped",
            "published_image_verdict": "skipped",
            "provenance_release_verdict": "skipped",
            "failure_reasons": list(reasons),
            "skipped_reasons": list(reasons),
            "warnings": [],
            "decision_reasons": list(reasons),
            "job_results": jr,
            "release_gate": {
                "mode": "skipped-artifact-trigger-mismatch",
                "verdict": "skipped",
                "generated_at_utc": gen,
                "trust_model": "not-applicable",
                "summary": "Defensive check: default-branch / workflow_run trigger context does not match promotion-manifest for this Build run id.",
            },
        }
    )


def write_unsupported_trigger_skipped() -> None:
    """Skipped: triggering Build event is not an allowed build trigger type for release gating."""
    gen = _s("GENERATED_AT_UTC") or _utc_now()
    ev = _s("EVENT_NAME", "")
    wrid = _s("WORKFLOW_RUN_ID", "")
    wn = _s("WORKFLOW_NAME", "Security Release")
    te = _s("TRIGGERING_BUILD_EVENT", "")
    bid = _s("TRIGGERING_BUILD_ID", "")
    b_sha = _s("TRIGGERING_BUILD_HEAD_SHA", "")
    hb = _s("TRIGGERING_BUILD_HEAD_BRANCH", "")
    if (te or "").strip() == "workflow_run":
        reason = (
            "Non-release candidate: the triggering **Build and Push Images** run was started by a `workflow_run` (upstream CI chain), not by a direct "
            "`push` to `develop`/`main` or an allowed `workflow_dispatch`. Indirect/chain-only builds are not valid release candidates."
        )
    else:
        reason = (
            "Non-release candidate: triggering **Build and Push Images** GHA `event` is %r (release promotion only allows `push` or `workflow_dispatch` "
            "on `develop`/`main`). Full Security Release gate was not evaluated."
            % (te or "",)
        )
    reasons = [reason]
    jr = {
        "skip_job": "success",
        "resolve_build_run": "skipped",
        "resolve_image_refs": "skipped",
        "image_vulnerability_scan": "skipped",
    }
    payload = {
        "schema_version": "v1",
        "verdict": "skipped",
        "security_verdict": "skipped",
        "canonical_security_artifact": "security-verdict",
        "nightly_security_verdict": "not_applicable",
        "event_name": ev,
        "workflow_name": wn,
        "workflow_run_id": wrid,
        "security_workflow_run_id": wrid,
        "generated_at_utc": gen,
        "source_build_run_id": bid or "",
        "source_sha": b_sha or "",
        "source_branch": hb or "",
        "source_event": "",
        "trigger_workflow_event": te or "",
        "source_workflow_name": "Build and Push Images",
        "release_gate_verdict": "skipped",
        "release_gate_mode": "skipped-unsupported-build-event",
        "repo_security_verdict": "skipped",
        "repo_release_verdict": "skipped",
        "published_image_verdict": "skipped",
        "provenance_release_verdict": "skipped",
        "published_images": {
            "app_image_ref": "",
            "app_digest": "",
            "goose_image_ref": "",
            "goose_digest": "",
            "provenance_verdict": "",
            "provenance_verdict_source": "not_applicable",
        },
        "failure_reasons": list(reasons),
        "warnings": [],
        "decision_reasons": list(reasons),
        "job_results": jr,
        "repo_level_checks": {
            "current_event_verdict": "skipped",
            "release_evidence": {
                "evidence_source": "skipped",
                "source_event_name": "",
                "matched_workflow_run_id": "",
                "matched_workflow_conclusion": "",
                "required_for_release_verdict": "skipped",
                "summary": "Unsupported triggering Build GHA event for release gating.",
            },
            "checks": {},
        },
        "release_gate": {
            "mode": "skipped-unsupported-build-event",
            "verdict": "skipped",
            "generated_at_utc": gen,
            "trust_model": "not-applicable",
            "required_evidence": {
                "repo_level_checks": "skipped",
                "image_level_checks": "skipped",
                "provenance_release_checks": "skipped",
            },
            "summary": "Upstream **Build and Push Images** was not a supported automatic release candidate; event type does not match policy.",
        },
    }
    _write(payload)


def write_no_candidate() -> None:
    gen = _s("GENERATED_AT_UTC") or _utc_now()
    ev = _s("EVENT_NAME", "")
    wrid = _s("WORKFLOW_RUN_ID", "")
    wn = _s("WORKFLOW_NAME", "Security Release")
    reasons: list[str] = []
    acn = (_s("ARTIFACT_CONSISTENCY_NOTE") or "").strip()
    if acn:
        reasons.append(acn)
    if ev == "workflow_run":
        tconc = (_s("TRIGGERING_BUILD_CONCLUSION") or "").strip()
        tname = (_s("TRIGGERING_BUILD_WF_NAME") or "").strip()
        thb = (_s("TRIGGERING_BUILD_HEAD_BRANCH") or "").strip()
        if tconc and tconc != "success":
            reasons.append(
                "Triggering Build and Push Images workflow_run conclusion was %r (required: success to produce a releasable candidate)."
                % tconc
            )
        if tname and tname != "Build and Push Images":
            reasons.append("Triggering workflow name was %r (expected Build and Push Images)." % tname)
        if thb and thb not in ("develop", "main"):
            reasons.append("Triggering head_branch was %r (expected develop or main for release gating)." % thb)
    rb_res = (_s("RESOLVE_BUILD_RUN_RESULT") or "").strip()
    rir = (_s("RESOLVE_IMAGE_REFS_RESULT") or "").strip()
    ris = (_s("IMAGE_SCAN_RESULT") or "").strip()
    if rb_res != "success":
        reasons.append(
            "resolve-build-run-id did not succeed (result=%r); required build artifacts (image-metadata, promotion-manifest, immutable-image-contract) were not available for the release chain."
            % rb_res
        )
    if rir != "success":
        reasons.append(
            "resolve-image-refs did not succeed (result=%r); digest-pinned app and goose image refs were not established from build artifacts."
            % rir
        )
    if not reasons:
        reasons.append("Triggering Build and Push Images run did not produce a releasable candidate")
    tw_ev = (_s("TRIGGERING_BUILD_EVENT") or "workflow_run").strip() or "workflow_run"
    jr = {
        "resolve_build_run": rb_res,
        "resolve_image_refs": rir,
        "image_vulnerability_scan": ris,
    }
    release_summary = "No releasable candidate. Security release gate did not run a full image/repo gate."
    rsha = (_s("RESOLVED_SOURCE_SHA") or _s("CANONICAL_SOURCE_SHA") or "").strip()
    src_branch = (_s("RESOLVED_SOURCE_BRANCH") or _s("CANONICAL_SOURCE_BRANCH") or "").strip()
    brid = (_s("BUILD_RUN_ID") or "").strip()
    payload = {
        "schema_version": "v1",
        "verdict": "no-candidate",
        "security_verdict": "no-candidate",
        "canonical_security_artifact": "security-verdict",
        "nightly_security_verdict": "not_applicable",
        "event_name": ev,
        "workflow_name": wn,
        "workflow_run_id": wrid,
        "security_workflow_run_id": wrid,
        "generated_at_utc": gen,
        "source_build_run_id": brid,
        "source_sha": rsha,
        "source_branch": src_branch,
        "source_event": "",
        "trigger_workflow_event": tw_ev,
        "source_workflow_name": "Build and Push Images",
        "release_gate_verdict": "no-candidate",
        "release_gate_mode": "no-release-candidate",
        "repo_security_verdict": "skipped",
        "repo_release_verdict": "skipped",
        "published_image_verdict": "skipped",
        "provenance_release_verdict": "skipped",
        "published_images": {
            "app_image_ref": "",
            "app_digest": "",
            "goose_image_ref": "",
            "goose_digest": "",
            "provenance_verdict": "",
            "provenance_verdict_source": "not_applicable",
        },
        "failure_reasons": list(reasons),
        "warnings": [],
        "decision_reasons": list(reasons),
        "job_results": jr,
        "repo_level_checks": {
            "current_event_verdict": "skipped",
            "release_evidence": {
                "evidence_source": "skipped",
                "source_event_name": "",
                "matched_workflow_run_id": "",
                "matched_workflow_conclusion": "",
                "required_for_release_verdict": "skipped",
                "summary": release_summary,
            },
            "checks": {},
        },
        "image_level_checks": {
            "required_for_release_verdict": "skipped",
            "signing_enforcement": (_s("SIGNING_ENFORCEMENT", "warn") or "warn").strip().lower(),
            "checks": {},
        },
        "provenance_release_checks": {
            "required_for_release_verdict": "skipped",
            "published_image_provenance_verdict": "",
            "evidence_source": "not_applicable",
        },
        "scanner_summary": {},
        "release_gate": {
            "mode": "no-release-candidate",
            "verdict": "no-candidate",
            "generated_at_utc": gen,
            "trust_model": "not-applicable",
            "summary": release_summary,
            "required_evidence": {
                "repo_level_checks": "skipped",
                "image_level_checks": "skipped",
                "provenance_release_checks": "skipped",
            },
        },
    }
    _write(payload)


def _trivy_norm(env_key: str) -> str:
    r = (_s(env_key) or "").strip()
    if r in ("success", "failure"):
        return r
    if r in ("", "skipped"):
        return "skipped"
    return "not_applicable"


def _cosign_norm(env_key: str) -> str:
    return _trivy_norm(env_key)


def _verdict_for(results: dict, job_names: list[str]) -> str:
    relevant = [results[name] for name in job_names if results.get(name) not in ("", "skipped", "not_applicable", None)]
    if not relevant:
        return "not_applicable"
    if all(result == "success" for result in relevant):
        return "pass"
    return "fail"


def _normalize_result(result_name: str) -> str:
    if result_name == "success":
        return "pass"
    if result_name in ("", "skipped", None):
        return "not_applicable"
    return "fail"


def _digest_from_ref(ref: str) -> str:
    if ref and "@sha256:" in ref:
        return ref.split("@", 1)[1]
    return ""


def _is_digest_pinned_image_ref(ref: str) -> bool:
    r = (ref or "").strip()
    return bool(r) and re.search(r"@sha256:[0-9a-f]{64}$", r) is not None


def _add_coordinate_drift_warnings(event_name: str, warnings: list[str]) -> None:
    """Triggering Build API coordinates may differ from promotion-manifest; resolved artifact fields win for matching."""
    if event_name != "workflow_run":
        return
    art_sha = (_s("RESOLVED_SOURCE_SHA") or "").strip()
    art_br = (_s("RESOLVED_SOURCE_BRANCH") or "").strip()
    tr_sha = (_s("TRIGGER_WORKFLOW_RUN_SOURCE_SHA") or "").strip()
    tr_br = (_s("BUILD_HEAD_BRANCH") or "").strip()
    if art_sha and tr_sha and art_sha != tr_sha:
        warnings.append(
            "source-coordinate-drift (diagnostic): artifact RESOLVED_SOURCE_SHA %r differs from triggering Build workflow_run head_sha %r; "
            "release matching uses RESOLVED_SOURCE_SHA."
            % (art_sha, tr_sha)
        )
    if art_br and tr_br and art_br != tr_br:
        warnings.append(
            "source-coordinate-drift (diagnostic): artifact RESOLVED_SOURCE_BRANCH %r differs from triggering Build workflow_run head_branch %r; "
            "release matching uses RESOLVED_SOURCE_BRANCH."
            % (art_br, tr_br)
        )


def write_full() -> None:
    """Full release gate verdict from environment (set by the workflow)."""
    generated_at_utc = _s("GENERATED_AT_UTC") or _utc_now()
    event_name = _s("EVENT_NAME", "")

    signing_mode = (_s("SIGNING_ENFORCEMENT", "warn") or "warn").strip().lower()
    if signing_mode not in ("warn", "enforce"):
        signing_mode = "warn"
    provenance_mode = (_s("PROVENANCE_ENFORCEMENT", "warn") or "warn").strip().lower()
    if provenance_mode not in ("warn", "enforce"):
        provenance_mode = "warn"
    allow_priv_fb = (_s("ALLOW_PRIVATE_REPO_PROVENANCE_FALLBACK", "true") or "true").strip().lower() in (
        "1",
        "true",
        "yes",
    )

    results = {
        "dependency_review": _s("DEPENDENCY_REVIEW_RESULT") or "not_applicable",
        "govulncheck_pr": _s("GOVULNCHECK_PR_RESULT") or "not_applicable",
        "secret_scan": _s("SECRET_SCAN_RESULT") or "not_applicable",
        "config_scan": _s("CONFIG_SCAN_RESULT") or "not_applicable",
        "resolve_build_run": _s("RESOLVE_BUILD_RUN_RESULT") or "not_applicable",
        "resolve_image_refs": _s("RESOLVE_IMAGE_REFS_RESULT") or "not_applicable",
        "image_vulnerability_scan": _s("IMAGE_SCAN_RESULT") or "not_applicable",
        "trivy_image_app": _trivy_norm("TRIVY_APP_RESULT"),
        "trivy_image_goose": _trivy_norm("TRIVY_GOOSE_RESULT"),
        "cosign_image_app": _cosign_norm("COSIGN_APP_RESULT"),
        "cosign_image_goose": _cosign_norm("COSIGN_GOOSE_RESULT"),
        "govulncheck_nightly": _s("GOVULNCHECK_NIGHTLY_RESULT") or "not_applicable",
        "dependency_snapshot": _s("DEPENDENCY_SNAPSHOT_RESULT") or "not_applicable",
    }

    caj = (_s("IMAGE_COSIGN_VERIFY_JOB_RESULT", "") or "").strip().lower()
    if signing_mode == "enforce":
        if caj == "failure":
            if results.get("cosign_image_app") == "skipped":
                results["cosign_image_app"] = "failure"
            if results.get("cosign_image_goose") == "skipped":
                results["cosign_image_goose"] = "failure"
        elif results.get("resolve_image_refs") == "success" and caj in ("skipped", ""):
            results["cosign_image_app"] = "failure"
            results["cosign_image_goose"] = "failure"

    repo_security_verdict = _verdict_for(
        results, ["dependency_review", "govulncheck_pr", "secret_scan", "config_scan"]
    )
    image_gate_keys = [
        "resolve_build_run",
        "resolve_image_refs",
        "image_vulnerability_scan",
        "trivy_image_app",
        "trivy_image_goose",
    ]
    if signing_mode == "enforce":
        image_gate_keys.extend(["cosign_image_app", "cosign_image_goose"])
    published_image_verdict = _verdict_for(results, image_gate_keys)
    nightly_security_verdict = _verdict_for(
        results, ["govulncheck_nightly", "dependency_snapshot"]
    )

    artifact_sha = (_s("RESOLVED_SOURCE_SHA") or "").strip()
    tr_sha = (_s("TRIGGER_WORKFLOW_RUN_SOURCE_SHA") or "").strip()
    wf_sha = (_s("WORKFLOW_SHA") or "").strip()
    can_sha = (_s("CANONICAL_SOURCE_SHA") or "").strip()
    can_br = (_s("CANONICAL_SOURCE_BRANCH") or "").strip()
    if can_sha:
        source_sha = can_sha
    elif artifact_sha:
        source_sha = artifact_sha
    elif event_name == "workflow_dispatch":
        # Do not use WORKFLOW_SHA for automatic `workflow_run` (must be promotion-manifest / RESOLVED or canonical from signal).
        source_sha = wf_sha
    else:
        source_sha = ""

    resolved_source_branch = (_s("RESOLVED_SOURCE_BRANCH") or "").strip()
    build_head_branch = (_s("BUILD_HEAD_BRANCH") or "").strip()  # diagnostic only (source_coordinates)
    manual_target_branch = (_s("MANUAL_TARGET_BRANCH") or "").strip()
    gref = (_s("GITHUB_REF_NAME") or "").strip()
    if can_br:
        source_branch = can_br
    elif resolved_source_branch:
        source_branch = resolved_source_branch
    elif manual_target_branch:
        source_branch = manual_target_branch
    else:
        source_branch = gref

    source_workflow_name = _s("RESOLVED_SOURCE_WORKFLOW_NAME") or _s("TRIGGER_WORKFLOW_RUN_NAME", "")
    source_build_run_id = _s("BUILD_RUN_ID", "")
    if not source_workflow_name and source_build_run_id:
        source_workflow_name = "Build and Push Images"

    artifact_source_event = (_s("ARTIFACT_SOURCE_EVENT") or "").strip()
    can_ev = (_s("CANONICAL_SOURCE_EVENT") or "").strip()
    trigger_workflow_event = (_s("TRIGGER_WORKFLOW_EVENT") or "").strip()
    if artifact_source_event == "manual":
        source_event = "workflow_dispatch"
    elif artifact_source_event and artifact_source_event not in ("manual",):
        source_event = artifact_source_event
    elif can_ev in ("push", "workflow_dispatch"):
        source_event = can_ev
    else:
        source_event = ""

    app_image_ref = _s("APP_IMAGE_REF", "")
    goose_image_ref = _s("GOOSE_IMAGE_REF", "")
    provenance_input = _s("PROVENANCE_VERDICT", "")

    failure_reasons: list[str] = []
    warnings: list[str] = []
    wnote = (_s("ARTIFACT_CONSISTENCY_NOTE") or "").strip()
    if wnote:
        warnings.append(wnote)

    if signing_mode == "warn":
        if results.get("cosign_image_app") == "failure":
            warnings.append(
                "Cosign: app image signature verification failed (SIGNING_ENFORCEMENT=warn; not blocking the release gate)."
            )
        if results.get("cosign_image_goose") == "failure":
            warnings.append(
                "Cosign: goose image signature verification failed (SIGNING_ENFORCEMENT=warn; not blocking the release gate)."
            )
    if (
        provenance_mode == "warn"
        and provenance_input == PRIVATE_REPO_PROVENANCE_FALLBACK
        and allow_priv_fb
        and event_name in ("workflow_run", "workflow_dispatch")
    ):
        warnings.append(
            "Provenance is **not fully enforced** on GitHub Artifact Attestations: `accepted-private-repo-no-github-attestations` is recorded because "
            "`ALLOW_PRIVATE_REPO_PROVENANCE_FALLBACK=true` and `PROVENANCE_ENFORCEMENT=warn`. "
            "Digest-pinned images, Trivy, and Cosign (per `SIGNING_ENFORCEMENT`) still apply. "
            "When the repo can run `gh attestation verify`, set `PROVENANCE_ENFORCEMENT=enforce` and `ALLOW_PRIVATE_REPO_PROVENANCE_FALLBACK=false`."
        )

    if not artifact_sha and not can_sha and event_name == "workflow_dispatch" and wf_sha:
        warnings.append(
            "source_sha: using Security Release job checkout (WORKFLOW_SHA) because RESOLVED was empty; prefer promotion-manifest from Build. "
            "Automatic `workflow_run` after Build does not use WORKFLOW_SHA for source_sha (fails closed if artifacts are missing)."
        )

    _add_coordinate_drift_warnings(event_name, warnings)

    repo_release_checks = {
        "dependency_review": {
            "status": _s("REPO_RELEASE_DEPENDENCY_REVIEW", "not_applicable"),
            "release_requirement": "advisory",
        },
        "govulncheck_repo": {
            "status": _s("REPO_RELEASE_GOVULNCHECK_REPO", "not_applicable"),
            "release_requirement": "required",
        },
        "secret_scan": {
            "status": _s("REPO_RELEASE_SECRET_SCAN", "not_applicable"),
            "release_requirement": "required",
        },
        "trivy_config": {
            "status": _s("REPO_RELEASE_TRIVY_CONFIG", "not_applicable"),
            "release_requirement": "required",
        },
    }
    repo_release_verdict = _s("REPO_RELEASE_VERDICT", "")

    release_gate_mode = "not-applicable"
    repo_required_for_release_verdict = "not_applicable"
    image_required_for_release_verdict = "not_applicable"
    provenance_release_verdict = "not_applicable"
    release_gate_verdict = "not_applicable"
    release_gate_trust_model = "not_applicable"
    security_verdict = "fail"
    if event_name in ("workflow_run", "workflow_dispatch"):
        release_gate_mode = "full-security-release-gate"
        if event_name == "workflow_dispatch":
            repo_required_for_release_verdict = "pass"
            release_gate_trust_model = "manual-workflow-dispatch-image-and-provenance-only"
        else:
            repo_required_for_release_verdict = repo_release_verdict or "unavailable"
        image_required_for_release_verdict = published_image_verdict
        if provenance_input == "verified":
            provenance_release_verdict = "pass"
            if event_name != "workflow_dispatch":
                release_gate_trust_model = "fully-verified-workflow-run-evidence"
        elif provenance_input == PRIVATE_REPO_PROVENANCE_FALLBACK:
            if provenance_mode == "enforce":
                provenance_release_verdict = "fail"
                release_gate_trust_model = "private-repo-provenance-fallback-rejected-provenance-enforce"
            elif not allow_priv_fb:
                provenance_release_verdict = "fail"
                release_gate_trust_model = "private-repo-fallback-not-allowed"
            else:
                provenance_release_verdict = PRIVATE_REPO_PROVENANCE_FALLBACK
                if event_name == "workflow_dispatch":
                    release_gate_trust_model = "manual-security-private-repo-no-github-artifact-attestations"
                else:
                    release_gate_trust_model = "private-repo-no-github-artifact-attestations"
        elif provenance_input == "attestation-verify-failed":
            provenance_release_verdict = "fail"
            release_gate_trust_model = "gh-attestation-verify-failed-warn-mode"
        elif provenance_input:
            provenance_release_verdict = "fail"
            release_gate_trust_model = "provenance-verification-failed"
        else:
            provenance_release_verdict = "unavailable"
            release_gate_trust_model = "provenance-evidence-unavailable"
        release_gate_verdict = (
            "pass"
            if (
                repo_required_for_release_verdict == "pass"
                and image_required_for_release_verdict == "pass"
                and provenance_release_verdict in ("pass", PRIVATE_REPO_PROVENANCE_FALLBACK)
            )
            else "fail"
        )
        security_verdict = release_gate_verdict
    else:
        release_gate_mode = "not-applicable"
        repo_required_for_release_verdict = "not_applicable"
        image_required_for_release_verdict = "not_applicable"
        provenance_release_verdict = "not_applicable"
        release_gate_verdict = "not_applicable"
        release_gate_trust_model = "not_applicable"
        applicable = [
            v
            for v in [repo_security_verdict, published_image_verdict, nightly_security_verdict]
            if v != "not_applicable"
        ]
        security_verdict = "pass" if (not applicable or all(v == "pass" for v in applicable)) else "fail"

    sbom_from_pm: dict | None = None
    _pmp = (_s("PROMOTION_MANIFEST_FOR_SBOM") or "").strip()
    if _pmp:
        try:
            _pmt = json.loads(Path(_pmp).read_text(encoding="utf-8"))
            _sbt = _pmt.get("sbom")
            if isinstance(_sbt, dict) and _sbt:
                sbom_from_pm = _sbt
        except (OSError, json.JSONDecodeError):
            pass
    release_candidate_error = ""
    release_candidate_doc: dict | None = None
    _rcp = (_s("RELEASE_CANDIDATE_JSON") or "").strip()
    if _rcp:
        try:
            _rct = json.loads(Path(_rcp).read_text(encoding="utf-8"))
            if isinstance(_rct, dict):
                if not _rct:
                    release_candidate_error = "release-candidate.json must be a non-empty object"
                else:
                    release_candidate_doc = _rct
            else:
                release_candidate_error = "release-candidate.json root must be a JSON object"
        except OSError as e:
            release_candidate_error = "could not read release-candidate.json: %s" % e
        except json.JSONDecodeError as e:
            release_candidate_error = "invalid JSON in release-candidate.json: %s" % e
    sbom_policy = (_s("SBOM_POLICY") or "warn").strip().lower()
    if sbom_policy not in ("warn", "enforce"):
        sbom_policy = "warn"

    if event_name in ("workflow_run", "workflow_dispatch") and not source_event:
        release_gate_verdict = "fail"
        security_verdict = "fail"
    def add_failure_reason(message: str) -> None:
        if message and message not in failure_reasons:
            failure_reasons.append(message)
    if source_sha and len(source_sha) != 40:
        add_failure_reason("Source SHA is not a 40-character hex commit (got %r)." % source_sha)
    if event_name in ("workflow_run", "workflow_dispatch") and not source_event:
        add_failure_reason(
            "Semantic source_event is missing: required from build **promotion-manifest** (do not use `github.event.workflow_run.event` as source_event; see `trigger_workflow_event` for the Build run's GHA event type)."
        )
    if event_name == "workflow_run":
        if repo_required_for_release_verdict != "pass":
            if _s("REPO_RELEASE_EVIDENCE_SOURCE", "") == "security-workflow-run-not-successful":
                add_failure_reason(
                    "Repo-level Security: a matching **Security** workflow run was found for this SHA/branch but it did not complete successfully "
                    "(see run id in repo_level_checks.release_evidence; conclusion=%r)."
                    % _s("REPO_RELEASE_MATCHED_RUN_CONCLUSION", "")
                )
            elif _s("REPO_RELEASE_EVIDENCE_SOURCE", "") in (
                "security-workflow-not-found-or-not-successful",
                "security-workflow-not-found",
            ):
                add_failure_reason(
                    "Repo-level Security: no successful **Security** (repo) run matched after retries for this source_sha and source_branch "
                    "(automatic releases require a **push** Security run when promotion source_event is `push`)."
                )
            else:
                add_failure_reason(
                    "Repo-level Security: release evidence is not sufficient (repo_release_verdict / repo gate not pass). " + _s("REPO_RELEASE_SUMMARY", "")
                )
    if event_name in ("workflow_run", "workflow_dispatch"):
        if image_required_for_release_verdict != "pass":
            rb = results.get("resolve_build_run", "")
            ri = results.get("resolve_image_refs", "")
            im = results.get("image_vulnerability_scan", "")
            if rb not in ("success", "skipped"):
                add_failure_reason(
                    "Image reference resolution / build linkage: **resolve-build-run-id** did not succeed — check the Build and Push Images chain for this commit."
                )
            if ri not in ("success", "skipped"):
                add_failure_reason(
                    "Image reference resolution failed: **_reusable-deploy** did not succeed (missing or inconsistent **image-metadata** / **promotion-manifest** / **immutable-image-contract**, digest-pinned ref mismatch, or workflow_run_id mismatch with artifacts)."
                )
            if im not in ("success", "skipped"):
                add_failure_reason(
                    "Image vulnerability scan: the image scan job did not complete successfully (see `job_results.image_vulnerability_scan` and trivy-image-reports artifact)."
                )
            if results.get("trivy_image_app") == "failure":
                add_failure_reason(
                    "Published image scan (**Trivy**): **app** image did not pass policy (see `trivy-image-reports` / `trivy-image-app.txt`)."
                )
            if results.get("trivy_image_goose") == "failure":
                add_failure_reason(
                    "Published image scan (**Trivy**): **goose** image did not pass policy (see `trivy-image-reports` / `trivy-image-goose.txt`)."
                )
            if signing_mode == "enforce" and (
                results.get("cosign_image_app") == "failure" or results.get("cosign_image_goose") == "failure"
            ):
                add_failure_reason(
                    "Published image **Cosign** verification failed (SIGNING_ENFORCEMENT=enforce): "
                    "app and/or goose digest image is not signed or does not match GitHub Actions OIDC policy for this repository."
                )
        if provenance_release_verdict not in ("pass", PRIVATE_REPO_PROVENANCE_FALLBACK):
            if provenance_release_verdict == "fail":
                if provenance_input == "attestation-verify-failed":
                    add_failure_reason(
                        "Provenance: `gh attestation verify` **failed** for a digest-pinned image (published_images provenance verdict `attestation-verify-failed`). "
                        "Fix attestations or image refs; under `PROVENANCE_ENFORCEMENT=enforce` the reusable-deploy job fails closed instead of recording this verdict."
                    )
                else:
                    add_failure_reason(
                        "Provenance: GitHub artifact attestation verification **failed** or policy rejected the provenance state (not an accepted private-repo fallback)."
                    )
            elif provenance_release_verdict == "unavailable" or not provenance_input:
                add_failure_reason(
                    "Provenance: verification result is **unavailable** or empty (expected `verified`, or private-repo fallback only when ALLOW_PRIVATE_REPO_PROVENANCE_FALLBACK=true and PROVENANCE_ENFORCEMENT=warn)."
                )
            else:
                add_failure_reason("Provenance: release gate rejected provenance state (%r)." % provenance_release_verdict)
    if event_name in ("workflow_run", "workflow_dispatch") and results.get("resolve_image_refs") == "success":
        if not _is_digest_pinned_image_ref(app_image_ref):
            add_failure_reason(
                "Published app image ref must be digest-pinned (…@sha256:…); mutable image tags are not accepted for the Security Release gate."
            )
        if not _is_digest_pinned_image_ref(goose_image_ref):
            add_failure_reason(
                "Published goose image ref must be digest-pinned (…@sha256:…); mutable image tags are not accepted for the Security Release gate."
            )
        if _rcp and release_candidate_error:
            add_failure_reason("Build %s" % release_candidate_error)
        elif not _rcp:
            add_failure_reason(
                "Build **release-candidate** artifact: path not set (expected `RELEASE_CANDIDATE_JSON` from downloaded Build run artifact `release-candidate`)."
            )
        elif release_candidate_doc:
            rcb = str(
                release_candidate_doc.get("build_run_id")
                or release_candidate_doc.get("workflow_run_id")
                or ""
            ).strip()
            if rcb and (source_build_run_id or "").strip() and rcb != str(source_build_run_id).strip():
                add_failure_reason(
                    "Build release-candidate.json `build_run_id` / `workflow_run_id` %r does not match resolved Build run id %r."
                    % (rcb, source_build_run_id)
                )
            ra = (release_candidate_doc.get("app_image_ref") or "").strip() if isinstance(release_candidate_doc.get("app_image_ref"), str) else ""
            rg = (release_candidate_doc.get("goose_image_ref") or "").strip() if isinstance(release_candidate_doc.get("goose_image_ref"), str) else ""
            if ra and (app_image_ref or "").strip() and ra != (app_image_ref or "").strip():
                add_failure_reason(
                    "Build **release-candidate.json** `app_image_ref` does not match the Security Release resolved app digest ref; scans and verdict must use the same Build evidence."
                )
            if rg and (goose_image_ref or "").strip() and rg != (goose_image_ref or "").strip():
                add_failure_reason(
                    "Build **release-candidate.json** `goose_image_ref` does not match the Security Release resolved goose digest ref; scans and verdict must use the same Build evidence."
                )
            for _label, rref, field in (
                ("app", release_candidate_doc.get("app_image_ref"), "app_image_ref"),
                ("goose", release_candidate_doc.get("goose_image_ref"), "goose_image_ref"),
            ):
                sref = (rref or "") if isinstance(rref, str) else ""
                if sref and not _is_digest_pinned_image_ref(sref):
                    add_failure_reason(
                        "Build **release-candidate.json** field %r must be a digest-pinned ref (mutable tags are not accepted)."
                        % (field,)
                    )
        if sbom_policy == "enforce" and not sbom_from_pm:
            add_failure_reason(
                "Supply chain: CycloneDX **sbom** metadata is missing from the Build **promotion-manifest** (expected when `sbom-reports` is produced; SBOM_POLICY=enforce)."
            )
        if sbom_policy == "warn" and not sbom_from_pm:
            warnings.append(
                "Supply chain: **sbom** block missing from the downloaded **promotion-manifest** (SBOM_POLICY=warn; set repository variable `SBOM_POLICY=enforce` to fail closed if SBOM is required)."
            )
    if security_verdict != "pass" and not failure_reasons:
        add_failure_reason("Release gate failed; inspect `job_results` and component verdict fields for the failing upstream job.")
    rr = (_s("RELEASE_RESOLUTION_ERROR") or "").strip()
    if rr:
        add_failure_reason(rr)
        security_verdict = "fail"
        release_gate_verdict = "fail"
    if (not (app_image_ref or "").strip() or not (goose_image_ref or "").strip()) and event_name in (
        "workflow_run",
        "workflow_dispatch",
    ):
        if results.get("resolve_image_refs") in ("failure", "cancelled"):
            add_failure_reason(
                "Build artifacts missing or not resolved: required **image-metadata**, **promotion-manifest**, and **immutable-image-contract** (digest-pinned app/goose refs) from the Build and Push Images run id; see resolve-image-refs and _reusable-deploy."
            )
    if (security_verdict or "") not in ("pass", "fail", "skipped", "no-candidate"):
        add_failure_reason(
            "Internal normalization: security_verdict was %r (expected pass, fail, skipped, or no-candidate; coercing to fail)"
            % (security_verdict,)
        )
        security_verdict = "fail"
        if (release_gate_verdict or "") == "pass":
            release_gate_verdict = "fail"

    if failure_reasons and event_name in ("workflow_run", "workflow_dispatch"):
        security_verdict = "fail"
        release_gate_verdict = "fail"

    decision_reasons = list(failure_reasons)
    _consistency = wnote
    _priv_fb_summary = (
        "workflow_run release gating accepted digest-pinned published images with ALLOW_PRIVATE_REPO_PROVENANCE_FALLBACK=true "
        "(GitHub gh attestation verify is not used on private repos; Cosign + Trivy + digest contract still apply; not full SLSA attestation enforcement)"
        if provenance_input == PRIVATE_REPO_PROVENANCE_FALLBACK and event_name == "workflow_run"
        else (
            "workflow_dispatch release gating accepted digest-pinned published images with private-repo provenance fallback where applicable"
            if provenance_input == PRIVATE_REPO_PROVENANCE_FALLBACK and event_name == "workflow_dispatch"
            else None
        )
    )
    prov_summary_chain = (
        "workflow_run release gating requires repo-level push security evidence plus published-image and provenance checks "
        "(PROVENANCE_ENFORCEMENT=%s, SIGNING_ENFORCEMENT=%s)" % (provenance_mode, signing_mode)
        if provenance_input != PRIVATE_REPO_PROVENANCE_FALLBACK and event_name == "workflow_run"
        else (
            _priv_fb_summary
            if _priv_fb_summary
            else (
                "workflow_dispatch release gating uses the current manual security run plus resolved published-image evidence for the selected build"
                if provenance_input != PRIVATE_REPO_PROVENANCE_FALLBACK and event_name == "workflow_dispatch"
                else "release gating is not evaluated for this event type"
            )
        )
    )
    wrid = _s("WORKFLOW_RUN_ID", "")

    payload: dict = {
        "schema_version": "v1",
        "verdict": security_verdict,
        "security_verdict": security_verdict,
        "canonical_security_artifact": "security-verdict",
        "event_name": event_name,
        "workflow_name": _s("WORKFLOW_NAME", ""),
        "workflow_run_id": wrid,
        "security_workflow_run_id": wrid,
        "generated_at_utc": generated_at_utc,
        "source_build_run_id": source_build_run_id,
        "source_sha": source_sha,
        "source_branch": source_branch,
        "source_event": source_event,
        "trigger_workflow_event": trigger_workflow_event,
        "source_workflow_name": source_workflow_name,
        "release_gate_verdict": release_gate_verdict,
        "release_gate_mode": release_gate_mode,
        "repo_security_verdict": repo_security_verdict,
        "repo_release_verdict": repo_required_for_release_verdict,
        "published_image_verdict": published_image_verdict,
        "provenance_release_verdict": provenance_release_verdict,
        "nightly_security_verdict": nightly_security_verdict,
        "warnings": warnings,
        "verification_scope": {
            "verified_by_this_artifact": [
                "workflow-scoped security verdict for the current event",
                "published image vulnerability scan status included in this run when applicable",
                "published image provenance verdict imported from immutable image resolution when applicable",
                "CycloneDX SBOM pointers from promotion-manifest when PROMOTION_MANIFEST_FOR_SBOM is available",
                "Build **release-candidate** identity and digest-pinned app/goose refs when **RELEASE_CANDIDATE_JSON** is available (Security Release does not accept mutable tags)",
                "Cosign keyless signatures for digest-pinned images when SIGNING_ENFORCEMENT applies",
                "matched push security evidence for repo-level release checks when event_name=workflow_run",
                "current workflow repo-level security evidence when event_name=workflow_dispatch",
            ],
            "not_verified_by_this_artifact": [
                "staging deployment execution",
                "production deployment execution",
                "runtime environment readiness",
                "backup execution",
                "restore drill execution",
            ],
        },
        "release_policy": {
            "supply_chain_sbom_policy": sbom_policy,
            "repo_level_required_for_release": [
                "govulncheck_repo",
                "secret_scan",
                "trivy_config",
            ],
            "repo_level_advisory_for_release": [
                "dependency_review",
            ],
            "not_release_gating": [
                "govulncheck_nightly",
                "dependency_snapshot",
            ],
            "image_level_required_for_release": [
                "resolve_build_run",
                "resolve_image_refs",
                "trivy_images",
                "cosign_images_when_enforced",
            ],
            "provenance_required_for_release": [
                "published_image_provenance",
            ],
        },
        "repo_level_checks": {
            "current_event_verdict": repo_security_verdict,
            "release_evidence": {
                "evidence_source": _s("REPO_RELEASE_EVIDENCE_SOURCE", ""),
                "source_event_name": _s("REPO_RELEASE_SOURCE_EVENT", ""),
                "matched_workflow_run_id": _s("REPO_RELEASE_MATCHED_RUN_ID", ""),
                "matched_workflow_conclusion": _s("REPO_RELEASE_MATCHED_RUN_CONCLUSION", ""),
                "required_for_release_verdict": repo_required_for_release_verdict,
                "summary": _s("REPO_RELEASE_SUMMARY", ""),
            },
            "checks": repo_release_checks,
        },
        "image_level_checks": {
            "required_for_release_verdict": image_required_for_release_verdict,
            "signing_enforcement": signing_mode,
            "checks": {
                "resolve_build_run": _normalize_result(results["resolve_build_run"]),
                "resolve_image_refs": _normalize_result(results["resolve_image_refs"]),
                "trivy_images": _normalize_result(results["image_vulnerability_scan"]),
                "trivy_image_app": _normalize_result(results.get("trivy_image_app", "skipped")),
                "trivy_image_goose": _normalize_result(results.get("trivy_image_goose", "skipped")),
                "cosign_image_app": _normalize_result(results.get("cosign_image_app", "skipped")),
                "cosign_image_goose": _normalize_result(results.get("cosign_image_goose", "skipped")),
            },
        },
        "provenance_release_checks": {
            "required_for_release_verdict": provenance_release_verdict,
            "published_image_provenance_verdict": provenance_input,
            "provenance_enforcement": provenance_mode,
            "allow_private_repo_provenance_fallback": "true" if allow_priv_fb else "false",
            "signing_enforcement": signing_mode,
            "evidence_source": (
                "resolve-image-refs workflow output (private repo fallback: GitHub Artifact Attestations unavailable; explicit ALLOW_PRIVATE_REPO_PROVENANCE_FALLBACK)"
                if provenance_input == PRIVATE_REPO_PROVENANCE_FALLBACK
                else ("resolve-image-refs workflow output" if provenance_input else "not_applicable")
            ),
        },
        "release_gate": {
            "mode": release_gate_mode,
            "verdict": release_gate_verdict,
            "generated_at_utc": generated_at_utc,
            "trust_model": release_gate_trust_model,
            "required_evidence": {
                "repo_level_checks": repo_required_for_release_verdict,
                "image_level_checks": image_required_for_release_verdict,
                "provenance_release_checks": provenance_release_verdict,
            },
            "summary": prov_summary_chain,
        },
        "scanner_summary": {
            "dependency_review": _normalize_result(results["dependency_review"]),
            "govulncheck_repo": _normalize_result(results["govulncheck_pr"]),
            "secret_scan": _normalize_result(results["secret_scan"]),
            "trivy_config": _normalize_result(results["config_scan"]),
            "trivy_images": _normalize_result(results["image_vulnerability_scan"]),
            "trivy_image_app": _normalize_result(results.get("trivy_image_app", "skipped")),
            "trivy_image_goose": _normalize_result(results.get("trivy_image_goose", "skipped")),
            "cosign_image_app": _normalize_result(results.get("cosign_image_app", "skipped")),
            "cosign_image_goose": _normalize_result(results.get("cosign_image_goose", "skipped")),
            "govulncheck_nightly": _normalize_result(results["govulncheck_nightly"]),
        },
        "published_images": {
            "app_image_ref": app_image_ref,
            "app_digest": _digest_from_ref(app_image_ref),
            "goose_image_ref": goose_image_ref,
            "goose_digest": _digest_from_ref(goose_image_ref),
            "provenance_verdict": provenance_input,
            "provenance_verdict_source": (
                "resolve-image-refs workflow output (private repo fallback: GitHub Artifact Attestations unavailable)"
                if provenance_input == PRIVATE_REPO_PROVENANCE_FALLBACK
                else "resolve-image-refs workflow output"
            ),
        },
        "build_release_evidence": {
            "consumed_artifact": "release-candidate",
            "local_path": _rcp,
            "sbom_policy": sbom_policy,
            "source_sha": (release_candidate_doc or {}).get("source_sha", "") if release_candidate_doc else "",
            "source_branch": (release_candidate_doc or {}).get("source_branch", "") if release_candidate_doc else "",
            "build_workflow_run_id": (
                (str((release_candidate_doc or {}).get("workflow_run_id", "") or "").strip() or str((release_candidate_doc or {}).get("build_run_id", "") or "").strip())
                if release_candidate_doc
                else ""
            ),
            "repo": (release_candidate_doc or {}).get("repo", "") if release_candidate_doc else "",
            "app_image_ref": (release_candidate_doc or {}).get("app_image_ref", "") if release_candidate_doc else "",
            "goose_image_ref": (release_candidate_doc or {}).get("goose_image_ref", "") if release_candidate_doc else "",
            "generated_at_utc": (release_candidate_doc or {}).get("generated_at_utc", "") if release_candidate_doc else "",
        },
        "job_results": results,
        "failure_reasons": failure_reasons,
        "decision_reasons": decision_reasons,
    }
    _ca = results.get("cosign_image_app", "skipped")
    _cg = results.get("cosign_image_goose", "skipped")
    _app_sl = "pass" if _ca == "success" else ("fail" if _ca == "failure" else "skipped")
    _goose_sl = "pass" if _cg == "success" else ("fail" if _cg == "failure" else "skipped")
    if _app_sl == "pass" and _goose_sl == "pass":
        _overall_sl = "pass"
    elif _app_sl == "fail" or _goose_sl == "fail":
        _overall_sl = "fail"
    else:
        _overall_sl = "skipped"
    payload["image_signing"] = {
        "enforcement": signing_mode,
        "app": _app_sl,
        "goose": _goose_sl,
        "overall": _overall_sl,
        "oidc_issuer": "https://token.actions.githubusercontent.com",
    }
    if _consistency:
        payload["artifact_consistency"] = {
            "note": _consistency,
            "source": "build-run-immutable-image-contract-compare",
        }

    if sbom_from_pm:
        payload["sbom"] = sbom_from_pm
    else:
        sbom_prom_path = (_s("PROMOTION_MANIFEST_FOR_SBOM") or "").strip()
        if sbom_prom_path:
            try:
                pm_data = json.loads(Path(sbom_prom_path).read_text(encoding="utf-8"))
                _sb = pm_data.get("sbom")
                if isinstance(_sb, dict) and _sb:
                    payload["sbom"] = _sb
            except (OSError, json.JSONDecodeError):
                pass

    # Observed coordinates for debugging (never silent mixing)
    if event_name == "workflow_run":
        payload["source_coordinates"] = {
            "canonical_source_sha": can_sha or artifact_sha or source_sha,
            "canonical_source_branch": can_br or resolved_source_branch or source_branch,
            "canonical_source_event": (can_ev or source_event) or "",
            "artifact_source_sha": artifact_sha,
            "artifact_source_branch": resolved_source_branch,
            "artifact_source_event": (_s("ARTIFACT_SOURCE_EVENT") or "").strip(),
            "trigger_workflow_head_sha": tr_sha,
            "trigger_workflow_head_branch": build_head_branch,
            "trigger_workflow_event": (_s("TRIGGERING_BUILD_EVENT") or "").strip(),
        }
        payload["metadata_validation"] = {
            "triggering_build_run_id": _s("TRIGGERING_BUILD_ID", ""),
            "triggering_build_event": (_s("TRIGGERING_BUILD_EVENT") or "").strip(),
            "triggering_build_head_branch": build_head_branch,
            "triggering_build_head_sha": tr_sha,
            "artifact_source_event": (_s("ARTIFACT_SOURCE_EVENT") or "").strip(),
            "resolved_source_branch": resolved_source_branch,
            "resolved_source_sha": artifact_sha,
            "build_run_id_from_artifacts": source_build_run_id,
            "decision": security_verdict,
        }

    _write(payload)


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument(
        "mode",
        choices=CONTRACT_VERDICT_MODES,
        help="Verdict document to write",
    )
    ap.add_argument("--emergency-reason", default="", help="Only for mode=emergency")
    ap.add_argument(
        "--emergency-force",
        action="store_true",
        help="Only for mode=emergency: overwrite an existing pass/fail/skipped/no-candidate verdict (default: preserve)",
    )
    args = ap.parse_args()
    exit_code = 0
    try:
        if args.mode == "skipped":
            write_skipped()
        elif args.mode == "no-candidate":
            write_no_candidate()
        elif args.mode == "unsupported-trigger":
            write_unsupported_trigger_skipped()
        elif args.mode == "ineligible-branch":
            write_ineligible_branch_skipped()
        elif args.mode == "unsupported-artifact-source-event":
            write_unsupported_artifact_source_event_skipped()
        elif args.mode == "metadata-mismatch":
            write_metadata_mismatch_skipped()
        elif args.mode == "full":
            write_full()
        else:
            write_emergency(args.emergency_reason, force=args.emergency_force)
    except Exception as e:  # noqa: BLE001 — last-resort so CI always gets a machine-readable file
        try:
            write_emergency("Unexpected exception in write_security_verdict mode=%r: %s" % (args.mode, e), force=False)
        except Exception as e2:  # noqa: BLE001
            _write(
                {
                    "verdict": "fail",
                    "security_verdict": "fail",
                    "release_gate_mode": "emergency-fail",
                    "release_gate_verdict": "fail",
                    "failure_reasons": [
                        "Internal: write_emergency after exception failed: %r; original: %r" % (e2, e)
                    ],
                }
            )
        exit_code = 1
    if not VERDICT_PATH.is_file():
        try:
            write_emergency("Internal: verdict writer completed without creating the output file", force=False)
        except Exception:  # noqa: BLE001
            _write(
                {
                    "verdict": "fail",
                    "security_verdict": "fail",
                    "failure_reasons": ["Internal: could not write emergency verdict; minimal fail payload"],
                }
            )
        exit_code = max(exit_code, 2)
    # Validate JSON
    try:
        json.loads(VERDICT_PATH.read_text(encoding="utf-8"))
    except json.JSONDecodeError as e:
        try:
            write_emergency("Internal: written verdict was not valid JSON: %s" % e, force=False)
        except Exception:  # noqa: BLE001
            _write(
                {
                    "verdict": "fail",
                    "security_verdict": "fail",
                    "failure_reasons": ["Internal: invalid JSON after write; minimal fail payload"],
                }
            )
        exit_code = max(exit_code, 3)
    if exit_code:
        sys.exit(exit_code)


if __name__ == "__main__":
    main()
