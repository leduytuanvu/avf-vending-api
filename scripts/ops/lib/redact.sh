#!/usr/bin/env bash
# Redact secrets from URLs and connection strings for ops evidence output.
set -Eeuo pipefail

ops_redact_url() {
	local raw="${1-}"
	if [[ -z "${raw}" ]]; then
		printf ''
		return 0
	fi
	if command -v python3 >/dev/null 2>&1; then
		python3 - "$raw" <<'PY'
import sys
from urllib.parse import urlsplit, urlunsplit

raw = sys.argv[1].strip()
if not raw:
    sys.exit(0)
u = urlsplit(raw)
user = u.username or ""
host = u.hostname or ""
port = f":{u.port}" if u.port else ""
db = (u.path or "").lstrip("/")
netloc = host + port
if user:
    netloc = f"{user}@{netloc}"
out = urlunsplit((u.scheme, netloc, f"/{db}" if db else "", "", ""))
print(out)
PY
		return 0
	fi
	# Fallback: strip password between :// and @
	printf '%s' "${raw}" | sed -E 's#(://)[^/@]+@#\1***@#g'
}
