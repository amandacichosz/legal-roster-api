-- +goose Up
-- Existing application users cannot be mapped safely to Supabase identities, so
-- this migration intentionally removes them and their custom auth state.
DROP TABLE IF EXISTS user_invitations;
DROP TABLE IF EXISTS sessions;
DROP TABLE users;

CREATE TABLE users (
	id UUID PRIMARY KEY REFERENCES auth.users (id) ON DELETE CASCADE,
	email TEXT NOT NULL,
	name TEXT NOT NULL DEFAULT '',
	role TEXT NOT NULL DEFAULT 'admin',
	status TEXT NOT NULL DEFAULT 'active',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT users_email_unique UNIQUE (email),
	CONSTRAINT users_role_check CHECK (role IN ('platform_admin', 'admin')),
	CONSTRAINT users_status_check CHECK (status IN ('active', 'disabled'))
);

CREATE UNIQUE INDEX users_email_lower_unique_idx ON users (LOWER(email));

-- +goose Down
DROP TABLE users;

CREATE TABLE users (
	id BIGSERIAL PRIMARY KEY,
	email TEXT NOT NULL,
	password_hash TEXT NOT NULL,
	name TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'contractor',
	status TEXT NOT NULL DEFAULT 'active',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT users_status_check CHECK (status IN ('active', 'disabled')),
	CONSTRAINT users_role_check CHECK (role IN ('platform_admin', 'admin', 'law_firm_user', 'contractor', 'reviewer'))
);

CREATE UNIQUE INDEX users_email_unique_idx ON users (LOWER(email));

CREATE TABLE sessions (
	id BIGSERIAL PRIMARY KEY,
	user_id BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	token_hash TEXT NOT NULL UNIQUE,
	expires_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE TABLE user_invitations (
	id BIGSERIAL PRIMARY KEY,
	email TEXT NOT NULL,
	name TEXT NOT NULL,
	role TEXT NOT NULL,
	token_hash TEXT NOT NULL UNIQUE,
	invited_by BIGINT NOT NULL REFERENCES users (id),
	accepted_by BIGINT REFERENCES users (id),
	expires_at TIMESTAMPTZ NOT NULL,
	accepted_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT user_invitations_role_check CHECK (role IN ('platform_admin', 'admin', 'law_firm_user', 'contractor', 'reviewer'))
);

CREATE INDEX user_invitations_email_idx ON user_invitations (LOWER(email));
CREATE INDEX user_invitations_expires_at_idx ON user_invitations (expires_at);
