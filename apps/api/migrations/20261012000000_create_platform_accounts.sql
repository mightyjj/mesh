-- +goose Up
CREATE TABLE platform_accounts (
    platform_account_id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform TEXT NOT NULL CHECK (platform = 'fake'),
    external_account_id TEXT NOT NULL CHECK (external_account_id ~ '[^[:space:]]'),
    display_name TEXT NOT NULL CHECK (display_name ~ '[^[:space:]]'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (platform, external_account_id)
);

CREATE INDEX platform_accounts_user_id_idx ON platform_accounts (user_id);

-- +goose Down
DROP TABLE platform_accounts;
