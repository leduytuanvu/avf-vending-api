-- +goose Up
ALTER TABLE platform_auth_accounts ADD COLUMN username text;

-- Deterministic backfill from the email local part, de-duplicated by a stable
-- ordering so repeated runs on the same data produce the same result.
WITH candidate AS (
    SELECT id,
           lower(split_part(email, '@', 1)) AS base,
           row_number() OVER (
               PARTITION BY lower(split_part(email, '@', 1))
               ORDER BY created_at, id
           ) AS n
    FROM platform_auth_accounts
)
UPDATE platform_auth_accounts a
SET username = CASE WHEN c.n = 1 THEN c.base ELSE c.base || '-' || c.n END
FROM candidate c
WHERE a.id = c.id;

ALTER TABLE platform_auth_accounts ALTER COLUMN username SET NOT NULL;
ALTER TABLE platform_auth_accounts
    ADD CONSTRAINT platform_auth_accounts_username_format
    CHECK (username ~ '^[A-Za-z0-9][A-Za-z0-9._-]{1,30}[A-Za-z0-9]$');
CREATE UNIQUE INDEX uniq_platform_auth_accounts_username_lower
    ON platform_auth_accounts (lower(username));

ALTER TABLE platform_auth_accounts ALTER COLUMN email DROP NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS uniq_platform_auth_accounts_username_lower;
ALTER TABLE platform_auth_accounts DROP CONSTRAINT IF EXISTS platform_auth_accounts_username_format;
ALTER TABLE platform_auth_accounts DROP COLUMN IF EXISTS username;
-- email NOT NULL is intentionally NOT restored: rows created after this
-- migration may legitimately have NULL email.
