#!/usr/bin/env bash
# Print redacted DB identity and validate APP_ENV vs DATABASE_URL (migration / ops guard).
# Never echoes full DATABASE_URL or password. Uses python3 (stdlib) for URL encoding rules.
set -Eeuo pipefail

fail() {
	echo "verify_database_environment: $*" >&2
	exit 2
}

: "${APP_ENV:=}"
db_url="${DATABASE_URL:-}"
if [[ -z "${db_url}" ]]; then
	fail "DATABASE_URL is empty or unset"
fi

export DATABASE_URL="${db_url}"
export PAYMENT_ENV="${PAYMENT_ENV:-}"
export STAGING_ALLOW_LOCAL_DATABASE="${STAGING_ALLOW_LOCAL_DATABASE:-}"
export PRODUCTION_DATABASE_URL="${PRODUCTION_DATABASE_URL:-}"
export PRODUCTION_DATABASE_HOST="${PRODUCTION_DATABASE_HOST:-}"
export STAGING_DATABASE_URL="${STAGING_DATABASE_URL:-}"
export STAGING_DATABASE_HOST="${STAGING_DATABASE_HOST:-}"
export APP_ENV="${APP_ENV}"

# shellcheck disable=SC2016
python3 <<'PY'
import os, sys
from urllib.parse import urlparse, unquote

raw = os.environ.get("DATABASE_URL", "").strip()
if not raw:
    print("error:empty DATABASE_URL", file=sys.stderr)
    sys.exit(2)

u = urlparse(raw)
if not u.scheme or not u.hostname:
    print("error:unparseable DATABASE_URL", file=sys.stderr)
    sys.exit(2)

up = u.username or ""
pw = u.password or ""
user = unquote(up) if up else ""
password = unquote(pw) if pw else ""
port = u.port
if port is None and (u.scheme.startswith("postgres") or u.scheme == "postgresql"):
    port = 5432
host = (u.hostname or "").lower()
path = (u.path or "").lstrip("/")
dbn = path.split("?", 1)[0] or ""
q = u.query or ""
sslm = "default"
if q:
    for pair in q.split("&"):
        if pair.startswith("sslmode="):
            sslm = pair.split("=", 1)[1]
            break

app = (os.environ.get("APP_ENV") or "").strip()
prod_ref = (os.environ.get("PRODUCTION_DATABASE_HOST") or "").strip().lower()
if not prod_ref and os.environ.get("PRODUCTION_DATABASE_URL"):
    try:
        prod_ref = (urlparse(os.environ["PRODUCTION_DATABASE_URL"]).hostname or "").lower()
    except Exception:
        prod_ref = ""
stg_ref = (os.environ.get("STAGING_DATABASE_HOST") or "").strip().lower()
if not stg_ref and os.environ.get("STAGING_DATABASE_URL"):
    try:
        stg_ref = (urlparse(os.environ["STAGING_DATABASE_URL"]).hostname or "").lower()
    except Exception:
        stg_ref = ""
pay = (os.environ.get("PAYMENT_ENV") or "").strip().lower()
allow_stg = os.environ.get("STAGING_ALLOW_LOCAL_DATABASE", "").lower() in ("1", "true", "yes")

is_local = host in ("localhost", "127.0.0.1", "::1")

if app in ("staging", "production"):
    p = (os.environ.get("PRODUCTION_DATABASE_URL") or "").strip()
    s = (os.environ.get("STAGING_DATABASE_URL") or "").strip()
    if app == "staging" and p and p == raw:
        print("error:staging DATABASE_URL must not equal PRODUCTION_DATABASE_URL", file=sys.stderr)
        sys.exit(2)
    if app == "production" and s and s == raw:
        print("error:production DATABASE_URL must not equal STAGING_DATABASE_URL", file=sys.stderr)
        sys.exit(2)

if app == "staging":
    if is_local and not allow_stg:
        print("error:staging DATABASE_URL must not be localhost (set STAGING_ALLOW_LOCAL_DATABASE=true to override)", file=sys.stderr)
        sys.exit(2)
    if prod_ref and host == prod_ref:
        print("error:staging host matches production ref", file=sys.stderr)
        sys.exit(2)
    if pay != "sandbox":
        if not pay:
            print("error:staging requires PAYMENT_ENV=sandbox (set explicitly)", file=sys.stderr)
        else:
            print("error:staging requires PAYMENT_ENV=sandbox", file=sys.stderr)
        sys.exit(2)
if app == "production":
    if is_local:
        print("error:production must not use localhost/loopback database", file=sys.stderr)
        sys.exit(2)
    if stg_ref and host == stg_ref:
        print("error:production host matches staging ref", file=sys.stderr)
        sys.exit(2)
    if pay and pay != "live":
        print("error:production requires PAYMENT_ENV=live", file=sys.stderr)
        sys.exit(2)

print(f"scheme={u.scheme}")
print(f"host={host}")
print(f"port={port}")
print(f"database={dbn}")
print(f"username={user}")
if password:
    print(f"password=<redacted len={len(password)}>")
else:
    print("password=<empty>")
print(f"sslmode={sslm}")
print(f"app_env={app}")
PY

echo "verify_database_environment: OK"
