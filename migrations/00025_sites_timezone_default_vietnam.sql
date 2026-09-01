-- +goose Up
ALTER TABLE sites ALTER COLUMN timezone SET DEFAULT 'Asia/Ho_Chi_Minh';
UPDATE sites SET timezone = 'Asia/Ho_Chi_Minh' WHERE timezone IN ('', 'UTC');

-- +goose Down
ALTER TABLE sites ALTER COLUMN timezone SET DEFAULT 'UTC';
