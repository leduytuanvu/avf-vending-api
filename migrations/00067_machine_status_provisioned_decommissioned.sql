-- Expand machines.status CHECK for enterprise lifecycle vocabulary (additive; keeps existing values).

-- +goose Up
ALTER TABLE machines
    DROP CONSTRAINT IF EXISTS machines_status_check;

ALTER TABLE machines
    ADD CONSTRAINT machines_status_check CHECK (status IN (
        'draft',
        'provisioned',
        'active',
        'maintenance',
        'suspended',
        'retired',
        'decommissioned',
        'compromised',
        'provisioning',
        'online',
        'offline'
    ));

-- +goose Down
UPDATE machines
SET status = 'draft'
WHERE status = 'provisioned';

UPDATE machines
SET status = 'retired'
WHERE status = 'decommissioned';

ALTER TABLE machines
    DROP CONSTRAINT IF EXISTS machines_status_check;

ALTER TABLE machines
    ADD CONSTRAINT machines_status_check CHECK (status IN (
        'draft',
        'active',
        'maintenance',
        'suspended',
        'retired',
        'compromised',
        'provisioning',
        'online',
        'offline'
    ));
