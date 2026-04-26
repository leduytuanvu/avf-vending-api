#!/usr/bin/env python3
"""
Offline goose migration safety checks (no database).

Detects layout issues and destructive SQL patterns in -- +goose Up sections
(for deploy gates, only Up is considered; CI scans Up and Down for reporting).
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

FNAME_RE = re.compile(r"^(\d{5})_(.+)\.sql$")


@dataclass
class Finding:
    file: str
    goose_section: str
    rule: str
    detail: str
    line_hint: int | None = None

    def as_dict(self) -> dict[str, Any]:
        d: dict[str, Any] = {"file": self.file, "goose_section": self.goose_section, "rule": self.rule, "detail": self.detail}
        if self.line_hint is not None:
            d["line_hint"] = self.line_hint
        return d


@dataclass
class Report:
    migrations_dir: str
    deploy_target: str
    files_checked: list[str] = field(default_factory=list)
    findings: list[Finding] = field(default_factory=list)
    destructive_findings: list[Finding] = field(default_factory=list)
    exit_code: int = 0
    blocked: bool = False
    allow_destructive: bool = False

    def add(self, f: Finding, *, destructive: bool) -> None:
        self.findings.append(f)
        if destructive:
            self.destructive_findings.append(f)

    def to_json(self) -> dict[str, Any]:
        return {
            "migrations_dir": self.migrations_dir,
            "deploy_target": self.deploy_target,
            "allow_destructive": self.allow_destructive,
            "blocked": self.blocked,
            "exit_code": self.exit_code,
            "files_checked": self.files_checked,
            "findings": [x.as_dict() for x in self.findings],
            "destructive_findings": [x.as_dict() for x in self.destructive_findings],
        }


def strip_sql_comments(sql: str) -> str:
    """Remove -- line and /* block */ comments (best-effort; ignores string edge cases)."""
    out = re.sub(r"/\*.*?\*/", "", sql, flags=re.DOTALL)
    lines: list[str] = []
    for line in out.splitlines():
        if "--" in line:
            line = line.split("--", 1)[0]
        lines.append(line)
    return "\n".join(lines)


def split_goose_sections(text: str) -> dict[str, str]:
    """Return {'Up': str, 'Down': str} for goose migration files."""
    lines = text.splitlines()
    sections: dict[str, list[str]] = {"Up": [], "Down": []}
    current: str | None = None
    for line in lines:
        up_m = re.match(r"^--\s*\+goose\s+Up\s*$", line.strip(), re.I)
        down_m = re.match(r"^--\s*\+goose\s+Down\s*$", line.strip(), re.I)
        if up_m:
            current = "Up"
            continue
        if down_m:
            current = "Down"
            continue
        if current:
            sections[current].append(line)
    return {"Up": "\n".join(sections["Up"]).strip(), "Down": "\n".join(sections["Down"]).strip()}


def validate_layout(mig_dir: Path, report: Report) -> None:
    if not mig_dir.is_dir():
        report.add(
            Finding(str(mig_dir), "layout", "missing_dir", "migrations directory does not exist"),
            destructive=False,
        )
        report.exit_code = 1
        return

    files = sorted(mig_dir.glob("*.sql"))
    if not files:
        report.add(Finding("", "layout", "no_sql", "no .sql files under migrations/"), destructive=False)
        report.exit_code = 1
        return

    prev_num = -1
    seen_versions: dict[int, str] = {}
    for f in files:
        base = f.name
        m = FNAME_RE.match(base)
        if not m:
            report.add(
                Finding(base, "layout", "bad_filename", "expected NNNNN_description.sql (5-digit prefix)"),
                destructive=False,
            )
            report.exit_code = 1
            continue
        ver = int(m.group(1))
        if ver in seen_versions:
            report.add(
                Finding(base, "layout", "duplicate_version", f"version {ver:05d} also used by {seen_versions[ver]}"),
                destructive=False,
            )
            report.exit_code = 1
        else:
            seen_versions[ver] = base
        if prev_num >= 0 and ver <= prev_num:
            report.add(
                Finding(base, "layout", "version_order", f"versions must be strictly increasing (after {prev_num:05d})"),
                destructive=False,
            )
            report.exit_code = 1
        prev_num = ver

        text = f.read_text(encoding="utf-8", errors="replace")
        if not text.strip():
            report.add(Finding(base, "layout", "empty_file", "migration file is empty"), destructive=False)
            report.exit_code = 1
            continue

        if not re.search(r"(?m)^--\s*\+goose\s+Up\s*$", text):
            report.add(Finding(base, "layout", "missing_goose_up", "missing '-- +goose Up' directive"), destructive=False)
            report.exit_code = 1
        if not re.search(r"(?m)^--\s*\+goose\s+Down\s*$", text):
            report.add(Finding(base, "layout", "missing_goose_down", "missing '-- +goose Down' directive"), destructive=False)
            report.exit_code = 1

        sec = split_goose_sections(text)
        if not sec["Up"].strip():
            report.add(Finding(base, "layout", "empty_goose_up", "goose Up section has no SQL body"), destructive=False)
            report.exit_code = 1


def _drop_index_unsafe(sql: str) -> bool:
    """True if a DROP INDEX appears without PostgreSQL IF EXISTS (index replacement uses DROP INDEX IF EXISTS)."""
    for m in re.finditer(r"(?is)\bDROP\s+INDEX\b", sql):
        rest = sql[m.end() : m.end() + 200].lstrip()
        upper = rest.upper()
        if upper.startswith("CONCURRENTLY"):
            rest = rest[len("CONCURRENTLY") :].lstrip()
            upper = rest.upper()
        if upper.startswith("IF EXISTS"):
            continue
        return True
    return False


def scan_destructive(path: str, section_name: str, sql_raw: str, report: Report) -> None:
    sql = strip_sql_comments(sql_raw)
    if not sql.strip():
        return
    norm = re.sub(r"\s+", " ", sql)

    checks: list[tuple[re.Pattern[str], str]] = [
        (re.compile(r"\bDROP\s+TABLE\b", re.I), "DROP TABLE"),
        (re.compile(r"\bDROP\s+COLUMN\b", re.I), "DROP COLUMN"),
        (re.compile(r"\bTRUNCATE\b", re.I), "TRUNCATE"),
        (re.compile(r"\bALTER\s+TYPE\b[\s\S]{0,800}?\bDROP\b", re.I), "ALTER TYPE ... DROP"),
        (re.compile(r"\bALTER\s+TABLE\b[\s\S]{0,1200}?\bALTER\s+COLUMN\b[\s\S]{0,400}?\bTYPE\b", re.I), "ALTER COLUMN TYPE"),
    ]
    for rx, label in checks:
        if rx.search(sql) or rx.search(norm):
            report.add(Finding(path, section_name, label, f"matched pattern: {label}"), destructive=True)

    if _drop_index_unsafe(sql):
        report.add(
            Finding(path, section_name, "DROP INDEX", "DROP INDEX without IF EXISTS (unsafe for online index replacement)"),
            destructive=True,
        )

    # DELETE / UPDATE without WHERE (statement split — best effort)
    cleaned = strip_sql_comments(sql_raw)
    for stmt in re.split(r";+", cleaned):
        s = stmt.strip()
        if not s or s.startswith("--"):
            continue
        if re.match(r"(?is)^\s*DELETE\s+FROM\b", s) and not re.search(r"(?is)\bWHERE\b", s):
            report.add(
                Finding(path, section_name, "DELETE without WHERE", "DELETE FROM without WHERE clause"),
                destructive=True,
            )
        if re.match(r"(?is)^\s*UPDATE\b", s) and not re.search(r"(?is)\bWHERE\b", s):
            report.add(
                Finding(path, section_name, "UPDATE without WHERE", "UPDATE without WHERE clause"),
                destructive=True,
            )


def scan_files(mig_dir: Path, report: Report) -> None:
    """Scan only goose Up bodies for destructive patterns (Down is expected to reverse Up)."""
    if not mig_dir.is_dir():
        return
    for f in sorted(mig_dir.glob("*.sql")):
        report.files_checked.append(f.name)
        text = f.read_text(encoding="utf-8", errors="replace")
        if not text.strip():
            continue
        sec = split_goose_sections(text)
        scan_destructive(f.name, "Up", sec["Up"], report)


def apply_policy(report: Report) -> None:
    if not report.destructive_findings:
        report.blocked = False
        return

    target = report.deploy_target
    allow = report.allow_destructive

    if target == "ci":
        report.blocked = True
        report.exit_code = 1
        return

    if target == "staging":
        if not allow:
            report.blocked = True
            report.exit_code = 1
        return

    if target == "production":
        if not allow:
            report.blocked = True
            report.exit_code = 1
        return

    report.blocked = True
    report.exit_code = 1


def write_report(report: Report, path: Path | None) -> None:
    if path is None:
        return
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(report.to_json(), indent=2) + "\n", encoding="utf-8")


def main() -> int:
    ap = argparse.ArgumentParser(description="Verify goose migration safety (offline).")
    ap.add_argument("--root", type=Path, default=Path.cwd(), help="Repository root")
    ap.add_argument(
        "--migrations-dir",
        type=Path,
        default=None,
        help="Migrations directory (default: <root>/migrations)",
    )
    ap.add_argument(
        "--deploy-target",
        choices=("ci", "staging", "production"),
        default=os.environ.get("DEPLOY_TARGET", "ci"),
        help="Policy for destructive findings (default: ci, or DEPLOY_TARGET env)",
    )
    ap.add_argument("--report", type=Path, default=None, help="Write JSON report to this path")
    args = ap.parse_args()

    root = args.root.resolve()
    mig_dir = args.migrations_dir or (root / "migrations")
    if args.migrations_dir is not None:
        mig_dir = Path(args.migrations_dir).resolve()

    deploy_target: str = args.deploy_target
    allow = False
    if deploy_target == "staging":
        allow = os.environ.get("ALLOW_DESTRUCTIVE_MIGRATIONS", "").lower() in ("1", "true", "yes")
    elif deploy_target == "production":
        allow = os.environ.get("ALLOW_PROD_DESTRUCTIVE_MIGRATIONS", "").lower() in ("1", "true", "yes")

    try:
        mig_rel = str(mig_dir.relative_to(root))
    except ValueError:
        mig_rel = str(mig_dir)

    report = Report(
        migrations_dir=mig_rel,
        deploy_target=deploy_target,
        allow_destructive=allow,
    )

    validate_layout(mig_dir, report)
    if mig_dir.is_dir():
        scan_files(mig_dir, report)

    if report.exit_code == 0:
        apply_policy(report)
    else:
        report.blocked = True

    out_path = args.report
    if out_path is None:
        out_path = root / "migration-evidence" / "migration-safety-report.json"
    write_report(report, out_path)

    summary_lines = [
        f"verify_migrations: target={deploy_target} allow_destructive={allow}",
        f"  files_checked={len(report.files_checked)} findings={len(report.findings)} destructive={len(report.destructive_findings)}",
        f"  blocked={report.blocked} report={out_path}",
    ]
    print("\n".join(summary_lines), file=sys.stderr)

    if report.destructive_findings:
        print("verify_migrations: destructive pattern hits:", file=sys.stderr)
        for f in report.destructive_findings:
            print(f"  - {f.file} [{f.goose_section}] {f.rule}: {f.detail}", file=sys.stderr)

    if report.findings and not report.destructive_findings:
        for f in report.findings:
            print(f"verify_migrations: {f.file} [{f.goose_section}] {f.rule}: {f.detail}", file=sys.stderr)

    if report.exit_code != 0:
        print(
            "verify_migrations: FAILED. Fix migrations or set explicit approval env vars "
            "(see docs/runbooks/migration-safety.md).",
            file=sys.stderr,
        )
    return report.exit_code


if __name__ == "__main__":
    raise SystemExit(main())
