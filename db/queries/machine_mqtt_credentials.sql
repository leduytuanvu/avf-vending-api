-- name: UpsertMachineMQTTCredentials :exec
INSERT INTO machine_mqtt_credentials (
    machine_id,
    mqtt_broker_shard,
    username,
    secret_ref,
    updated_at
)
VALUES (
    @machine_id,
    COALESCE(NULLIF(@mqtt_broker_shard, ''), 'default'),
    @username,
    @secret_ref,
    now()
)
ON CONFLICT (machine_id) DO UPDATE
SET
    username = EXCLUDED.username,
    secret_ref = EXCLUDED.secret_ref,
    updated_at = now();

-- name: GetMachineMQTTCredentials :one
SELECT
    *
FROM
    machine_mqtt_credentials
WHERE
    machine_id = $1;

-- name: RevokeMachineMQTTCredentials :exec
UPDATE machine_mqtt_credentials
SET
    username = NULL,
    secret_ref = 'revoked',
    updated_at = now()
WHERE
    machine_id = $1;

-- name: DeleteMachineMQTTCredentials :exec
DELETE FROM machine_mqtt_credentials
WHERE
    machine_id = $1;
