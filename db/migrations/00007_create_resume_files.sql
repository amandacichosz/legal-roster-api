-- +goose Up
CREATE TABLE resume_files (
	id BIGSERIAL PRIMARY KEY,
	public_id TEXT NOT NULL UNIQUE,
	storage_key TEXT NOT NULL UNIQUE,
	original_filename TEXT NOT NULL,
	content_type TEXT NOT NULL,
	size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX resume_files_created_at_idx ON resume_files (created_at DESC);

-- +goose Down
DROP TABLE resume_files;
