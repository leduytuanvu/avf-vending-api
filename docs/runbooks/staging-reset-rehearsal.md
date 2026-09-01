# Staging reset rehearsal checklist

Execute on a **disposable** database restored from a recent staging or production backup. Never run against production from this checklist alone.

## 1. Restore drill

- [ ] `backup_managed_postgres.sh execute` on source
- [ ] `restore_managed_postgres.sh drill` onto disposable target (`RESTORE_TARGET_DISPOSABLE=1`)
- [ ] Record backup path, sha256, goose version

## 2. Reset execution

- [ ] `SET avf.confirm_production_reset='I_UNDERSTAND_THIS_WIPES_PRODUCTION'`
- [ ] `psql -f scripts/ops/production-reset-bootstrap-admin.sql`
- [ ] `goose status` — version unchanged
- [ ] Row counts: `machines=0`, `orders=0`, `platform_auth_accounts=0`

## 3. Bootstrap + auth

- [ ] `/app/bootstrap-admin` with test credentials from env (not committed)
- [ ] `POST /v1/auth/login` with `username` succeeds
- [ ] `/v1/auth/me` returns `roles: ["platform_admin"]`
- [ ] Platform-admin-only route (e.g. admin outbox) returns 200

## 4. Migration rollback drill (optional)

- [ ] On another disposable DB: `goose up` through `00025`, then `goose down` 00025, then `goose up` — verify clean cycle
- [ ] Rehearse image rollback procedure per `production-rollback.md` with 00024+ image pairing documented

## 5. Evidence

Archive command output, row-count queries, and login response redacted of tokens in the change ticket.
