#!/usr/bin/env bash
# Assemble an enterprise release evidence folder: static verify, monitoring readiness,
# telemetry storm (when fleet scale > pilot), production deployment manifest, checksums, and summary.
#
# Required env:
#   RELEASE_TAG
#   SOURCE_COMMIT_SHA
#   VERIFY_RESULT_PATH          JSON with final_result "pass" (see docs/runbooks/production-release-readiness.md)
#   DEPLOYMENT_MANIFEST_PATH    production-deployment-manifest.json from a successful deploy
#   OUTPUT_DIR                  directory to create (must not exist, or use ALLOW_OUTPUT_DIR_OVERWRITE=1)
#   KNOWN_RISKS_PATH            non-empty markdown file (organization-specific + repo baseline risks)
#
# Conditionally required when manifest fleet_scale_target is not "pilot":
#   MONITORING_RESULT_PATH      monitoring-readiness-result.json (final_result must be pass)
#   STORM_RESULT_PATH           telemetry-storm-result.json (strict validation below)
#
# Optional:
#   FLEET_SCALE_TARGET           alias for EXPECTED_FLEET_SCALE_TARGET; if set, must equal manifest fleet_scale_target
#   EXPECTED_FLEET_SCALE_TARGET  if set, must equal manifest fleet_scale_target
#   STORM_EVIDENCE_MAX_AGE_DAYS  max age for storm completed_at_utc (default 30 for evidence pack assembly)
#   ALLOW_OUTPUT_DIR_OVERWRITE   if "1", rm -rf OUTPUT_DIR first
#   ROLLBACK_UNAVAILABLE_EXPLANATION  required when manifest rollback_available_before_deploy is false
#
# Scale tiers (not pilot): storm JSON must pass deployments/prod/shared/scripts/validate_production_scale_storm_evidence.py
# field rules (machine_count/events_per_machine thresholds, critical_lost=0, duplicate_critical_effects=0,
# db_pool_result/health_result/restart_result=pass, execute_load_test=true, dry_run=false).
#
#
# Outputs in OUTPUT_DIR (on PASS, includes evidence copies; on FAIL, pack + summary only):
#   release-evidence-pack.json
#   release-evidence-summary.md
#   evidence-verify.json (PASS only)
#   evidence-monitoring-readiness.json (PASS only, when required or provided)
#   evidence-telemetry-storm.json (PASS only, when required or provided)
#   evidence-production-deployment-manifest.json (PASS only)
#   known-risks.md (PASS only)
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
cd "${REPO_ROOT}"
export REPO_ROOT

EXPECTED_FLEET_SCALE_TARGET="${EXPECTED_FLEET_SCALE_TARGET:-${FLEET_SCALE_TARGET:-}}"
export EXPECTED_FLEET_SCALE_TARGET

fail() {
	echo "build_release_evidence_pack: error: $*" >&2
	exit 1
}

: "${RELEASE_TAG:?set RELEASE_TAG}"
: "${SOURCE_COMMIT_SHA:?set SOURCE_COMMIT_SHA}"
: "${VERIFY_RESULT_PATH:?set VERIFY_RESULT_PATH}"
: "${DEPLOYMENT_MANIFEST_PATH:?set DEPLOYMENT_MANIFEST_PATH}"
: "${OUTPUT_DIR:?set OUTPUT_DIR}"
: "${KNOWN_RISKS_PATH:?set KNOWN_RISKS_PATH}"

[[ -f "${VERIFY_RESULT_PATH}" ]] || fail "VERIFY_RESULT_PATH not a file: ${VERIFY_RESULT_PATH}"
[[ -f "${DEPLOYMENT_MANIFEST_PATH}" ]] || fail "DEPLOYMENT_MANIFEST_PATH not a file: ${DEPLOYMENT_MANIFEST_PATH}"
[[ -f "${KNOWN_RISKS_PATH}" ]] || fail "KNOWN_RISKS_PATH not a file: ${KNOWN_RISKS_PATH}"
[[ -s "${KNOWN_RISKS_PATH}" ]] || fail "KNOWN_RISKS_PATH must be non-empty: ${KNOWN_RISKS_PATH}"

if [[ "${ALLOW_OUTPUT_DIR_OVERWRITE:-0}" == "1" ]]; then
	rm -rf "${OUTPUT_DIR}"
fi
if [[ -e "${OUTPUT_DIR}" ]]; then
	fail "OUTPUT_DIR already exists: ${OUTPUT_DIR} (set ALLOW_OUTPUT_DIR_OVERWRITE=1 to replace)"
fi
mkdir -p "${OUTPUT_DIR}"

export _OUT="${OUTPUT_DIR}"
export _VERIFY="${VERIFY_RESULT_PATH}"
export _MON="${MONITORING_RESULT_PATH:-}"
export _STORM="${STORM_RESULT_PATH:-}"
export _MAN="${DEPLOYMENT_MANIFEST_PATH}"
export _RISKS="${KNOWN_RISKS_PATH}"
export _TAG="${RELEASE_TAG}"
export _SHA="${SOURCE_COMMIT_SHA}"
export _EXP_SCALE="${EXPECTED_FLEET_SCALE_TARGET:-}"
export _RB_EXPLAIN="${ROLLBACK_UNAVAILABLE_EXPLANATION:-}"
export _STORM_MAX_AGE="${STORM_EVIDENCE_MAX_AGE_DAYS:-30}"

python3 - <<'PY'
import hashlib
import importlib.util
import json
import os
import shutil
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Optional

def load_storm_validator():
    root = Path(os.environ["REPO_ROOT"])
    path = root / "deployments/prod/shared/scripts/validate_production_scale_storm_evidence.py"
    spec = importlib.util.spec_from_file_location("storm_gate", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load storm validator from {path}")
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod

out = Path(os.environ["_OUT"])
verify_path = Path(os.environ["_VERIFY"])
mon_path = (os.environ.get("_MON") or "").strip()
storm_path = (os.environ.get("_STORM") or "").strip()
man_path = Path(os.environ["_MAN"])
risks_src = Path(os.environ["_RISKS"])
tag = os.environ["_TAG"].strip()
sha = os.environ["_SHA"].strip()
exp_scale = (os.environ.get("_EXP_SCALE") or "").strip()
rb_explain = (os.environ.get("_RB_EXPLAIN") or "").strip()
storm_max_age_raw = (os.environ.get("_STORM_MAX_AGE") or "30").strip()

reasons: list[str] = []
ALLOWED_FLEETS = {"pilot", "scale-100", "scale-500", "scale-1000"}

storm_max_age_days: Optional[int] = None
try:
    storm_max_age_days = int(storm_max_age_raw)
    if storm_max_age_days < 1:
        raise ValueError
except ValueError:
    reasons.append("STORM_EVIDENCE_MAX_AGE_DAYS must be a positive integer")


def load_json(path: Path):
    try:
        doc = json.loads(path.read_text(encoding="utf-8"))
        if not isinstance(doc, dict):
            reasons.append(f"{path.name}: JSON root must be an object")
            return None
        return doc
    except (OSError, json.JSONDecodeError) as e:
        reasons.append(f"cannot read JSON {path}: {e}")
        return None


def require_pass(doc: dict | None, label: str) -> None:
    if doc is None:
        return
    fr = str(doc.get("final_result", "")).strip().lower()
    if fr != "pass":
        reasons.append(f"{label}: expected final_result=pass, got {doc.get('final_result')!r}")


def storm_pass(doc: dict | None, label: str) -> None:
    if doc is None:
        return
    if str(doc.get("final_result", "")).strip().lower() == "pass":
        return
    if doc.get("final_suite_pass") is True:
        return
    reasons.append(
        f"{label}: storm evidence must have final_result=pass or final_suite_pass=true, "
        f"got final_result={doc.get('final_result')!r} final_suite_pass={doc.get('final_suite_pass')!r}"
    )


def digest_pinned(ref: str) -> bool:
    r = (ref or "").strip()
    return "@sha256:" in r and ":latest" not in r


def sha256_file(p: Path) -> str:
    h = hashlib.sha256()
    with p.open("rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()


manifest = load_json(man_path)
fleet = "unknown"
app_ref = ""
goose_ref = ""
rb_ok = False
verify = None
mon_doc = None
storm_doc = None

if manifest is not None:
    fleet = str(manifest.get("fleet_scale_target") or "pilot").strip().lower()
    if fleet not in ALLOWED_FLEETS:
        reasons.append(
            f"fleet_scale_target must be one of {sorted(ALLOWED_FLEETS)} (got {fleet!r})"
        )
    if exp_scale and exp_scale.lower() != fleet:
        reasons.append(
            f"fleet_scale_target mismatch: manifest={fleet!r} "
            f"EXPECTED_FLEET_SCALE_TARGET/FLEET_SCALE_TARGET={exp_scale!r}"
        )

    msha = str(manifest.get("source_commit_sha") or "").strip()
    mtag = str(manifest.get("release_tag") or "").strip()
    if msha and msha != sha:
        reasons.append(f"SOURCE_COMMIT_SHA {sha!r} != manifest source_commit_sha {msha!r}")
    if mtag and mtag != tag:
        reasons.append(f"RELEASE_TAG {tag!r} != manifest release_tag {mtag!r}")

    app_ref = str(manifest.get("app_image_ref") or "")
    goose_ref = str(manifest.get("goose_image_ref") or "")
    if not digest_pinned(app_ref):
        reasons.append("app_image_ref must be digest-pinned (contain @sha256:, not :latest)")
    if not digest_pinned(goose_ref):
        reasons.append("goose_image_ref must be digest-pinned (contain @sha256:, not :latest)")

    rb = manifest.get("rollback_available_before_deploy")
    rb_ok = rb is True or str(rb).lower() == "true"
    if not rb_ok:
        if not rb_explain.strip():
            reasons.append(
                "rollback_available_before_deploy is false: set ROLLBACK_UNAVAILABLE_EXPLANATION "
                "(e.g. first production deploy, expired manifest artifact)"
            )

verify = load_json(verify_path)
require_pass(verify, "static verify")

scale_requires_live = fleet != "pilot" if manifest is not None else False

if manifest is not None:
    if scale_requires_live:
        if not mon_path:
            reasons.append("MONITORING_RESULT_PATH is required when fleet_scale_target is not pilot")
        else:
            mp = Path(mon_path)
            if not mp.is_file():
                reasons.append(f"MONITORING_RESULT_PATH not a file: {mp}")
            else:
                mon_doc = load_json(mp)
                require_pass(mon_doc, "monitoring readiness")
        if not storm_path:
            reasons.append("STORM_RESULT_PATH is required when fleet_scale_target is not pilot")
        else:
            sp = Path(storm_path)
            if not sp.is_file():
                reasons.append(f"STORM_RESULT_PATH not a file: {sp}")
            else:
                storm_doc = load_json(sp)
                storm_pass(storm_doc, "telemetry storm")
                if (
                    storm_doc is not None
                    and fleet in ("scale-100", "scale-500", "scale-1000")
                    and storm_max_age_days is not None
                ):
                    try:
                        vmod = load_storm_validator()
                        me = vmod._min_machines_events(fleet)
                        if not me:
                            reasons.append(f"storm strict validation: unknown fleet {fleet!r}")
                        else:
                            min_m, min_e = me
                            errs = vmod.validate_payload(
                                storm_doc,
                                min_m=min_m,
                                min_e=min_e,
                                max_age_days=storm_max_age_days,
                            )
                            for e in errs:
                                reasons.append(f"storm evidence: {e}")
                    except Exception as ex:
                        reasons.append(f"storm strict validation failed: {ex}")
    else:
        if mon_path:
            mp = Path(mon_path)
            if not mp.is_file():
                reasons.append(f"MONITORING_RESULT_PATH not a file: {mp}")
            else:
                mon_doc = load_json(mp)
                require_pass(mon_doc, "monitoring readiness")
        if storm_path:
            sp = Path(storm_path)
            if not sp.is_file():
                reasons.append(f"STORM_RESULT_PATH not a file: {sp}")
            else:
                storm_doc = load_json(sp)
                storm_pass(storm_doc, "telemetry storm")

verdict = "pass" if not reasons else "fail"

now = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

def storm_gate_label() -> Optional[str]:
    if storm_doc is None:
        if fleet == "pilot" and not storm_path:
            return "not_required"
        return None
    if str(storm_doc.get("final_result", "")).strip().lower() == "pass":
        return "pass"
    if storm_doc.get("final_suite_pass") is True:
        return "pass"
    return None


def mon_gate_label() -> Optional[str]:
    if mon_doc is None:
        if fleet == "pilot" and not mon_path:
            return "not_required"
        return None
    return str(mon_doc.get("final_result", "")).strip().lower() or None

checksums: dict[str, str] = {}

readiness = {
    "pilot_ready": verdict == "pass" and fleet == "pilot",
    "scale_100_ready": verdict == "pass" and fleet == "scale-100",
    "scale_500_ready": verdict == "pass" and fleet == "scale-500",
    "scale_1000_ready": verdict == "pass" and fleet == "scale-1000",
}
readiness_statement = "fail"
if verdict == "pass":
    if fleet == "pilot":
        readiness_statement = "PILOT_READY (storm evidence not required; optional attachments must still be internally consistent)"
    elif fleet == "scale-100":
        readiness_statement = "SCALE_100_READY (100x100 storm thresholds and strict pass fields satisfied)"
    elif fleet == "scale-500":
        readiness_statement = "SCALE_500_READY (500x200 storm thresholds and strict pass fields satisfied)"
    elif fleet == "scale-1000":
        readiness_statement = "SCALE_1000_READY (1000x500 storm thresholds and strict pass fields satisfied)"
    else:
        readiness_statement = f"PASS_WITH_UNKNOWN_FLEET_{fleet}"

pack = {
    "schema_version": 1,
    "assembled_at_utc": now,
    "final_verdict": verdict,
    "failure_reasons": reasons,
    "release_readiness": readiness,
    "release_readiness_statement": readiness_statement,
    "storm_evidence_max_age_days_applied": storm_max_age_days,
    "release_tag": tag,
    "source_commit_sha": sha,
    "fleet_scale_target": fleet,
    "expected_fleet_scale_target": exp_scale or None,
    "image_refs": {"app_image_ref": app_ref, "goose_image_ref": goose_ref},
    "rollback_available_before_deploy": rb_ok,
    "rollback_unavailable_explanation": rb_explain or None,
    "inputs": {
        "verify_result_path": str(verify_path.resolve()),
        "monitoring_result_path": str(Path(mon_path).resolve()) if mon_path else None,
        "storm_result_path": str(Path(storm_path).resolve()) if storm_path else None,
        "deployment_manifest_path": str(man_path.resolve()),
        "known_risks_path": str(risks_src.resolve()),
    },
    "artifact_checksums_sha256": checksums,
    "gates": {
        "static_verify": (verify or {}).get("final_result") if verify else None,
        "monitoring_readiness": mon_gate_label(),
        "telemetry_storm": storm_gate_label(),
    },
}

if verdict == "pass":
    shutil.copy2(verify_path, out / "evidence-verify.json")
    shutil.copy2(man_path, out / "evidence-production-deployment-manifest.json")
    shutil.copy2(risks_src, out / "known-risks.md")
    checksums["evidence-verify.json"] = sha256_file(out / "evidence-verify.json")
    checksums["evidence-production-deployment-manifest.json"] = sha256_file(
        out / "evidence-production-deployment-manifest.json"
    )
    checksums["known-risks.md"] = sha256_file(out / "known-risks.md")
    if mon_path and Path(mon_path).is_file():
        shutil.copy2(mon_path, out / "evidence-monitoring-readiness.json")
        checksums["evidence-monitoring-readiness.json"] = sha256_file(
            out / "evidence-monitoring-readiness.json"
        )
    if storm_path and Path(storm_path).is_file():
        shutil.copy2(storm_path, out / "evidence-telemetry-storm.json")
        checksums["evidence-telemetry-storm.json"] = sha256_file(out / "evidence-telemetry-storm.json")
    pack["artifact_checksums_sha256"] = checksums

(out / "release-evidence-pack.json").write_text(json.dumps(pack, indent=2) + "\n", encoding="utf-8")

lines = [
    "# Enterprise release evidence summary",
    "",
    f"- **Assembled (UTC):** `{now}`",
    f"- **Final verdict:** **`{verdict}`**",
    f"- **Release readiness statement:** `{readiness_statement}`",
    f"- **Release tag:** `{tag}`",
    f"- **Source commit SHA:** `{sha}`",
    f"- **Fleet scale target:** `{fleet}`",
    f"- **App image:** `{app_ref}`",
    f"- **Goose image:** `{goose_ref}`",
    f"- **Rollback available before deploy:** `{rb_ok}`",
]
if not rb_ok and rb_explain:
    lines.append(f"- **Rollback note:** {rb_explain}")
lines.extend(
    [
        "",
        "## Gate results",
        "",
        f"- Static verify: `{pack['gates']['static_verify']}`",
        f"- Monitoring readiness: `{pack['gates']['monitoring_readiness']}`",
        f"- Telemetry storm: `{pack['gates']['telemetry_storm']}`",
        "",
    ]
)
if reasons:
    lines.append("## Validation failures")
    lines.append("")
    for r in reasons:
        lines.append(f"- {r}")
    lines.append("")

lines.extend(
    [
        "## Known risks",
        "",
        "See **`known-risks.md`** in this folder when the verdict is **pass** (copied from `KNOWN_RISKS_PATH`). "
        "On **fail**, fix validation errors and re-run the pack builder.",
        "",
        "## Operator sign-off",
        "",
        "- [ ] I confirm the evidence files in this folder match the production promotion under review.",
        "- [ ] I confirm digest-pinned images above are the intended immutable refs.",
        "- [ ] I have read **`known-risks.md`** and accept residual risk for this fleet tier.",
        "- [ ] For `fleet_scale_target` above **pilot**, staging storm evidence and live monitoring readiness were run against the intended environment.",
        "- [ ] Name: _________________________  Date: _________________________  Role: _________________________",
        "",
        "## Attaching this pack",
        "",
        "Zip **`OUTPUT_DIR`** and attach to the internal release record, GitHub Release assets, or change ticket. "
        "Keep **`release-evidence-pack.json`** and **`release-evidence-summary.md`** with the `evidence-*.json` files.",
        "",
    ]
)

(out / "release-evidence-summary.md").write_text("\n".join(lines), encoding="utf-8")

if verdict != "pass":
    print(json.dumps({"final_verdict": verdict, "failure_reasons": reasons}, indent=2))
    sys.exit(1)
print(json.dumps({"final_verdict": verdict, "output_dir": str(out.resolve())}, indent=2))
PY
