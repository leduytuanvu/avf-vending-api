#!/usr/bin/env bash
# AVF MQTT smoke — mosquitto_pub coverage for inventory topics.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXAMPLES="${SCRIPT_DIR}/AVF_MQTT_EXAMPLES.json"
MODE="${1:-run}"
MQTT_HOST="${MQTT_HOST:-localhost}"
MQTT_PORT="${MQTT_PORT:-1883}"
MQTT_USERNAME="${MQTT_USERNAME:-}"
MQTT_PASSWORD="${MQTT_PASSWORD:-}"
MQTT_TOPIC_PREFIX="${MQTT_TOPIC_PREFIX:-avf}"
MACHINE_ID="${MACHINE_ID:-test-machine}"

mask_pw() { echo "${MQTT_PASSWORD:+***}"; }

if [ "$MODE" = "list" ] || [ "$MODE" = "--list" ]; then
  export EXAMPLES
  PY=""
  for cand in python python3; do command -v "$cand" >/dev/null 2>&1 && PY="$cand" && break; done
  [ -n "$PY" ] || { echo "MISSING: python"; exit 1; }
  "$PY" - <<'PY'
import json, os, pathlib
for row in json.loads(pathlib.Path(os.environ["EXAMPLES"]).read_text(encoding="utf-8")):
    print(" ", row.get("direction", ""), row.get("topic", ""))
PY
  exit 0
fi

if [ "$MODE" = "dry-run" ] || [ "$MODE" = "--dry-run" ]; then
  echo "DRY-RUN MQTT host=${MQTT_HOST} port=${MQTT_PORT} user=${MQTT_USERNAME} pw=$(mask_pw)"
  export EXAMPLES MQTT_TOPIC_PREFIX MACHINE_ID
  PY=""
  for cand in python python3; do command -v "$cand" >/dev/null 2>&1 && PY="$cand" && break; done
  [ -n "$PY" ] || { echo "MISSING: python"; exit 1; }
  "$PY" - <<'PY'
import json, os, pathlib
prefix = os.environ.get("MQTT_TOPIC_PREFIX", "avf")
machine = os.environ.get("MACHINE_ID", "test-machine")
for row in json.loads(pathlib.Path(os.environ["EXAMPLES"]).read_text(encoding="utf-8")):
    topic = row["topic"].replace("{{MACHINE_ID}}", machine).replace("{{MQTT_TOPIC_PREFIX}}", prefix)
    print(f"mosquitto_pub -t {topic} -m ...")
PY
  exit 0
fi

need() { command -v "$1" >/dev/null 2>&1 || { echo "MISSING: $1"; exit 1; }; }
need mosquitto_pub
PY=""
for cand in python python3; do command -v "$cand" >/dev/null 2>&1 && PY="$cand" && break; done
[ -n "$PY" ] || { echo "MISSING: python"; exit 1; }
export EXAMPLES MQTT_HOST MQTT_PORT MQTT_USERNAME MQTT_PASSWORD MQTT_TOPIC_PREFIX MACHINE_ID
"$PY" - <<'PY'
import json, os, subprocess, pathlib, sys
examples = json.loads(pathlib.Path(os.environ["EXAMPLES"]).read_text(encoding="utf-8"))
host = os.environ["MQTT_HOST"]
port = os.environ["MQTT_PORT"]
user = os.environ.get("MQTT_USERNAME", "")
password = os.environ.get("MQTT_PASSWORD", "")
prefix = os.environ.get("MQTT_TOPIC_PREFIX", "avf")
machine = os.environ.get("MACHINE_ID", "test-machine")
pass_count = fail_count = 0
for row in examples:
    topic = row["topic"].replace("{{MACHINE_ID}}", machine).replace("{{MQTT_TOPIC_PREFIX}}", prefix)
    payload = json.dumps(row.get("payload") or {})
    cmd = ["mosquitto_pub", "-h", host, "-p", port, "-t", topic, "-m", payload, "-q", "1"]
    if user:
        cmd.extend(["-u", user])
    if password:
        cmd.extend(["-P", password])
    try:
        subprocess.run(cmd, check=True, capture_output=True, timeout=15)
        print(f"PASS {topic}")
        pass_count += 1
    except Exception:
        print(f"FAIL {topic}")
        fail_count += 1
print(f"SUMMARY pass={pass_count} fail={fail_count}")
sys.exit(1 if fail_count else 0)
PY
