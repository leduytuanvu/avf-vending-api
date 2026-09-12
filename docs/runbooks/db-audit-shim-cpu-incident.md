# DB audit python3 shim CPU incident

## Tracked toolchain (use reviewed scripts only)

After merge of the DB audit/wipe upstream PRs, use **only** the tracked scripts under `scripts/ops/` at the commit recorded in `scripts/ops/TOOLCHAIN_SHA` on the host.

- Read-only audit: `docs/runbooks/database-audit.md`
- Destructive wipe: `docs/runbooks/database-wipe.md` (dry-run default; production requires explicit gates)

**Never** run ad-hoc copies or scripts that prepend `.db-destroy-evidence/.bin` to `PATH` without `scripts/ops/lib/python3_shim.sh`. Historical local versions can recreate the recursive `python3` shim incident. Preserve local evidence; do not delete `.db-destroy-evidence/` artifacts during triage.

## Symptoms

- Sustained host CPU saturation on an app node (load average well above vCPU count)
- Multiple `bash …/.db-destroy-evidence/.bin/python3` processes in `R` state
- Parent chain includes `verify_database_environment.sh` from a prior audit/wipe session
- AVF containers may remain healthy while the host is saturated

## Identification

```bash
uptime
cat /proc/loadavg
ps -eo pid,ppid,user,%cpu,%mem,etime,stat,cmd --sort=-%cpu | head -30
pgrep -af 'db-destroy-evidence|verify_database_environment\.sh' || echo none
```

For each candidate PID, verify `/proc/<pid>/cmdline` and parent chain. Do not trust historical PIDs from earlier reports.

## Do not

- `pkill python3`
- `killall python3`
- Reboot the host to “clear CPU” without preserving evidence
- Delete `.db-destroy-evidence/` artifacts during triage

## Safe termination (operator approval required)

The repository ships a gated script with **dry-run default**:

```bash
cd /opt/avf-vending-api
bash scripts/ops/terminate-verified-db-audit-orphans.sh
bash scripts/ops/terminate-verified-db-audit-orphans.sh --execute
```

Review every `VERIFY` / `SKIP` line before `--execute`.

## Post-mitigation verification

```bash
uptime; cat /proc/loadavg
ps -eo pid,ppid,user,%cpu,%mem,etime,cmd --sort=-%cpu | head -30
docker stats --no-stream
pgrep -af 'db-destroy-evidence|verify_database_environment' || echo none
```

If CPU remains high after verified orphans are gone, stop and re-open RCA.

## Permanent fix

Future audit/wipe tooling must install a `python3` shim via `scripts/ops/lib/python3_shim.sh`:

- Resolve the real interpreter **before** prepending `.db-destroy-evidence/.bin` to `PATH`
- Shim executes an **absolute** `sys.executable` path (never bare `exec python3`)
- `verify_database_environment.sh` is bounded by a 120s timeout when `timeout` is available

## Evidence preservation

Keep `.db-destroy-evidence/` JSONL/logs for post-incident review. Do not commit secrets from that directory.
