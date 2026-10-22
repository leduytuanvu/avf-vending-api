#!/usr/bin/env python3
import socket
import subprocess


def redact(line: str) -> str:
    if "postgres://" in line:
        return line.split("postgres://", 1)[0][-80:] + "[redacted-url]"
    return line[:240]


out = subprocess.check_output(
    ["docker", "logs", "--tail", "50", "avf-vending-prod-app-a-api-1"],
    text=True,
    stderr=subprocess.STDOUT,
)
print("logs_api")
for line in out.splitlines():
    low = line.lower()
    if any(x in low for x in ["error", "fatal", "ready", "listen", "connect", "sql", "database", "health"]):
        print(redact(line))

print("--- probes ---")
for host, port in [("127.0.0.1", 6432), ("172.17.0.1", 6432), ("72.62.244.94", 6432)]:
    cmd = (
        "python3 -c "
        f"\"import socket; s=socket.create_connection(('{host}',{port}),2); print('ok'); s.close()\""
    )
    r = subprocess.run(
        ["docker", "exec", "avf-vending-prod-app-a-api-1", "sh", "-c", cmd],
        capture_output=True,
        text=True,
    )
    detail = (r.stdout or r.stderr).strip().splitlines()[-1:] or [""]
    print("from_api", f"{host}:{port}", "ok" if r.returncode == 0 else "fail", detail[0][:120])
