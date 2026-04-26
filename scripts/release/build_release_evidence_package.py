#!/usr/bin/env python3
"""
Assemble a canonical release-evidence/ directory for enterprise audit (immutable, checksummed).

Does not fabricate missing files; manifest.json records which canonical pieces are present or absent.
Exit code 2 when --strict mode requires a file that is missing.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import shutil
import sys
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

SCHEMA_VERSION = "release-evidence-package-v1"

# Canonical output names -> search paths (relative to --root), first match wins
CANONICAL_MEMBERS: dict[str, list[str]] = {
    "image-metadata.json": [
        "build-evidence/image-build-metadata.json",
        "image-metadata.json",
    ],
    "promotion-manifest.json": [
        "build-evidence/promotion-manifest.json",
        "promotion-manifest.json",
    ],
    "cosign-signing-evidence.json": [
        "cosign-evidence/cosign-signing-evidence.json",
    ],
    "sbom-app.cdx.json": [
        "security-reports/sbom-app.cdx.json",
    ],
    "sbom-goose.cdx.json": [
        "security-reports/sbom-goose.cdx.json",
    ],
    "security-verdict.json": [
        "security-reports/security-verdict.json",
    ],
    "staging-deploy-evidence.json": [
        "staging-evidence/staging-deploy-evidence.json",
        "deployment-evidence/staging-deploy-evidence.json",
    ],
    "backup-evidence.json": [
        "backup-evidence/backup-evidence.json",
    ],
    "production-deploy-evidence.json": [
        "deployment-evidence/production-deploy-evidence.json",
    ],
}

# Phases: which members must exist when --strict
STRICT_MEMBERS: dict[str, frozenset[str]] = {
    "build": frozenset(
        {
            "image-metadata.json",
            "promotion-manifest.json",
            "cosign-signing-evidence.json",
            "sbom-app.cdx.json",
            "sbom-goose.cdx.json",
        }
    ),
    "security": frozenset({"security-verdict.json"}),
    "production": frozenset({"production-deploy-evidence.json"}),
    "staging": frozenset({"staging-deploy-evidence.json"}),
    "backup": frozenset({"backup-evidence.json"}),
    "release": frozenset(
        {
            "image-metadata.json",
            "promotion-manifest.json",
            "cosign-signing-evidence.json",
            "sbom-app.cdx.json",
            "sbom-goose.cdx.json",
            "security-verdict.json",
        }
    ),
}


@dataclass
class FileRecord:
    canonical_name: str
    status: str  # "present" | "absent"
    source_relpath: str | None = None
    sha256: str | None = None
    error: str | None = None


@dataclass
class PackageResult:
    out_dir: Path
    files: list[FileRecord] = field(default_factory=list)
    missing_required: list[str] = field(default_factory=list)


def _sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def _find_source(root: Path, canonical: str) -> Path | None:
    for rel in CANONICAL_MEMBERS.get(canonical, []):
        p = root / rel
        if p.is_file():
            return p
    return None


def _copy_to_package(
    root: Path,
    out_dir: Path,
) -> list[FileRecord]:
    """Copy canonical members from --root; retain merge-preplaced files in out_dir if no source in root."""
    records: list[FileRecord] = []
    for canonical in CANONICAL_MEMBERS:
        dest = out_dir / canonical
        src = _find_source(root, canonical)
        if src is None and dest.is_file():
            # left by --merge-from
            try:
                digest = _sha256_file(dest)
                records.append(
                    FileRecord(
                        canonical_name=canonical,
                        status="present",
                        source_relpath=f"(merged){canonical}",
                        sha256=digest,
                    )
                )
            except OSError as e:
                records.append(FileRecord(canonical_name=canonical, status="absent", error=str(e)))
            continue
        if src is None:
            records.append(FileRecord(canonical_name=canonical, status="absent", source_relpath=None, sha256=None))
            continue
        try:
            dest.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(src, dest)
            try:
                rel = str(src.resolve().relative_to(root.resolve()))
            except ValueError:
                rel = str(src)
            digest = _sha256_file(dest)
            records.append(
                FileRecord(
                    canonical_name=canonical,
                    status="present",
                    source_relpath=rel,
                    sha256=digest,
                )
            )
        except OSError as e:
            records.append(
                FileRecord(
                    canonical_name=canonical,
                    status="absent",
                    error=str(e),
                )
            )
    return records


def _write_checksums(out_dir: Path, records: list[FileRecord]) -> None:
    lines: list[str] = []
    for r in sorted(records, key=lambda x: x.canonical_name):
        if r.status == "present" and r.sha256:
            lines.append(f"{r.sha256}  {r.canonical_name}\n")
    (out_dir / "checksums.txt").write_text("".join(lines), encoding="utf-8")


def _build_manifest(
    *,
    phase: str,
    strict_phases: list[str],
    records: list[FileRecord],
    root: Path,
) -> dict[str, Any]:
    present = [r for r in records if r.status == "present"]
    absent = [r for r in records if r.status == "absent"]
    m: dict[str, Any] = {
        "schema_version": SCHEMA_VERSION,
        "generated_at_utc": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "phase_argument": phase,
        "strict_phases": strict_phases,
        "search_root": str(root.resolve()),
        "github_server_url": (os.environ.get("GITHUB_SERVER_URL") or "").strip() or None,
        "github_repository": (os.environ.get("GITHUB_REPOSITORY") or "").strip() or None,
        "github_run_id": (os.environ.get("GITHUB_RUN_ID") or "").strip() or None,
        "github_workflow": (os.environ.get("GITHUB_WORKFLOW") or "").strip() or None,
        "summary": {
            "files_present": len(present),
            "files_absent": len(absent),
            "canonical_names_total": len(CANONICAL_MEMBERS),
        },
        "files": {
            r.canonical_name: {
                "status": r.status,
                "source_relpath": r.source_relpath,
                "sha256": r.sha256,
                "error": r.error,
            }
            for r in records
        },
        "absent_if_expected_unavailable": [
            r.canonical_name for r in absent
        ],
        "disclaimer": (
            "Missing entries are not fabricated. Absent means no matching source file was found "
            "under the search root; strict mode fails only for phases that require them."
        ),
    }
    return m


def _check_strict(records: list[FileRecord], strict_phases: list[str]) -> list[str]:
    missing: list[str] = []
    for sp in strict_phases:
        want = STRICT_MEMBERS.get(sp)
        if not want:
            continue
        have = {r.canonical_name: r for r in records}
        for name in want:
            r = have.get(name)
            if r is None or r.status != "present":
                missing.append(f"{sp}:{name}")
    return missing


def run_package(args: argparse.Namespace) -> int:
    root = Path(args.root).resolve()
    out_dir = Path(args.out).resolve()
    out_dir.mkdir(parents=True, exist_ok=True)
    if not out_dir.is_dir():
        print(f"error: output {out_dir} is not a directory", file=sys.stderr)
        return 1

    if args.clear_output:
        for child in out_dir.iterdir():
            if child.is_file():
                child.unlink()
            elif child.is_dir():
                shutil.rmtree(child)

    # Optional pre-populated work dir: copy from merge dirs first (for operator merges)
    for mdir in args.merge_from:
        mp = Path(mdir)
        if not mp.is_dir():
            print(f"error: --merge-from not a directory: {mp}", file=sys.stderr)
            return 1
        for f in mp.iterdir():
            if f.is_file() and (
                f.name in CANONICAL_MEMBERS or f.name in ("manifest.json", "checksums.txt")
            ):
                if f.name in ("manifest.json", "checksums.txt") and not args.include_previous_manifest:
                    continue
                dest = out_dir / f.name
                shutil.copy2(f, dest)

    records = _copy_to_package(root, out_dir)

    strict: list[str] = list(args.strict_phase)

    missing_required = _check_strict(records, strict) if strict else []
    manifest = _build_manifest(phase=args.phase, strict_phases=strict, records=records, root=root)
    manifest["missing_required_for_strict"] = missing_required

    (out_dir / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    # checksums: include manifest at end
    _write_checksums(out_dir, records)
    m_digest = _sha256_file(out_dir / "manifest.json")
    with (out_dir / "checksums.txt").open("a", encoding="utf-8") as c:
        c.write(f"{m_digest}  manifest.json\n")

    if missing_required:
        print("error: strict mode missing required file(s):", ", ".join(missing_required), file=sys.stderr)
        return 2
    if args.print_manifest_path:
        print(out_dir / "manifest.json")
    return 0


def main() -> int:
    p = argparse.ArgumentParser(
        description="Build release-evidence/ audit package: copy canonical JSONs, write manifest.json and checksums.txt."
    )
    p.add_argument("--out", default="release-evidence", help="Output directory (default: release-evidence)")
    p.add_argument("--root", default=".", help="Search root for source files (default: .)")
    p.add_argument(
        "--phase",
        default="collect",
        help=(
            "Label for manifest and default strict set: build | security | production | staging | backup | "
            "release | collect (default: collect = no required files unless --strict-phase / phase match)"
        ),
    )
    p.add_argument(
        "--strict-phase",
        action="append",
        default=[],
        dest="strict_phase",
        metavar="PHASE",
        help="Require all files for this phase (repeatable). E.g. --strict-phase build --strict-phase security",
    )
    p.add_argument(
        "--merge-from",
        action="append",
        default=[],
        help="Optional directory to copy canonical files from before reading --root (merge)",
    )
    p.add_argument(
        "--include-previous-manifest",
        action="store_true",
        help="When using --merge-from, also copy manifest.json / checksums (usually false)",
    )
    p.add_argument(
        "--clear-output",
        action="store_true",
        help="Remove files in --out before writing (not removing --out itself)",
    )
    p.add_argument("--print-manifest-path", action="store_true", help="Print manifest path to stdout on success")
    args = p.parse_args()
    if not args.strict_phase and args.phase in STRICT_MEMBERS and args.phase != "collect":
        args.strict_phase = [args.phase]
    return run_package(args)


if __name__ == "__main__":
    raise SystemExit(main())
