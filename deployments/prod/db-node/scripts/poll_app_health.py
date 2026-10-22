#!/usr/bin/env python3
import subprocess
import sys
import time

prefix = sys.argv[1] if len(sys.argv) > 1 else "a"
names = [
    f"avf-vending-prod-app-{prefix}-api-1",
    f"avf-vending-prod-app-{prefix}-worker-1",
    f"avf-vending-prod-app-{prefix}-mqtt-ingest-1",
    f"avf-vending-prod-app-{prefix}-reconciler-1",
    f"avf-vending-prod-app-{prefix}-caddy-1",
]


def status(name: str) -> str:
    return subprocess.check_output(
        [
            "docker",
            "inspect",
            "-f",
            "{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}",
            name,
        ],
        text=True,
    ).strip()


def redact(line: str) -> str:
    if "postgres://" in line:
        return "[redacted-url]"
    return line[:240]


ok = False
for _ in range(24):
    rows = [f"{n.split('-')[-2]}={status(n)}" for n in names]
    print(" ".join(rows), flush=True)
    if rows[0].endswith("healthy") and all("unhealthy" not in r for r in rows[:4]):
        ok = True
        break
    time.sleep(5)

if not ok:
    logs = subprocess.check_output(
        ["docker", "logs", "--tail", "30", names[0]],
        text=True,
        stderr=subprocess.STDOUT,
    )
    print("api_logs")
    for line in logs.splitlines():
        print(redact(line))
    raise SystemExit(1)

curl = subprocess.check_output(
    [
        "docker",
        "exec",
        names[0],
        "sh",
        "-c",
        "curl -fsS http://127.0.0.1:8080/health/live; echo; curl -fsS http://127.0.0.1:8080/health/ready; echo",
    ],
    text=True,
)
print(curl)
print("HEALTH_OK", prefix)
