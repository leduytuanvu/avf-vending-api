# Machine credential and MQTT access rotation

Use when rotating **machine JWT**, **MQTT username/password**, or **broker trust** so field devices keep working without cross-machine topic leakage.

## Scope

- **HTTP/gRPC machine JWT**: issued by AVF auth; `credential_version` on `machines` advances.
- **MQTT**: EMQX built-in user or JWT-authenticated client; **topic ACL** in `deployments/prod/emqx/acl.conf.example` must still map each live principal to **only** `machines/{that_machine_id}/...` (enterprise) or `{machine_id}/...` (legacy).

## Safe rotation sequence

1. **Overlap window**: issue new MQTT password (or new cert) **before** revoking the old one; keep both valid in EMQX until the fleet rollout completes or per-device confirmation exists.
2. **Update device config**: new password + same `machine_id` / username rule (UUID-as-username pattern for `%%u` ACLs).
3. **Broker**: update EMQX authentication store (`scripts/emqx_bootstrap.sh` pattern); **do not** widen ACL patterns.
4. **Backend**: PostgreSQL `credential_version` / lifecycle timestamps must match what auth middleware expects; see machine activation runbooks.
5. **Revoke old secret**: remove old MQTT user password or disable old cert only after traffic confirms on the new credential.

## TLS rotation

- **Broker server cert**: see `docs/api/mqtt-contract.md` (TLS and server certificate rotation).
- **Device trust store**: ship updated CA before the old chain expires.

## Related

- `docs/api/mqtt-contract.md` (broker ACL contract)
- [machine-activation.md](./machine-activation.md)
- `deployments/prod/emqx/README.md`
