# BLOCKER: Production reset retention sign-off

**Do not schedule** `production-reset-bootstrap-admin.sql` until one of the following is complete:

1. **Written retention sign-off** from the business owner confirming that destruction of orders, payments, refunds, financial ledger entries, audit events, and reconciliation data is acceptable; or
2. **Export and archive** of all records subject to legal, tax, or contractual retention, with archive location and checksum recorded in the change ticket.

## Records at risk

- Orders, payments, payment provider events, refunds, vend sessions
- Financial ledger, cash settlement, reconciliation cases
- `audit_events` / `audit_logs`
- Machine telemetry and offline event history

## Sign-off template

```
Business owner: ______________________  Date: __________
Retention decision: [ ] destroy  [ ] archive first (path: _______________)
Ticket / change ID: ______________________
```

File the signed copy outside the repository (ticket system or compliance store). This document remains in-repo as a permanent gate reminder.
