# Temporal workflow operations

Temporal is optional in local development and only participates when `TEMPORAL_ENABLED=true`. The API, worker, and reconciler keep their existing non-Temporal paths when the workflow boundary is disabled.

## Registered workflows

- `workflow.payment_to_vend`: ensures a paid order's vend is started through the existing commerce service, waits for the configured vend result window, then either no-ops on success, queues refund review for captured payment plus failed vend, or escalates manual review on timeout.
- `workflow.refund`: sends an idempotent provider refund request only when a real refund provider activity is wired. If no provider is configured, or the provider errors, it queues refund review instead of faking success.
- `workflow.command_ack`: waits for the command ACK window, checks the latest command attempt status when an ACK reader is wired, and escalates manual review for timeout/failure states.
- `workflow.payment_pending_timeout_follow_up`, `workflow.vend_failure_after_payment_success`, `workflow.refund_orchestration`, and `workflow.manual_review_escalation`: existing compensation and review workflows used by worker/reconciler scheduling flags.

## Local startup

PowerShell:

```powershell
$env:TEMPORAL_ENABLED = "true"
$env:TEMPORAL_HOST_PORT = "127.0.0.1:7233"
$env:TEMPORAL_NAMESPACE = "default"
$env:TEMPORAL_TASK_QUEUE = "avf-workflows"
go run ./cmd/temporal-worker
```

Git Bash:

```bash
TEMPORAL_ENABLED=true \
TEMPORAL_HOST_PORT=127.0.0.1:7233 \
TEMPORAL_NAMESPACE=default \
TEMPORAL_TASK_QUEUE=avf-workflows \
go run ./cmd/temporal-worker
```

The Temporal worker also needs `DATABASE_URL` and `NATS_URL` because its activities read Postgres state and enqueue refund/manual review tickets through the existing NATS refund review sink.

## Incident checks

1. Confirm the worker is alive with `/health/live` and ready with `/health/ready` on `TEMPORAL_WORKER_METRICS_LISTEN` or `127.0.0.1:9094`.
2. Confirm the task queue name in logs matches `TEMPORAL_TASK_QUEUE`.
3. If workflows are not starting, check the scheduling flags on the process that owns the source event, for example `TEMPORAL_SCHEDULE_PAYMENT_PENDING_TIMEOUT`, `TEMPORAL_SCHEDULE_VEND_FAILURE_FOLLOW_UP`, `TEMPORAL_SCHEDULE_REFUND_ORCHESTRATION`, and `TEMPORAL_SCHEDULE_MANUAL_REVIEW_ESCALATION`.
4. If refund or manual review activities fail, verify `NATS_URL` and `RECONCILER_REFUND_REVIEW_SUBJECT`; the workflow intentionally does not mark refunds successful without a real provider response.

## Verification

Run the focused workflow tests before a rollout:

```powershell
go test ./internal/app/workfloworch ./internal/bootstrap
```

Then run the full repository test suite before release:

```powershell
go test ./...
```
