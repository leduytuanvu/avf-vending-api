-- name: PromotionAdminListPromotions :many
SELECT
    p.id,
    p.organization_id,
    p.name,
    p.approval_status,
    p.lifecycle_status,
    p.priority,
    p.stackable,
    p.starts_at,
    p.ends_at,
    p.budget_limit_minor,
    p.redemption_limit,
    p.channel_scope,
    p.created_at,
    p.updated_at
FROM promotions p
WHERE p.organization_id = $1
  AND ($4::bool OR p.lifecycle_status <> 'deactivated')
ORDER BY p.priority DESC, p.starts_at DESC, p.name ASC, p.id ASC
LIMIT $2 OFFSET $3;

-- name: PromotionAdminCountPromotions :one
SELECT count(*)::bigint
FROM promotions p
WHERE p.organization_id = $1
  AND ($2::bool OR p.lifecycle_status <> 'deactivated');

-- name: PromotionAdminGetPromotion :one
SELECT
    p.id,
    p.organization_id,
    p.name,
    p.approval_status,
    p.lifecycle_status,
    p.priority,
    p.stackable,
    p.starts_at,
    p.ends_at,
    p.budget_limit_minor,
    p.redemption_limit,
    p.channel_scope,
    p.created_at,
    p.updated_at
FROM promotions p
WHERE p.organization_id = $1 AND p.id = $2;

-- name: PromotionAdminInsertPromotion :one
INSERT INTO promotions (
    organization_id,
    name,
    approval_status,
    lifecycle_status,
    priority,
    stackable,
    starts_at,
    ends_at,
    budget_limit_minor,
    redemption_limit,
    channel_scope
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: PromotionAdminUpdatePromotion :one
UPDATE promotions p
SET
    name = $3,
    approval_status = $4,
    lifecycle_status = $5,
    priority = $6,
    stackable = $7,
    starts_at = $8,
    ends_at = $9,
    budget_limit_minor = $10,
    redemption_limit = $11,
    channel_scope = $12,
    updated_at = now()
WHERE p.organization_id = $1 AND p.id = $2
RETURNING *;

-- name: PromotionAdminSetLifecycle :one
UPDATE promotions p
SET
    lifecycle_status = $3,
    updated_at = now()
WHERE p.organization_id = $1 AND p.id = $2
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
INSERT INTO promotion_rules (promotion_id, rule_type, payload, priority)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: PromotionAdminUpsertPromotionRule :one
INSERT INTO promotion_rules (promotion_id, rule_type, payload, priority)
VALUES ($1, $2, $3, $4)
ON CONFLICT ON CONSTRAINT ux_promotion_rules_promo_type_priority
DO UPDATE SET payload = EXCLUDED.payload
RETURNING *;

-- name: PromotionAdminListTargetsForPromotion :many
SELECT
    id,
    promotion_id,
    organization_id,
    target_type,
    product_id,
    category_id,
    machine_id,
    site_id,
    organization_target_id,
    tag_id,
    created_at
FROM promotion_targets
WHERE organization_id = $1 AND promotion_id = $2
ORDER BY created_at ASC, id ASC;

-- name: PromotionAdminGetPromotionTarget :one
SELECT
    id,
    promotion_id,
    organization_id,
    target_type,
    product_id,
    category_id,
    machine_id,
    site_id,
    organization_target_id,
    tag_id,
    created_at
FROM promotion_targets
WHERE organization_id = $1 AND id = $2;

-- name: PromotionAdminInsertPromotionTarget :one
INSERT INTO promotion_targets (
    promotion_id,
    organization_id,
    target_type,
    product_id,
    category_id,
    machine_id,
    site_id,
    organization_target_id,
    tag_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: PromotionAdminDeletePromotionTarget :execrows
DELETE FROM promotion_targets pt
WHERE pt.organization_id = $1 AND pt.id = $2;

-- name: PromotionAdminListPromotionsForPreview :many
SELECT
    p.id,
    p.organization_id,
    p.name,
    p.approval_status,
    p.lifecycle_status,
    p.priority,
    p.stackable,
    p.starts_at,
    p.ends_at,
    p.budget_limit_minor,
    p.redemption_limit,
    p.channel_scope,
    p.created_at,
    p.updated_at
FROM promotions p
WHERE p.organization_id = $1
  AND p.lifecycle_status = 'active'
  AND p.approval_status = 'approved'
  AND p.starts_at <= $2::timestamptz
  AND p.ends_at > $2::timestamptz
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
    organization_id,
    target_type,
    product_id,
    category_id,
    machine_id,
    site_id,
    organization_target_id,
    tag_id,
    created_at
FROM promotion_targets
WHERE organization_id = $1 AND promotion_id = ANY($2::uuid[])
ORDER BY promotion_id, created_at ASC;

-- name: PromotionAdminListProductTagIDs :many
SELECT tag_id
FROM product_tags
WHERE organization_id = $1 AND product_id = $2;

-- name: PromotionAdminGetProductCategory :one
SELECT category_id
FROM products
WHERE organization_id = $1 AND id = $2;
