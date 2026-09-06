-- +goose Up
ALTER TABLE content
	ADD COLUMN original_filename TEXT,
	ADD COLUMN mime_type TEXT,
	ADD COLUMN byte_size BIGINT,
	ADD COLUMN storage_key TEXT,
	ADD CONSTRAINT content_media_metadata_check CHECK (
		(original_filename IS NULL AND mime_type IS NULL AND byte_size IS NULL AND storage_key IS NULL) OR
		(original_filename IS NOT NULL AND mime_type IS NOT NULL AND byte_size IS NOT NULL AND storage_key IS NOT NULL)
	),
	ADD CONSTRAINT content_original_filename_nonblank_check CHECK (original_filename IS NULL OR BTRIM(original_filename) <> ''),
	ADD CONSTRAINT content_mime_type_nonblank_check CHECK (mime_type IS NULL OR BTRIM(mime_type) <> ''),
	ADD CONSTRAINT content_byte_size_positive_check CHECK (byte_size IS NULL OR byte_size > 0),
	ADD CONSTRAINT content_storage_key_nonblank_check CHECK (storage_key IS NULL OR BTRIM(storage_key) <> ''),
	ADD CONSTRAINT content_storage_key_key UNIQUE (storage_key);

-- +goose Down
ALTER TABLE content
	DROP CONSTRAINT content_storage_key_key,
	DROP CONSTRAINT content_storage_key_nonblank_check,
	DROP CONSTRAINT content_byte_size_positive_check,
	DROP CONSTRAINT content_mime_type_nonblank_check,
	DROP CONSTRAINT content_original_filename_nonblank_check,
	DROP CONSTRAINT content_media_metadata_check,
	DROP COLUMN storage_key,
	DROP COLUMN byte_size,
	DROP COLUMN mime_type,
	DROP COLUMN original_filename;
