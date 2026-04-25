#!/usr/bin/env python3
"""Print markdown sections for GITHUB_STEP_SUMMARY from security-verdict.json."""
from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path


def _load() -> dict:
    path = Path(os.environ.get("SECURITY_VERDICT_JSON", "security-reports/security-verdict.json"))
    if not path.is_file():
        print("error: security verdict JSON not found: %s" % path, file=sys.stderr)
        sys.exit(1)
    return json.loads(path.read_text(encoding="utf-8"))


def emit_main(payload: dict) -> None:
    print("## Security Release Signal")
    print("- source of truth: `security-verdict` is the canonical release/security artifact")
    print("- scope: `security-verdict` distinguishes current-event security results from workflow_run release-gate approval")
    print("- event: `%s`" % payload.get("event_name", ""))
    print("- verdict: `%s`" % payload.get("verdict", ""))
    print("- release gate: mode=`%s` verdict=`%s`" % (payload.get("release_gate_mode", ""), payload.get("release_gate_verdict", "")))
    rg = payload.get("release_gate") or {}
    print("- release gate generated at (UTC): `%s`" % rg.get("generated_at_utc", payload.get("generated_at_utc", "")))
    print("- release gate trust model: `%s`" % rg.get("trust_model", ""))
    print("- repo security verdict: `%s`" % payload.get("repo_security_verdict", ""))
    print("- repo release verdict: `%s`" % payload.get("repo_release_verdict", ""))
    print("- published image verdict: `%s`" % payload.get("published_image_verdict", ""))
    print("- provenance release verdict: `%s`" % payload.get("provenance_release_verdict", ""))
    if payload.get("source_build_run_id"):
        print("- source build run id: `%s`" % payload["source_build_run_id"])
    if payload.get("source_sha"):
        print("- source SHA: `%s`" % payload["source_sha"])
    print("- source_event (from promotion-manifest / artifact): `%s`" % payload.get("source_event", ""))
    print(
        "- trigger_workflow_event (Build run / this Security run GHA event, not semantic `source_event`): `%s`"
        % payload.get("trigger_workflow_event", "")
    )
    vd = (payload.get("verdict") or "").lower()
    if vd not in ("pass", "skipped"):
        print("")
        print("### Why `SECURITY_VERDICT` is not pass")
        for reason in payload.get("failure_reasons") or []:
            print("- %s" % reason)
    repo_release = (payload.get("repo_level_checks") or {}).get("release_evidence") or {}
    if repo_release.get("evidence_source"):
        print("- repo release evidence source: `%s`" % repo_release["evidence_source"])
    if repo_release.get("matched_workflow_run_id"):
        print("- matched push security workflow run: `%s`" % repo_release["matched_workflow_run_id"])
    pub = payload.get("published_images") or {}
    if pub.get("app_image_ref"):
        print("- app image: `%s`" % pub["app_image_ref"])
    if pub.get("goose_image_ref"):
        print("- goose image: `%s`" % pub["goose_image_ref"])
    if pub.get("provenance_verdict"):
        print(
            "- published image provenance verdict: `%s` (from `%s`)"
            % (pub["provenance_verdict"], pub.get("provenance_verdict_source", ""))
        )
    if pub.get("provenance_verdict") == "accepted-private-repo-no-github-attestations":
        print(
            "- provenance note: `GitHub Artifact Attestations are unavailable for this private repository, "
            "so release gating falls back to digest-pinned images plus the remaining security checks`"
        )
    print("- required release checks: `govulncheck_repo, secret_scan, trivy_config, published image scan, published image provenance`")
    print("- advisory release checks: `dependency_review`")
    print("- excluded controls: `deployment execution, runtime readiness, backup execution, restore drill execution`")


def emit_observability(payload: dict) -> None:
    print("### Verdict payload (canonical)")
    print("- **verdict (blocking)**: `", payload.get("verdict", ""), "`", sep="")
    print("- **release_gate_verdict**: `%s`" % payload.get("release_gate_verdict", ""))
    print("- **release_gate_mode**: `%s`" % payload.get("release_gate_mode", ""))
    print("- **repo_security_verdict**: `%s`" % payload.get("repo_security_verdict", ""))
    print("- **repo_release_verdict**: `%s`" % payload.get("repo_release_verdict", ""))
    print("- **published_image_verdict**: `%s`" % payload.get("published_image_verdict", ""))
    print("- **provenance_release_verdict**: `%s`" % payload.get("provenance_release_verdict", ""))
    print("- **nightly_security_verdict**: `%s`" % payload.get("nightly_security_verdict", ""))
    print("- **source_sha**: `%s`" % payload.get("source_sha", ""))
    print("- **source_build_run_id**: `%s`" % payload.get("source_build_run_id", ""))
    print("- **source_workflow_name**: `%s`" % payload.get("source_workflow_name", ""))
    print(
        "- **security workflow_run_id (artifact)**: `%s`"
        % (payload.get("security_workflow_run_id") or payload.get("workflow_run_id", ""))
    )
    jr = payload.get("job_results") or {}
    print("")
    print("#### job_results (upstream job conclusions)")
    for k, v in sorted(jr.items()):
        print("- `%s`: `%s`" % (k, v))
    v = (payload.get("verdict") or "").lower()
    if v not in ("pass", "skipped"):
        print("")
        print("#### Why `SECURITY_VERDICT` is not pass (structured)")
        fr = payload.get("failure_reasons") or []
        if fr:
            for reason in fr:
                print("- **%s**" % reason)
        else:
            print("- (no structured failure_reasons; see hints below)")
        print("")
        print("#### Additional hints")
        rg = payload.get("release_gate_verdict", "")
        rs = payload.get("repo_security_verdict", "")
        rrel = payload.get("repo_release_verdict", "")
        pub = payload.get("published_image_verdict", "")
        prov = payload.get("provenance_release_verdict", "")
        ev = payload.get("event_name", "")
        hints = []
        if ev == "workflow_run" and (not rrel or rrel not in ("pass",)):
            hints.append(
                "`repo_release_verdict` is not `pass` (for example `unavailable`); check that a **push** `Security` run "
                "for this SHA published a `security-verdict` artifact, or that freshness/overrides are policy-correct."
            )
        if rs != "pass" and rs != "not_applicable":
            hints.append(
                "`repo_security_verdict` is not `pass` — a repo-level check (dependency review, govulncheck-pr, "
                "secret scan, trivy config) failed or was not successful."
            )
        if pub != "pass" and pub != "not_applicable":
            hints.append(
                "`published_image_verdict` is not `pass` — build resolution, image refs, and/or the image "
                "vulnerability scan did not all succeed."
            )
        if prov not in ("pass", "accepted-private-repo-no-github-attestations") and prov != "not_applicable":
            hints.append(
                "`provenance_release_verdict` is not acceptable for the gate (expect `pass` or the private-repo "
                "provenance fallback when applicable)."
            )
        if prov == "unavailable" or (not prov and ev == "workflow_run"):
            hints.append("Provenance was **unavailable** or missing — `resolve-image-refs` may not have produced a verdict.")
        if rg and rg != "pass":
            hints.append(
                "`release_gate_verdict` is `%s` — at least one required component for the full release gate did not pass "
                "(see table above and `job_results`)." % rg
            )
        if not hints:
            hints.append("Compare `job_results` to see which `needs.*` job did not report `success`.")
        for h in hints:
            print("- %s" % h)


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--section", choices=("main", "observability"), required=True)
    args = ap.parse_args()
    payload = _load()
    if args.section == "main":
        emit_main(payload)
    else:
        emit_observability(payload)


if __name__ == "__main__":
    main()
