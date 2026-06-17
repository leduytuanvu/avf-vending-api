-- name: FleetAdminListMachines :many
SELECT
    m.id,
    m.site_id,
    m.hardware_profile_id,
    m.serial_number,
    m.name,
    m.status,
    m.command_sequence,
    m.created_at,
    m.updated_at,
    s.name AS site_name,
    snap.android_id,
    snap.sim_serial,
    snap.sim_iccid,
    snap.app_version,
    snap.firmware_version,
    snap.last_heartbeat_at,
    COALESCE(
        NULLIF(btrim(COALESCE(m.timezone_override, '')), ''),
        s.timezone,
        'UTC'
    ) AS effective_timezone
FROM machines m
INNER JOIN sites s ON s.id = m.site_id
LEFT JOIN machine_current_snapshot snap ON snap.machine_id = m.id
WHERE
    ($1::boolean IS FALSE OR m.site_id = $2::uuid)
    AND ($3::boolean IS FALSE OR m.id = $4::uuid)
    AND ($5::boolean IS FALSE OR m.status = $6::text)
    AND m.updated_at >= $7::timestamptz
    AND m.updated_at <= $8::timestamptz
ORDER BY
    m.updated_at DESC,
    m.name ASC
LIMIT $9 OFFSET $10;

-- name: FleetAdminGetMachineDetail :one
SELECT
    m.id,
    m.site_id,
    m.hardware_profile_id,
    m.serial_number,
    m.name,
    m.status,
    m.command_sequence,
    m.created_at,
    m.updated_at,
    s.name AS site_name,
    snap.android_id,
    snap.sim_serial,
    snap.sim_iccid,
    snap.app_version,
    snap.firmware_version,
    snap.last_heartbeat_at,
    snap.effective_device_config,
    snap.device_config_field_ack,
    COALESCE(
        NULLIF(btrim(COALESCE(m.timezone_override, '')), ''),
        s.timezone,
        'UTC'
    ) AS effective_timezone
FROM machines m
INNER JOIN sites s ON s.id = m.site_id
LEFT JOIN machine_current_snapshot snap ON snap.machine_id = m.id
WHERE
    m.id = $1;

-- name: FleetAdminListViewOperatorsForMachines :many
SELECT
    v.machine_id,
    v.operator_session_id,
    v.actor_type,
    v.technician_id,
    v.technician_display_name,
    v.user_principal,
    v.session_started_at,
    v.session_status,
    v.session_expires_at
FROM v_machine_current_operator v
WHERE
    v.machine_id = ANY($1::uuid[]);

-- name: FleetAdminListActiveTechnicianAssignmentsForMachines :many
SELECT
    tma.machine_id,
    tma.technician_id,
    t.display_name AS technician_display_name,
    tma.role,
    tma.valid_from,
    tma.valid_to
FROM technician_machine_assignments tma
INNER JOIN technicians t ON t.id = tma.technician_id
WHERE
    tma.machine_id = ANY($1::uuid[])
    AND (
        tma.valid_to IS NULL
        OR tma.valid_to > now()
    )
ORDER BY
    tma.machine_id ASC,
    t.display_name ASC;

-- name: FleetAdminCountMachines :one
SELECT
    count(*)::bigint AS cnt
FROM machines m
WHERE
    ($1::boolean IS FALSE OR m.site_id = $2::uuid)
    AND ($3::boolean IS FALSE OR m.id = $4::uuid)
    AND ($5::boolean IS FALSE OR m.status = $6::text)
    AND m.updated_at >= $7::timestamptz
    AND m.updated_at <= $8::timestamptz;

-- name: FleetAdminListTechnicians :many
SELECT
    t.id,
    t.display_name,
    t.email,
    t.phone,
    t.external_subject,
    t.status,
    t.created_at,
    t.updated_at
FROM technicians t
WHERE
    ($1::boolean IS FALSE OR t.id = $2::uuid)
    AND (
        $3::boolean IS FALSE
        OR t.display_name ILIKE ('%' || $4::text || '%')
        OR (
            t.email IS NOT NULL
            AND t.email::text ILIKE ('%' || $4::text || '%')
        )
    )
    AND t.created_at >= $5::timestamptz
    AND t.created_at <= $6::timestamptz
ORDER BY
    t.display_name ASC
LIMIT $7 OFFSET $8;

-- name: FleetAdminCountTechnicians :one
SELECT
    count(*)::bigint AS cnt
FROM technicians t
WHERE
    ($1::boolean IS FALSE OR t.id = $2::uuid)
    AND (
        $3::boolean IS FALSE
        OR t.display_name ILIKE ('%' || $4::text || '%')
        OR (
            t.email IS NOT NULL
            AND t.email::text ILIKE ('%' || $4::text || '%')
        )
    )
    AND t.created_at >= $5::timestamptz
    AND t.created_at <= $6::timestamptz;

-- name: FleetAdminListAssignments :many
SELECT
    tma.id AS assignment_id,
    tma.technician_id,
    t.display_name AS technician_display_name,
    tma.machine_id,
    m.name AS machine_name,
    m.serial_number AS machine_serial_number,
    tma.role,
    tma.valid_from,
    tma.valid_to,
    tma.created_at
FROM technician_machine_assignments tma
INNER JOIN technicians t ON t.id = tma.technician_id
INNER JOIN machines m ON m.id = tma.machine_id
WHERE
    ($1::boolean IS FALSE OR tma.technician_id = $2::uuid)
    AND ($3::boolean IS FALSE OR tma.machine_id = $4::uuid)
    AND tma.created_at >= $5::timestamptz
    AND tma.created_at <= $6::timestamptz
ORDER BY
    tma.valid_from DESC
LIMIT $7 OFFSET $8;

-- name: FleetAdminCountAssignments :one
SELECT
    count(*)::bigint AS cnt
FROM technician_machine_assignments tma
INNER JOIN technicians t ON t.id = tma.technician_id
WHERE
    ($1::boolean IS FALSE OR tma.technician_id = $2::uuid)
    AND ($3::boolean IS FALSE OR tma.machine_id = $4::uuid)
    AND tma.created_at >= $5::timestamptz
    AND tma.created_at <= $6::timestamptz;

-- name: FleetAdminListCommands :many
SELECT
    cl.id AS command_id,
    cl.machine_id,
    m.name AS machine_name,
    m.serial_number AS machine_serial_number,
    cl.sequence,
    cl.command_type,
    cl.created_at,
    cl.attempt_count,
    cl.correlation_id,
    la.status AS latest_attempt_status
FROM command_ledger cl
INNER JOIN machines m ON m.id = cl.machine_id
LEFT JOIN LATERAL (
    SELECT
        mca.status
    FROM machine_command_attempts mca
    WHERE
        mca.command_id = cl.id
    ORDER BY
        mca.attempt_no DESC
    LIMIT
        1
) la ON TRUE
WHERE
    ($1::boolean IS FALSE OR cl.machine_id = $2::uuid)
    AND ($3::boolean IS FALSE OR COALESCE(la.status, 'pending') = $4::text)
    AND cl.created_at >= $5::timestamptz
    AND cl.created_at <= $6::timestamptz
ORDER BY
    cl.created_at DESC
LIMIT $7 OFFSET $8;

-- name: FleetAdminCountCommands :one
SELECT
    count(*)::bigint AS cnt
FROM command_ledger cl
INNER JOIN machines m ON m.id = cl.machine_id
LEFT JOIN LATERAL (
    SELECT
        mca.status
    FROM machine_command_attempts mca
    WHERE
        mca.command_id = cl.id
    ORDER BY
        mca.attempt_no DESC
    LIMIT
        1
) la ON TRUE
WHERE
    ($1::boolean IS FALSE OR cl.machine_id = $2::uuid)
    AND ($3::boolean IS FALSE OR COALESCE(la.status, 'pending') = $4::text)
    AND cl.created_at >= $5::timestamptz
    AND cl.created_at <= $6::timestamptz;

-- name: FleetAdminListOTACampaigns :many
SELECT
    c.id AS campaign_id,
    c.name AS campaign_name,
    c.rollout_strategy AS strategy,
    c.status AS campaign_status,
    c.created_at,
    a.id AS artifact_id,
    a.semver AS artifact_semver,
    a.storage_key AS artifact_storage_key
FROM ota_campaigns c
INNER JOIN ota_artifacts a ON a.id = c.artifact_id
WHERE
    ($1::boolean IS FALSE OR c.status = $2::text)
    AND c.created_at >= $3::timestamptz
    AND c.created_at <= $4::timestamptz
ORDER BY
    c.created_at DESC
LIMIT $5 OFFSET $6;

-- name: FleetAdminCountOTACampaigns :one
SELECT
    count(*)::bigint AS cnt
FROM ota_campaigns c
WHERE
    ($1::boolean IS FALSE OR c.status = $2::text)
    AND c.created_at >= $3::timestamptz
    AND c.created_at <= $4::timestamptz;

-- name: FleetAdminUpsertMachineCabinet :one
INSERT INTO machine_cabinets (
    machine_id,
    cabinet_code,
    title,
    sort_order,
    cabinet_index,
    slot_capacity,
    status,
    metadata
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
)
ON CONFLICT (machine_id, cabinet_code) DO UPDATE
SET
    title = EXCLUDED.title,
    sort_order = EXCLUDED.sort_order,
    cabinet_index = EXCLUDED.cabinet_index,
    slot_capacity = EXCLUDED.slot_capacity,
    status = EXCLUDED.status,
    metadata = EXCLUDED.metadata,
    updated_at = now()
RETURNING
    id,
    machine_id,
    cabinet_code,
    title,
    sort_order,
    cabinet_index,
    slot_capacity,
    status,
    metadata,
    created_at,
    updated_at;

-- name: FleetAdminUpsertMachineSlotLayout :one
INSERT INTO machine_slot_layouts (
    machine_id,
    machine_cabinet_id,
    layout_key,
    revision,
    layout_spec,
    status
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
ON CONFLICT (machine_id, machine_cabinet_id, layout_key, revision) DO UPDATE
SET
    layout_spec = EXCLUDED.layout_spec,
    status = EXCLUDED.status
RETURNING
    id,
    machine_id,
    machine_cabinet_id,
    layout_key,
    revision,
    layout_spec,
    status,
    created_at;

-- name: FleetAdminListMachineTopology :many
SELECT
    mc.id AS cabinet_id,
    mc.machine_id,
    mc.cabinet_code,
    mc.title AS cabinet_title,
    mc.sort_order AS cabinet_sort_order,
    mc.metadata AS cabinet_metadata,
    msl.id AS slot_layout_id,
    msl.layout_key AS slot_layout_key,
    msl.revision AS slot_layout_revision,
    msl.status AS slot_layout_status,
    msl.layout_spec AS slot_layout_spec,
    msc.id AS slot_config_id,
    msc.slot_code,
    msc.slot_index AS slot_config_slot_index,
    msc.product_id AS slot_config_product_id,
    msc.max_quantity AS slot_config_max_quantity,
    msc.price_minor AS slot_config_price_minor,
    msc.is_current AS slot_config_is_current,
    msc.metadata AS slot_config_metadata
FROM
    machine_cabinets mc
    LEFT JOIN LATERAL (
        SELECT
            l.*
        FROM
            machine_slot_layouts l
        WHERE
            l.machine_cabinet_id = mc.id
            AND l.status = 'published'
        ORDER BY
            l.revision DESC,
            l.created_at DESC
        LIMIT
            1
    ) msl ON TRUE
    LEFT JOIN machine_slot_configs msc ON msc.machine_cabinet_id = mc.id
    AND msc.is_current
WHERE
    mc.machine_id = $1
ORDER BY
    mc.sort_order ASC,
    mc.cabinet_code ASC,
    msc.slot_code ASC;

-- name: FleetAdminInsertAssortment :one
INSERT INTO assortments (
    name,
    status,
    description,
    meta
)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING
    id,
    name,
    status,
    description,
    meta,
    created_at,
    updated_at;

-- name: FleetAdminUpdateAssortment :one
UPDATE assortments
SET
    name = $1,
    status = $2,
    description = $3,
    meta = $4,
    updated_at = now()
WHERE
    id = $5
    AND TRUE
RETURNING
    id,
    name,
    status,
    description,
    meta,
    created_at,
    updated_at;

-- name: FleetAdminUpsertAssortmentItem :one
INSERT INTO assortment_items (
    assortment_id,
    product_id,
    sort_order,
    notes
)
VALUES (
    $1,
    $2,
    $3,
    $4
)
ON CONFLICT (assortment_id, product_id) DO UPDATE
SET
    sort_order = EXCLUDED.sort_order,
    notes = EXCLUDED.notes
RETURNING
    id,
    assortment_id,
    product_id,
    sort_order,
    notes,
    created_at;

-- name: FleetAdminBindMachinePrimaryAssortment :execrows
WITH closed AS (
    UPDATE machine_assortment_bindings b
    SET
        valid_to = now()
    FROM
        machines m
    WHERE
        b.machine_id = $1
        AND m.id = b.machine_id
        AND TRUE
        AND b.is_primary
        AND b.valid_to IS NULL
)
INSERT INTO machine_assortment_bindings (
    machine_id,
    assortment_id,
    is_primary,
    valid_from
)
SELECT
    m.id,
    $2,
    TRUE,
    now()
FROM
    machines m
    INNER JOIN assortments a ON a.id = $2
    AND TRUE
WHERE
    m.id = $1
    AND TRUE;

-- name: FleetAdminListAssortmentProductsByMachine :many
SELECT
    p.id AS product_id,
    p.sku AS product_sku,
    p.name AS product_name,
    ai.sort_order AS assortment_item_sort_order,
    a.id AS assortment_id,
    a.name AS assortment_name
FROM
    machines m
    INNER JOIN machine_assortment_bindings b ON b.machine_id = m.id
    AND b.is_primary
    AND b.valid_to IS NULL
    INNER JOIN assortments a ON a.id = b.assortment_id
    AND TRUE
    INNER JOIN assortment_items ai ON ai.assortment_id = a.id
    INNER JOIN products p ON p.id = ai.product_id
    AND TRUE
WHERE
    m.id = $1
    AND TRUE
ORDER BY
    ai.sort_order ASC,
    p.name ASC;

-- name: FleetAdminApplyMachineSlotConfigCurrent :one
WITH cleared AS (
    UPDATE machine_slot_configs
    SET
        is_current = FALSE,
        effective_to = coalesce(effective_to, now()),
        updated_at = now()
    WHERE
        machine_id = $1
        AND slot_code = $2
        AND is_current
)
INSERT INTO machine_slot_configs (
    machine_id,
    machine_cabinet_id,
    machine_slot_layout_id,
    slot_code,
    slot_index,
    product_id,
    max_quantity,
    price_minor,
    effective_from,
    is_current,
    metadata
)
VALUES (
    $1,
    $3,
    $4,
    $2,
    $5,
    $6,
    $7,
    $8,
    $9,
    TRUE,
    $10
)
RETURNING
    id,
    machine_id,
    machine_cabinet_id,
    machine_slot_layout_id,
    slot_code,
    slot_index,
    product_id,
    max_quantity,
    price_minor,
    effective_from,
    effective_to,
    is_current,
    metadata,
    created_at,
    updated_at;

-- name: FleetAdminListMachineCabinets :many
SELECT
    id,
    machine_id,
    cabinet_code,
    title,
    sort_order,
    cabinet_index,
    slot_capacity,
    status,
    metadata,
    created_at,
    updated_at
FROM
    machine_cabinets
WHERE
    machine_id = $1
ORDER BY
    sort_order ASC,
    cabinet_code ASC;

-- name: FleetAdminGetMachineCabinetByMachineAndCode :one
SELECT
    id,
    machine_id,
    cabinet_code,
    title,
    sort_order,
    cabinet_index,
    slot_capacity,
    status,
    metadata,
    created_at,
    updated_at
FROM
    machine_cabinets
WHERE
    machine_id = $1
    AND cabinet_code = $2;

-- name: FleetAdminGetMachineSlotLayoutByKey :one
SELECT
    id,
    machine_id,
    machine_cabinet_id,
    layout_key,
    revision,
    layout_spec,
    status,
    created_at
FROM
    machine_slot_layouts
WHERE
    machine_id = $1
    AND machine_cabinet_id = $2
    AND layout_key = $3
    AND revision = $4;

-- name: FleetAdminInsertMachineSlotConfigDraft :one
INSERT INTO machine_slot_configs (
    machine_id,
    machine_cabinet_id,
    machine_slot_layout_id,
    slot_code,
    slot_index,
    product_id,
    max_quantity,
    price_minor,
    effective_from,
    is_current,
    metadata
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    FALSE,
    $10
)
RETURNING
    id,
    machine_id,
    machine_cabinet_id,
    machine_slot_layout_id,
    slot_code,
    slot_index,
    product_id,
    max_quantity,
    price_minor,
    effective_from,
    effective_to,
    is_current,
    metadata,
    created_at,
    updated_at;
