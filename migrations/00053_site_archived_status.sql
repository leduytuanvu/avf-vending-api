-- +goose Up
-- Rename legacy site lifecycle value inactive → archived (enterprise naming).

ALTER TABLE sites DROP CONSTRAINT IF EXISTS sites_status_check;

UPDATE sites
SET
    status = 'archived'
WHERE
    status = 'inactive';

ALTER TABLE sites
ADD CONSTRAINT sites_status_check CHECK (status IN ('active', 'archived'));

-- +goose Down

ALTER TABLE sites DROP CONSTRAINT IF EXISTS sites_status_check;

UPDATE sites
SET
    status = 'inactive'
WHERE
    status = 'archived';

ALTER TABLE sites
ADD CONSTRAINT sites_status_check CHECK (status IN ('active', 'inactive'));
