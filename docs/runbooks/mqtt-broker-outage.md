# MQTT broker outage

The device **realtime plane** depends on a reachable MQTT broker for ingress telemetry and egress commands.

## Symptoms

- Devices cannot publish/subscribe; command publishes fail from API (`internal/platform/mqtt`).
- **`cmd/mqtt-ingest`** disconnect logs / reconnect storms.
- Operators see delayed telemetry and stalled command ACK paths — correlate with [`mqtt-command-stuck.md`](mqtt-command-stuck.md).

## Mitigations

1. Restore broker HA endpoints / TLS certs before scaling writers.
2. Expect **replay/backlog** once connectivity returns — devices retain offline queues per [`../api/mqtt-contract.md`](../api/mqtt-contract.md).
3. Validate **`mqtt-ingest`** lag and Postgres ingest health before declaring green — [`mqtt-ingest-telemetry-limits.md`](mqtt-ingest-telemetry-limits.md).

## Related

- Production rollout shape: [`production-2-vps.md`](production-2-vps.md).
- NATS JetStream (telemetry buffering) is orthogonal—still required when configured: [`telemetry-jetstream-resilience.md`](telemetry-jetstream-resilience.md).
