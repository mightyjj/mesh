-- +goose Up
CREATE TABLE content (
	content_id BIGSERIAL PRIMARY KEY,
	creator_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
	title TEXT NOT NULL CHECK (BTRIM(title) <> ''),
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX content_creator_id_idx ON content (creator_id);

-- +goose Down
DROP TABLE content;
