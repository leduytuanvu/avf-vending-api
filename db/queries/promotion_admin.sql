-- name: PromotionAdminListPromotions :many
SELECT
    p.id,
    p.name,
    p.approval_status,
    p.lifecycle_status,
    p.priority,
    p.stackable,
    p.starts_at,
    p.ends_at,
    p.budget_limit_minor,
    p.redemption_limit,
    p.promotion_channel_kind,
    p.created_at,
    p.updated_at
FROM promotions p
WHERE TRUE
  AND ($1::bool OR p.lifecycle_status <> 'deactivated')
ORDER BY p.priority DESC, p.starts_at DESC, p.name ASC, p.id ASC
LIMIT $2 OFFSET $3;

-- name: PromotionAdminCountPromotions :one
SELECT count(*)::bigint
FROM promotions p
WHERE TRUE
  AND ($1::bool OR p.lifecycle_status <> 'deactivated');

-- name: PromotionAdminGetPromotion :one
SELECT
    p.id,
    p.name,
    p.approval_status,
    p.lifecycle_status,
    p.priority,
    p.stackable,
    p.starts_at,
    p.ends_at,
    p.budget_limit_minor,
    p.redemption_limit,
    p.promotion_channel_kind,
    p.created_at,
    p.updated_at
FROM promotions p
WHERE
    p.id = $1;

-- name: PromotionAdminInsertPromotion :one
INSERT INTO promotions (
    name,
    approval_status,
    lifecycle_status,
    priority,
    stackable,
    starts_at,
    ends_at,
    budget_limit_minor,
    redemption_limit,
    promotion_channel_kind
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9,
    $10
)
RETURNING *;

-- name: PromotionAdminUpdatePromotion :one
UPDATE promotions p
SET
    name = $1,
    approval_status = $2,
    lifecycle_status = $3,
    priority = $4,
    stackable = $5,
    starts_at = $6,
    ends_at = $7,
    budget_limit_minor = $8,
    redemption_limit = $9,
    promotion_channel_kind = $10,
    updated_at = now()
WHERE TRUE
RETURNING *;

-- name: PromotionAdminSetLifecycle :one
UPDATE promotions p
SET
    lifecycle_status = $1,
    updated_at = now()
WHERE TRUE
RETURNING *;

-- name: PromotionAdminListRulesForPromotion :many
SELECT
    id,
    promotion_id,
    rule_type,
    payload,
    priority,
    created_at
FROM promotion_rules
WHERE promotion_id = $1
ORDER BY priority DESC, rule_type ASC, id ASC;

-- name: PromotionAdminDeleteRulesForPromotion :exec
DELETE FROM promotion_rules pr WHERE pr.promotion_id = $1;

-- name: PromotionAdminInsertPromotionRule :one
INSERT INTO promotion_rules (
    promotion_id,
    rule_type,
    payload,
    priority
)
VALUES (
    $1,
    $2,
    $3,
    $4
)
RETURNING *;

-- name: PromotionAdminUpsertPromotionRule :one
INSERT INTO promotion_rules (
    promotion_id,
    rule_type,
    payload,
    priority
)
VALUES (
    $1,
    $2,
    $3,
    $4
)
ON CONFLICT ON CONSTRAINT ux_promotion_rules_promo_type_priority
DO UPDATE SET payload = EXCLUDED.payload
RETURNING *;

-- name: PromotionAdminListTargetsForPromotion :many
SELECT
    id,
    promotion_id,
    target_type,
    product_id,
    category_id,
    machine_id,
    site_id,
    tag_id,
    created_at
FROM promotion_targets
WHERE TRUE
ORDER BY created_at ASC, id ASC;

-- name: PromotionAdminGetPromotionTarget :one
SELECT
    id,
    promotion_id,
    target_type,
    product_id,
    category_id,
    machine_id,
    site_id,
    tag_id,
    created_at
FROM promotion_targets
WHERE TRUE;

-- name: PromotionAdminInsertPromotionTarget :one
INSERT INTO promotion_targets (
    promotion_id,
    target_type,
    product_id,
    category_id,
    machine_id,
    site_id,
    tag_id
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7
)
RETURNING *;

-- name: PromotionAdminDeletePromotionTarget :execrows
DELETE FROM promotion_targets pt
WHERE TRUE;

-- name: PromotionAdminListPromotionsForPreview :many
SELECT
    p.id,
    p.name,
    p.approval_status,
    p.lifecycle_status,
    p.priority,
    p.stackable,
    p.starts_at,
    p.ends_at,
    p.budget_limit_minor,
    p.redemption_limit,
    p.promotion_channel_kind,
    p.created_at,
    p.updated_at
FROM promotions p
WHERE TRUE
  AND p.lifecycle_status = 'active'
  AND p.approval_status = 'approved'
  AND p.starts_at <= $1::timestamptz
  AND p.ends_at > $1::timestamptz
ORDER BY p.priority DESC, p.starts_at DESC, p.id DESC;

-- name: PromotionAdminListRulesForPromotions :many
SELECT
    id,
    promotion_id,
    rule_type,
    payload,
    priority,
    created_at
FROM promotion_rules
WHERE promotion_id = ANY($1::uuid[])
ORDER BY promotion_id, priority DESC, rule_type ASC;

-- name: PromotionAdminListTargetsForOrgPromotions :many
SELECT
    id,
    promotion_id,
    target_type,
    product_id,
    category_id,
    machine_id,
    site_id,
    tag_id,
    created_at
FROM promotion_targets
WHERE TRUE
ORDER BY promotion_id, created_at ASC;

-- name: PromotionAdminListProductTagIDs :many
SELECT tag_id
FROM product_tags
WHERE TRUE;

-- name: PromotionAdminGetProductCategory :one
SELECT category_id
FROM products
WHERE TRUE;
