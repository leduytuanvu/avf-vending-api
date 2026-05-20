# MQTT Verification Report

Generated: 2026-05-20

## Topic contract

Source: `internal/platform/mqtt/topics.go`, `docs/api/mqtt-contract.md`

### Inbound subscribe patterns

| Layout | Count | Example |
|--------|-------|---------|
| Legacy | 12 | `{prefix}/+/telemetry` |
| Enterprise | 13 | `{prefix}/machines/+/telemetry/snapshot` |

### Key relative tails

`presence`, `state/heartbeat`, `telemetry`, `telemetry/snapshot`, `telemetry/incident`, `events/vend`, `events/cash`, `events/inventory`, `commands/dispatch`, `commands/ack`, `commands/receipt`, `shadow/desired`, `shadow/reported`

### Outbound (API publish)

- Legacy: `{prefix}/{machineId}/commands/dispatch`
- Enterprise: `{prefix}/machines/{machineId}/commands`

## Automated unit tests

```bash
go test -count=1 ./internal/platform/mqtt/...
```

**Result: PASS**

## Broker integration

Local broker (Docker profile `broker`):

```bash
docker compose -f deployments/docker/docker-compose.yml --profile broker up -d
# EMQX on localhost:1883, dashboard :18083
```

Live integration (optional):

```bash
VERIFY_WITH_BROKER=1 MQTT_HOST=127.0.0.1 MQTT_TOPIC_PREFIX=avf/local \
  bash scripts/local/verify-full-system.sh
```

**Status this pass: SKIPPED** — broker containers running (`avf-emqx`) but full coverage script not invoked with env vars.

## Manual mosquitto probes

```bash
# Subscribe to command dispatch (replace prefix + machineId)
mosquitto_sub -h 127.0.0.1 -p 1883 -t 'avf/local/+/commands/dispatch' -v

# Publish telemetry sample
mosquitto_pub -h 127.0.0.1 -p 1883 \
  -t 'avf/local/test-machine-001/telemetry' \
  -m '{"schema_version":1,"machine_id":"test-machine-001","ts":"2026-05-20T00:00:00Z"}'
```

Requires `cmd/mqtt-ingest` running with `DATABASE_URL`, `MQTT_BROKER_URL`, `NATS_URL`.

## Coverage script

`bash scripts/test/run-mqtt-full-coverage.sh` — documents flows:

| Flow | Classification |
|------|----------------|
| connect | safe-read |
| telemetry_publish | local-write |
| command_subscribe | safe-read |
| command_dispatch | canary-write |
| ack_success / ack_duplicate | canary-write |
| ack_timeout_no_ack | **hardware-required** |
| invalid_payload | local-write |
| acl_topic_isolation | production-readonly |

## External / manual-only

| Scenario | Reason |
|----------|--------|
| Real machine ACK timing | Physical vend hardware |
| Production ACL isolation | Production broker credentials |
| Reconnect storm | Long-running soak test |

## Verdict

**MQTT: PASS (unit + contract)** — topic naming verified in code; broker live E2E optional and documented.
