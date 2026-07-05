#!/usr/bin/env python3
"""Generate Postman v3 YAML assets under postman/v3/ from canonical JSON v2.1 sources."""
from __future__ import annotations

import argparse
import json
import os
import shutil
import stat
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[1]
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from tools.postman_v3_environment import export_environment_yaml, validate_no_secret_patterns  # noqa: E402
from tools.postman_v3_script_fixup import fix_all_json_collections, fix_all_v3_yaml  # noqa: E402
from tools.postman_v3_parity import (  # noqa: E402
    compare_collection_pair,
    compare_environment_pair,
    count_v3_requests,
    load_json,
    walk_json_requests,
)

V3_ROOT = ROOT / "postman" / "v3"
MIGRATE_TMP = ROOT / ".tmp-postman-migrate"

COLLECTION_MAP: list[tuple[Path, Path]] = [
    (ROOT / "postman/collections/avf-vending-api.postman_collection.json", V3_ROOT / "collections/avf-vending-api"),
    (
        ROOT / "postman/collections/avf-vending-api-function-path.postman_collection.json",
        V3_ROOT / "collections/avf-vending-api-function-path",
    ),
    (ROOT / "postman/production/avf-production-e2e.postman_collection.json", V3_ROOT / "collections/avf-production-e2e"),
    (
        ROOT / "postman/suites/production-full/avf-vending-production.full.postman_collection.json",
        V3_ROOT / "suites/production-full/avf-vending-production-full",
    ),
]

ENVIRONMENT_MAP: list[tuple[Path, Path]] = [
    (ROOT / "postman/environments/avf-local.postman_environment.json", V3_ROOT / "environments/avf-local.environment.yaml"),
    (ROOT / "postman/environments/avf-staging.postman_environment.json", V3_ROOT / "environments/avf-staging.environment.yaml"),
    (ROOT / "postman/environments/avf-production.postman_environment.json", V3_ROOT / "environments/avf-production.environment.yaml"),
    (
        ROOT / "postman/suites/production-full/avf-vending-production.full.postman_environment.json",
        V3_ROOT / "environments/avf-production-full.environment.yaml",
    ),
    (
        ROOT / "postman/production/avf-production-e2e.postman_environment.json",
        V3_ROOT / "environments/avf-production-e2e.environment.yaml",
    ),
]


def git_commit() -> str:
    try:
        out = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=ROOT, text=True, stderr=subprocess.DEVNULL)
        return out.strip()
    except (subprocess.CalledProcessError, FileNotFoundError):
        return "unknown"


PRODUCTION_FULL_JSON = ROOT / "postman/suites/production-full/avf-vending-production.full.postman_collection.json"


def _compare_kwargs(json_path: Path) -> dict[str, bool]:
    if json_path.resolve() == PRODUCTION_FULL_JSON.resolve():
        return {"exclude_doc_only": True}
    return {"exclude_doc_only": False}


def find_postman_cli() -> str | None:
    for name in ("postman", "postman.cmd"):
        p = shutil.which(name)
        if p:
            return p
    return None


def _rm_tree(path: Path) -> None:
    if not path.exists():
        return

    if sys.platform == "win32":
        ext = "\\\\?\\" + str(path.resolve())
        subprocess.run(["cmd", "/c", "rmdir", "/s", "/q", ext], check=False)
        if not path.exists():
            return

    def _onerror(func, p, _exc_info):  # type: ignore[no-untyped-def]
        try:
            os.chmod(p, stat.S_IWRITE)
            func(p)
        except OSError:
            pass

    shutil.rmtree(path, onerror=_onerror)


def migrate_collection(cli: str, json_path: Path, out_dir: Path) -> None:
    if not json_path.is_file():
        raise FileNotFoundError(f"missing JSON collection: {json_path}")

    MIGRATE_TMP.mkdir(parents=True, exist_ok=True)
    env = os.environ.copy()
    env["TEMP"] = str(MIGRATE_TMP.resolve())
    env["TMP"] = env["TEMP"]

    if out_dir.exists():
        _rm_tree(out_dir)
    out_dir.parent.mkdir(parents=True, exist_ok=True)

    rel_json = json_path.relative_to(ROOT) if json_path.is_relative_to(ROOT) else json_path
    rel_out = out_dir.relative_to(ROOT) if out_dir.is_relative_to(ROOT) else out_dir
    cmd = [cli, "collection", "migrate", str(rel_json), "-o", str(rel_out)]
    proc = subprocess.run(cmd, cwd=ROOT, env=env, capture_output=True, text=True)
    if proc.returncode != 0:
        raise RuntimeError(
            f"postman collection migrate failed for {json_path}:\n{proc.stdout}\n{proc.stderr}"
        )


def run_postman_lint(cli: str, v3_dir: Path) -> str:
    if not v3_dir.is_dir():
        return "skipped-missing-dir"
    env = os.environ.copy()
    env["TEMP"] = str(MIGRATE_TMP.resolve())
    env["TMP"] = env["TEMP"]
    proc = subprocess.run(
        [cli, "collection", "lint", str(v3_dir), "--fail-severity", "error"],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
    )
    return "pass" if proc.returncode == 0 else f"fail: {proc.stderr.strip() or proc.stdout.strip()}"


def regen_json_sources(skip: bool) -> None:
    if skip:
        return
    subprocess.check_call([sys.executable, str(ROOT / "tools/build_postman_collection.py")], cwd=ROOT)
    subprocess.check_call(
        [sys.executable, str(ROOT / "scripts/postman/generate_production_full_suite.py")],
        cwd=ROOT,
    )
    subprocess.check_call(
        [sys.executable, str(ROOT / "postman/production/generate_postman_from_manifest.py")],
        cwd=ROOT,
    )


def sanitize_stale_paths_in_v3() -> None:
    replacements = {
        "postman/suites/full-production-suite/": "postman/suites/production-full/",
        "postman/production-full-suite/": "postman/suites/production-full/",
    }
    for ypath in V3_ROOT.rglob("*.yaml"):
        try:
            text = ypath.read_text(encoding="utf-8")
        except OSError:
            continue
        new = text
        for old, new_path in replacements.items():
            new = new.replace(old, new_path)
        if new != text:
            ypath.write_text(new, encoding="utf-8", newline="\n")


def copy_testing_guide() -> None:
    src = ROOT / "postman/suites/production-full/TESTING_GUIDE.md"
    dst = V3_ROOT / "suites/production-full/TESTING_GUIDE.md"
    if src.is_file():
        dst.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(src, dst)


def write_readme() -> None:
    readme = V3_ROOT / "README.md"
    readme.parent.mkdir(parents=True, exist_ok=True)
    readme.write_text(
        "# Postman v3 YAML (Local Mode / Native Git)\n\n"
        "These assets are **generated** from the canonical JSON v2.1 Postman files. Do not edit by hand.\n\n"
        "## Layout\n\n"
        "| Path | Purpose |\n"
        "|------|--------|\n"
        "| `collections/` | Primary, function-path, and production E2E collections (v3 folder format) |\n"
        "| `environments/` | Local, staging, production, production-full, production-e2e environments |\n"
        "| `suites/production-full/` | Full OpenAPI + gRPC/MQTT documentation suite |\n"
        "| `manifest.json` | Generation metadata and parity counts |\n\n"
        "## Regenerate\n\n"
        "```bash\n"
        "make postman-generate-v3    # v3 only (from current JSON)\n"
        "make postman-generate       # JSON + production suites + v3\n"
        "```\n\n"
        "Requires **Postman CLI** on PATH (`npm install -g postman-cli`). On Windows, set `TEMP`/`TMP` to a directory on the same drive as the repo if migrate fails with `EXDEV`.\n\n"
        "## Validate\n\n"
        "```bash\n"
        "make postman-check-v3\n"
        "python tools/check_postman_v3_artifacts.py\n"
        "python scripts/postman/validate_v3_yaml.py\n"
        "```\n\n"
        "Optional local lint (not required in CI):\n\n"
        "```bash\n"
        "postman collection lint postman/v3/collections/avf-vending-api --fail-severity warning\n"
        "```\n\n"
        "## Newman / CI\n\n"
        "CI and Newman continue to use **JSON** under `postman/collections/` and `postman/environments/`.\n"
        "Newman does not run v3 YAML collections; use Postman CLI `postman collection run <v3-folder>` instead.\n\n"
        "Import guide: [`docs/postman/POSTMAN_V3_LOCAL_MODE_IMPORT_GUIDE.md`](../../docs/postman/POSTMAN_V3_LOCAL_MODE_IMPORT_GUIDE.md)\n",
        encoding="utf-8",
    )


def build_manifest(
    *,
    lint_results: dict[str, str],
    parity_errors: list[str],
) -> dict[str, Any]:
    json_requests = 0
    yaml_requests = 0
    json_variables = 0
    yaml_variables = 0

    source_json_collections: list[str] = []
    v3_collections: list[str] = []
    for jpath, vpath in COLLECTION_MAP:
        if jpath.is_file():
            source_json_collections.append(str(jpath.relative_to(ROOT)).replace("\\", "/"))
            coll = load_json(jpath)
            json_requests += len(walk_json_requests(coll, exclude_doc_only=_compare_kwargs(jpath).get("exclude_doc_only", False)))
        if vpath.is_dir():
            v3_collections.append(str(vpath.relative_to(ROOT)).replace("\\", "/"))
            yaml_requests += count_v3_requests(vpath, exclude_doc_only=_compare_kwargs(jpath).get("exclude_doc_only", False))

    source_json_environments: list[str] = []
    v3_environments: list[str] = []
    for jpath, ypath in ENVIRONMENT_MAP:
        if jpath.is_file():
            source_json_environments.append(str(jpath.relative_to(ROOT)).replace("\\", "/"))
            json_variables += len(load_json(jpath).get("values") or [])
        if ypath.is_file():
            v3_environments.append(str(ypath.relative_to(ROOT)).replace("\\", "/"))
            data = json.loads(json.dumps({}))  # placeholder
            try:
                import yaml

                data = yaml.safe_load(ypath.read_text(encoding="utf-8")) or {}
            except Exception:
                pass
            yaml_variables += len(data.get("values") or [])

    return {
        "schemaVersion": "postman-v3-yaml-generation-v1",
        "generatedAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "sourceCommit": git_commit(),
        "sourceJsonCollections": source_json_collections,
        "sourceJsonEnvironments": source_json_environments,
        "v3Collections": v3_collections,
        "v3Environments": v3_environments,
        "counts": {
            "jsonRequests": json_requests,
            "yamlRequests": yaml_requests,
            "jsonVariables": json_variables,
            "yamlVariables": yaml_variables,
        },
        "validation": {
            "parityOk": len(parity_errors) == 0,
            "parityErrors": parity_errors,
            "postmanCliLint": lint_results,
        },
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Generate Postman v3 YAML under postman/v3/")
    parser.add_argument("--regen-json-skip", action="store_true", help="Do not regenerate JSON sources first")
    parser.add_argument(
        "--skip-cli-migrate",
        action="store_true",
        help="Skip collection migrate (dev only; requires existing v3 collection dirs)",
    )
    parser.add_argument("--skip-lint", action="store_true", help="Skip optional postman collection lint")
    args = parser.parse_args()

    regen_json_sources(args.regen_json_skip)
    fix_all_json_collections()

    cli = find_postman_cli()
    if not cli and not args.skip_cli_migrate:
        print(
            "ERROR: Postman CLI not found. Install: npm install -g postman-cli\n"
            "       Or set TEMP/TMP on same drive as repo and ensure `postman` is on PATH.",
            file=sys.stderr,
        )
        return 1

    V3_ROOT.mkdir(parents=True, exist_ok=True)
    write_readme()

    if cli and not args.skip_cli_migrate:
        for json_path, out_dir in COLLECTION_MAP:
            print(f"migrate {json_path.name} -> {out_dir.relative_to(ROOT)}")
            migrate_collection(cli, json_path, out_dir)
    elif args.skip_cli_migrate:
        missing = [str(p) for _, p in COLLECTION_MAP if not p.is_dir()]
        if missing:
            print("ERROR: --skip-cli-migrate but missing v3 dirs:\n" + "\n".join(missing), file=sys.stderr)
            return 1
        print("WARN: skipped postman collection migrate (--skip-cli-migrate)")

    for json_path, yaml_path in ENVIRONMENT_MAP:
        if not json_path.is_file():
            print(f"WARN: skip env export; missing {json_path}", file=sys.stderr)
            continue
        print(f"export env {json_path.name} -> {yaml_path.relative_to(ROOT)}")
        export_environment_yaml(json_path, yaml_path)
        validate_no_secret_patterns(yaml_path.read_text(encoding="utf-8"), str(yaml_path))

    copy_testing_guide()
    sanitize_stale_paths_in_v3()
    fix_all_v3_yaml(V3_ROOT)

    parity_errors: list[str] = []
    for json_path, v3_dir in COLLECTION_MAP:
        parity_errors.extend(compare_collection_pair(json_path, v3_dir, **_compare_kwargs(json_path)))
    for json_path, yaml_path in ENVIRONMENT_MAP:
        parity_errors.extend(compare_environment_pair(json_path, yaml_path))

    lint_results: dict[str, str] = {}
    if cli and not args.skip_lint:
        for _, v3_dir in COLLECTION_MAP:
            if v3_dir.is_dir():
                rel = str(v3_dir.relative_to(ROOT)).replace("\\", "/")
                lint_results[rel] = run_postman_lint(cli, v3_dir)
    else:
        lint_results["note"] = "skipped"

    manifest = build_manifest(lint_results=lint_results, parity_errors=parity_errors)
    manifest_path = V3_ROOT / "manifest.json"
    manifest_path.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    print(f"Wrote {manifest_path}")

    if parity_errors:
        for err in parity_errors:
            print(f"ERROR: {err}", file=sys.stderr)
        return 1

    failed_lint = [k for k, v in lint_results.items() if v.startswith("fail")]
    if failed_lint:
        for k in failed_lint:
            print(f"WARN: lint {k}: {lint_results[k]}", file=sys.stderr)

    print(
        "OK: v3 generation "
        f"requests={manifest['counts']['yamlRequests']} "
        f"envVars={manifest['counts']['yamlVariables']}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
