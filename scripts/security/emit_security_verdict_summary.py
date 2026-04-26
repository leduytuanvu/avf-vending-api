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
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (json.JSONDecodeError, OSError) as e:
        print("error: could not parse verdict JSON: %s" % e, file=sys.stderr)
        sys.exit(1)


def _image_refs_from_payload(payload: dict) -> tuple[str, str]:
    pub = payload.get("published_images")
    if not isinstance(pub, dict):
        pub = {}
    a = str(payload.get("app_image_ref", "") or "").strip()
    g = str(payload.get("goose_image_ref", "") or "").strip()
    if not a:
        a = str(pub.get("app_image_ref", "") or "")
    if not g:
        g = str(pub.get("goose_image_ref", "") or "")
    return (a, g)


def emit_main(payload: dict) -> None:
    print("## Security Release Signal")
    print("- source of truth: `security-verdict` is the canonical release/security artifact")
    print("- scope: `security-verdict` distinguishes current-event security results from workflow_run release-gate approval")
    print("- event: `%s`" % payload.get("event_name", ""))
    print("")
    print("### Verdict summary (observability)")
    print("| Field | Value |")
    print("| --- | --- |")
    _iapp, _igoose = _image_refs_from_payload(payload)
    _rows = (
        ("verdict", payload.get("verdict", "")),
        ("release_gate_verdict", payload.get("release_gate_verdict", "")),
        ("release_gate_mode", payload.get("release_gate_mode", "")),
        ("repo_security_verdict", payload.get("repo_security_verdict", "")),
        ("repo_release_verdict", payload.get("repo_release_verdict", "")),
        ("published_image_verdict", payload.get("published_image_verdict", "")),
        ("provenance_release_verdict", payload.get("provenance_release_verdict", "")),
        ("source_sha", payload.get("source_sha", "")),
        ("source_branch", payload.get("source_branch", "")),
        ("source_build_run_id", payload.get("source_build_run_id", "")),
        (
            "security_workflow_run_id",
            payload.get("security_workflow_run_id") or payload.get("workflow_run_id", ""),
        ),
        ("source_workflow_name", payload.get("source_workflow_name", "")),
        ("generated_at_utc", payload.get("generated_at_utc", "")),
        ("app_image_ref", _iapp),
        ("goose_image_ref", _igoose),
    )
    for k, v in _rows:
        print("| `%s` | `%s` |" % (k, v))
    _fr = payload.get("failure_reasons") or []
    print("| `failure_reasons` | %s |" % ("; ".join(str(x) for x in _fr) if _fr else "—"))
    print("")
    print("- verdict: `%s`" % payload.get("verdict", ""))
    print("- release gate: mode=`%s` verdict=`%s`" % (payload.get("release_gate_mode", ""), payload.get("release_gate_verdict", "")))
    mval = payload.get("metadata_validation")
    if isinstance(mval, dict) and mval:
        print("")
        print("### Build vs artifact metadata (observability)")
        print("")
        print("| Field | Value |")
        print("| --- | --- |")
        _order = (
            "triggering_build_run_id",
            "build_run_id_from_artifacts",
            "triggering_build_event",
            "triggering_build_head_branch",
            "triggering_workflow_name",
            "triggering_workflow_conclusion",
            "artifact_source_event",
            "resolved_source_branch",
            "resolved_source_sha",
            "decision",
        )
        _seen: set[str] = set()
        for key in _order:
            if key in mval and mval[key] is not None and str(mval[key]) != "":
                print("| `%s` | `%s` |" % (key, mval.get(key, "")))
                _seen.add(key)
        for k in sorted(mval.keys()):
            if k not in _seen:
                print("| `%s` | `%s` |" % (k, mval.get(k, "")))
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
    print("")
    print("### Canonical verdict (blocking for deploy)")
    print("- **verdict**: `%s` — deploy workflows accept **pass** only." % payload.get("verdict", ""))
    if vd == "skipped":
        print("- **Meaning**: ineligible Security Release run (upstream Build not a promotion candidate). **Not deployable.**")
    elif vd == "no-candidate":
        print("- **Meaning**: no valid release candidate for this chain (resolve/build prerequisites). **Not deployable.**")
    elif vd == "fail":
        print("- **Meaning**: policy or evidence failure on a real candidate. **Not deployable.**")
    elif vd == "pass":
        print("- **Meaning**: release candidate passed policy; downstream deploy workflows may proceed (they still require **pass** and branch/digest gates).")
    _reasons = payload.get("failure_reasons") or []
    if vd in ("skipped", "no-candidate") and _reasons:
        print("")
        print("### Why this outcome is not deployable")
        for reason in _reasons:
            print("- %s" % reason)
    if vd == "fail":
        print("")
        print("### Failure reasons (policy / evidence)")
        for reason in _reasons:
            print("- %s" % reason)
    repo_release = (payload.get("repo_level_checks") or {}).get("release_evidence") or {}
    if repo_release.get("evidence_source"):
        print("- repo release evidence source: `%s`" % repo_release["evidence_source"])
    if repo_release.get("matched_workflow_run_id"):
        print("- matched push security workflow run: `%s`" % repo_release["matched_workflow_run_id"])
    pub = payload.get("published_images") or {}
    print("")
    print("### Published image refs (digest-pinned when present)")
    if pub.get("app_image_ref"):
        print("- app image: `%s`" % pub["app_image_ref"])
    else:
        print("- app image: _(none — gate did not resolve images for this outcome)_")
    if pub.get("goose_image_ref"):
        print("- goose image: `%s`" % pub["goose_image_ref"])
    else:
        print("- goose image: _(none — gate did not resolve images for this outcome)_")
    if pub.get("provenance_verdict"):
        print(
            "- published image provenance verdict: `%s` (from `%s`)"
            % (pub["provenance_verdict"], pub.get("provenance_verdict_source", ""))
        )
    if pub.get("provenance_verdict") == "accepted-private-repo-no-github-attestations":
        print(
            "- provenance note: **`Provenance is not fully enforced` on GitHub attestations** — private-repo fallback "
            "(`ALLOW_PRIVATE_REPO_PROVENANCE_FALLBACK=true`, `PROVENANCE_ENFORCEMENT=warn`). "
            "Digest-pinned refs, Trivy, and Cosign (per `SIGNING_ENFORCEMENT`) still apply."
        )
    prc = payload.get("provenance_release_checks") or {}
    if isinstance(prc, dict) and prc:
        print("")
        print("### Supply-chain policy (observability)")
        print("| Variable (verdict snapshot) | Value |")
        print("| --- | --- |")
        for key in ("provenance_enforcement", "allow_private_repo_provenance_fallback", "signing_enforcement"):
            if key in prc and prc[key] not in (None, ""):
                print("| `%s` | `%s` |" % (key, prc[key]))
        if prc.get("evidence_source"):
            print("| `evidence_source` | `%s` |" % prc["evidence_source"])
    prv_rel = (payload.get("provenance_release_verdict") or "").strip()
    if prv_rel == "accepted-private-repo-no-github-attestations" or pub.get("provenance_verdict") == "accepted-private-repo-no-github-attestations":
        print("")
        print(
            "> **Visibility:** GitHub `gh attestation verify` was not used as the trust anchor for this release verdict. "
            "Treat Cosign + digest contract + scan gates as the active controls until you move to `PROVENANCE_ENFORCEMENT=enforce` on a repo that supports attestations."
        )
    sbom = payload.get("sbom") or {}
    if isinstance(sbom, dict) and sbom.get("artifact_name"):
        _sbr = payload.get("source_build_run_id", "")
        print(
            "- SBOM: artifact `%s` (%s **%s**) for digest-pinned app/goose — download from **Build and Push Images** run `%s`"
            % (
                sbom.get("artifact_name", ""),
                sbom.get("tool", ""),
                sbom.get("format", ""),
                _sbr,
            )
        )
        if sbom.get("generated_at_utc"):
            print("  - SBOM generated at (UTC): `%s`" % sbom["generated_at_utc"])
    img_sig = payload.get("image_signing") or {}
    if isinstance(img_sig, dict) and img_sig.get("enforcement"):
        print(
            "- image signing: enforcement=`%s` overall=`%s` app=`%s` goose=`%s` (OIDC issuer `%s`)"
            % (
                img_sig.get("enforcement", ""),
                img_sig.get("overall", ""),
                img_sig.get("app", ""),
                img_sig.get("goose", ""),
                img_sig.get("oidc_issuer", ""),
            )
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
    _prc2 = payload.get("provenance_release_checks") or {}
    if isinstance(_prc2, dict) and _prc2:
        print(
            "- **provenance_enforcement / allow_private_repo_provenance_fallback / signing_enforcement**: "
            "`%s` / `%s` / `%s`"
            % (
                _prc2.get("provenance_enforcement", ""),
                _prc2.get("allow_private_repo_provenance_fallback", ""),
                _prc2.get("signing_enforcement", ""),
            )
        )
    print("- **nightly_security_verdict**: `%s`" % payload.get("nightly_security_verdict", ""))
    print("- **source_sha**: `%s`" % payload.get("source_sha", ""))
    print("- **source_branch**: `%s`" % payload.get("source_branch", ""))
    print("- **source_build_run_id**: `%s`" % payload.get("source_build_run_id", ""))
    print("- **source_workflow_name**: `%s`" % payload.get("source_workflow_name", ""))
    _fr2 = payload.get("failure_reasons") or []
    if _fr2:
        print("- **failure_reasons**:")
        for r in _fr2:
            print("  - %s" % r)
    else:
        print("- **failure_reasons**: _(none)_")
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
    if v in ("skipped", "no-candidate"):
        print("")
        print("#### Not deployable (structured)")
        fr = payload.get("failure_reasons") or []
        if fr:
            for reason in fr:
                print("- **%s**" % reason)
        else:
            print("- (no structured failure_reasons; see release_gate summary)")
    elif v != "pass":
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
