-- +goose Up
ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('platform_admin', 'admin', 'user'));
ALTER TABLE users ALTER COLUMN role SET DEFAULT 'user';

-- Users created by the initial Supabase sync were disabled admins. Make those
-- default profiles ordinary active users while preserving explicitly enabled admins.
UPDATE users
SET role = 'user', status = 'active', updated_at = NOW()
WHERE role = 'admin' AND status = 'disabled';

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION public.sync_auth_user_profile()
RETURNS TRIGGER
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
	INSERT INTO public.users (id, email, name, role, status)
	VALUES (
		NEW.id,
		LOWER(COALESCE(NEW.email, '')),
		COALESCE(NEW.raw_user_meta_data->>'name', NEW.raw_user_meta_data->>'full_name', ''),
		'user',
		'active'
	)
	ON CONFLICT (id) DO UPDATE
	SET email = EXCLUDED.email,
		name = CASE WHEN public.users.name = '' THEN EXCLUDED.name ELSE public.users.name END,
		updated_at = NOW();
	RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION public.sync_auth_user_profile()
RETURNS TRIGGER
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = public
AS $$
BEGIN
	INSERT INTO public.users (id, email, name, role, status)
	VALUES (
		NEW.id,
		LOWER(COALESCE(NEW.email, '')),
		COALESCE(NEW.raw_user_meta_data->>'name', NEW.raw_user_meta_data->>'full_name', ''),
		'admin',
		'disabled'
	)
	ON CONFLICT (id) DO UPDATE
	SET email = EXCLUDED.email,
		name = CASE WHEN public.users.name = '' THEN EXCLUDED.name ELSE public.users.name END,
		updated_at = NOW();
	RETURN NEW;
END;
$$;
-- +goose StatementEnd

UPDATE users SET role = 'admin', status = 'disabled' WHERE role = 'user';
ALTER TABLE users ALTER COLUMN role SET DEFAULT 'admin';
ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('platform_admin', 'admin'));
