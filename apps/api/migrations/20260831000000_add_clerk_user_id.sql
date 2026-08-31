-- +goose Up
ALTER TABLE users ADD COLUMN clerk_user_id TEXT UNIQUE;

-- +goose Down
ALTER TABLE users DROP COLUMN clerk_user_id;
