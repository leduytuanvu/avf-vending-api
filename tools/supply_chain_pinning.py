#!/usr/bin/env python3
"""
Offline supply-chain pinning: GitHub Actions SHAs, production Docker digests, pinned Go install targets,
Trivy scanner image, and high-risk curl|bash patterns.

Exceptions live in scripts/ci/supply-chain-allowlist.txt (expiring, with owner + reason per row).
"""
from __future__ import annotations

import fnmatch
import re
import sys
from dataclasses import dataclass
from datetime import date, datetime, timezone
from pathlib import Path
from typing import Iterable

ROOT = Path(__file__).resolve().parents[1]
WF = ROOT / ".github" / "workflows"
ALLOWLIST = ROOT / "scripts" / "ci" / "supply-chain-allowlist.txt"
FULL_SHA = re.compile(r"^[0-9a-fA-F]{40}$")
DIGEST = re.compile(r"@sha256:[0-9a-fA-F]{64}\b")

# Trivy: semver tag (exact) or digest-pinned ref (optional migration path).
TRIVY_PINNED_VERSION = "0.57.1"
# Shell: trivy_image="aquasec/trivy:0.57.1"  JSON: "trivy_image": "aquasec/trivy:0.57.1"
TRIVY_SNIPPET_RE = re.compile(
    r'trivy_image\s*=\s*["\']([^"\']+)["\']|"trivy_image"\s*:\s*"(?P<json>[^"]+)"',
    re.IGNORECASE,
)

CURL_BASH = re.compile(
    r"curl[^{\n]*\|[^\n#]*\b(sudo\s+)?(ba)?sh\b|"
    r"wget[^{\n]*\|[^\n#]*\b(sudo\s+)?(ba)?sh\b|"
    r"\b(ba)?sh\b\s*<\(curl|"
    r"curl[^{\n;]*\|\s*sudo\s+tee[^\n#]*\|\s*ba?sh"
)


@dataclass(frozen=True)
class AllowRow:
    expiry: date
    owner: str
    path_glob: str
    check_kind: str
    match_sub: str
    reason: str


def _parse_allowlist() -> list[AllowRow]:
    if not ALLOWLIST.is_file():
        return []
    rows: list[AllowRow] = []
    for n, line in enumerate(ALLOWLIST.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
        s = line.strip()
        if not s or s.startswith("#"):
            continue
        parts = [p.strip() for p in s.split("|")]
        if len(parts) < 6:
            print(f"ERROR: {ALLOWLIST.relative_to(ROOT)}:{n}: need 6 fields (expiry|owner|path|kind|match|reason)", file=sys.stderr)
            raise SystemExit(1)
        exp_s, owner, g, kind, msub, *rest = parts
        reason = "|".join(rest) if rest else ""
        if not exp_s or not owner or not g or not kind:
            print(f"ERROR: {ALLOWLIST.relative_to(ROOT)}:{n}: empty required field", file=sys.stderr)
            raise SystemExit(1)
        try:
            exp = datetime.strptime(exp_s, "%Y-%m-%d").replace(tzinfo=timezone.utc).date()
        except ValueError:
            print(f"ERROR: {ALLOWLIST.relative_to(ROOT)}:{n}: bad expiry (use YYYY-MM-DD)", file=sys.stderr)
            raise SystemExit(1)
        rows.append(AllowRow(exp, owner, g, kind, msub, reason or "(no reason)"))
    return rows


def _today_utc() -> date:
    return datetime.now(tz=timezone.utc).date()


def _rel(p: Path) -> str:
    return p.relative_to(ROOT).as_posix()


def is_allowed(
    rows: list[AllowRow],
    relpath: str,
    check_kind: str,
    violation_text: str,
) -> bool:
    today = _today_utc()
    for r in rows:
        if r.check_kind not in (check_kind, "generic", "*"):
            continue
        if r.expiry < today:
            continue
        if not fnmatch.fnmatch(relpath, r.path_glob):
            continue
        if r.match_sub and r.match_sub not in violation_text:
            if r.match_sub not in (".*",):
                continue
        return True
    return False


def assert_external_workflow_uses_pinned(allow: list[AllowRow]) -> None:
    bad: list[str] = []
    for path in sorted(WF.glob("*.yml")):
        relp = _rel(path)
        for n, line in enumerate(path.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
            s = line
            if "#" in s and not s.lstrip().startswith("#"):
                s = s.split("#", 1)[0].rstrip()
            t = s.strip()
            if not t.startswith("uses:"):
                continue
            m = re.match(r"^uses:\s*(\S+)", t)
            if not m:
                continue
            spec = m.group(1)
            if spec.startswith("./"):
                continue
            if spec.lower().startswith("docker://"):
                continue
            if "@" not in spec:
                msg = f"{relp}:{n}: uses {spec!r} has no @ref (expected owner/repo@<sha> # vN)"
                if not is_allowed(allow, relp, "uses_ref", msg):
                    bad.append(msg)
                continue
            ref = spec.rsplit("@", 1)[-1]
            if not FULL_SHA.match(ref):
                msg = (
                    f"{relp}:{n}: external action ref {ref!r} is not a full 40-char commit SHA "
                    f"(pin: uses: owner/repo@<sha> # vN) — {spec!r}"
                )
                if not is_allowed(allow, relp, "uses_ref", msg):
                    bad.append(msg)
    if bad:
        print(
            "ERROR: external GitHub Actions must be pinned to full commit SHAs (not tags/branches). "
            "Example: uses: actions/checkout@<40-hex-sha> # v4",
            file=sys.stderr,
        )
        for b in bad:
            print(f"  {b}", file=sys.stderr)
        print(f"  Allowlist: {ALLOWLIST.relative_to(ROOT)}", file=sys.stderr)
        raise SystemExit(1)


def assert_production_docker_supply_chain_pinned(allow: list[AllowRow]) -> None:
    image_line = re.compile(r"^\s*image:\s*(.+?)\s*(?:#.*)?$")
    prod = ROOT / "deployments" / "prod"
    if not prod.is_dir():
        return

    bad: list[str] = []

    for path in sorted(prod.rglob("Dockerfile*")):
        if not path.is_file():
            continue
        relp = _rel(path)
        for n, line in enumerate(path.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
            s0 = line.split("#", 1)[0].strip()
            if not s0.upper().startswith("FROM "):
                continue
            if s0.rstrip().upper() == "FROM SCRATCH" or s0.upper().startswith("FROM SCRATCH "):
                continue
            if not DIGEST.search(s0):
                msg = f"{relp}:{n}: FROM must pin public base to @sha256:<64-hex> (e.g. image:tag@sha256:... # tag)"
                if not is_allowed(allow, relp, "dockerfile_from", msg):
                    bad.append(msg)

    for path in sorted(prod.rglob("docker-compose*.yml")):
        relp = _rel(path)
        for n, line in enumerate(path.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
            m = image_line.match(line)
            if not m:
                continue
            val = m.group(1).strip().strip("'\"")
            if not val or val.startswith("*"):
                continue
            if "${" in val:
                continue
            if not DIGEST.search(val):
                msg = f"{relp}:{n}: public service `image: {val}` must include @sha256:<64-hex>"
                if not is_allowed(allow, relp, "dockerfile_from", msg):
                    bad.append(msg)

    if bad:
        print("ERROR: production Dockerfiles / compose use digest-pinned public images only.", file=sys.stderr)
        for b in bad:
            print(f"  {b}", file=sys.stderr)
        raise SystemExit(1)


def _go_install_lines_in(paths: Iterable[Path]) -> list[tuple[Path, int, str]]:
    out: list[tuple[Path, int, str]] = []
    for base in paths:
        if not base.is_dir():
            continue
        for path in base.rglob("*"):
            if not path.is_file():
                continue
            if path.suffix not in (".yml", ".yaml", ".sh", ".md"):
                continue
            rel = _rel(path)
            if "vendor" in rel or ".git" in rel:
                continue
            for n, line in enumerate(path.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
                if re.search(r"\bgo install\b", line) and not line.lstrip().startswith("#"):
                    out.append((path, n, line))
    return out


def assert_go_install_pinned(allow: list[AllowRow]) -> None:
    watch = (ROOT / ".github" / "workflows", ROOT / "scripts" / "ci", ROOT / "tools")
    bad: list[str] = []
    for path, n, line in _go_install_lines_in(watch):
        relp = _rel(path)
        code = line.split("#", 1)[0]
        if "@latest" in code:
            msg = f"{relp}:{n}: go install must not use @latest"
            if not is_allowed(allow, relp, "go_install", msg):
                bad.append(msg)
            continue
        m = re.search(r"\bgo install\s+(\S+)", code)
        if not m:
            continue
        arg = m.group(1)
        if "@" in code:
            continue
        if arg.startswith(".") or arg.rstrip("\\").endswith("/..."):
            continue
        if re.match(r"^[./]", arg):
            continue
        msg = f"{relp}:{n}: go install of remote module {arg!r} must use an explicit @version (not @latest)"
        if not is_allowed(allow, relp, "go_install", msg):
            bad.append(msg)
    if bad:
        print("ERROR: go install in CI and tooling must pin versions (e.g. @v1.2.3), never @latest.", file=sys.stderr)
        for b in bad:
            print(f"  {b}", file=sys.stderr)
        raise SystemExit(1)


def assert_pinned_go_tool_table(allow: list[AllowRow]) -> None:
    """Enforce the canonical go install @version for workflow-installed tools (must match this file + docs)."""

    def check_file(path: Path, pairs: list[tuple[re.Pattern[str], str, str]]) -> None:
        if not path.is_file():
            return
        text = path.read_text(encoding="utf-8", errors="replace")
        relp = _rel(path)
        for rgx, tool, want in pairs:
            m = rgx.search(text)
            if not m:
                print(f"ERROR: {relp}: missing `go install` line for {tool} @ {want}", file=sys.stderr)
                raise SystemExit(1)
            got = m.group(1).rstrip("\\")
            if got != want:
                msg = f"{relp}: go install {tool} expected {want!r}, got {got!r}"
                if not is_allowed(allow, relp, "go_install", msg):
                    print(f"ERROR: {msg}", file=sys.stderr)
                    raise SystemExit(1)

    check_file(
        WF / "ci.yml",
        [
            (re.compile(r"go install github\.com/rhysd/actionlint/cmd/actionlint@(\S+)", re.I), "actionlint", "v1.7.12"),
            (re.compile(r"go install mvdan\.cc/sh/v3/cmd/shfmt@(\S+)", re.I), "shfmt", "v3.13.1"),
        ],
    )
    for sec in ("security.yml", "nightly-security.yml"):
        check_file(
            WF / sec,
            [
                (re.compile(r"go install golang\.org/x/vuln/cmd/govulncheck@(\S+)", re.I), "govulncheck", "v1.3.0"),
                (re.compile(r"go install github\.com/zricethezav/gitleaks/v8@(\S+)", re.I), "gitleaks", "v8.24.2"),
            ],
        )


def _scan_paths() -> list[Path]:
    out: list[Path] = []
    for p in (WF, ROOT / "scripts" / "ci", ROOT / "tools"):
        if p.is_dir():
            out.append(p)
    return out


def assert_no_curl_pipe_shell(allow: list[AllowRow]) -> None:
    bad: list[str] = []
    for base in _scan_paths():
        for path in base.rglob("*"):
            if not path.is_file():
                continue
            if path.suffix not in (".yml", ".yaml", ".sh", ".md"):
                continue
            relp = _rel(path)
            if "vendor" in relp:
                continue
            for n, line in enumerate(path.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
                if line.lstrip().startswith("#"):
                    continue
                if not CURL_BASH.search(line):
                    continue
                msg = f"{relp}:{n}: reject curl|bash (or similar) — use pinned packages or vendored scripts ({line.strip()[:160]})"
                if not is_allowed(allow, relp, "curl_bash", msg):
                    bad.append(msg)
    if bad:
        print("ERROR: curl|bash / wget|bash one-liner installers are not allowed (add a dated allowlist entry if vetted).", file=sys.stderr)
        for b in bad:
            print(f"  {b}", file=sys.stderr)
        raise SystemExit(1)


def _trivy_ref_ok(image: str) -> bool:
    s = image.strip()
    if s.startswith("aquasec/trivy@sha256:") and len(s.split(":", 2)[-1]) == 64:
        return True
    if s == f"aquasec/trivy:{TRIVY_PINNED_VERSION}":
        return True
    if f":{TRIVY_PINNED_VERSION}@sha256:" in s and s.startswith("aquasec/trivy:"):
        return True
    return False


def assert_trivy_scanner_pinned(allow: list[AllowRow]) -> None:
    bad: list[str] = []
    for path in sorted(WF.glob("*.yml")):
        relp = _rel(path)
        text = path.read_text(encoding="utf-8", errors="replace")
        for m in TRIVY_SNIPPET_RE.finditer(text):
            img = m.group(1) or m.group("json")
            if not img:
                continue
            if _trivy_ref_ok(img):
                continue
            msg = f"{relp}: Trivy image {img!r} must be aquasec/trivy:{TRIVY_PINNED_VERSION} or digest @sha256:... (or allowlisted)"
            if not is_allowed(allow, relp, "trivy_image", msg):
                bad.append(msg)
    if bad:
        print("ERROR: Trivy must use the pinned version or an explicit digest; bump TRIVY_PINNED_VERSION in tools/supply_chain_pinning.py when upgrading.", file=sys.stderr)
        for b in bad:
            print(f"  {b}", file=sys.stderr)
        raise SystemExit(1)


def assert_supply_chain_pinned() -> None:
    allow = _parse_allowlist()
    assert_external_workflow_uses_pinned(allow)
    assert_production_docker_supply_chain_pinned(allow)
    assert_go_install_pinned(allow)
    assert_pinned_go_tool_table(allow)
    assert_trivy_scanner_pinned(allow)
    assert_no_curl_pipe_shell(allow)


def main() -> None:
    assert_supply_chain_pinned()
    print("OK: supply chain pinning (Actions SHA, prod Docker, Go tools, Trivy, no curl|bash in CI paths)")


if __name__ == "__main__":
    main()
