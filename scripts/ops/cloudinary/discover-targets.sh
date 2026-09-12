#!/usr/bin/env bash
# Discover Cloudinary targets from env files (no secrets printed).
# Usage:
#   bash discover-targets.sh --production-env PATH [--staging-env PATH] [--local-env PATH]
set -Eeuo pipefail

# shellcheck source=lib_cloudinary.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib_cloudinary.sh"

PROD_ENV=""
STG_ENV=""
LOCAL_ENV=""

while [[ $# -gt 0 ]]; do
	case "$1" in
	--production-env)
		PROD_ENV="$2"
		shift 2
		;;
	--staging-env)
		STG_ENV="$2"
		shift 2
		;;
	--local-env)
		LOCAL_ENV="$2"
		shift 2
		;;
	-h | --help)
		echo "usage: discover-targets.sh --production-env PATH [--staging-env PATH] [--local-env PATH]"
		exit 0
		;;
	*)
		cloudinary_fail "unknown argument: $1"
		;;
	esac
done

cloudinary_ensure_evidence_dir
ts="$(date -u +%Y%m%dT%H%M%SZ 2>/dev/null || python3 -c 'from datetime import datetime,timezone;print(datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ"))')"
out="${CLOUDINARY_EVIDENCE_DIR}/cloudinary-targets-${ts}.json"

export PROD_ENV_FILE="${PROD_ENV}"
export STAGING_ENV_FILE="${STG_ENV}"
cloudinary_py "${CLOUDINARY_OPS_DIR}/cloudinary_ops.py" discover-targets >"${out}.tmp"

# Append local env scan
cloudinary_py - "${LOCAL_ENV}" "${out}.tmp" "${out}" <<'PY'
import json, sys
from pathlib import Path

local = sys.argv[1]
tmp = Path(sys.argv[2])
out = Path(sys.argv[3])
data = json.loads(tmp.read_text(encoding="utf-8"))
local_cloud = None
if local and Path(local).is_file():
    for line in Path(local).read_text(encoding="utf-8", errors="replace").splitlines():
        line = line.strip()
        if line.startswith("CLOUDINARY_CLOUD_NAME="):
            v = line.split("=", 1)[1].strip().strip('"').strip("'")
            if v and "your-cloudinary" not in v:
                local_cloud = v
            break
data["targets"].append({
    "environment": "development",
    "configured": bool(local_cloud),
    "cloud_name": local_cloud,
    "env_file": local or None,
})
clouds = sorted({t["cloud_name"] for t in data["targets"] if t.get("cloud_name")})
data["unique_cloud_names"] = clouds
data["deduplicated_wipe_targets"] = [
    {"cloud_name": c, "environments": [t["environment"] for t in data["targets"] if t.get("cloud_name") == c]}
    for c in clouds
]
out.write_text(json.dumps(data, indent=2), encoding="utf-8")
print(json.dumps(data, indent=2))
PY

rm -f "${out}.tmp"
cloudinary_note "wrote ${out}"
