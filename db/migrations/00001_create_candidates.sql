-- +goose Up
CREATE TABLE candidates (
	id BIGSERIAL PRIMARY KEY,
	name TEXT NOT NULL,
	role TEXT NOT NULL,
	focus TEXT NOT NULL,
	score INTEGER NOT NULL CHECK (score >= 0 AND score <= 100),
	status TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX candidates_created_at_idx ON candidates (created_at DESC);

-- +goose Down
DROP TABLE candidates;
