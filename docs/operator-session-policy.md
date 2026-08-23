# Operator session origin policy

An **operator session** is a human physically operating a specific machine (kiosk APP or technician on the device). It is not Web Admin login.

## Policies

- **SESSION_REQUIRED** — cash collection open/close, technician-on-app physical fleet ops, gRPC kiosk operator/inventory/cash. Missing session → `400 operator_session_required`.
- **SESSION_OPTIONAL_BY_ORIGIN** — topology, planogram draft/publish, setup sync, stock adjustments.
  - Session **sent** → must be `ACTIVE` and `session.machine_id == path machine`. Invalid/ended/mismatched → reject (never fall back to admin origin).
  - Session **omitted** → origin `api`; actor is `auth.Principal` from the request (subject, roles). Zero UUID is invalid, not omit.
- **SESSION_NOT_APPLICABLE** — session has no meaning.

Origin is **server-derived**. Clients cannot set `action_origin_type`. Remote HTTP admin is `api` (not `remote_support` unless that domain path already exists).

## Attribution

Every successful optional-by-origin mutation writes `machine_action_attributions` (`operator_session_id` nullable). Metadata includes `actor_subject`, `actor_type`, `roles`, `origin`. Attribution insert failure returns **500** (fail-closed for the HTTP response). Topology upsert may already have committed; retry is safe.

Cash collection keeps its existing audit/fail-open behavior.

## Must not

- Auto `StartOperatorSession` / `force_admin_takeover` from topology, planogram, sync, or stock.
- Treat operator session as a planogram lock (revision + Idempotency-Key remain).
- Silently ignore an invalid `operator_session_id`.
- Allow sessionless cash collection.

## Compatibility

APP physical sessions still work when `operator_session_id` is sent. Remote admin while a technician session is ACTIVE does not take over or insert a new `machine_operator_sessions` row.
