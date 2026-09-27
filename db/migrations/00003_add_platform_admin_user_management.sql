-- +goose Up
ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('platform_admin', 'admin', 'law_firm_user', 'contractor', 'reviewer'));

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

-- +goose Down
DROP TABLE user_invitations;
ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'law_firm_user', 'contractor', 'reviewer'));
