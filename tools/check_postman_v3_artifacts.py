#!/usr/bin/env python3
"""Validate postman/v3 YAML artifacts (offline parity + safety)."""
from __future__ import annotations

import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
V3_ROOT = ROOT / "postman" / "v3"

if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

from tools.postman_v3_parity import (  # noqa: E402
    compare_collection_pair,
    compare_environment_pair,
    load_yaml,
)
from tools.postman_v3_environment import FORBIDDEN_ENV_KEY_PARTS  # noqa: E402

try:
    import yaml
except ImportError:
    yaml = None  # type: ignore[assignment]

PRODUCTION_FULL_JSON = ROOT / "postman/suites/production-full/avf-vending-production.full.postman_collection.json"


def collection_compare_kwargs(json_path: Path) -> dict[str, bool]:
    if json_path.resolve() == PRODUCTION_FULL_JSON.resolve():
        return {"exclude_doc_only": True}
    return {"exclude_doc_only": False}


COLLECTION_MAP = [
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

ENVIRONMENT_MAP = [
    (ROOT / "postman/environments/avf-local.postman_environment.json", V3_ROOT / "environments/avf-local.environment.yaml"),
    (ROOT / "postman/environments/avf-staging.postman_environment.json", V3_ROOT / "environments/avf-staging.environment.yaml"),
    (ROOT / "postman/environments/avf-production.postman_environment.json", V3_ROOT / "environments/avf-production.environment.yaml"),
    (
        ROOT / "postman/suites/production-full/avf-vending-production.full.postman_environment.json",
        V3_ROOT / "environments/avf-production-full.environment.yaml",
    ),
    (ROOT / "postman/production/avf-production-e2e.postman_environment.json", V3_ROOT / "environments/avf-production-e2e.environment.yaml"),
]

STALE_PATH_FRAGMENTS = (
    "postman/suites/full-production-suite/",
    "postman/production-full-suite/",
)

SECRET_KEY_HINTS = ("password", "secret", "webhook", "api_key")
SECRET_ALLOW_BLANK = frozenset(
    {
        "adminemail",
        "token_type",
        "tokentype",
        "auth_type",
        "authtype",
        "grpcusereflection",
        "swagger_enabled",
    }
)


def die(msg: str) -> None:
    print(f"ERROR: {msg}", file=sys.stderr)
    raise SystemExit(1)


def safe_yaml_files() -> list[Path]:
    out: list[Path] = []
    for pattern in ("*.request.yaml", "*.environment.yaml", "definition.yaml"):
        for p in V3_ROOT.rglob(pattern):
            try:
                if p.is_file():
                    out.append(p)
            except OSError:
                continue
    return out


def main() -> None:
    if yaml is None:
        die("PyYAML required (pip install pyyaml)")

    manifest_path = V3_ROOT / "manifest.json"
    if not manifest_path.is_file():
        die(f"missing {manifest_path} — run make postman-generate-v3")

    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    if manifest.get("schemaVersion") != "postman-v3-yaml-generation-v1":
        die("manifest schemaVersion mismatch")

    errors: list[str] = []
    for jpath, vpath in COLLECTION_MAP:
        errors.extend(compare_collection_pair(jpath, vpath, **collection_compare_kwargs(jpath)))
    for jpath, ypath in ENVIRONMENT_MAP:
        errors.extend(compare_environment_pair(jpath, ypath))

    for ypath in safe_yaml_files():
        try:
            load_yaml(ypath)
        except Exception as e:
            errors.append(f"invalid YAML {ypath.relative_to(ROOT)}: {e}")

    for ypath in (V3_ROOT / "environments").glob("*.environment.yaml"):
        data = load_yaml(ypath)
        for v in data.get("values") or []:
            key = str(v.get("key") or "")
            kl = key.lower().replace("-", "_")
            for bad in FORBIDDEN_ENV_KEY_PARTS:
                if bad in kl:
                    errors.append(f"forbidden env key {key!r} in {ypath.name}")
            val = str(v.get("value") or "")
            if val and any(h in kl for h in SECRET_KEY_HINTS) and kl not in SECRET_ALLOW_BLANK:
                if "token" in kl and kl.endswith("type"):
                    continue
                errors.append(f"secret-like env var {key!r} must be blank in {ypath.name}")

    banned = ("DATABASE_URL=", "Bearer eyJ", "JWT_SECRET=", "WEBHOOK_SECRET=")
    for ypath in safe_yaml_files():
        try:
            text = ypath.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue
        for b in banned:
            if b in text:
                errors.append(f"forbidden pattern {b!r} in {ypath.relative_to(ROOT)}")

    for frag in STALE_PATH_FRAGMENTS:
        for ypath in safe_yaml_files():
            try:
                text = ypath.read_text(encoding="utf-8", errors="replace")
            except OSError:
                continue
            if frag in text:
                errors.append(f"stale path reference {frag!r} in {ypath.relative_to(ROOT)}")

    readme = V3_ROOT / "README.md"
    if not readme.is_file():
        errors.append("missing postman/v3/README.md")

    if errors:
        for e in errors:
            print(f"ERROR: {e}", file=sys.stderr)
        die(f"{len(errors)} v3 artifact check failure(s)")

    counts = manifest.get("counts") or {}
    print(
        "OK: Postman v3 artifact checks "
        f"(requests={counts.get('yamlRequests')}, envVars={counts.get('yamlVariables')})",
        flush=True,
    )


if __name__ == "__main__":
    main()
