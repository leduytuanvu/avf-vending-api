-- Machine runtime fleet: device attachments + app runtime sessions.
-- +goose Up
-- +goose StatementBegin

CREATE TABLE machine_device_attachments (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    previous_attachment_id uuid NULL REFERENCES machine_device_attachments (id) ON DELETE SET NULL,
    status text NOT NULL CHECK (status IN ('active', 'replaced', 'revoked', 'compromised')),
    reason text NOT NULL CHECK (reason IN (
        'first_install', 'board_replacement', 'reinstall', 'clear_data', 'maintenance',
        'recovery', 'admin_reattach', 'technician_reattach', 'unknown'
    )),
    attached_at timestamptz NOT NULL DEFAULT now(),
    detached_at timestamptz NULL,
    attached_by_account_id uuid NULL,
    operator_session_id uuid NULL REFERENCES machine_operator_sessions (id) ON DELETE SET NULL,
    correlation_id uuid NULL,
    android_id text NULL,
    android_serial text NULL,
    board_serial text NULL,
    device_serial text NULL,
    sim_serial text NULL,
    sim_iccid text NULL,
    sim_operator text NULL,
    sim_country_iso text NULL,
    manufacturer text NULL,
    brand text NULL,
    model text NULL,
    device_model text NULL,
    hardware text NULL,
    product text NULL,
    android_release text NULL,
    sdk_int int NULL,
    package_name text NULL,
    version_name text NULL,
    version_code bigint NULL,
    app_build_sha text NULL,
    boot_id text NULL,
    network_type text NULL,
    network_state text NULL,
    ip_address inet NULL,
    user_agent text NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ux_machine_device_attachments_one_active
    ON machine_device_attachments (machine_id)
    WHERE status = 'active';

CREATE INDEX ix_machine_device_attachments_machine_attached
    ON machine_device_attachments (machine_id, attached_at DESC);

CREATE INDEX ix_machine_device_attachments_android_id
    ON machine_device_attachments (android_id)
    WHERE android_id IS NOT NULL;

CREATE INDEX ix_machine_device_attachments_sim_iccid
    ON machine_device_attachments (sim_iccid)
    WHERE sim_iccid IS NOT NULL;

CREATE INDEX ix_machine_device_attachments_operator_session
    ON machine_device_attachments (operator_session_id)
    WHERE operator_session_id IS NOT NULL;

CREATE INDEX ix_machine_device_attachments_correlation
    ON machine_device_attachments (correlation_id)
    WHERE correlation_id IS NOT NULL;

COMMENT ON TABLE machine_device_attachments IS 'Android board/device instances attached to a stable physical machine.';
COMMENT ON COLUMN machine_device_attachments.status IS 'active=current board; replaced/revoked/compromised=historical.';

CREATE TABLE machine_runtime_app_sessions (
    id uuid PRIMARY KEY DEFAULT public.uuid_generate_v7(),
    machine_id uuid NOT NULL REFERENCES machines (id) ON DELETE CASCADE,
    device_attachment_id uuid NULL REFERENCES machine_device_attachments (id) ON DELETE SET NULL,
    machine_session_id uuid NULL REFERENCES machine_sessions (id) ON DELETE SET NULL,
    operator_session_id uuid NULL REFERENCES machine_operator_sessions (id) ON DELETE SET NULL,
    previous_runtime_session_id uuid NULL REFERENCES machine_runtime_app_sessions (id) ON DELETE SET NULL,
    boot_id text NOT NULL DEFAULT '',
    app_start_id text NOT NULL DEFAULT '',
    app_instance_id text NOT NULL DEFAULT '',
    package_name text NOT NULL DEFAULT '',
    app_version text NOT NULL DEFAULT '',
    app_build_sha text NOT NULL DEFAULT '',
    start_reason text NOT NULL,
    end_reason text NULL,
    status text NOT NULL DEFAULT 'STARTING' CHECK (status IN (
        'STARTING', 'ONLINE', 'STALE', 'OFFLINE', 'ENDED', 'CRASHED', 'BLOCKED', 'MAINTENANCE', 'REPLACED'
    )),
    started_at timestamptz NOT NULL DEFAULT now(),
    ended_at timestamptz NULL,
    last_heartbeat_at timestamptz NULL,
    last_check_in_at timestamptz NULL,
    last_mqtt_seen_at timestamptz NULL,
    last_network_state text NOT NULL DEFAULT '',
    last_mqtt_state text NOT NULL DEFAULT '',
    storefront_state text NOT NULL DEFAULT 'INITIALIZING' CHECK (storefront_state IN (
        'INITIALIZING', 'COMMISSIONING', 'OUT_OF_SERVICE', 'SELLABLE', 'CHECKOUT_ACTIVE',
        'PAYMENT_ACTIVE', 'VEND_ACTIVE', 'RECOVERY_REQUIRED'
    )),
    sell_ready boolean NOT NULL DEFAULT false,
    blockers jsonb NOT NULL DEFAULT '[]'::jsonb,
    hardware_status jsonb NOT NULL DEFAULT '{}'::jsonb,
    catalog_status jsonb NOT NULL DEFAULT '{}'::jsonb,
    outbox_status jsonb NOT NULL DEFAULT '{}'::jsonb,
    recovery_status jsonb NOT NULL DEFAULT '{}'::jsonb,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT machine_runtime_app_sessions_start_reason_check CHECK (start_reason IN (
        'FIRST_INSTALL', 'NORMAL_BOOT', 'APP_RESTART', 'APP_CRASH_RECOVERY', 'WATCHDOG_RESTART',
        'SYSTEM_REBOOT', 'POWER_LOSS_RECOVERY', 'APK_UPDATE', 'TOKEN_REFRESH_RECOVERY',
        'NETWORK_RECONNECT', 'TECHNICIAN_STARTED', 'BOARD_REPLACEMENT', 'REATTACH_AFTER_CLEAR_DATA',
        'RETURN_FROM_MAINTENANCE', 'ADMIN_COMMAND_RESTART', 'UNKNOWN'
    )),
    CONSTRAINT machine_runtime_app_sessions_end_reason_check CHECK (
        end_reason IS NULL OR end_reason IN (
            'NORMAL_SHUTDOWN', 'APP_EXIT', 'TECHNICIAN_LOGOUT', 'APP_CRASH_DETECTED',
            'WATCHDOG_TIMEOUT', 'HEARTBEAT_TIMEOUT', 'SYSTEM_REBOOT', 'POWER_LOSS', 'APK_UPDATE',
            'MACHINE_DEACTIVATED', 'MAINTENANCE_STARTED', 'TOKEN_REVOKED', 'BOARD_REPLACED',
            'SUPERSEDED_BY_NEW_SESSION', 'UNKNOWN_LOST_CONTACT'
        )
    )
);

CREATE INDEX ix_machine_runtime_app_sessions_machine_started
    ON machine_runtime_app_sessions (machine_id, started_at DESC);

CREATE INDEX ix_machine_runtime_app_sessions_machine_heartbeat
    ON machine_runtime_app_sessions (machine_id, last_heartbeat_at DESC NULLS LAST);

CREATE INDEX ix_machine_runtime_app_sessions_device_attachment
    ON machine_runtime_app_sessions (device_attachment_id)
    WHERE device_attachment_id IS NOT NULL;

CREATE INDEX ix_machine_runtime_app_sessions_machine_session
    ON machine_runtime_app_sessions (machine_session_id)
    WHERE machine_session_id IS NOT NULL;

CREATE INDEX ix_machine_runtime_app_sessions_operator_session
    ON machine_runtime_app_sessions (operator_session_id)
    WHERE operator_session_id IS NOT NULL;

CREATE INDEX ix_machine_runtime_app_sessions_boot_id
    ON machine_runtime_app_sessions (boot_id)
    WHERE boot_id <> '';

CREATE INDEX ix_machine_runtime_app_sessions_status
    ON machine_runtime_app_sessions (status);

CREATE INDEX ix_machine_runtime_app_sessions_start_reason
    ON machine_runtime_app_sessions (start_reason);

CREATE INDEX ix_machine_runtime_app_sessions_end_reason
    ON machine_runtime_app_sessions (end_reason)
    WHERE end_reason IS NOT NULL;

CREATE INDEX ix_machine_runtime_app_sessions_sell_ready
    ON machine_runtime_app_sessions (sell_ready)
    WHERE sell_ready = true;

COMMENT ON TABLE machine_runtime_app_sessions IS 'App runtime lifecycle sessions (distinct from machine_sessions credential sessions).';

ALTER TABLE machines
    ADD COLUMN IF NOT EXISTS current_device_attachment_id uuid NULL REFERENCES machine_device_attachments (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS current_runtime_app_session_id uuid NULL REFERENCES machine_runtime_app_sessions (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS online_status text NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS sale_enabled boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS machine_type text NULL;

ALTER TABLE machines DROP CONSTRAINT IF EXISTS machines_online_status_check;

ALTER TABLE machines
    ADD CONSTRAINT machines_online_status_check CHECK (
        online_status IN ('unknown', 'online', 'stale', 'offline', 'crashed_suspected')
    );

COMMENT ON COLUMN machines.online_status IS 'Connectivity projection; lifecycle remains in status.';
COMMENT ON COLUMN machines.sale_enabled IS 'Admin-controlled sale permission; distinct from finalSellReady.';

ALTER TABLE machine_check_ins
    ADD COLUMN IF NOT EXISTS sim_iccid text NULL,
    ADD COLUMN IF NOT EXISTS app_build_sha text NULL,
    ADD COLUMN IF NOT EXISTS runtime_app_session_id uuid NULL REFERENCES machine_runtime_app_sessions (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS device_attachment_id uuid NULL REFERENCES machine_device_attachments (id) ON DELETE SET NULL;

ALTER TABLE machine_current_snapshot
    ADD COLUMN IF NOT EXISTS current_device_attachment_id uuid NULL REFERENCES machine_device_attachments (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS current_runtime_app_session_id uuid NULL REFERENCES machine_runtime_app_sessions (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS online_status text NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS runtime_session_status text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS runtime_start_reason text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS runtime_started_at timestamptz NULL,
    ADD COLUMN IF NOT EXISTS runtime_last_heartbeat_at timestamptz NULL,
    ADD COLUMN IF NOT EXISTS last_mqtt_state text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS storefront_state text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS sell_ready boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS blockers jsonb NOT NULL DEFAULT '[]'::jsonb;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE machine_current_snapshot
    DROP COLUMN IF EXISTS blockers,
    DROP COLUMN IF EXISTS sell_ready,
    DROP COLUMN IF EXISTS storefront_state,
    DROP COLUMN IF EXISTS last_mqtt_state,
    DROP COLUMN IF EXISTS runtime_last_heartbeat_at,
    DROP COLUMN IF EXISTS runtime_started_at,
    DROP COLUMN IF EXISTS runtime_start_reason,
    DROP COLUMN IF EXISTS runtime_session_status,
    DROP COLUMN IF EXISTS online_status,
    DROP COLUMN IF EXISTS current_runtime_app_session_id,
    DROP COLUMN IF EXISTS current_device_attachment_id;

ALTER TABLE machine_check_ins
    DROP COLUMN IF EXISTS device_attachment_id,
    DROP COLUMN IF EXISTS runtime_app_session_id,
    DROP COLUMN IF EXISTS app_build_sha,
    DROP COLUMN IF EXISTS sim_iccid;

ALTER TABLE machines DROP CONSTRAINT IF EXISTS machines_online_status_check;

ALTER TABLE machines
    DROP COLUMN IF EXISTS machine_type,
    DROP COLUMN IF EXISTS sale_enabled,
    DROP COLUMN IF EXISTS online_status,
    DROP COLUMN IF EXISTS current_runtime_app_session_id,
    DROP COLUMN IF EXISTS current_device_attachment_id;

DROP TABLE IF EXISTS machine_runtime_app_sessions;
DROP TABLE IF EXISTS machine_device_attachments;

-- +goose StatementEnd
